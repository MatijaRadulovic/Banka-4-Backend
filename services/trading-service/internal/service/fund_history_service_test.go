package service

import (
	"context"
	"errors"
	"testing"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/stretchr/testify/require"
)

type fakeFundHistoryRepo struct {
	saveAllRecords []*model.FundHistoryRecord
	saveAllErr     error
}

func (f *fakeFundHistoryRepo) Save(ctx context.Context, record *model.FundHistoryRecord) error {
	return nil
}

func (f *fakeFundHistoryRepo) SaveAll(ctx context.Context, records []*model.FundHistoryRecord) error {
	if f.saveAllErr != nil {
		return f.saveAllErr
	}
	f.saveAllRecords = records
	return nil
}

func TestCalculateAndSaveDailyHistory_Success(t *testing.T) {
	ctx := context.Background()

	fund1 := model.InvestmentFund{
		FundID:        1,
		AccountNumber: "FUND-123",
		Positions: []model.ClientFundPosition{
			{TotalInvestedAmount: 500},
			{TotalInvestedAmount: 1500},
		},
	}
	fund2 := model.InvestmentFund{
		FundID:        2,
		AccountNumber: "FUND-456",
		Positions: []model.ClientFundPosition{
			{TotalInvestedAmount: 1000},
		},
	}

	fundRepo := &fakeFundRepo{
		findAllResult: []model.InvestmentFund{fund1, fund2},
	}

	bankingClient := &fakeFundBankingClient{
		getAccountResult: &pb.GetAccountByNumberResponse{
			AvailableBalance: 1000.0,
		},
	}

	listingRepo := &fakeListingRepo{}
	ownershipRepo := &fakeAssetOwnershipRepo{}

	// Let sumSecuritiesValue be 0 via fake ownership missing matching user items
	svc := newTestFundService(fundRepo, ownershipRepo, listingRepo, bankingClient, &fakeFundUserClient{})
	
	historyRepo := &fakeFundHistoryRepo{}
	err := svc.CalculateAndSaveDailyHistory(ctx, historyRepo)
	require.NoError(t, err)

	require.Len(t, historyRepo.saveAllRecords, 2)

	// Fund 1 check
	rec1 := historyRepo.saveAllRecords[0]
	require.Equal(t, uint(1), rec1.FundID)
	// LiquidAssets: 1000 + SecVal: 0 = TotalValue: 1000
	require.Equal(t, 1000.0, rec1.TotalValue)
	require.Equal(t, 1000.0, rec1.LiquidAssets)
	// Profit = TotalValue (1000) - TotalInvested (2000) = -1000
	require.Equal(t, -1000.0, rec1.Profit)

	// Fund 2 check
	rec2 := historyRepo.saveAllRecords[1]
	require.Equal(t, uint(2), rec2.FundID)
	require.Equal(t, 1000.0, rec2.TotalValue)
	require.Equal(t, 1000.0, rec2.LiquidAssets)
	// Profit = TotalValue (1000) - TotalInvested (1000) = 0
	require.Equal(t, 0.0, rec2.Profit)
}

func TestCalculateAndSaveDailyHistory_ErrorHandlingSkip(t *testing.T) {
	ctx := context.Background()

	fund1 := model.InvestmentFund{
		FundID:        1,
		AccountNumber: "FUND-123", // Will succeed
	}
	fund2 := model.InvestmentFund{
		FundID:        2,
		AccountNumber: "FUND-ERROR", // Will fail getLiquidAssets
	}

	fundRepo := &fakeFundRepo{
		findAllResult: []model.InvestmentFund{fund1, fund2},
	}

	bankingClient := &fakeFundBankingClient{
		// Custom handler for getAccount
		getAccountResult: &pb.GetAccountByNumberResponse{
			AvailableBalance: 1000.0,
		},
	}
	// Unfortunately, fakeFundBankingClient does not easily allow erroring out selectively by account number.
	// I'll create a targeted mock here instead just for this test.
	
	customBankingClient := &testCustomBankingClient{
		getAccountByNumberFunc: func(ctx context.Context, accNum string) (*pb.GetAccountByNumberResponse, error) {
			if accNum == "FUND-ERROR" {
				return nil, errors.New("banking api error")
			}
			return &pb.GetAccountByNumberResponse{AvailableBalance: 2000.0}, nil
		},
	}

	svc := newTestFundService(fundRepo, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, customBankingClient, &fakeFundUserClient{})
	
	historyRepo := &fakeFundHistoryRepo{}
	err := svc.CalculateAndSaveDailyHistory(ctx, historyRepo)
	require.NoError(t, err)

	// Expected only 1 record (Fund 1) because Fund 2 should be skipped using `continue`
	require.Len(t, historyRepo.saveAllRecords, 1)
	require.Equal(t, uint(1), historyRepo.saveAllRecords[0].FundID)
	require.Equal(t, 2000.0, historyRepo.saveAllRecords[0].TotalValue)
}

func TestCalculateAndSaveDailyHistory_RepositoryError(t *testing.T) {
	ctx := context.Background()
	
	fundRepo := &fakeFundRepo{
		findAllErr: errors.New("database connection failed"),
	}

	svc := newTestFundService(fundRepo, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeFundUserClient{})
	historyRepo := &fakeFundHistoryRepo{}

	err := svc.CalculateAndSaveDailyHistory(ctx, historyRepo)
	require.Error(t, err)
	require.Equal(t, "database connection failed", err.Error())
}

// ── Custom Banking Client for advanced tests ─────────────────────────────
type testCustomBankingClient struct {
	getAccountByNumberFunc func(ctx context.Context, accountNumber string) (*pb.GetAccountByNumberResponse, error)
}

func (c *testCustomBankingClient) GetAccountByNumber(ctx context.Context, accountNumber string) (*pb.GetAccountByNumberResponse, error) {
	if c.getAccountByNumberFunc != nil {
		return c.getAccountByNumberFunc(ctx, accountNumber)
	}
	return nil, nil
}
func (c *testCustomBankingClient) HasActiveLoan(_ context.Context, _ uint64) (*pb.HasActiveLoanResponse, error) { return nil, nil }
func (c *testCustomBankingClient) CreatePaymentWithoutVerification(_ context.Context, _ *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) { return nil, nil }
func (c *testCustomBankingClient) GetAccountsByClientID(_ context.Context, _ uint64) (*pb.GetAccountsByClientIDResponse, error) { return nil, nil }
func (c *testCustomBankingClient) ConvertCurrency(_ context.Context, amount float64, _, _ string) (float64, error) { return amount, nil }
func (c *testCustomBankingClient) ExecuteTradeSettlement(_ context.Context, _, _ string, _ pb.TradeSettlementDirection, _ float64) (*pb.ExecuteTradeSettlementResponse, error) { return nil, nil }
func (c *testCustomBankingClient) GetAccountCurrency(_ context.Context, _ string) (string, error) { return "RSD", nil }
func (c *testCustomBankingClient) CreateFundAccount(_ context.Context, _ string, _ uint64) (string, error) { return "", nil }
