package repository

import (
	"context"
	"errors"
	"fmt"
	"log"

	"gorm.io/gorm"

	"jaga-warung-go/internal/domain"
	"jaga-warung-go/pkg/auth"
)

// SeedAdminAccount provisions the single login from env. Idempotent: if the
// phone already exists the row is left untouched, so restarting the server
// never resets a rotated password.
func SeedAdminAccount(ctx context.Context, db *gorm.DB, phone, password string) error {
	if phone == "" || password == "" {
		return fmt.Errorf("SeedAdminAccount: ADMIN_PHONE and ADMIN_PASSWORD are required")
	}

	repo := NewAccountRepo(db)
	if _, err := repo.FindByPhone(ctx, phone); err == nil {
		return nil // already seeded
	} else if !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("SeedAdminAccount: %w", err)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("SeedAdminAccount hash: %w", err)
	}

	acc := &domain.Account{Phone: phone, PasswordHash: hash}
	if err := repo.Create(ctx, acc); err != nil {
		return fmt.Errorf("SeedAdminAccount create: %w", err)
	}

	log.Printf("Seeding akun admin berhasil: %s", phone)
	return nil
}

// RunSeeders mengisolasi seluruh eksekusi seeder di satu tempat
func RunSeeders(db *gorm.DB, cfg SeedConfig) {
	ctx := context.Background()

	categoryRepo := NewCategoryRepository(db)
	if err := categoryRepo.SeedDefaultCategories(ctx); err != nil {
		log.Printf("Peringatan seeder kategori: %v", err)
	}

	if err := SeedAdminAccount(ctx, db, cfg.AdminPhone, cfg.AdminPassword); err != nil {
		log.Printf("Peringatan seeder akun admin: %v", err)
	}
}

// SeedConfig carries the env values the seeders need. Kept separate from
// config.Config so this package does not import the config layer.
type SeedConfig struct {
	AdminPhone    string
	AdminPassword string
}
