package service

import (
	"context"
	"fmt"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
)

// OtcOfferService implementira poslovnu logiku za OTC pregovore i sklapanje
// opcionih ugovora prema specifikaciji Celine 4 (OTC trgovina).
//
// Ključni principi koje servis poštuje:
//
//  1. Aktivna ponuda i opcioni ugovor su DVA RAZLIČITA entiteta. Pregovor
//     se vodi nad OtcOffer-om; tek po prihvatanju nastaje OtcOptionContract.
//
//  2. Kontraponuda AŽURIRA postojeći OtcOffer (Amount, Price, Premium,
//     SettlementDate, ModifiedBy, LastModified). Strane se NE menjaju.
//
//  3. Kontraponudu naizmenično šalju obe strane. Isti korisnik ne može
//     dvaput za redom — druga strana mora odgovoriti.
//
//  4. Prihvata isključivo strana KOJOJ je stigla zadnja izmena (suprotna
//     od ModifiedBy).
//
//  5. Pri prihvatanju: kreira se OtcOptionContract i premium se prebacuje
//     sa kupčevog računa na prodavčev. (TODO za pravu SAGA implementaciju.)
//
//  6. Kapacitet prodavca se validuje prema speci 3+7+2: PublicAmount mora
//     pokriti zbir svih aktivnih pregovora i važećih opcionih ugovora za
//     isti stock.
type OtcOfferService struct {
	offerRepo          repository.OtcOfferRepository
	optionContractRepo repository.OtcOptionContractRepository
	assetOwnershipRepo repository.AssetOwnershipRepository
	stockRepo          repository.StockRepository
	bankingClient      client.BankingClient
	now                func() time.Time
}

func NewOtcOfferService(
	offerRepo repository.OtcOfferRepository,
	optionContractRepo repository.OtcOptionContractRepository,
	assetOwnershipRepo repository.AssetOwnershipRepository,
	stockRepo repository.StockRepository,
	bankingClient client.BankingClient,
) *OtcOfferService {
	return &OtcOfferService{
		offerRepo:          offerRepo,
		optionContractRepo: optionContractRepo,
		assetOwnershipRepo: assetOwnershipRepo,
		stockRepo:          stockRepo,
		bankingClient:      bankingClient,
		now:                time.Now,
	}
}

