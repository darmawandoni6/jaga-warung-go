package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"jaga-warung-go/internal/domain"
)

// transactionItemRepo implements domain.TransactionItemRepository.
//
// Writes here only touch the line rows. Stock is rebuilt by the Kafka worker
// from transaction.item.* events, so the API never blocks on the movement log.
type transactionItemRepo struct{ db *gorm.DB }

func NewTransactionItemRepo(db *gorm.DB) domain.TransactionItemRepository {
	return &transactionItemRepo{db: db}
}

func (r *transactionItemRepo) List(ctx context.Context, page, limit int, transactionID *uuid.UUID) ([]*domain.TransactionItem, int64, error) {
	q := r.db.WithContext(ctx).Model(&domain.TransactionItem{}).
		Where("deleted_at IS NULL")
	if transactionID != nil {
		q = q.Where("transaction_id = ?", *transactionID)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []*domain.TransactionItem
	offset := (page - 1) * limit
	if err := q.Order("created_at ASC").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *transactionItemRepo) Get(ctx context.Context, id uuid.UUID) (*domain.TransactionItem, error) {
	var item domain.TransactionItem
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &item, nil
}

// Create appends one line to an existing sale. The worker rebuilds that
// product's stock from the resulting event.
func (r *transactionItemRepo) Create(ctx context.Context, item *domain.TransactionItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// The parent sale must exist and be live, otherwise the FK would still
		// accept the row while the sale is gone.
		var count int64
		if err := tx.Model(&domain.Transaction{}).
			Where("id = ? AND deleted_at IS NULL", item.TransactionID).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return domain.ErrNotFound
		}

		// Rejects an unknown or inactive barcode with a clean 404 rather than a
		// raw FK violation.
		if _, err := lockProduct(tx, item.ProductBarcode); err != nil {
			return err
		}

		return tx.Omit(clause.Associations).Create(item).Error
	})
}

// Update writes the mutable columns. Stock follows through the event.
func (r *transactionItemRepo) Update(ctx context.Context, item *domain.TransactionItem) error {
	res := r.db.WithContext(ctx).Model(&domain.TransactionItem{}).
		Where("id = ? AND deleted_at IS NULL", item.ID).
		Select("ProductName", "SellPrice", "Quantity", "Subtotal").
		Updates(item)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Delete soft-deletes the line. The worker releases its quantity from
// transaction.item.deleted.
func (r *transactionItemRepo) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Exec(`UPDATE transaction_items SET deleted_at = now() WHERE id = ? AND deleted_at IS NULL`, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
