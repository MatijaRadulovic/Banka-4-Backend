package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
	"github.com/stretchr/testify/require"
)

var errTest = errors.New("repo error")

// --- Fake repos ---

type fakeAssetOwnershipRepo struct {
	ownerships []model.AssetOwnership
	upsertErr  error
	findErr    error
}

func (r *fakeAssetOwnershipRepo) FindByIdentity(_ context.Context, _ uint, _ model.OwnerType) ([]model.AssetOwnership, error) {
	return r.ownerships, r.findErr
}

func (r *fakeAssetOwnershipRepo) Upsert(_ context.Context, _ *model.AssetOwnership) error {
	return r.upsertErr
}

func (r *fakeAssetOwnershipRepo) IncreaseReservedAmount(_ context.Context, _ uint, _ model.OwnerType, _ uint, _ float64) error {
	return nil
}

type fakeStockRepo struct {
	stocks []model.Stock
	err    error
}

func (r *fakeStockRepo) Upsert(_ context.Context, _ *model.Stock) error   { return nil }
func (r *fakeStockRepo) FindAll(_ context.Context) ([]model.Stock, error) { return nil, nil }
func (r *fakeStockRepo) FindByAssetIDs(_ context.Context, _ []uint) ([]model.Stock, error) {
	return r.stocks, r.err
}
func (r *fakeStockRepo) Count(_ context.Context) (int64, error) { return 0, nil }

type fakeOptionRepo struct {
	options []model.Option
	err     error
}

func (r *fakeOptionRepo) Upsert(_ context.Context, _ *model.Option) error { return nil }
func (r *fakeOptionRepo) FindByAssetIDs(_ context.Context, _ []uint) ([]model.Option, error) {
	return r.options, r.err
}
func (r *fakeOptionRepo) FindByStockID(_ context.Context, _ uint) ([]model.Option, error) {
	return nil, nil
}

type fakeFuturesRepo struct {
	futures []model.FuturesContract
	err     error
}

func (r *fakeFuturesRepo) FindByAssetIDs(_ context.Context, _ []uint) ([]model.FuturesContract, error) {
	return r.futures, r.err
}

type fakeForexRepo struct {
	forex []model.ForexPair
	err   error
}

func (r *fakeForexRepo) FindByAssetIDs(_ context.Context, _ []uint) ([]model.ForexPair, error) {
	return r.forex, r.err
}
func (r *fakeForexRepo) Count(_ context.Context) (int64, error)            { return int64(len(r.forex)), nil }
func (r *fakeForexRepo) Upsert(_ context.Context, _ model.ForexPair) error { return nil }
func (r *fakeForexRepo) FindAll(_ context.Context, _ repository.ListingFilter) ([]model.ForexPair, int64, error) {
	return r.forex, int64(len(r.forex)), r.err
}

// --- Helpers ---

func makeOwnership(assetID uint, ticker string, amount, avgBuyPrice float64) model.AssetOwnership {
	return model.AssetOwnership{
		UserId:         1,
		OwnerType:      model.OwnerTypeClient,
		AssetID:        assetID,
		Asset:          model.Asset{AssetID: assetID, Ticker: ticker, AssetType: model.AssetTypeStock},
		Amount:         amount,
		AvgBuyPriceRSD: avgBuyPrice,
		UpdatedAt:      time.Now(),
	}
}

func makeListing(assetID uint, price float64) *model.Listing {
	return &model.Listing{
		ListingID: assetID,
		AssetID:   assetID,
		Price:     price,
		Exchange: &model.Exchange{
			Currency: "USD",
		},
	}
}

// --- Tests ---

func TestGetPortfolio_HappyPath_Stock(t *testing.T) {
	ownership := makeOwnership(10, "AAPL", 10, 100.0)

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}},
		&fakeStockRepo{stocks: []model.Stock{{StockID: 1, AssetID: 10, OutstandingShares: 1_000_000, Listing: makeListing(10, 150.0)}}},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.NoError(t, err)
	require.Len(t, result, 1)

	a := result[0]
	require.Equal(t, dto.AssetTypeStock, a.Type)
	require.Equal(t, "AAPL", a.Ticker)
	require.Equal(t, float64(10), a.Amount)
	require.Equal(t, 150.0, a.PricePerUnitRSD)
	require.InDelta(t, (150.0-100.0)*10, a.Profit, 0.001)
	require.NotNil(t, a.OutstandingShares)
	require.Equal(t, float64(1_000_000), *a.OutstandingShares)
}

