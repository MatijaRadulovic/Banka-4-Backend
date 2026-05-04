package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"gorm.io/gorm"
)

type fundHistoryRecordRepositoryImpl struct {
	db *gorm.DB
}

func NewFundHistoryRecordRepository(db *gorm.DB) FundHistoryRecordRepository {
	return &fundHistoryRecordRepositoryImpl{db: db}
}

func (r *fundHistoryRecordRepositoryImpl) Save(ctx context.Context, record *model.FundHistoryRecord) error {
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *fundHistoryRecordRepositoryImpl) SaveAll(ctx context.Context, records []*model.FundHistoryRecord) error {
	for _, record := range records {
		if err := r.Save(ctx, record); err != nil {
			return err
		}
	}
	return nil
}
