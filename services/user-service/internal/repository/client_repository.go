package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/model"
)

type ClientRepository interface {
	Create(ctx context.Context, client *model.Client) error
	FindByIdentityID(ctx context.Context, identityID uint) (*model.Client, error)
	FindByID(ctx context.Context, id uint) (*model.Client, error)
	FindByIDs(ctx context.Context, ids []uint) ([]model.Client, error)
	FindAll(ctx context.Context, query *dto.ListClientsQuery) ([]model.Client, int64, error)
	Update(ctx context.Context, client *model.Client) error
}
