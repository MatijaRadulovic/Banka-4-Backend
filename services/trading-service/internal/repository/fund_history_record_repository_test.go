package repository

import (
	"context"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	// Use in-memory SQLite for repository testing
	database, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	err = database.AutoMigrate(
		&model.InvestmentFund{},
		&model.FundHistoryRecord{},
	)
	require.NoError(t, err)

	return database
}

func TestFundHistoryRecordRepository_Save(t *testing.T) {
	database := setupTestDB(t)
	repo := NewFundHistoryRecordRepository(database)
	ctx := context.Background()

	fund := &model.InvestmentFund{
		Name:          "Test Fund 1",
		Description:   "Desc",
		AccountNumber: "123",
	}
	err := database.Create(fund).Error
	require.NoError(t, err)

	record := &model.FundHistoryRecord{
		FundID:       fund.FundID,
		RecordDate:   time.Now(),
		TotalValue:   1500.0,
		Profit:       200.0,
		LiquidAssets: 500.0,
	}

	err = repo.Save(ctx, record)
	require.NoError(t, err)
	require.NotZero(t, record.FundHistoryRecordID)

	var saved model.FundHistoryRecord
	err = database.First(&saved, record.FundHistoryRecordID).Error
	require.NoError(t, err)
	require.Equal(t, fund.FundID, saved.FundID)
	require.Equal(t, 1500.0, saved.TotalValue)
}

func TestFundHistoryRecordRepository_SaveAll(t *testing.T) {
	database := setupTestDB(t)
	repo := NewFundHistoryRecordRepository(database)
	ctx := context.Background()

	fund1 := &model.InvestmentFund{
		Name:          "Test Fund A",
		Description:   "Desc",
		AccountNumber: "AAA",
	}
	fund2 := &model.InvestmentFund{
		Name:          "Test Fund B",
		Description:   "Desc",
		AccountNumber: "BBB",
	}
	err := database.Create(fund1).Error
	require.NoError(t, err)
	err = database.Create(fund2).Error
	require.NoError(t, err)

	records := []*model.FundHistoryRecord{
		{
			FundID:       fund1.FundID,
			RecordDate:   time.Now(),
			TotalValue:   1000.0,
			Profit:       100.0,
			LiquidAssets: 100.0,
		},
		{
			FundID:       fund2.FundID,
			RecordDate:   time.Now(),
			TotalValue:   2000.0,
			Profit:       200.0,
			LiquidAssets: 200.0,
		},
	}

	err = repo.SaveAll(ctx, records)
	require.NoError(t, err)

	require.NotZero(t, records[0].FundHistoryRecordID)
	require.NotZero(t, records[1].FundHistoryRecordID)

	var count int64
	err = database.Model(&model.FundHistoryRecord{}).Count(&count).Error
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
}
