package database

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"jaga-warung-go/internal/auth"
	"jaga-warung-go/internal/config"
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

// Migrate runs AutoMigrate plus the one constraint GORM tags cannot express:
// the expression index on lower(categories.name) (D6). Foreign keys and their
// ON DELETE actions come from the model associations.
//
// Single tenant, so the index is global. AutoMigrate is allowed only while
// there is no production data; once real data exists, freeze the schema and
// move to versioned SQL migrations.
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		return err
	}
	return db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_categories_name
	                ON categories (lower(name)) WHERE deleted_at IS NULL`).Error
}

// Seed is idempotent: it creates the one account from env if none exists, and
// the five default categories if missing. It never overwrites user values.
func Seed(db *gorm.DB, cfg *config.Config) error {
	var accounts int64
	if err := db.Model(&model.Account{}).Count(&accounts).Error; err != nil {
		return err
	}
	if accounts == 0 {
		hash, err := auth.HashPassword(cfg.AdminPassword)
		if err != nil {
			return err
		}
		now := time.Now()
		acc := model.Account{
			ID:           uuid.New().String(),
			Phone:        cfg.AdminPhone,
			PasswordHash: hash,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := db.Create(&acc).Error; err != nil {
			return err
		}
	}

	for _, name := range []string{"Makanan", "Minuman", "Sembako", "Rokok", "Lainnya"} {
		var n int64
		if err := db.Model(&model.Category{}).Where("lower(name) = lower(?)", name).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			now := time.Now()
			cat := model.Category{ID: uuid.New().String(), Name: name, CreatedAt: now, UpdatedAt: now}
			if err := db.Create(&cat).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
