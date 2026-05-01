//go:build integration

package integration_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

func TestGetClientPortfolio_Empty(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForClient(t, 50, 1)

	rec := performRequest(t, router, http.MethodGet, "/api/client/1/assets", nil, auth)
	requireStatus(t, rec, http.StatusOK)

	var body []any
	body = decodeResponse[[]any](t, rec)
	require.Empty(t, body)
}

func TestGetClientPortfolio_WithOwnership(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNYS")
	listing := seedListing(t, db, "AAPL", ex.MicCode, model.AssetTypeStock, 150.0)
	seedStock(t, db, listing.ListingID)

	_ = seedOrder(t, db, 1, listing.ListingID, model.OrderDirectionBuy, model.OrderStatusApproved)

	ownership := &model.AssetOwnership{
		UserId:         1,
		OwnerType:      model.OwnerTypeClient,
		AssetID:        listing.AssetID,
		Amount:         10.0,
		AvgBuyPriceRSD: 100.0,
	}
	if err := db.Create(ownership).Error; err != nil {
		t.Fatalf("seed ownership: %v", err)
	}

	auth := authHeaderForClient(t, 50, 1)
	rec := performRequest(t, router, http.MethodGet, "/api/client/1/assets", nil, auth)
	requireStatus(t, rec, http.StatusOK)
}

func TestGetActuaryPortfolio_Empty(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForSupervisor(t)

	rec := performRequest(t, router, http.MethodGet, "/api/actuary/10/assets", nil, auth)
	requireStatus(t, rec, http.StatusOK)
}

func TestGetClientPortfolio_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodGet, "/api/client/1/assets", nil, "")
	require.NotEqual(t, http.StatusOK, rec.Code)
}

func TestGetClientPortfolio_InvalidID(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForClient(t, 50, 1)

	rec := performRequest(t, router, http.MethodGet, "/api/client/abc/assets", nil, auth)
	require.NotEqual(t, http.StatusOK, rec.Code)
}

func TestGetActuaryPortfolio_InvalidID(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForSupervisor(t)

	rec := performRequest(t, router, http.MethodGet, "/api/actuary/abc/assets", nil, auth)
	require.NotEqual(t, http.StatusOK, rec.Code)
}
