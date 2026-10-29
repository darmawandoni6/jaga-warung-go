package database

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"jaga-warung-go/internal/model"
)

// Connect opens the pool. TranslateError turns driver errors into gorm's
// ErrDuplicatedKey / ErrForeignKeyViolated so handler.DBError can map them.
func Connect(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		TranslateError: true,
		Logger:         logger.Default.LogMode(logger.Warn),
	})
}

// Migrate runs AutoMigrate plus the constraints GORM tags cannot express:
// the expression index on lower(categories.name) (D6) and the FK actions.
//
// AutoMigrate is allowed only while there is no production data; once real data
// exists, freeze the schema and move to versioned SQL migrations.
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		return err
	}

	stmts := []string{
		// Per-owner, not global: two warungs both seed "Makanan" (B3).
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_categories_owner_name
		   ON categories (owner_id, lower(name)) WHERE deleted_at IS NULL`,

		`DO $$ BEGIN
		   IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_transaction_items_transaction') THEN
		     ALTER TABLE transaction_items ADD CONSTRAINT fk_transaction_items_transaction
		       FOREIGN KEY (transaction_id) REFERENCES transactions (id) ON DELETE CASCADE;
		   END IF;
		   IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_transaction_items_product') THEN
		     ALTER TABLE transaction_items ADD CONSTRAINT fk_transaction_items_product
		       FOREIGN KEY (product_barcode) REFERENCES products (barcode) ON DELETE RESTRICT;
		   END IF;
		   IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_stock_movements_product') THEN
		     ALTER TABLE stock_movements ADD CONSTRAINT fk_stock_movements_product
		       FOREIGN KEY (product_barcode) REFERENCES products (barcode) ON DELETE CASCADE;
		   END IF;
		   IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_debt_payments_debt') THEN
		     ALTER TABLE debt_payments ADD CONSTRAINT fk_debt_payments_debt
		       FOREIGN KEY (debt_id) REFERENCES debts (id) ON DELETE CASCADE;
		   END IF;
		 END $$`,
	}

	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return err
		}
	}
	return nil
}
