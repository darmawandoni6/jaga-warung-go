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
	repo domain.ProductRepository
}

func NewProductUsecase(repo domain.ProductRepository) domain.ProductUseCase {
	return &ProductUsecase{repo: repo}
}

func (uc *ProductUsecase) List(ctx context.Context, page, limit int, search string) (*domain.ProductListResult, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	data, total, err := uc.repo.List(ctx, page, limit, search)
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
	p := &domain.Product{
		Barcode:   req.Barcode,
		Name:      req.Name,
		BuyPrice:  req.BuyPrice,
		SellPrice: req.SellPrice,
		Stock:     req.Stock,
		MinStock:  req.MinStock,
		Type:      req.Type,
		Image:     req.Image,
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
	if req.Type != nil {
		cur.Type = req.Type
	}
	if req.Image != nil {
		cur.Image = req.Image
	}
	if err := uc.repo.Update(ctx, cur); err != nil {
		return nil, err
	}
	return cur, nil
}

func (uc *ProductUsecase) Delete(ctx context.Context, barcode string) error {
	return uc.repo.Delete(ctx, barcode)
}

func (uc *ProductUsecase) LowStock(ctx context.Context) ([]*domain.Product, int64, error) {
	return uc.repo.CountBelow(ctx, 0)
}