func TestGetPortfolio_HappyPath_Option(t *testing.T) {
	ownership := makeOwnership(20, "MSFT220404C00180000", 200, 5.0)
	ownership.Asset.AssetType = model.AssetTypeOption

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}},
		&fakeStockRepo{},
		&fakeOptionRepo{options: []model.Option{{OptionID: 1, AssetID: 20, Listing: makeListing(20, 8.0)}}},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.NoError(t, err)
	require.Len(t, result, 1)

	a := result[0]
	require.Equal(t, dto.AssetTypeOption, a.Type)
	require.Equal(t, float64(200), a.Amount)
	require.InDelta(t, (8.0-5.0)*200, a.Profit, 0.001)
	require.Nil(t, a.OutstandingShares)
}

func TestGetPortfolio_HappyPath_Futures(t *testing.T) {
	ownership := makeOwnership(30, "CLJ22", 5, 200.0)
	ownership.Asset.AssetType = model.AssetTypeFuture

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}},
		&fakeStockRepo{},
		&fakeOptionRepo{},
		&fakeFuturesRepo{futures: []model.FuturesContract{{FuturesContractID: 1, AssetID: 30, Listing: makeListing(30, 210.0)}}},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.NoError(t, err)
	require.Len(t, result, 1)

	a := result[0]
	require.Equal(t, dto.AssetTypeFutures, a.Type)
	require.Equal(t, float64(5), a.Amount)
	require.InDelta(t, (210.0-200.0)*5, a.Profit, 0.001)
}

func TestGetPortfolio_ZeroAmountFiltered(t *testing.T) {
	// Ownership with 0 amount should be excluded
	ownership := makeOwnership(10, "AAPL", 0, 100.0)

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}},
		&fakeStockRepo{stocks: []model.Stock{{StockID: 1, AssetID: 10}}},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestGetPortfolio_NetAmountAfterSell(t *testing.T) {
	// After buying 10 and selling 10, net amount is 0 — position is filtered out
	ownership := makeOwnership(10, "AAPL", 0, 100.0)

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}},
		&fakeStockRepo{stocks: []model.Stock{{StockID: 1, AssetID: 10, Listing: makeListing(10, 150.0)}}},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestGetPortfolio_PartialSell(t *testing.T) {
	// After buying 10 and selling 4, net amount is 6
	ownership := makeOwnership(10, "AAPL", 6, 100.0)

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}},
		&fakeStockRepo{stocks: []model.Stock{{StockID: 1, AssetID: 10, Listing: makeListing(10, 150.0)}}},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, float64(6), result[0].Amount)
}

func TestGetPortfolio_EmptyOwnerships(t *testing.T) {
	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{}},
		&fakeStockRepo{},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestGetPortfolio_RepoError(t *testing.T) {
	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{findErr: errTest},
		&fakeStockRepo{},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
	)

	_, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.Error(t, err)
}

func TestGetPortfolio_NegativeProfit_NoTax(t *testing.T) {
	// Buy at 200, current price 150 -> negative profit, no tax
	ownership := makeOwnership(10, "AAPL", 20, 200.0)

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}},
		&fakeStockRepo{stocks: []model.Stock{{StockID: 1, AssetID: 10, Listing: makeListing(10, 150.0)}}},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.InDelta(t, (150.0-200.0)*20, result[0].Profit, 0.001)
	require.Equal(t, float64(20), result[0].Amount)
}

func TestGetPortfolio_MultipleAssets_ProfitAccumulation(t *testing.T) {
	// Two stocks with different profit/loss — verifies values the handler will sum.
	// AAPL: buy 10 @ 100, now 150 -> profit = +500
	// MSFT: buy 5  @ 300, now 250 -> profit = -250
	// Expected total if summed: +250
	ownershipA := makeOwnership(10, "AAPL", 10, 100.0)
	ownershipB := makeOwnership(20, "MSFT", 5, 300.0)
	ownershipB.Asset.AssetID = 20
	ownershipB.Asset.Ticker = "MSFT"

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownershipA, ownershipB}},
		&fakeStockRepo{stocks: []model.Stock{
			{StockID: 1, AssetID: 10, Listing: makeListing(10, 150.0)},
			{StockID: 2, AssetID: 20, Listing: makeListing(20, 250.0)},
		}},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.NoError(t, err)
	require.Len(t, result, 2)

	var total float64
	for _, a := range result {
		total += a.Profit
	}
	require.InDelta(t, 250.0, total, 0.001)
}

func TestGetPortfolio_EmptyPortfolio_ZeroProfit(t *testing.T) {
	// Empty ownership list -> GetPortfolio returns empty -> handler sums to 0
	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{}},
		&fakeStockRepo{},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeActuary)
	require.NoError(t, err)
	require.Empty(t, result)

	var total float64
	for _, a := range result {
		total += a.Profit
	}
	require.InDelta(t, 0.0, total, 0.001)
}
