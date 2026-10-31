package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"jaga-warung-go/internal/domain"
)

// DebtPaymentUsecase implements domain.DebtPaymentUseCase. Create is the payment
// flow: the repository records the payment, adds to the debt's paid_amount,
// re-derives status and optionally emits a cash-flow income, atomically.
type DebtPaymentUsecase struct {
	repo domain.DebtPaymentRepository
}

func NewDebtPaymentUsecase(repo domain.DebtPaymentRepository) domain.DebtPaymentUseCase {
	return &DebtPaymentUsecase{repo: repo}
}

func (uc *DebtPaymentUsecase) List(ctx context.Context, page, limit int, debtID *uuid.UUID) (*domain.DebtPaymentListResult, error) {
	page, limit = normalizePage(page, limit)
	data, total, err := uc.repo.List(ctx, page, limit, debtID)
	if err != nil {
		return nil, fmt.Errorf("debt payment list: %w", err)
	}
	return &domain.DebtPaymentListResult{
		Data: data,
		Meta: domain.ProductListMeta{Page: page, Limit: limit, Total: total},
	}, nil
}

func (uc *DebtPaymentUsecase) Create(ctx context.Context, req *domain.CreateDebtPaymentRequest) (*domain.DebtPaymentResult, error) {
	// No uuid.Nil guard here: the all-zero UUID is a well-formed but absent id,
	// so the repository's lookup must decide between 404 and success. The handler
	// already rejects an empty or unparseable debt_id with 400.
	if req.Amount <= 0 {
		return nil, domain.ErrBadRequest
	}

	payment := &domain.DebtPayment{
		DebtID: req.DebtID,
		Amount: req.Amount,
		Note:   req.Note,
	}

	debt, err := uc.repo.Create(ctx, payment, req.CreateCashFlow)
	if err != nil {
		return nil, err
	}
	return &domain.DebtPaymentResult{Payment: payment, Debt: debt}, nil
}

func (uc *DebtPaymentUsecase) Get(ctx context.Context, id uuid.UUID) (*domain.DebtPayment, error) {
	p, err := uc.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("debt payment get: %w", err)
	}
	return p, nil
}

func (uc *DebtPaymentUsecase) Patch(ctx context.Context, id uuid.UUID, req *domain.PatchDebtPaymentRequest) (*domain.DebtPayment, error) {
	cur, err := uc.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("debt payment patch get: %w", err)
	}

	if req.Amount != nil {
		if *req.Amount <= 0 {
			return nil, domain.ErrBadRequest
		}
		cur.Amount = *req.Amount
	}
	if req.Note != nil {
		cur.Note = req.Note
	}

	if err := uc.repo.Update(ctx, cur); err != nil {
		return nil, err
	}
	return cur, nil
}

func (uc *DebtPaymentUsecase) Delete(ctx context.Context, id uuid.UUID) error {
	return uc.repo.Delete(ctx, id)
}
