package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/model"
)

type clientRepository struct {
	db *gorm.DB
}

func NewClientRepository(db *gorm.DB) ClientRepository {
	return &clientRepository{db: db}
}

func (r *clientRepository) Create(ctx context.Context, client *model.Client) error {
	db := db.DBFromContext(ctx, r.db)
	return db.WithContext(ctx).Create(client).Error
}

func (r *clientRepository) FindByIdentityID(ctx context.Context, identityID uint) (*model.Client, error) {
	var c model.Client

	result := r.db.WithContext(ctx).
		Preload("Permissions").
		Preload("Identity").
		Where("identity_id = ?", identityID).
		First(&c)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if result.Error != nil {
		return nil, result.Error
	}

	return &c, nil
}
func (r *clientRepository) FindByID(ctx context.Context, id uint) (*model.Client, error) {
	var c model.Client
	result := r.db.WithContext(ctx).
		Preload("Permissions").
		Preload("Identity").
		First(&c, id)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &c, nil
}

func (r *clientRepository) FindByIDs(ctx context.Context, ids []uint) ([]model.Client, error) {
	var clients []model.Client

	if len(ids) == 0 {
		return clients, nil
	}

	result := r.db.WithContext(ctx).
		Preload("Permissions").
		Preload("Identity").
		Where("client_id IN ?", ids).
		Find(&clients)

	if result.Error != nil {
		return nil, result.Error
	}

	return clients, nil
}

func (r *clientRepository) FindAll(ctx context.Context, query *dto.ListClientsQuery) ([]model.Client, int64, error) {
	var clients []model.Client
	var count int64

	db := r.db.WithContext(ctx).Model(&model.Client{})

	// Filteri
	if query.Email != "" {
		db = db.Joins("JOIN identities ON identities.id = clients.identity_id").
			Where("identities.email ILIKE ?", "%"+query.Email+"%")
	}
	if query.FirstName != "" {
		db = db.Where("first_name ILIKE ?", "%"+query.FirstName+"%")
	}
	if query.LastName != "" {
		db = db.Where("last_name ILIKE ?", "%"+query.LastName+"%")
	}

	// Ukupan broj
	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}
	// Paginacija
	offset := (query.Page - 1) * query.PageSize
	err := db.Preload("Identity").Preload("Permissions").Limit(query.PageSize).Offset(offset).Find(&clients).Error

	return clients, count, err
}

func (r *clientRepository) Update(ctx context.Context, client *model.Client) error {
	currentDB := db.DBFromContext(ctx, r.db)
	return currentDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Permissions", "Identity").Save(client).Error; err != nil {
			return err
		}

		if err := tx.Where("client_id = ?", client.ClientID).Delete(&model.ClientPermission{}).Error; err != nil {
			return err
		}

		if len(client.Permissions) > 0 {
			if err := tx.Create(&client.Permissions).Error; err != nil {
				return err
			}
		}

		return nil
	})
}
