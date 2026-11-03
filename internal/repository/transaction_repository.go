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
// Create commits the header and its items only. The stock side effects are
// applied by the Kafka worker from transaction.created: the sale must not wait
// on a rebuild, and the worker recomputes from the committed rows, so a lost
// event is the only way stock can drift (see docs/kafka-plan.md § 5).
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
	err := q.Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Preload("Items").
		Find(&list).Error

	if err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

func (r *transactionRepo) Get(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	var t domain.Transaction
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).Preload("Items").
		First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

// Create inserts the sale header plus its items. Items must already be validated
// by the usecase; stock is rebuilt later by the worker.
func (r *transactionRepo) Create(ctx context.Context, t *domain.Transaction, items []*domain.TransactionItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Omit associations: the models carry FK constraints but GORM would
		// otherwise try to upsert the zero-value Product / Items graphs.
		if err := tx.Omit(clause.Associations).Create(t).Error; err != nil {
			return err
		}

		for _, item := range items {
			item.TransactionID = t.ID
			// Still locked: this rejects an unknown or inactive barcode with a
			// clean 404 instead of a raw FK violation, and it serializes two
			// sales of the same product so the worker's rebuilds stay ordered.
			if _, err := lockProduct(tx, item.ProductBarcode); err != nil {
				return err
			}
			if err := tx.Omit(clause.Associations).Create(item).Error; err != nil {
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

// Delete soft-deletes the header and its items in one transaction. The worker
// releases the stock from transaction.deleted.
func (r *transactionRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Exec(`UPDATE transactions SET deleted_at = now() WHERE id = ? AND deleted_at IS NULL`, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrNotFound
		}

		return tx.Exec(`UPDATE transaction_items SET deleted_at = now() WHERE transaction_id = ? AND deleted_at IS NULL`, id).Error
	})
}
