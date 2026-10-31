package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"jaga-warung-go/internal/domain"
)

// debtPaymentRepo implements domain.DebtPaymentRepository.
//
// Create is the payment flow from docs/api.md § Business endpoints: the payment
// row, the parent debt's paid_amount/status, and the optional cash-flow income
// all land in one DB transaction, so a payment can never be recorded without the
// debt reflecting it.
type debtPaymentRepo struct{ db *gorm.DB }

func NewDebtPaymentRepository(db *gorm.DB) domain.DebtPaymentRepository {
	return &debtPaymentRepo{db: db}
}

func (r *debtPaymentRepo) List(ctx context.Context, page, limit int, debtID *uuid.UUID) ([]*domain.DebtPayment, int64, error) {
	q := r.db.WithContext(ctx).Model(&domain.DebtPayment{}).
		Where("deleted_at IS NULL")
	if debtID != nil {
		q = q.Where("debt_id = ?", *debtID)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []*domain.DebtPayment
	offset := (page - 1) * limit
	if err := q.Order("created_at DESC").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *debtPaymentRepo) Get(ctx context.Context, id uuid.UUID) (*domain.DebtPayment, error) {
	var p domain.DebtPayment
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

// Create records a payment and re-derives the debt's paid_amount and status.
// Returns the updated debt so the handler can echo both rows.
func (r *debtPaymentRepo) Create(ctx context.Context, p *domain.DebtPayment, createCashFlow bool) (*domain.Debt, error) {
	var updated *domain.Debt

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the debt: two concurrent payments must not both read the same
		// paid_amount and lose one of the increments.
		var debt domain.Debt
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND deleted_at IS NULL", p.DebtID).
			First(&debt).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return err
		}

		if err := tx.Omit(clause.Associations).Create(p).Error; err != nil {
			return err
		}

		debt.PaidAmount += p.Amount
		// Status is derived, never taken from the request: bad_debt is cleared
		// here by design when a payment arrives.
		debt.Status = domain.DeriveDebtStatus(debt.TotalDebt, debt.PaidAmount)

		res := tx.Model(&domain.Debt{}).
			Where("id = ?", debt.ID).
			Select("PaidAmount", "Status", "UpdatedAt").
			Updates(&debt)
		if res.Error != nil {
			return res.Error
		}

		if createCashFlow {
			note := "debt payment"
			flow := &domain.CashFlow{
				Type:   "income",
				Amount: p.Amount,
				Note:   note,
				Date:   time.Now().Format("2006-01-02"),
			}
			if err := tx.Omit(clause.Associations).Create(flow).Error; err != nil {
				return err
			}
		}

		updated = &debt
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Update rewrites a payment's amount and re-syncs the parent debt, so the sum of
// payments and paid_amount cannot drift apart.
func (r *debtPaymentRepo) Update(ctx context.Context, p *domain.DebtPayment) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var before domain.DebtPayment
		err := tx.Where("id = ? AND deleted_at IS NULL", p.ID).First(&before).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return err
		}

		res := tx.Model(&domain.DebtPayment{}).
			Where("id = ? AND deleted_at IS NULL", p.ID).
			Select("Amount", "Note").
			Updates(p)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrNotFound
		}

		return resyncDebt(tx, before.DebtID)
	})
}

// Delete soft-deletes the payment and re-syncs the parent debt.
func (r *debtPaymentRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var p domain.DebtPayment
		err := tx.Where("id = ? AND deleted_at IS NULL", id).First(&p).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return err
		}

		if err := tx.Exec(`UPDATE debt_payments SET deleted_at = now() WHERE id = ?`, id).Error; err != nil {
			return err
		}
		return resyncDebt(tx, p.DebtID)
	})
}

// resyncDebt recomputes paid_amount from the live payments and re-derives status.
// Editing or deleting a payment must not leave the debt holding a stale total.
func resyncDebt(tx *gorm.DB, debtID uuid.UUID) error {
	var paid int64
	err := tx.Raw(`
		SELECT COALESCE(SUM(amount), 0)
		FROM debt_payments
		WHERE debt_id = ? AND deleted_at IS NULL`, debtID).
		Scan(&paid).Error
	if err != nil {
		return err
	}

	var debt domain.Debt
	err = tx.Where("id = ? AND deleted_at IS NULL", debtID).First(&debt).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		return err
	}

	debt.PaidAmount = paid
	debt.Status = domain.DeriveDebtStatus(debt.TotalDebt, paid)

	res := tx.Model(&domain.Debt{}).
		Where("id = ?", debtID).
		Select("PaidAmount", "Status", "UpdatedAt").
		Updates(&debt)
	return res.Error
}
