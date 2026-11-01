package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

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
// deleted_at is filtered manually (plan § E): SoftDelete.DeletedAt is a plain
// pointer, so GORM adds no filter for it.
func (r *categoryRepo) List(ctx context.Context) ([]*domain.Category, error) {
	var categories []*domain.Category
	if err := r.db.WithContext(ctx).Where("deleted_at IS NULL").Find(&categories).Error; err != nil {
		return nil, fmt.Errorf("categoryRepository.List: %w", err)
	}
	return categories, nil
}

// FindByID returns one category by ID, ErrNotFound when missing or deleted
// (interface name: Get).
func (r *categoryRepo) Get(ctx context.Context, id int64) (*domain.Category, error) {
	var category domain.Category
	err := r.db.WithContext(ctx).Where("deleted_at IS NULL").First(&category, id).Error
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

// Delete soft-deletes the category and moves its products to 'Lainnya', so no
// product ever references a deleted category (plan § D, Rule 1). SoftDelete
// is a plain pointer, not gorm.DeletedAt, so GORM Delete would hard-delete the
// row here — hence the explicit deleted_at update.
func (r *categoryRepo) Delete(ctx context.Context, id int64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&domain.Category{}).
			Where("id = ? AND deleted_at IS NULL", id).
			Update("deleted_at", now)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrNotFound
		}
		target, err := ensureCategoryByName(tx, "Lainnya")
		if err != nil {
			return fmt.Errorf("category delete reassign: %w", err)
		}
		return tx.Model(&domain.Product{}).
			Where("category_id = ?", id).
			Update("category_id", target).Error
	})
}

// ensureCategoryByName returns the id of a visible category with that name,
// creating it when absent and reviving it when soft-deleted (the unique index
// on name forbids a second row, so revive is the only option). Name, never id:
// ids are not stable across DB rebuilds (plan § C).
func ensureCategoryByName(db *gorm.DB, name string) (int64, error) {
	var c domain.Category
	err := db.Unscoped().Where("name = ?", name).First(&c).Error
	switch {
	case err == nil:
		if c.DeletedAt != nil {
			if err := db.Unscoped().Model(&c).Update("deleted_at", nil).Error; err != nil {
				return 0, err
			}
		}
		return c.ID, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		created := domain.Category{Name: name}
		if err := db.Create(&created).Error; err != nil {
			return 0, err
		}
		return created.ID, nil
	default:
		return 0, err
	}
}

// Restore undeletes a previously soft-deleted category by clearing deleted_at.
func (r *categoryRepo) Restore(ctx context.Context, id int64) (*domain.Category, error) {
	var category domain.Category
	res := r.db.WithContext(ctx).Unscoped().Model(&domain.Category{}).
		Where("id = ? AND deleted_at IS NOT NULL", id).
		Update("deleted_at", nil)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, domain.ErrNotFound
	}
	if err := r.db.WithContext(ctx).First(&category, id).Error; err != nil {
		return nil, err
	}
	return &category, nil
}
