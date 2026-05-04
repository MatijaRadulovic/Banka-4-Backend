package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type FundHistoryRecordRepository interface {
	Save(ctx context.Context, record *model.FundHistoryRecord) error
	SaveAll(ctx context.Context, records []*model.FundHistoryRecord) error
}
