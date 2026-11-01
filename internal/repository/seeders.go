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

	// Product reset + seed run on EVERY boot, by design — stateless demo
	// behavior confirmed in docs/product-category-plan.md (Q1/Q8): sales history
	// dies with the process. Categories themselves are never wiped (Q9).
	if err := ResetProductData(ctx, db); err != nil {
		log.Printf("Peringatan reset produk: %v", err)
	} else if err := SeedProducts(ctx, db); err != nil {
		log.Printf("Peringatan seeder produk: %v", err)
	}

	if err := SeedAdminAccount(ctx, db, cfg.AdminPhone, cfg.AdminPassword); err != nil {
		log.Printf("Peringatan seeder akun admin: %v", err)
	}
}

// curatedProduct is one row of the boot-time product set. Category is a NAME:
// ids are not stable across DB rebuilds (plan § C).
type curatedProduct struct {
	Barcode   string
	Name      string
	Category  string
	BuyPrice  int64
	SellPrice int64
	Stock     int64
	MinStock  int64
}

// Every product carries a category id — Rule 3 (category required). Placeholder
// prices; edit here, not in the DB.
var curatedProducts = []curatedProduct{
	{Barcode: "8990000000011", Name: "Air Mineral 600ml", Category: "Minuman", BuyPrice: 3000, SellPrice: 5000, Stock: 20},
	{Barcode: "8990000000022", Name: "Teh Botol", Category: "Minuman", BuyPrice: 3000, SellPrice: 4000, Stock: 22},
	{Barcode: "8990000000033", Name: "Kopi Sachet", Category: "Minuman", BuyPrice: 1200, SellPrice: 2000, Stock: 10, MinStock: 3},
	{Barcode: "8990000000044", Name: "Mie Instan", Category: "Makanan", BuyPrice: 2500, SellPrice: 3500, Stock: 24},
	{Barcode: "8990000000055", Name: "Beras 5kg", Category: "Sembako", BuyPrice: 65000, SellPrice: 72000, Stock: 8, MinStock: 2},
	{Barcode: "8990000000066", Name: "Minyak Goreng 1L", Category: "Sembako", BuyPrice: 17000, SellPrice: 19500, Stock: 12},
	{Barcode: "8990000000077", Name: "Rokok", Category: "Rokok", BuyPrice: 25000, SellPrice: 28000, Stock: 15},
	{Barcode: "8990000000088", Name: "Sabun Mandi", Category: "Lainnya", BuyPrice: 8000, SellPrice: 11000, Stock: 6},
}

// ResetProductData wipes product-linked tables on every boot. FK RESTRICT
// forces the order: movements → transaction_items → transactions → products.
// Debts/debt_payments/cash_flows have no product link and survive (plan Q8);
// categories are never wiped (plan Q9).
func ResetProductData(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, table := range []string{"stock_movements", "transaction_items", "transactions", "products"} {
			if err := tx.Exec("DELETE FROM " + table).Error; err != nil {
				return fmt.Errorf("ResetProductData %s: %w", table, err)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	log.Println("Reset data produk: semua products + sales terkait dihapus")
	return nil
}

// SeedProducts inserts the curated product set, each with a category id
// resolved by name (auto-create when absent, revive when soft-deleted).
// Creation goes through productRepo.Create, so opening stock lands as an
// adjustment_gain movement and products.stock stays derived from the sum.
// Existing barcodes are skipped: standalone calls must never duplicate (Q4).
func SeedProducts(ctx context.Context, db *gorm.DB) error {
	repo := NewProductRepo(db)
	for _, s := range curatedProducts {
		var n int64
		if err := db.WithContext(ctx).Model(&domain.Product{}).Where("barcode = ?", s.Barcode).Count(&n).Error; err != nil {
			return fmt.Errorf("SeedProducts count %s: %w", s.Barcode, err)
		}
		if n > 0 {
			continue
		}
		catID, err := ensureCategoryByName(db.WithContext(ctx), s.Category)
		if err != nil {
			return fmt.Errorf("SeedProducts category %q: %w", s.Category, err)
		}
		p := &domain.Product{
			Barcode:    s.Barcode,
			Name:       s.Name,
			BuyPrice:   s.BuyPrice,
			SellPrice:  s.SellPrice,
			Stock:      s.Stock,
			MinStock:   s.MinStock,
			CategoryID: &catID,
		}
		if err := repo.Create(ctx, p); err != nil {
			return fmt.Errorf("SeedProducts %s: %w", s.Barcode, err)
		}
	}
	log.Printf("Seeding produk berhasil: %d baris", len(curatedProducts))
	return nil
}

// SeedConfig carries the env values the seeders need. Kept separate from
// config.Config so this package does not import the config layer.
type SeedConfig struct {
	AdminPhone    string
	AdminPassword string
}
