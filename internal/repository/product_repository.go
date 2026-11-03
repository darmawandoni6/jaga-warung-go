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

// productRepo implements domain.ProductRepository.
//
// PATCH write semantics (AGENTS §4, Q8):
//   - PK (barcode) and created_at are immutable; the caller must not pass them
//     in a PatchProductRequest, and Update enforces this by reading them
//     back after bind.
//   - Select(mutable cols) forces explicit 0/""/null from the body to persist;
//     ommitting them would drop them via gorm's zero-value-skipping behavior.
type productRepo struct{ db *gorm.DB }

func NewProductRepo(db *gorm.DB) domain.ProductRepository {
	return &productRepo{db: db}
}

func (r *productRepo) RebuildStockForTransaction(ctx context.Context, transactionID uuid.UUID) error {
	return rebuildStockForTransaction(ctx, r.db, transactionID)
}

func (r *productRepo) List(ctx context.Context, page, limit int, search string, categoryID *int64, includeInactive bool) ([]*domain.Product, int64, error) {
	q := r.db.Model(&domain.Product{}).Preload("Category", preloadLiveCategory)
	if !includeInactive {
		// Soft-deleted rows are inactive; DeletedAt is a plain pointer so GORM
		// filters nothing for us.
		q = q.Where("deleted_at IS NULL")
	}
	if search != "" {
		search = "%" + search + "%"
		q = q.Where("name LIKE ? OR barcode LIKE ?", search, search)
	}
	if categoryID != nil {
		q = q.Where("category_id = ?", *categoryID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var products []*domain.Product
	offset := (page - 1) * limit
	if err := q.Offset(offset).Limit(limit).Find(&products).Error; err != nil {
		return nil, 0, err
	}
	return products, total, nil
}

func (r *productRepo) Get(ctx context.Context, barcode string) (*domain.Product, error) {
	var p domain.Product
	err := r.db.Preload("Category", preloadLiveCategory).
		Where("barcode = ? AND deleted_at IS NULL", barcode).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *productRepo) Create(ctx context.Context, p *domain.Product) error {
	// Stock is derived from stock_movements (docs/schema.md § Stock is derived),
	// so the opening stock a client sends is recorded as one adjustment_gain
	// movement — "opening balance" (B12) — and products.stock is rebuilt from it.
	// Writing p.Stock directly would leave the movement log disagreeing with the
	// cache and make the product unsellable.
	opening := p.Stock
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		p.Stock = 0
		if err := tx.Omit(clause.Associations).Create(p).Error; err != nil {
			return err
		}
		if opening > 0 {
			movement := &domain.StockMovement{
				ProductBarcode: p.Barcode,
				Type:           domain.MovementAdjustmentGain,
				Quantity:       opening,
				PreviousStock:  0,
				FinalStock:     opening,
				Note:           strPtr("opening balance"),
			}
			if err := tx.Omit(clause.Associations).Create(movement).Error; err != nil {
				return err
			}
		}
		if err := rebuildStock(tx, p.Barcode); err != nil {
			return err
		}
		// Reflect the rebuilt cache on the returned row so the client sees the
		// same stock the database now holds.
		stock, err := sumMovements(tx, p.Barcode)
		if err != nil {
			return err
		}
		p.Stock = stock
		return nil
	})
}

// strPtr is a tiny helper for the optional note fields the repositories write.
func strPtr(s string) *string { return &s }

// preloadLiveCategory hides soft-deleted categories from product responses
// (plan § E): GORM's Preload adds no deleted_at filter for the custom
// SoftDelete type.
func preloadLiveCategory(db *gorm.DB) *gorm.DB {
	return db.Where("deleted_at IS NULL")
}

func (r *productRepo) Update(ctx context.Context, p *domain.Product) error {
	// Immutable columns: barcode is PK, created_at is audit.
	savedBarcode, savedCreatedAt := p.Barcode, p.CreatedAt
	defer func() { p.Barcode, p.CreatedAt = savedBarcode, savedCreatedAt }()

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// A PATCH may carry a new stock figure. products.stock is a cache, so
		// writing it directly would be thrown away by the next rebuild; instead
		// the delta is recorded as an adjustment movement and the cache is
		// recomputed from the movement sum.
		current, err := sumMovements(tx, p.Barcode)
		if err != nil {
			return err
		}
		wanted := p.Stock
		if wanted != current {
			delta := wanted - current
			movementType := domain.MovementAdjustmentGain
			quantity := delta
			if delta < 0 {
				movementType = domain.MovementAdjustmentLoss
				quantity = -delta
			}
			movement := &domain.StockMovement{
				ProductBarcode: p.Barcode,
				Type:           movementType,
				Quantity:       quantity,
				PreviousStock:  current,
				FinalStock:     wanted,
				Note:           strPtr("stock adjustment"),
			}
			if err := tx.Omit(clause.Associations).Create(movement).Error; err != nil {
				return err
			}
		}

		// Stock is written below only as the cache value; rebuildStock overwrites
		// it with the authoritative sum. Omit(clause.Associations) keeps the
		// preloaded Category from being written back as its own row.
		upd := tx.Omit(clause.Associations).Model(p).
			Select("Name", "BuyPrice", "SellPrice", "Stock", "MinStock", "Image", "CategoryID", "UpdatedAt").
			Save(p)
		if upd.Error != nil {
			return upd.Error
		}

		// Recompute last so the stored cache matches the movement log even when
		// the caller sent an unchanged stock value.
		stock, err := sumMovements(tx, p.Barcode)
		if err != nil {
			return err
		}
		if err := writeStockCache(tx, p.Barcode, stock); err != nil {
			return err
		}
		p.Stock = stock
		return nil
	})
}

// Delete deactivates the product (soft delete): the row stays so movements and
// transaction_items keep their FK target, but List/Get/sale flows ignore it.
// Undo with Restore.
func (r *productRepo) Delete(ctx context.Context, barcode string) error {
	res := r.db.WithContext(ctx).Model(&domain.Product{}).
		Where("barcode = ? AND deleted_at IS NULL", barcode).
		Update("deleted_at", time.Now())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Restore reactivates a soft-deleted product and returns it with its category.
func (r *productRepo) Restore(ctx context.Context, barcode string) (*domain.Product, error) {
	res := r.db.WithContext(ctx).Model(&domain.Product{}).
		Where("barcode = ? AND deleted_at IS NOT NULL", barcode).
		Update("deleted_at", nil)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, domain.ErrNotFound
	}
	return r.Get(ctx, barcode)
}

func (r *productRepo) CountBelow(ctx context.Context, minStock int64) ([]*domain.Product, int64, error) {
	var list []*domain.Product
	err := r.db.Where("stock <= ? AND deleted_at IS NULL", minStock).Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	return list, int64(len(list)), nil
}

func (r *productRepo) All(ctx context.Context) []*domain.Product {
	var list []*domain.Product
	r.db.Where("deleted_at IS NULL").Find(&list)
	return list
}
