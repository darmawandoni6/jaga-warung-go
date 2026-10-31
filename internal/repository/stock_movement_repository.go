package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"jaga-warung-go/internal/domain"
)

// stockMovementRepo implements domain.StockMovementRepository.
//
// Every write keeps the invariant products.stock = Σ non-deleted movements:
// Create appends a movement and rebuilds the cache; Update re-builds it from the
// revised log; Delete removes a live movement and rebuilds it from what remains.
// quantity is stored positive and the direction comes from the type (stockSign).
type stockMovementRepo struct{ db *gorm.DB }

func NewStockMovementRepo(db *gorm.DB) domain.StockMovementRepository {
	return &stockMovementRepo{db: db}
}

func (r *stockMovementRepo) List(ctx context.Context, page, limit int, productBarcode string) ([]*domain.StockMovement, int64, error) {
	q := r.db.WithContext(ctx).Model(&domain.StockMovement{}).
		Where("deleted_at IS NULL")
	if productBarcode != "" {
		q = q.Where("product_barcode = ?", productBarcode)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []*domain.StockMovement
	offset := (page - 1) * limit
	if err := q.Order("created_at DESC").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *stockMovementRepo) Get(ctx context.Context, id uuid.UUID) (*domain.StockMovement, error) {
	var m domain.StockMovement
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

// Create is the business POST: it locks the product row, appends the movement
// with advisory previous/final stock, then rebuilds the cache. Locking first
// serializes concurrent movements on the same product so the advisory fields are
// meaningful and the rebuild sees a stable base.
func (r *stockMovementRepo) Create(ctx context.Context, m *domain.StockMovement) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockProduct(tx, m.ProductBarcode); err != nil {
			return err
		}
		previous, err := sumMovements(tx, m.ProductBarcode)
		if err != nil {
			return err
		}
		m.PreviousStock = previous
		m.FinalStock = previous + stockSign(m.Type)*m.Quantity

		if err := tx.Omit(clause.Associations).Create(m).Error; err != nil {
			return err
		}
		return writeStockCacheWithCheck(tx, m.ProductBarcode)
	})
}

// Update writes the mutable columns and rebuilds the cache from the revised
// movement log, so a changed type/quantity/delta is reflected in products.stock.
func (r *stockMovementRepo) Update(ctx context.Context, m *domain.StockMovement) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var before domain.StockMovement
		err := tx.Where("id = ? AND deleted_at IS NULL", m.ID).First(&before).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return err
		}

		res := tx.Model(&domain.StockMovement{}).
			Where("id = ? AND deleted_at IS NULL", m.ID).
			Select("Type", "Quantity", "TotalCost", "Note").
			Updates(m)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrNotFound
		}

		// The advisory previous/final columns describe the state at write time;
		// they are not authoritative (docs/schema.md § Stock is derived). Summing
		// the log including the edited row would count the movement twice, so
		// previous is the total minus this movement's own delta and final is the
		// total.
		total, err := sumMovements(tx, before.ProductBarcode)
		if err != nil {
			return err
		}
		m.PreviousStock = total - stockSign(m.Type)*m.Quantity
		m.FinalStock = total
		if err := tx.Model(&domain.StockMovement{}).
			Where("id = ?", m.ID).
			Select("PreviousStock", "FinalStock").
			Updates(m).Error; err != nil {
			return err
		}

		return writeStockCacheWithCheck(tx, before.ProductBarcode)
	})
}

// Delete soft-deletes the movement and rebuilds the cache from the remaining
// log, so the removed delta is released back into products.stock.
func (r *stockMovementRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m domain.StockMovement
		err := tx.Where("id = ? AND deleted_at IS NULL", id).First(&m).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return err
		}

		if err := tx.Exec(`UPDATE stock_movements SET deleted_at = now() WHERE id = ?`, id).Error; err != nil {
			return err
		}
		return writeStockCacheWithCheck(tx, m.ProductBarcode)
	})
}
