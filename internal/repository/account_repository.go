package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"jaga-warung-go/internal/domain"
)

// accountRepo implements domain.AccountRepository against a Postgres connection.
type accountRepo struct{ db *gorm.DB }

func NewAccountRepo(db *gorm.DB) domain.AccountRepository {
	return &accountRepo{db: db}
}

func (r *accountRepo) FindByPhone(ctx context.Context, phone string) (*domain.Account, error) {
	var acc domain.Account
	err := r.db.Where("phone = ?", phone).First(&acc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &acc, nil
}

func (r *accountRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Account, error) {
	var acc domain.Account
	err := r.db.First(&acc, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &acc, nil
}

func (r *accountRepo) Create(ctx context.Context, acc *domain.Account) error {
	return r.db.Create(acc).Error
}
