package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"jaga-warung-go/internal/domain"
)

// StockMovementUsecase implements domain.StockMovementUseCase. Create is the
// business POST from docs/api.md § Business endpoints: the repository applies
// the delta and appends the movement atomically, then rebuilds the stock cache.
// This layer validates the movement type and quantity shape.
type StockMovementUsecase struct {
	repo domain.StockMovementRepository
}

func NewStockMovementUsecase(repo domain.StockMovementRepository) domain.StockMovementUseCase {
	return &StockMovementUsecase{repo: repo}
}

func (uc *StockMovementUsecase) List(ctx context.Context, page, limit int, productBarcode string) (*domain.StockMovementListResult, error) {
	page, limit = normalizePage(page, limit)
	data, total, err := uc.repo.List(ctx, page, limit, productBarcode)
	if err != nil {
		return nil, fmt.Errorf("stock movement list: %w", err)
	}
	return &domain.StockMovementListResult{
		Data: data,
		Meta: domain.ProductListMeta{Page: page, Limit: limit, Total: total},
	}, nil
}

func (uc *StockMovementUsecase) Create(ctx context.Context, req *domain.CreateStockMovementRequest) (*domain.StockMovement, error) {
	if req.ProductBarcode == "" {
		return nil, domain.ErrBadRequest
	}
	if !domain.ValidMovementType(req.Type) {
		return nil, domain.ErrBadRequest
	}
	// Quantity is always stored positive; the direction comes from the type.
	if req.Quantity <= 0 {
		return nil, domain.ErrBadRequest
	}
	if req.TotalCost < 0 {
		return nil, domain.ErrBadRequest
	}

	m := &domain.StockMovement{
		ProductBarcode: req.ProductBarcode,
		Type:           req.Type,
		Quantity:       req.Quantity,
		TotalCost:      req.TotalCost,
		Note:           req.Note,
	}
	if err := uc.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (uc *StockMovementUsecase) Get(ctx context.Context, id uuid.UUID) (*domain.StockMovement, error) {
	m, err := uc.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("stock movement get: %w", err)
	}
	return m, nil
}

func (uc *StockMovementUsecase) Patch(ctx context.Context, id uuid.UUID, req *domain.PatchStockMovementRequest) (*domain.StockMovement, error) {
	cur, err := uc.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("stock movement patch get: %w", err)
	}

	if req.Type != nil {
		if !domain.ValidMovementType(*req.Type) {
			return nil, domain.ErrBadRequest
		}
		cur.Type = *req.Type
	}
	if req.Quantity != nil {
		if *req.Quantity <= 0 {
			return nil, domain.ErrBadRequest
		}
		cur.Quantity = *req.Quantity
	}
	if req.TotalCost != nil {
		if *req.TotalCost < 0 {
			return nil, domain.ErrBadRequest
		}
		cur.TotalCost = *req.TotalCost
	}
	if req.Note != nil {
		cur.Note = req.Note
	}

	if err := uc.repo.Update(ctx, cur); err != nil {
		return nil, err
	}
	return cur, nil
}

func (uc *StockMovementUsecase) Delete(ctx context.Context, id uuid.UUID) error {
	return uc.repo.Delete(ctx, id)
}