// CreateOffer — kupac inicira OTC pregovor sa prodavcem za njegove javne akcije.
func (s *OtcOfferService) CreateOffer(ctx context.Context, req dto.CreateOtcOfferRequest) (*model.OtcOffer, error) {
	buyerID, err := auth.GetSubjectFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if buyerID == req.SellerID {
		return nil, errors.BadRequestErr("ne možete poslati ponudu samom sebi")
	}
	if req.SettlementDate.Before(s.now()) {
		return nil, errors.BadRequestErr("settlement date mora biti u budućnosti")
	}

	// Validacija da prodavac može da pokrije i ovu ponudu uz sve postojeće obaveze.
	if err := s.validateSellerCapacity(ctx, req.SellerID, req.StockID, req.Amount, nil); err != nil {
		return nil, err
	}

	// Validacija da kupčev račun postoji.
	if _, err := s.bankingClient.GetAccountByNumber(ctx, req.BuyerAccountNumber); err != nil {
		return nil, errors.BadRequestErr("kupčev račun nije validan")
	}

	now := s.now()
	offer := &model.OtcOffer{
		BuyerID:            buyerID,
		SellerID:           req.SellerID,
		StockID:            req.StockID,
		Amount:             req.Amount,
		PricePerStock:      req.PricePerStock,
		Premium:            req.Premium,
		SettlementDate:     req.SettlementDate,
		BuyerAccountNumber: req.BuyerAccountNumber,
		Status:             model.OtcOfferStatusActive,
		LastModified:       now,
		ModifiedBy:         buyerID, // kupac je inicirao -> sad prodavac na potezu
	}

	if err := s.offerRepo.Create(ctx, offer); err != nil {
		return nil, errors.InternalErr(err)
	}

	// Reload da bismo uključili Stock/Asset preload za response.
	created, err := s.offerRepo.FindByID(ctx, offer.OtcOfferID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return created, nil
}

// SendCounterOffer — bilo koja strana ažurira parametre postojećeg pregovora.
//
// Spec: pregovor je back-and-forth sve dok jedna strana ne odustane ili dok
// druga strana ne prihvati. Strane se ne menjaju, ne pravi se nova ponuda —
// samo se ažuriraju polja i postavlja se novi ModifiedBy.
func (s *OtcOfferService) SendCounterOffer(ctx context.Context, offerID uint, req dto.CounterOfferRequest) (*model.OtcOffer, error) {
	callerID, err := auth.GetSubjectFromContext(ctx)
	if err != nil {
		return nil, err
	}

	offer, err := s.offerRepo.FindByID(ctx, offerID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if offer == nil {
		return nil, errors.NotFoundErr("ponuda nije pronađena")
	}

	if err := s.validateParticipantAndState(offer, callerID); err != nil {
		return nil, err
	}

	// Ne može isti korisnik dvaput za redom — drugi je na potezu.
	if offer.ModifiedBy == callerID {
		return nil, errors.BadRequestErr("druga strana je na potezu — ne možete poslati dve uzastopne kontraponude")
	}

	if req.SettlementDate.Before(s.now()) {
		return nil, errors.BadRequestErr("settlement date mora biti u budućnosti")
	}

	// Ako prodavac menja Amount, validuj kapacitet (izuzimajući trenutnu ponudu iz sume).
	if callerID == offer.SellerID {
		if err := s.validateSellerCapacity(ctx, offer.SellerID, offer.StockID, req.Amount, &offer.OtcOfferID); err != nil {
			return nil, err
		}
	}

	// Ako prodavac prvi put učestvuje, postavi njegov račun za kasniji premium transfer.
	if callerID == offer.SellerID && offer.SellerAccountNumber == nil {
		if req.AccountNumber == nil {
			return nil, errors.BadRequestErr("seller_account_number je obavezan u prvoj prodavčevoj kontraponudi")
		}
		if _, err := s.bankingClient.GetAccountByNumber(ctx, *req.AccountNumber); err != nil {
			return nil, errors.BadRequestErr("prodavčev račun nije validan")
		}
		offer.SellerAccountNumber = req.AccountNumber
	}

	offer.Amount = req.Amount
	offer.PricePerStock = req.PricePerStock
	offer.Premium = req.Premium
	offer.SettlementDate = req.SettlementDate
	offer.LastModified = s.now()
	offer.ModifiedBy = callerID

	if err := s.offerRepo.Save(ctx, offer); err != nil {
		return nil, errors.InternalErr(err)
	}

	updated, err := s.offerRepo.FindByID(ctx, offer.OtcOfferID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return updated, nil
}

// AcceptOffer — strana SUPROTNA od ModifiedBy prihvata ponudu.
//
// Pri prihvatanju:
//
//  1. finalna validacija kapaciteta prodavca,
//  2. prebacivanje premije sa kupčevog računa na prodavčev,
//  3. kreiranje OtcOptionContract,
//  4. prelazak ponude u status ACCEPTED sa linkom na ugovor.
func (s *OtcOfferService) AcceptOffer(ctx context.Context, offerID uint, req dto.AcceptOfferRequest) (*model.OtcOptionContract, error) {
	callerID, err := auth.GetSubjectFromContext(ctx)
	if err != nil {
		return nil, err
	}

	offer, err := s.offerRepo.FindByID(ctx, offerID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if offer == nil {
		return nil, errors.NotFoundErr("ponuda nije pronađena")
	}

	if err := s.validateParticipantAndState(offer, callerID); err != nil {
		return nil, err
	}

	// Prihvata samo strana kojoj je stigla zadnja ponuda (suprotna od ModifiedBy).
	if offer.ModifiedBy == callerID {
		return nil, errors.BadRequestErr("ne možete prihvatiti sopstvenu ponudu — druga strana treba da prihvati ili pošalje kontraponudu")
	}

	// Ako prodavac prvi put učestvuje (kupac kreirao -> prodavac direktno prihvata),
	// prodavčev račun mora biti prosleđen ovde.
	if callerID == offer.SellerID && offer.SellerAccountNumber == nil {
		if req.AccountNumber == nil {
			return nil, errors.BadRequestErr("seller_account_number je obavezan pri prihvatanju")
		}
		if _, err := s.bankingClient.GetAccountByNumber(ctx, *req.AccountNumber); err != nil {
			return nil, errors.BadRequestErr("prodavčev račun nije validan")
		}
		offer.SellerAccountNumber = req.AccountNumber
	}
	if offer.SellerAccountNumber == nil {
		return nil, errors.BadRequestErr("nedostaje prodavčev račun — prodavac mora najpre poslati kontraponudu ili prihvatiti")
	}

	// Re-validuj kapacitet — situacija se mogla promeniti tokom pregovora.
	if err := s.validateSellerCapacity(ctx, offer.SellerID, offer.StockID, offer.Amount, &offer.OtcOfferID); err != nil {
		return nil, err
	}

	// 1) Prebaci premium kupac -> prodavac.
	// TODO(SAGA): Ovo treba zameniti pravom SAGA orkestracijom kada se uvede.
	// Trenutno koristimo direct-payment (CreatePaymentWithoutVerification) jer
	// premium nije settlement, već neposredan prenos po dogovoru.
	if _, err := s.bankingClient.CreatePaymentWithoutVerification(ctx, &pb.CreatePaymentRequest{
		PayerAccountNumber:     offer.BuyerAccountNumber,
		RecipientAccountNumber: *offer.SellerAccountNumber,
		Amount:                 offer.Premium,
		PaymentCode:            "289", // ostala plaćanja (po potrebi prilagoditi)
		Purpose:                fmt.Sprintf("OTC premium za ponudu #%d", offer.OtcOfferID),
	}); err != nil {
		return nil, errors.InternalErr(fmt.Errorf("premium transfer nije uspeo: %w", err))
	}

	// 2) Kreiraj sklopljen opcioni ugovor.
	now := s.now()
	contract := &model.OtcOptionContract{
		OtcOfferID:     offer.OtcOfferID,
		BuyerID:        offer.BuyerID,
		SellerID:       offer.SellerID,
		StockID:        offer.StockID,
		Amount:         offer.Amount,
		StrikePrice:    offer.PricePerStock,
		Premium:        offer.Premium,
		SettlementDate: offer.SettlementDate,
	}
	if err := s.optionContractRepo.Create(ctx, contract); err != nil {
		// Idealno ovde rollback premium transfera; bez SAGA-e to je manuelno.
		return nil, errors.InternalErr(fmt.Errorf("kreiranje opcionog ugovora nije uspelo (premium već prebačen): %w", err))
	}

	// 3) Označi ponudu kao prihvaćenu i poveži je sa ugovorom.
	offer.Status = model.OtcOfferStatusAccepted
	offer.OptionContractID = &contract.OtcOptionContractID
	offer.LastModified = now
	offer.ModifiedBy = callerID
	if err := s.offerRepo.Save(ctx, offer); err != nil {
		return nil, errors.InternalErr(err)
	}

	created, err := s.optionContractRepo.FindByID(ctx, contract.OtcOptionContractID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return created, nil
}

// RejectOffer — bilo koja strana može odustati od pregovora u svakom trenutku.
func (s *OtcOfferService) RejectOffer(ctx context.Context, offerID uint, req dto.RejectOfferRequest) (*model.OtcOffer, error) {
	callerID, err := auth.GetSubjectFromContext(ctx)
	if err != nil {
		return nil, err
	}

	offer, err := s.offerRepo.FindByID(ctx, offerID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if offer == nil {
		return nil, errors.NotFoundErr("ponuda nije pronađena")
	}

	if err := s.validateParticipantAndState(offer, callerID); err != nil {
		return nil, err
	}

	offer.Status = model.OtcOfferStatusRejected
	offer.LastModified = s.now()
	offer.ModifiedBy = callerID
	_ = req // komentar nije perzistiran u trenutnom modelu — može se dodati ako bude potrebno

	if err := s.offerRepo.Save(ctx, offer); err != nil {
		return nil, errors.InternalErr(err)
	}

	updated, err := s.offerRepo.FindByID(ctx, offer.OtcOfferID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return updated, nil
}

// GetActiveOffersForUser — stranica "Aktivne ponude": svi pregovori u kojima
// ulogovani korisnik trenutno učestvuje.
func (s *OtcOfferService) GetActiveOffersForUser(ctx context.Context, userID uint) ([]model.OtcOffer, error) {
	offers, err := s.offerRepo.FindActiveForUser(ctx, userID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return offers, nil
}

// GetOptionContractsForUser — stranica "Sklopljeni ugovori".
func (s *OtcOfferService) GetOptionContractsForUser(ctx context.Context, userID uint) ([]model.OtcOptionContract, error) {
	contracts, err := s.optionContractRepo.FindForUser(ctx, userID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return contracts, nil
}

// --- helpers ---

func (s *OtcOfferService) validateParticipantAndState(offer *model.OtcOffer, callerID uint) error {
	if callerID != offer.BuyerID && callerID != offer.SellerID {
		return errors.ForbiddenErr("niste učesnik ovog pregovora")
	}
	if offer.Status != model.OtcOfferStatusActive {
		return errors.BadRequestErr("ponuda nije aktivna")
	}
	return nil
}

// validateSellerCapacity proverava spec scenario 3+7+2:
// PublicAmount prodavca >= sum(amount aktivnih ponuda za isti stock)
//
//   - sum(amount važećih opcionih ugovora za isti stock)
//   - requestedAmount
//
// Ako je excludeOfferID != nil, ta ponuda se izuzima iz sume (npr. kad updateujemo
// postojeći entitet — njena trenutna količina se ne računa duplo).
func (s *OtcOfferService) validateSellerCapacity(
	ctx context.Context,
	sellerID, stockID uint,
	requestedAmount int,
	excludeOfferID *uint,
) error {
	// Stock -> Asset (AssetOwnership se vezuje za AssetID).
	stocks, err := s.stockRepo.FindByAssetIDs(ctx, []uint{stockID})
	if err != nil {
		return errors.InternalErr(err)
	}
	// Stock model koristi AssetID kao 1-1 relaciju, ali StockID je primarni ključ.
	// Pošto ne znamo direktno mapiranje stockID->assetID bez upita, tražimo po
	// AssetID prvo (česti slučaj kad UI šalje AssetID kao stockID).
	var stock *model.Stock
	for i := range stocks {
		if stocks[i].StockID == stockID || stocks[i].AssetID == stockID {
			stock = &stocks[i]
			break
		}
	}
	if stock == nil {
		return errors.BadRequestErr("stock nije pronađen")
	}

	ownerships, err := s.assetOwnershipRepo.FindByIdentity(ctx, sellerID, model.OwnerTypeClient)
	if err != nil {
		return errors.InternalErr(err)
	}

	publicAmount := 0.0
	for _, o := range ownerships {
		if o.AssetID == stock.AssetID {
			publicAmount = o.PublicAmount
			break
		}
	}

	// Već rezervisano u drugim aktivnim pregovorima.
	activeOffers, err := s.offerRepo.FindActiveBySellerAndStock(ctx, sellerID, stockID, excludeOfferID)
	if err != nil {
		return errors.InternalErr(err)
	}
	reserved := 0
	for _, o := range activeOffers {
		reserved += o.Amount
	}

	// Već rezervisano u važećim opcionim ugovorima (neiskorišćeni, settlement u budućnosti).
	activeContracts, err := s.optionContractRepo.FindActiveBySellerAndStock(ctx, sellerID, stockID, s.now())
	if err != nil {
		return errors.InternalErr(err)
	}
	for _, c := range activeContracts {
		reserved += c.Amount
	}

	totalNeeded := float64(reserved + requestedAmount)
	if publicAmount < totalNeeded {
		return errors.BadRequestErr(fmt.Sprintf(
			"prodavac nema dovoljno javnih akcija: javno=%.0f, već zauzeto u pregovorima/ugovorima=%d, traženo dodatno=%d",
			publicAmount, reserved, requestedAmount,
		))
	}
	return nil
}
