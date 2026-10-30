package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"jaga-warung-go/internal/domain"
)

// refreshTokenRepo implements domain.RefreshTokenRepository.
type refreshTokenRepo struct{ db *gorm.DB }

func NewRefreshTokenRepo(db *gorm.DB) domain.RefreshTokenRepository {
	return &refreshTokenRepo{db: db}
}

func (r *refreshTokenRepo) Create(ctx context.Context, rt *domain.RefreshToken) error {
	return r.db.Create(rt).Error
}

func (r *refreshTokenRepo) FindByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	var rt domain.RefreshToken
	err := r.db.Where("token_hash = ? AND revoked_at IS NULL", tokenHash).First(&rt).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &rt, nil
}

func (r *refreshTokenRepo) Revoke(ctx context.Context, hash string) error {
	now := time.Now()
	res := r.db.Model(&domain.RefreshToken{}).Where("token_hash = ?", hash).Update("revoked_at", now)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *refreshTokenRepo) ListByAccount(ctx context.Context, accountID string) ([]*domain.RefreshToken, error) {
	var list []*domain.RefreshToken
	err := r.db.Where("account_id = ?", accountID).Order("created_at DESC").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}
