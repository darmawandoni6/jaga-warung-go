package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"jaga-warung-go/internal/domain"
)

// TransactionUsecase implements domain.TransactionUseCase. Create is the sale
// flow from docs/api.md § Business endpoints: it validates the body, checks
// stock, and hands the header plus items to the repository, which writes them
// atomically. No echo / HTTP dependency lives here.
type TransactionUsecase struct {
	repo domain.TransactionRepository
}

func NewTransactionUsecase(repo domain.TransactionRepository) domain.TransactionUseCase {
	return &TransactionUsecase{repo: repo}
}

func (uc *TransactionUsecase) List(ctx context.Context, page, limit int) (*domain.TransactionListResult, error) {
	page, limit = normalizePage(page, limit)
	data, total, err := uc.repo.List(ctx, page, limit)
	if err != nil {
		return nil, fmt.Errorf("transaction list: %w", err)
	}
	return &domain.TransactionListResult{
		Data: data,
		Meta: domain.ProductListMeta{Page: page, Limit: limit, Total: total},
	}, nil
}

func (uc *TransactionUsecase) Create(ctx context.Context, req *domain.CreateTransactionRequest) (*domain.Transaction, error) {
	if len(req.Items) == 0 {
		return nil, domain.ErrBadRequest
	}
	if req.PaymentAmount < req.TotalAmount {
		return nil, domain.ErrBadRequest
	}

	totalAmount := int64(0)
	changeAmount := int64(0)

	items := make([]*domain.TransactionItem, 0, len(req.Items))
	for _, in := range req.Items {
		if in.ProductBarcode == "" {
			return nil, domain.ErrBadRequest
		}
		if in.Quantity <= 0 {
			return nil, domain.ErrBadRequest
		}
		subtotal := in.SellPrice * in.Quantity
		totalAmount += subtotal

		items = append(items, &domain.TransactionItem{
			ProductBarcode: in.ProductBarcode,
			ProductName:    in.ProductName,
			SellPrice:      in.SellPrice,
			Quantity:       in.Quantity,
			Subtotal:       subtotal,
		})
	}

	changeAmount = req.PaymentAmount - totalAmount

	if totalAmount != req.TotalAmount {
		return nil, domain.ErrBadRequest
	}

	if changeAmount > 0 && req.ChangeAmount != changeAmount {
		return nil, domain.ErrBadRequest
	}

	t := &domain.Transaction{
		TotalAmount:   totalAmount,
		PaymentAmount: req.PaymentAmount,
		ChangeAmount:  changeAmount,
		Note:          req.Note,
	}

	if err := uc.repo.Create(ctx, t, items); err != nil {
		return nil, err
	}
	return t, nil
}

func (uc *TransactionUsecase) Get(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	t, err := uc.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("transaction get: %w", err)
	}
	return t, nil
}

func (uc *TransactionUsecase) Patch(ctx context.Context, id uuid.UUID, req *domain.PatchTransactionRequest) (*domain.Transaction, error) {
	cur, err := uc.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("transaction patch get: %w", err)
	}
	// Apply explicit values; nil fields keep their current value.
	if req.TotalAmount != nil {
		cur.TotalAmount = *req.TotalAmount
	}
	if req.PaymentAmount != nil {
		cur.PaymentAmount = *req.PaymentAmount
	}
	if req.ChangeAmount != nil {
		cur.ChangeAmount = *req.ChangeAmount
	}
	if req.Note != nil {
		cur.Note = req.Note
	}
	if err := uc.repo.Update(ctx, cur); err != nil {
		return nil, err
	}
	return cur, nil
}

func (uc *TransactionUsecase) Delete(ctx context.Context, id uuid.UUID) error {
	return uc.repo.Delete(ctx, id)
}

// normalizePage clamps pagination to the same defaults as products so every
// paginated list reports a page/limit that matches the rows returned.
func normalizePage(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return page, limit
}
