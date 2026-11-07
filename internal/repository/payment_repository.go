package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"jaga-warung-go/internal/domain"
)

// paymentRepo implements domain.PaymentRepository for Postgres with GORM.
type paymentRepo struct{ db *gorm.DB }

func NewPaymentRepository(db *gorm.DB) domain.PaymentRepository {
	return &paymentRepo{db: db}
}

func (r *paymentRepo) Create(ctx context.Context, p *domain.Payment) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *paymentRepo) Get(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	var p domain.Payment
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *paymentRepo) GetByOrderID(ctx context.Context, orderID uuid.UUID) (*domain.Payment, error) {
	var p domain.Payment
	err := r.db.WithContext(ctx).
		Where("order_id = ? AND deleted_at IS NULL", orderID).
		First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *paymentRepo) FindPendingByTarget(ctx context.Context, targetType string, targetID uuid.UUID) (*domain.Payment, error) {
	var p domain.Payment
	err := r.db.WithContext(ctx).
		Where("target_type = ? AND target_id = ? AND status = ? AND deleted_at IS NULL",
			targetType, targetID, domain.PaymentStatusPending).
		Order("created_at DESC").
		First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *paymentRepo) List(ctx context.Context, page, limit int, targetType string, targetID *uuid.UUID) ([]*domain.Payment, int64, error) {
	q := r.db.WithContext(ctx).Model(&domain.Payment{}).Where("deleted_at IS NULL")
	if targetType != "" {
		q = q.Where("target_type = ?", targetType)
	}
	if targetID != nil {
		q = q.Where("target_id = ?", *targetID)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []*domain.Payment
	offset := (page - 1) * limit
	if err := q.Order("created_at DESC").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *paymentRepo) Update(ctx context.Context, p *domain.Payment) error {
	// Save, not Updates: settlement writes NULL-able proof fields and an empty
	// raw_status, which Updates would silently skip as zero values.
	return r.db.WithContext(ctx).Save(p).Error
}
