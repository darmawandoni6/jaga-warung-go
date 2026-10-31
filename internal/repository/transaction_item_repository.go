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
// Unlike the header, every write here has a stock side effect: creating,
// editing or deleting an item changes the quantity sold, so each mutation runs
// inside a DB transaction and ends with rebuildStock for the affected product.
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

// Create appends one line to an existing sale and rebuilds that product's stock.
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

		// Adding a line sells more. The product row is locked so the rebuild
		// below sees a stable movement sum, and rebuildStock rejects the write
		// if the total sold would exceed what is on hand.
		if _, err := lockProduct(tx, item.ProductBarcode); err != nil {
			return err
		}

		if err := tx.Omit(clause.Associations).Create(item).Error; err != nil {
			return err
		}
		return rebuildStock(tx, item.ProductBarcode)
	})
}

// Update writes the mutable columns and rebuilds stock for both the old and the
// new product when a line is moved between products.
func (r *transactionItemRepo) Update(ctx context.Context, item *domain.TransactionItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var before domain.TransactionItem
		err := tx.Where("id = ? AND deleted_at IS NULL", item.ID).First(&before).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return err
		}

		res := tx.Model(&domain.TransactionItem{}).
			Where("id = ? AND deleted_at IS NULL", item.ID).
			Select("ProductName", "SellPrice", "Quantity", "Subtotal").
			Updates(item)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrNotFound
		}
		if err := rebuildStock(tx, before.ProductBarcode); err != nil {
			return err
		}
		if before.ProductBarcode != item.ProductBarcode {
			if err := rebuildStock(tx, item.ProductBarcode); err != nil {
				return err
			}
		}
		return nil
	})
}

// Delete soft-deletes the line and releases its quantity back into stock.
func (r *transactionItemRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item domain.TransactionItem
		err := tx.Where("id = ? AND deleted_at IS NULL", id).First(&item).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return err
		}

		if err := tx.Exec(`UPDATE transaction_items SET deleted_at = now() WHERE id = ?`, id).Error; err != nil {
			return err
		}
		return rebuildStock(tx, item.ProductBarcode)
	})
}
