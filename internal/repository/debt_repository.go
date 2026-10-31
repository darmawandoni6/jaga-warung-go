package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"jaga-warung-go/internal/domain"
)

// debtRepo implements domain.DebtRepository.
type debtRepo struct{ db *gorm.DB }

func NewDebtRepository(db *gorm.DB) domain.DebtRepository {
	return &debtRepo{db: db}
}

func (r *debtRepo) List(ctx context.Context, page, limit int, status string) ([]*domain.Debt, int64, error) {
	q := r.db.WithContext(ctx).Model(&domain.Debt{}).
		Where("deleted_at IS NULL")
	if status != "" {
		q = q.Where("status = ?", status)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []*domain.Debt
	offset := (page - 1) * limit
	if err := q.Order("created_at DESC").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *debtRepo) Get(ctx context.Context, id uuid.UUID) (*domain.Debt, error) {
	var d domain.Debt
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

// Create stores the debt with a derived status. The caller may have set Status,
// but it is overwritten here so the stored row always agrees with the amounts.
func (r *debtRepo) Create(ctx context.Context, d *domain.Debt) error {
	d.Status = domain.DeriveDebtStatus(d.TotalDebt, d.PaidAmount)
	return r.db.WithContext(ctx).Omit(clause.Associations).Create(d).Error
}

// Update writes the mutable columns. Status is included because PATCH is the one
// path allowed to set bad_debt; the usecase re-derives it otherwise.
func (r *debtRepo) Update(ctx context.Context, d *domain.Debt) error {
	res := r.db.WithContext(ctx).Model(d).
		Where("deleted_at IS NULL").
		Select("CustomerName", "Phone", "TotalDebt", "PaidAmount", "Status", "Note", "UpdatedAt").
		Updates(d)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Delete soft-deletes the debt and its payments, so an orphan payment cannot
// outlive the debt it belongs to.
func (r *debtRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Exec(`UPDATE debts SET deleted_at = now() WHERE id = ? AND deleted_at IS NULL`, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrNotFound
		}
		return tx.Exec(`UPDATE debt_payments SET deleted_at = now() WHERE debt_id = ? AND deleted_at IS NULL`, id).Error
	})
}
