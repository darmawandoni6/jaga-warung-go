package usecase

import (
	"context"
	"errors"
	"fmt"

	"jaga-warung-go/internal/domain"
)

// ProductUsecase implements domain.ProductUseCase. The handler calls it after
// parsing the request body; the usecase owns validation and delegation to the
// repository. No echo / HTTP dependency lives here.
type ProductUsecase struct {
	repo       domain.ProductRepository
	categories domain.CategoryRepository
}

func NewProductUsecase(repo domain.ProductRepository, categories domain.CategoryRepository) domain.ProductUseCase {
	return &ProductUsecase{repo: repo, categories: categories}
}

// checkCategory rejects a category_id that does not exist, before any write,
// and returns the row so Create/Patch can answer with a fresh `category` object
// instead of a stale preload. ErrNotFound maps to 400: the client sent an
// unknown id, the product itself was never missing.
func (uc *ProductUsecase) checkCategory(ctx context.Context, id *int64) (*domain.Category, error) {
	if id == nil {
		return nil, nil
	}
	c, err := uc.categories.Get(ctx, *id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrBadRequest
		}
		return nil, fmt.Errorf("product category: %w", err)
	}
	return c, nil
}

func (uc *ProductUsecase) List(ctx context.Context, page, limit int, search string, categoryID *int64, includeInactive bool) (*domain.ProductListResult, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	data, total, err := uc.repo.List(ctx, page, limit, search, categoryID, includeInactive)
	if err != nil {
		return nil, fmt.Errorf("product list: %w", err)
	}
	return &domain.ProductListResult{
		Data: data,
		Meta: domain.ProductListMeta{Page: page, Limit: limit, Total: total},
	}, nil
}

func (uc *ProductUsecase) Create(ctx context.Context, req *domain.CreateProductRequest) (*domain.Product, error) {
	if req.Barcode == "" {
		return nil, domain.ErrBadRequest
	}
	// Rule: category is required on create (plan § A). PATCH stays optional.
	if req.CategoryID == nil {
		return nil, domain.ErrBadRequest
	}
	cat, err := uc.checkCategory(ctx, req.CategoryID)
	if err != nil {
		return nil, err
	}
	p := &domain.Product{
		Barcode:    req.Barcode,
		Name:       req.Name,
		BuyPrice:   req.BuyPrice,
		SellPrice:  req.SellPrice,
		Stock:      req.Stock,
		MinStock:   req.MinStock,
		Image:      req.Image,
		CategoryID: req.CategoryID,
		Category:   cat,
	}
	if err := uc.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (uc *ProductUsecase) Get(ctx context.Context, barcode string) (*domain.Product, error) {
	return uc.repo.Get(ctx, barcode)
}

func (uc *ProductUsecase) Patch(ctx context.Context, barcode string, req *domain.PatchProductRequest) (*domain.Product, error) {
	cur, err := uc.repo.Get(ctx, barcode)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("product patch get: %w", err)
	}
	// Apply explicit values; nil fields leave the current value unchanged.
	if req.Name != nil {
		cur.Name = *req.Name
	}
	if req.BuyPrice != nil {
		cur.BuyPrice = *req.BuyPrice
	}
	if req.SellPrice != nil {
		cur.SellPrice = *req.SellPrice
	}
	if req.Stock != nil {
		cur.Stock = *req.Stock
	}
	if req.MinStock != nil {
		cur.MinStock = *req.MinStock
	}
	if req.Image != nil {
		cur.Image = req.Image
	}
	if req.CategoryID != nil {
		cat, err := uc.checkCategory(ctx, req.CategoryID)
		if err != nil {
			return nil, err
		}
		cur.CategoryID = req.CategoryID
		// Refresh the association, otherwise the response keeps showing the
		// category the product was moved away from.
		cur.Category = cat
	}
	if err := uc.repo.Update(ctx, cur); err != nil {
		return nil, err
	}
	return cur, nil
}

func (uc *ProductUsecase) Delete(ctx context.Context, barcode string) error {
	return uc.repo.Delete(ctx, barcode)
}

// Restore reactivates a soft-deleted product (DELETE /products/:barcode is a
// soft delete — the row stays for movements and sale history).
func (uc *ProductUsecase) Restore(ctx context.Context, barcode string) (*domain.Product, error) {
	p, err := uc.repo.Restore(ctx, barcode)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("product restore: %w", err)
	}
	return p, nil
}

func (uc *ProductUsecase) LowStock(ctx context.Context) ([]*domain.Product, int64, error) {
	return uc.repo.CountBelow(ctx, 0)
}
