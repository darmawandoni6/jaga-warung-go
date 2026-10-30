package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"jaga-warung-go/internal/domain"
)

// productRepo implements domain.ProductRepository.
//
// PATCH write semantics (AGENTS §4, Q8):
//   - PK (barcode) and created_at are immutable; the caller must not pass them
//     in a PatchProductRequest, and Update enforces this by reading them
//     back after bind.
//   - Select(mutable cols) forces explicit 0/""/null from the body to persist;
//     ommitting them would drop them via gorm's zero-value-skipping behavior.
type productRepo struct{ db *gorm.DB }

func NewProductRepo(db *gorm.DB) domain.ProductRepository {
	return &productRepo{db: db}
}

func (r *productRepo) List(ctx context.Context, page, limit int, search string) ([]*domain.Product, int64, error) {
	q := r.db.Model(&domain.Product{})
	if search != "" {
		search = "%" + search + "%"
		q = q.Where("name LIKE ? OR barcode LIKE ?", search, search)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var products []*domain.Product
	offset := (page - 1) * limit
	if err := q.Offset(offset).Limit(limit).Find(&products).Error; err != nil {
		return nil, 0, err
	}
	return products, total, nil
}

func (r *productRepo) Get(ctx context.Context, barcode string) (*domain.Product, error) {
	var p domain.Product
	err := r.db.Where("barcode = ?", barcode).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *productRepo) Create(ctx context.Context, p *domain.Product) error {
	return r.db.Create(p).Error
}

func (r *productRepo) Update(ctx context.Context, p *domain.Product) error {
	// Immutable columns: barcode is PK, created_at is audit.
	savedBarcode, savedCreatedAt := p.Barcode, p.CreatedAt
	upd := r.db.Model(p).
		Select("Name", "BuyPrice", "SellPrice", "Stock", "MinStock", "Type", "Image", "UpdatedAt").
		Save(p)
	p.Barcode, p.CreatedAt = savedBarcode, savedCreatedAt
	if upd.Error != nil {
		return upd.Error
	}
	return nil
}

func (r *productRepo) Delete(ctx context.Context, barcode string) error {
	res := r.db.Where("barcode = ?", barcode).Delete(&domain.Product{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *productRepo) CountBelow(ctx context.Context, minStock int64) ([]*domain.Product, int64, error) {
	var list []*domain.Product
	err := r.db.Where("stock <= ? AND deleted_at IS NULL", minStock).Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	return list, int64(len(list)), nil
}

func (r *productRepo) All(ctx context.Context) []*domain.Product {
	var list []*domain.Product
	r.db.Find(&list)
	return list
}
