package repository

import (
	"fmt"

	"jaga-warung-go/internal/domain"

	"gorm.io/gorm"
)

func SyncDatabase(db *gorm.DB) error {

	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`).Error; err != nil {
		return fmt.Errorf("gagal mengaktifkan extension uuid-ossp: %w", err)
	}

	err := db.AutoMigrate(
		&domain.Category{},
		&domain.Product{},
		&domain.Account{},
	)
	if err != nil {
		return fmt.Errorf("gagal auto-migrate: %w", err)
	}

	return nil
}
