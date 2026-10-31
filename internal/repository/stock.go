package repository

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"jaga-warung-go/internal/domain"
)

// Stock is derived, never authoritative (docs/schema.md § Stock is derived):
//
//	stock = Σ stock_movements.quantity × sign
//
// where quantity is always stored positive and the sign comes from the type.
// products.stock is only a cache rebuilt from that sum. Every write that touches
// a sale line item goes through rebuildStock so the cache and the movement log
// cannot drift.

// stockSign maps a movement type to the direction it applies to stock.
func stockSign(movementType string) int64 {
	switch movementType {
	case domain.MovementRestock, domain.MovementAdjustmentGain:
		return 1
	case domain.MovementSale, domain.MovementAdjustmentLoss:
		return -1
	default:
		return 0
	}
}

// sumMovements computes current stock for one product from the movement log.
// Soft-deleted movements are excluded explicitly: DeletedAt is a plain pointer,
// so GORM does not filter them for us.
func sumMovements(tx *gorm.DB, barcode string) (int64, error) {
	var stock int64
	err := tx.Raw(`
		SELECT COALESCE(SUM(
			CASE WHEN type IN ('restock','adjustment_gain') THEN quantity
			     ELSE -quantity
			END
		), 0)
		FROM stock_movements
		WHERE product_barcode = ? AND deleted_at IS NULL`, barcode).
		Scan(&stock).Error
	if err != nil {
		return 0, err
	}
	return stock, nil
}

// writeStockCache stores the rebuilt sum on the product row.
func writeStockCache(tx *gorm.DB, barcode string, stock int64) error {
	res := tx.Model(&domain.Product{}).
		Where("barcode = ?", barcode).
		Update("stock", stock)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// lockProduct loads a product FOR UPDATE so concurrent sales cannot both pass
// the stock check and oversell. Returns ErrNotFound when the barcode is unknown.
func lockProduct(tx *gorm.DB, barcode string) (*domain.Product, error) {
	var p domain.Product
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("barcode = ?", barcode).
		First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// rebuildStock regenerates the product's sale movements from the live
// transaction_items and then recomputes products.stock from the movement sum.
//
// Sale movements are aggregated into a single row per product rather than one
// per item: the invariant is on the sum, so one aggregate row is equivalent and
// avoids a movement row per line. Existing sale movements are soft-deleted
// first, keeping the old ones as an audit trail instead of erasing them.
func rebuildStock(tx *gorm.DB, barcode string) error {
	var sold int64
	err := tx.Raw(`
		SELECT COALESCE(SUM(quantity), 0)
		FROM transaction_items
		WHERE product_barcode = ? AND deleted_at IS NULL`, barcode).
		Scan(&sold).Error
	if err != nil {
		return err
	}

	// Retire the previous aggregate sale movement(s) for this product.
	err = tx.Exec(`
		UPDATE stock_movements
		SET deleted_at = now()
		WHERE product_barcode = ? AND type = 'sale' AND deleted_at IS NULL`,
		barcode).Error
	if err != nil {
		return err
	}

	if sold > 0 {
		previous, err := sumMovements(tx, barcode)
		if err != nil {
			return err
		}
		// Invariant guard: the rebuilt stock may not go negative. This is the
		// single place overselling is caught, so creating and editing a line
		// both return ErrInsufficientStock instead of writing a negative stock.
		if previous-sold < 0 {
			return domain.ErrInsufficientStock
		}
		movement := &domain.StockMovement{
			ProductBarcode: barcode,
			Type:           domain.MovementSale,
			Quantity:       sold,
			PreviousStock:  previous,
			FinalStock:     previous - sold,
		}
		if err := tx.Create(movement).Error; err != nil {
			return err
		}
	}

	stock, err := sumMovements(tx, barcode)
	if err != nil {
		return err
	}
	return writeStockCache(tx, barcode, stock)
}
