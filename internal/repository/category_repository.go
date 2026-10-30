package repository

import (
	"context"
	"errors"
	"fmt"
	"log"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"jaga-warung-go/internal/domain"
)

type categoryRepo struct {
	db *gorm.DB
}

// NewCategoryRepository menginstansiasi implementation dari domain.CategoryRepository
func NewCategoryRepository(db *gorm.DB) domain.CategoryRepository {
	return &categoryRepo{
		db: db,
	}
}

// SeedDefaultCategories memasukkan data kategori default tanpa duplikasi (ON CONFLICT DO NOTHING)
func (r *categoryRepo) SeedDefaultCategories(ctx context.Context) error {
	defaultCategories := []domain.Category{
		{Name: "Makanan"},
		{Name: "Minuman"},
		{Name: "Sembako"},
		{Name: "Rokok"},
		{Name: "Lainnya"},
	}

	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "name"}},
			DoNothing: true,
		}).
		Create(&defaultCategories).Error

	if err != nil {
		return fmt.Errorf("categoryRepository.SeedDefaultCategories: %w", err)
	}

	log.Println("Seeding kategori default berhasil")
	return nil
}

// Create menambahkan kategori baru
func (r *categoryRepo) Create(ctx context.Context, category *domain.Category) error {
	if err := r.db.WithContext(ctx).Create(category).Error; err != nil {
		return fmt.Errorf("categoryRepository.Create: %w", err)
	}
	return nil
}

// FindAll returns all non-deleted categories (interface name: List).
func (r *categoryRepo) List(ctx context.Context) ([]*domain.Category, error) {
	var categories []*domain.Category
	if err := r.db.WithContext(ctx).Find(&categories).Error; err != nil {
		return nil, fmt.Errorf("categoryRepository.List: %w", err)
	}
	return categories, nil
}

// FindByID returns one category by ID, ErrNotFound when missing (interface name: Get).
func (r *categoryRepo) Get(ctx context.Context, id int64) (*domain.Category, error) {
	var category domain.Category
	err := r.db.WithContext(ctx).First(&category, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("categoryRepository.Get: %w", err)
	}
	return &category, nil
}

// Update replaces mutable fields; ID and created_at are never touched.
func (r *categoryRepo) Update(ctx context.Context, c *domain.Category) error {
	res := r.db.WithContext(ctx).Model(c).Select("Name", "UpdatedAt").Save(c)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Delete soft-deletes the category. The product FK uses ON DELETE SET NULL
// on the client side — no RESTRICT cascade here.
func (r *categoryRepo) Delete(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Delete(&domain.Category{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
