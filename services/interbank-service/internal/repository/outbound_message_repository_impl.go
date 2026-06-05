package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

type outboundMessageRepository struct {
	db *gorm.DB
}

func NewOutboundMessageRepository(database *gorm.DB) OutboundMessageRepository {
	return &outboundMessageRepository{db: database}
}

func (r *outboundMessageRepository) Enqueue(ctx context.Context, m *model.OutboundMessage) error {
	if m.NextRetryAt.IsZero() {
		m.NextRetryAt = time.Now()
	}
	if m.Status == "" {
		m.Status = model.OutboundPending
	}

	return db.DBFromContext(ctx, r.db).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "idempotence_key_local"}},
			DoNothing: true,
		}).
		Create(m).Error
}

func (r *outboundMessageRepository) NextBatch(ctx context.Context, limit int) ([]model.OutboundMessage, error) {
	var rows []model.OutboundMessage

	err := db.DBFromContext(ctx, r.db).
		Where("status = ? AND next_retry_at <= ?", model.OutboundPending, time.Now()).
		Order("next_retry_at ASC").
		Limit(limit).
		Find(&rows).Error

	return rows, err
}

func (r *outboundMessageRepository) MarkSent(ctx context.Context, id uint, status int, body []byte) error {
	return db.DBFromContext(ctx, r.db).
		Model(&model.OutboundMessage{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":               model.OutboundSent,
			"last_response_status": status,
			"last_response_body":   body,
			"last_error":           "",
		}).Error
}

func (r *outboundMessageRepository) MarkFailed(ctx context.Context, id uint, lastErr string) error {
	return db.DBFromContext(ctx, r.db).
		Model(&model.OutboundMessage{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     model.OutboundFailed,
			"last_error": lastErr,
		}).Error
}

func (r *outboundMessageRepository) Reschedule(ctx context.Context, id uint, attempts int, lastErr string, lastStatus int, lastBody []byte, nextRetryAt time.Time) error {
	return db.DBFromContext(ctx, r.db).
		Model(&model.OutboundMessage{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"attempts":             attempts,
			"last_error":           lastErr,
			"last_response_status": lastStatus,
			"last_response_body":   lastBody,
			"next_retry_at":        nextRetryAt,
		}).Error
}

func (r *outboundMessageRepository) Cancel(ctx context.Context, id uint) error {
	return db.DBFromContext(ctx, r.db).
		Model(&model.OutboundMessage{}).
		Where("id = ? AND status = ?", id, model.OutboundPending).
		Update("status", model.OutboundCanceled).Error
}
