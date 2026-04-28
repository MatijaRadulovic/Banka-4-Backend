package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
)

type OtcOfferHandler struct {
	service *service.OtcOfferService
}

func NewOtcOfferHandler(svc *service.OtcOfferService) *OtcOfferHandler {
	return &OtcOfferHandler{service: svc}
}

// CreateOffer — POST /api/otc/offers
// Kupac inicira novi OTC pregovor.
func (h *OtcOfferHandler) CreateOffer(c *gin.Context) {
	var req dto.CreateOtcOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}
	offer, err := h.service.CreateOffer(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.ToOtcOfferResponse(*offer))
}

// SendCounterOffer — PUT /api/otc/offers/:id/counter
// Bilo koja strana pregovora ažurira parametre. Ne kreira novi entitet.
func (h *OtcOfferHandler) SendCounterOffer(c *gin.Context) {
	id, err := parseOfferID(c)
	if err != nil {
		_ = c.Error(err)
		return
	}
	var req dto.CounterOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}
	offer, err := h.service.SendCounterOffer(c.Request.Context(), id, req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.ToOtcOfferResponse(*offer))
}

// AcceptOffer — PATCH /api/otc/offers/:id/accept
// Strana suprotna od ModifiedBy prihvata; nastaje opcioni ugovor i premium se transferiše.
func (h *OtcOfferHandler) AcceptOffer(c *gin.Context) {
	id, err := parseOfferID(c)
	if err != nil {
		_ = c.Error(err)
		return
	}
	// Body je opcioni: ako prodavac prvi put učestvuje, mora poslati account_number.
	var req dto.AcceptOfferRequest
	_ = c.ShouldBindJSON(&req)

	contract, err := h.service.AcceptOffer(c.Request.Context(), id, req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.ToOtcOptionContractResponse(*contract))
}

// RejectOffer — PATCH /api/otc/offers/:id/reject
// Bilo koja strana odustaje od pregovora.
func (h *OtcOfferHandler) RejectOffer(c *gin.Context) {
	id, err := parseOfferID(c)
	if err != nil {
		_ = c.Error(err)
		return
	}
	var req dto.RejectOfferRequest
	_ = c.ShouldBindJSON(&req) // body je opcioni

	offer, err := h.service.RejectOffer(c.Request.Context(), id, req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.ToOtcOfferResponse(*offer))
}

// GetMyActiveOffers — GET /api/otc/offers/active
// Stranica "Aktivne ponude" — svi pregovori u kojima je ulogovani korisnik učesnik.
func (h *OtcOfferHandler) GetMyActiveOffers(c *gin.Context) {
	userID, err := auth.GetSubjectFromContext(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	offers, err := h.service.GetActiveOffersForUser(c.Request.Context(), userID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.ToOtcOfferResponseList(offers))
}

// GetMyOptionContracts — GET /api/otc/contracts
// Stranica "Sklopljeni ugovori".
func (h *OtcOfferHandler) GetMyOptionContracts(c *gin.Context) {
	userID, err := auth.GetSubjectFromContext(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	contracts, err := h.service.GetOptionContractsForUser(c.Request.Context(), userID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.ToOtcOptionContractResponseList(contracts))
}

func parseOfferID(c *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return 0, errors.BadRequestErr("nevažeći id ponude")
	}
	return uint(id), nil
}
