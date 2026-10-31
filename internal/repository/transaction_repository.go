package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"jaga-warung-go/internal/domain"
)

// transactionRepo implements domain.TransactionRepository.
//
// Create is the only method that touches stock: it runs the header, the items,
// the sale movements and the stock cache in one DB transaction so a failure
// half-way cannot leave a sale without its stock decrement.
type transactionRepo struct{ db *gorm.DB }

func NewTransactionRepo(db *gorm.DB) domain.TransactionRepository {
	return &transactionRepo{db: db}
}

func (r *transactionRepo) List(ctx context.Context, page, limit int) ([]*domain.Transaction, int64, error) {
	q := r.db.WithContext(ctx).Model(&domain.Transaction{}).
		Where("deleted_at IS NULL")

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []*domain.Transaction
	offset := (page - 1) * limit
	if err := q.Order("created_at DESC").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *transactionRepo) Get(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	var t domain.Transaction
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

// Create inserts the sale header plus its items and applies the stock side
// effects. Items must already be validated by the usecase; this layer only
// guarantees atomicity and the stock rebuild.
func (r *transactionRepo) Create(ctx context.Context, t *domain.Transaction, items []*domain.TransactionItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Omit associations: the models carry FK constraints but GORM would
		// otherwise try to upsert the zero-value Product / Items graphs.
		if err := tx.Omit(clause.Associations).Create(t).Error; err != nil {
			return err
		}

		touched := make(map[string]struct{}, len(items))
		for _, item := range items {
			item.TransactionID = t.ID
			// Lock the product row before touching its stock. Two concurrent
			// sales of the same product then serialize instead of both passing
			// a check against the same stale value. Overselling itself is caught
			// by rebuildStock's invariant, which sees the summed quantity.
			if _, err := lockProduct(tx, item.ProductBarcode); err != nil {
				return err
			}
			if err := tx.Omit(clause.Associations).Create(item).Error; err != nil {
				return err
			}
			touched[item.ProductBarcode] = struct{}{}
		}

		for barcode := range touched {
			if err := rebuildStock(tx, barcode); err != nil {
				return err
			}
		}
		return nil
	})
}

// Update writes the mutable header columns only. created_at and the ID are
// immutable; items are edited through the item repository.
func (r *transactionRepo) Update(ctx context.Context, t *domain.Transaction) error {
	res := r.db.WithContext(ctx).Model(t).
		Select("TotalAmount", "PaymentAmount", "ChangeAmount", "Note").
		Where("deleted_at IS NULL").
		Updates(t)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Delete soft-deletes the header and its items in one transaction, then rebuilds
// the stock of every affected product so the sale's quantity is released.
func (r *transactionRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var items []domain.TransactionItem
		err := tx.Where("transaction_id = ? AND deleted_at IS NULL", id).
			Find(&items).Error
		if err != nil {
			return err
		}

		res := tx.Exec(`UPDATE transactions SET deleted_at = now() WHERE id = ? AND deleted_at IS NULL`, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrNotFound
		}

		if err := tx.Exec(`UPDATE transaction_items SET deleted_at = now() WHERE transaction_id = ? AND deleted_at IS NULL`, id).Error; err != nil {
			return err
		}

		for _, item := range items {
			if err := rebuildStock(tx, item.ProductBarcode); err != nil {
				return err
			}
		}
		return nil
	})
}
