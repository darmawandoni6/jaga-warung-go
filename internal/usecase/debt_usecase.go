package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"jaga-warung-go/internal/domain"
)

// DebtUsecase implements domain.DebtUseCase. Status is derived on create; the
// only way to force it is an explicit PATCH, which is how a debt becomes
// bad_debt. No echo / HTTP dependency lives here.
type DebtUsecase struct {
	repo domain.DebtRepository
}

func NewDebtUsecase(repo domain.DebtRepository) domain.DebtUseCase {
	return &DebtUsecase{repo: repo}
}

func (uc *DebtUsecase) List(ctx context.Context, page, limit int, status string) (*domain.DebtListResult, error) {
	if status != "" && !domain.ValidDebtStatus(status) {
		return nil, domain.ErrBadRequest
	}
	page, limit = normalizePage(page, limit)
	data, total, err := uc.repo.List(ctx, page, limit, status)
	if err != nil {
		return nil, fmt.Errorf("debt list: %w", err)
	}
	return &domain.DebtListResult{
		Data: data,
		Meta: domain.ProductListMeta{Page: page, Limit: limit, Total: total},
	}, nil
}

func (uc *DebtUsecase) Create(ctx context.Context, req *domain.CreateDebtRequest) (*domain.Debt, error) {
	if req.CustomerName == "" {
		return nil, domain.ErrBadRequest
	}
	if req.TotalDebt < 0 || req.PaidAmount < 0 {
		return nil, domain.ErrBadRequest
	}

	d := &domain.Debt{
		CustomerName: req.CustomerName,
		Phone:        req.Phone,
		TotalDebt:    req.TotalDebt,
		PaidAmount:   req.PaidAmount,
		Note:         req.Note,
		// Derived, never trusted from the client.
		Status: domain.DeriveDebtStatus(req.TotalDebt, req.PaidAmount),
	}
	if err := uc.repo.Create(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (uc *DebtUsecase) Get(ctx context.Context, id uuid.UUID) (*domain.Debt, error) {
	d, err := uc.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("debt get: %w", err)
	}
	return d, nil
}

func (uc *DebtUsecase) Patch(ctx context.Context, id uuid.UUID, req *domain.PatchDebtRequest) (*domain.Debt, error) {
	cur, err := uc.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("debt patch get: %w", err)
	}

	if req.CustomerName != nil {
		if *req.CustomerName == "" {
			return nil, domain.ErrBadRequest
		}
		cur.CustomerName = *req.CustomerName
	}
	if req.Phone != nil {
		cur.Phone = req.Phone
	}
	if req.TotalDebt != nil {
		if *req.TotalDebt < 0 {
			return nil, domain.ErrBadRequest
		}
		cur.TotalDebt = *req.TotalDebt
	}
	if req.PaidAmount != nil {
		if *req.PaidAmount < 0 {
			return nil, domain.ErrBadRequest
		}
		cur.PaidAmount = *req.PaidAmount
	}
	if req.Note != nil {
		cur.Note = req.Note
	}

	// Status precedence: an explicit status wins (that is how bad_debt is set);
	// otherwise changing the amounts re-derives it so it cannot drift.
	if req.Status != nil {
		if !domain.ValidDebtStatus(*req.Status) {
			return nil, domain.ErrBadRequest
		}
		cur.Status = *req.Status
	} else if req.TotalDebt != nil || req.PaidAmount != nil {
		cur.Status = domain.DeriveDebtStatus(cur.TotalDebt, cur.PaidAmount)
	}

	if err := uc.repo.Update(ctx, cur); err != nil {
		return nil, err
	}
	return cur, nil
}

func (uc *DebtUsecase) Delete(ctx context.Context, id uuid.UUID) error {
	return uc.repo.Delete(ctx, id)
}
