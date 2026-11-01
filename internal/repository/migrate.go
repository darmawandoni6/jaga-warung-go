package repository

import (
	"fmt"

	"jaga-warung-go/internal/domain"

	"gorm.io/gorm"
)

func SyncDatabase(db *gorm.DB) error {

	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`).Error; err != nil {
		return fmt.Errorf("gagal mengaktifkan extension uuid-ossp: %w", err)
	}

	// type was a free-form tag that duplicated category.name; product categories
	// now live on the proper FK column (category_id). Drop the legacy column
	// eagerly so it never surfaces again — one-time, safe, no-op if already gone.
	if err := db.Exec(`ALTER TABLE products DROP COLUMN IF EXISTS type`).Error; err != nil {
		return fmt.Errorf("drop legacy product.type column: %w", err)
	}

	err := db.AutoMigrate(
		&domain.Category{},
		&domain.Product{},
		&domain.Account{},
		&domain.RefreshToken{},
		&domain.Transaction{},
		&domain.TransactionItem{},
		&domain.StockMovement{},
		&domain.Debt{},
		&domain.DebtPayment{},
		&domain.CashFlow{},
	)
	if err != nil {
		return fmt.Errorf("gagal auto-migrate: %w", err)
	}

	// Plan Q2: hard-deleting a category must never orphan products, so the FK is
	// RESTRICT. GORM does not rewrite an existing constraint when the tag
	// changes, hence the explicit recreate. Idempotent: drop-if-exists + add.
	if err := db.Exec(`ALTER TABLE products DROP CONSTRAINT IF EXISTS fk_products_category`).Error; err != nil {
		return fmt.Errorf("drop fk_products_category: %w", err)
	}
	if err := db.Exec(`ALTER TABLE products ADD CONSTRAINT fk_products_category
		FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE RESTRICT`).Error; err != nil {
		return fmt.Errorf("add fk_products_category: %w", err)
	}

	return nil
}
