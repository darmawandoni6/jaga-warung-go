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

// FindAll mengambil semua data kategori
func (r *categoryRepo) FindAll(ctx context.Context) ([]*domain.Category, error) {
	var categories []*domain.Category
	if err := r.db.WithContext(ctx).Find(&categories).Error; err != nil {
		return nil, fmt.Errorf("categoryRepository.FindAll: %w", err)
	}
	return categories, nil
}

// FindByID mengambil data kategori berdasarkan ID
func (r *categoryRepo) FindByID(ctx context.Context, id int64) (*domain.Category, error) {
	var category domain.Category
	err := r.db.WithContext(ctx).First(&category, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("categoryRepository.FindByID: %w", err)
	}
	return &category, nil
}
