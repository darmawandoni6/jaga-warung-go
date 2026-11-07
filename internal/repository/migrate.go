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
		&domain.Payment{},
	)
	if err != nil {
		return fmt.Errorf("gagal auto-migrate: %w", err)
	}

	// transactions.status arrived after the table existed, so AutoMigrate gave
	// every old row the 'pending' default. Backfill from the amounts before the
	// CHECK is added, otherwise every historical cash sale would look unpaid.
	// The predicate makes it idempotent: a settled row has paid_at set, and a
	// genuinely pending QRIS row has payment_amount = 0, which the CASE keeps
	// pending. Old rows paid through the gateway (amount = total) flip to paid.
	if err := db.Exec(`
		UPDATE transactions
		SET status = CASE WHEN payment_amount >= total_amount THEN 'paid' ELSE 'pending' END,
		    paid_at = CASE WHEN payment_amount >= total_amount THEN created_at ELSE NULL END
		WHERE status = 'pending' AND paid_at IS NULL`).Error; err != nil {
		return fmt.Errorf("backfill transaction status: %w", err)
	}
	for _, q := range []string{
		`ALTER TABLE transactions DROP CONSTRAINT IF EXISTS chk_transactions_status`,
		`ALTER TABLE transactions ADD CONSTRAINT chk_transactions_status
		   CHECK (status IN ('pending','paid','expired'))`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions (status)`,
	} {
		if err := db.Exec(q).Error; err != nil {
			return fmt.Errorf("migrate transaction status: %w", err)
		}
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

	// transaction method moved from 'qris' to 'non_cash': Snap picks the actual
	// channel and it is stored on payments.payment_type, so the sale only needs
	// to know cash vs gateway. Drop the old CHECK first — it only allows
	// cash|qris, so the backfill would violate it.
	if err := db.Exec(`ALTER TABLE transactions DROP CONSTRAINT IF EXISTS chk_transactions_method`).Error; err != nil {
		return fmt.Errorf("drop chk_transactions_method: %w", err)
	}
	if err := db.Exec(`UPDATE transactions SET method = 'non_cash' WHERE method = 'qris'`).Error; err != nil {
		return fmt.Errorf("backfill transaction method: %w", err)
	}

	// method check enforces allowed values at the DB layer; a migration re-adds
	// it eagerly so GORM never creates a row without it. Idempotent because the
	// constraint is dropped first.
	for _, q := range []string{
		`ALTER TABLE transactions ADD CONSTRAINT chk_transactions_method CHECK (method IN ('cash','non_cash'))`,
		`ALTER TABLE debt_payments DROP CONSTRAINT IF EXISTS chk_debt_payments_method`,
		`ALTER TABLE debt_payments ADD CONSTRAINT chk_debt_payments_method CHECK (method IN ('cash','qris'))`,
		`ALTER TABLE payments DROP CONSTRAINT IF EXISTS chk_payments_status`,
		`ALTER TABLE payments ADD CONSTRAINT chk_payments_status CHECK (status IN ('pending','paid','expired','canceled'))`,
		`ALTER TABLE payments DROP CONSTRAINT IF EXISTS chk_payments_target_type`,
		`ALTER TABLE payments ADD CONSTRAINT chk_payments_target_type CHECK (target_type IN ('transaction','debt'))`,
	} {
		if err := db.Exec(q).Error; err != nil {
			return fmt.Errorf("migrate payment constraint: %w", err)
		}
	}

	// One target = one active QRIS. Older duplicates (created before this rule)
	// are expired first so the unique index can be built instead of failing boot.
	if err := db.Exec(`
		UPDATE payments p SET status = 'expired', updated_at = now()
		WHERE p.status = 'pending' AND p.deleted_at IS NULL
		  AND EXISTS (
		    SELECT 1 FROM payments q
		    WHERE q.target_type = p.target_type AND q.target_id = p.target_id
		      AND q.status = 'pending' AND q.deleted_at IS NULL
		      AND q.created_at > p.created_at)`).Error; err != nil {
		return fmt.Errorf("dedupe pending payments: %w", err)
	}
	for _, q := range []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_payments_target_pending
		   ON payments (target_type, target_id)
		   WHERE status = 'pending' AND deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_payments_target
		   ON payments (target_type, target_id)`,
	} {
		if err := db.Exec(q).Error; err != nil {
			return fmt.Errorf("migrate payment index: %w", err)
		}
	}

	return nil
}
