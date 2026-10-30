package repository

import (
	"context"
	"log"

	"gorm.io/gorm"
)

// RunSeeders mengisolasi seluruh eksekusi seeder di satu tempat
func RunSeeders(db *gorm.DB) {
	ctx := context.Background()

	categoryRepo := NewCategoryRepository(db)
	if err := categoryRepo.SeedDefaultCategories(ctx); err != nil {
		log.Printf("Peringatan seeder kategori: %v", err)
	}
}
