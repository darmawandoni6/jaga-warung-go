package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"jaga-warung-go/internal/domain"
)

type fakeDebtPaymentRepo struct {
	created       *domain.DebtPayment
	createdFlag   bool
	createDebt    *domain.Debt
	createErr     error
	getResult     *domain.DebtPayment
	getErr        error
	updated       *domain.DebtPayment
	updateErr     error
	deleted       uuid.UUID
	deleteErr     error
	listDebtIDArg *uuid.UUID
}

func (f *fakeDebtPaymentRepo) List(ctx context.Context, page, limit int, debtID *uuid.UUID) ([]*domain.DebtPayment, int64, error) {
	f.listDebtIDArg = debtID
	return nil, 0, nil
}

func (f *fakeDebtPaymentRepo) Get(ctx context.Context, id uuid.UUID) (*domain.DebtPayment, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getResult, nil
}

func (f *fakeDebtPaymentRepo) Create(ctx context.Context, p *domain.DebtPayment, createCashFlow bool) (*domain.Debt, error) {
	f.created, f.createdFlag = p, createCashFlow
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.createDebt, nil
}

func (f *fakeDebtPaymentRepo) Update(ctx context.Context, p *domain.DebtPayment) error {
	f.updated = p
	return f.updateErr
}

func (f *fakeDebtPaymentRepo) Delete(ctx context.Context, id uuid.UUID) error {
	f.deleted = id
	return f.deleteErr
}

func TestDebtPaymentCreate_RejectsZeroAmount(t *testing.T) {
	repo := &fakeDebtPaymentRepo{}
	uc := NewDebtPaymentUsecase(repo)

	_, err := uc.Create(context.Background(), &domain.CreateDebtPaymentRequest{
		DebtID: uuid.New(),
		Amount: 0,
	})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
	if repo.created != nil {
		t.Fatal("repository must not be called when validation fails")
	}
}

func TestDebtPaymentCreate_RejectsNegativeAmount(t *testing.T) {
	uc := NewDebtPaymentUsecase(&fakeDebtPaymentRepo{})

	_, err := uc.Create(context.Background(), &domain.CreateDebtPaymentRequest{
		DebtID: uuid.New(),
		Amount: -5000,
	})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

// A missing debt_id is rejected by the handler before it reaches the usecase, so
// an all-zero UUID here is a well-formed-but-absent id and must reach the
// repository, which answers 404 rather than 400.
func TestDebtPaymentCreate_NilUUIDReachesRepository(t *testing.T) {
	repo := &fakeDebtPaymentRepo{createErr: domain.ErrNotFound}
	uc := NewDebtPaymentUsecase(repo)

	_, err := uc.Create(context.Background(), &domain.CreateDebtPaymentRequest{
		DebtID: uuid.Nil,
		Amount: 5000,
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for absent debt, got %v", err)
	}
	if repo.created == nil {
		t.Fatal("repository must be called so it can report not-found")
	}
}

// The cash-flow flag must reach the repository untouched: it decides whether an
// income row is written.
func TestDebtPaymentCreate_PassesCashFlowFlag(t *testing.T) {
	for _, flag := range []bool{true, false} {
		repo := &fakeDebtPaymentRepo{createDebt: &domain.Debt{ID: uuid.New()}}
		uc := NewDebtPaymentUsecase(repo)

		_, err := uc.Create(context.Background(), &domain.CreateDebtPaymentRequest{
			DebtID:         uuid.New(),
			Amount:         5000,
			CreateCashFlow: flag,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.createdFlag != flag {
			t.Fatalf("expected create_cash_flow=%v, got %v", flag, repo.createdFlag)
		}
	}
}

// The handler needs the updated debt back to echo paid_amount + status.
func TestDebtPaymentCreate_ReturnsUpdatedDebt(t *testing.T) {
	updated := &domain.Debt{
		ID:         uuid.New(),
		TotalDebt:  100000,
		PaidAmount: 25000,
		Status:     domain.DebtStatusPartial,
	}
	repo := &fakeDebtPaymentRepo{createDebt: updated}
	uc := NewDebtPaymentUsecase(repo)

	res, err := uc.Create(context.Background(), &domain.CreateDebtPaymentRequest{
		DebtID: updated.ID,
		Amount: 25000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Debt == nil || res.Debt.PaidAmount != 25000 {
		t.Fatalf("expected updated debt in result, got %+v", res.Debt)
	}
	if res.Payment == nil || res.Payment.Amount != 25000 {
		t.Fatalf("expected payment in result, got %+v", res.Payment)
	}
}

func TestDebtPaymentCreate_PropagatesNotFound(t *testing.T) {
	repo := &fakeDebtPaymentRepo{createErr: domain.ErrNotFound}
	uc := NewDebtPaymentUsecase(repo)

	_, err := uc.Create(context.Background(), &domain.CreateDebtPaymentRequest{
		DebtID: uuid.New(),
		Amount: 5000,
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDebtPaymentPatch_RejectsZeroAmount(t *testing.T) {
	existing := &domain.DebtPayment{ID: uuid.New(), Amount: 5000}
	repo := &fakeDebtPaymentRepo{getResult: existing}
	uc := NewDebtPaymentUsecase(repo)

	_, err := uc.Patch(context.Background(), existing.ID, &domain.PatchDebtPaymentRequest{
		Amount: int64Ptr(0),
	})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
	if repo.updated != nil {
		t.Fatal("repository must not be updated when validation fails")
	}
}

func TestDebtPaymentList_PassesDebtIDFilter(t *testing.T) {
	repo := &fakeDebtPaymentRepo{}
	uc := NewDebtPaymentUsecase(repo)
	debtID := uuid.New()

	if _, err := uc.List(context.Background(), 1, 20, &debtID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.listDebtIDArg == nil || *repo.listDebtIDArg != debtID {
		t.Fatalf("expected debt_id filter %v, got %v", debtID, repo.listDebtIDArg)
	}
}
