package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"jaga-warung-go/internal/domain"
)

type fakeDebtRepo struct {
	created   *domain.Debt
	createErr error
	getResult *domain.Debt
	getErr    error
	updated   *domain.Debt
	updateErr error
	deleted   uuid.UUID
	deleteErr error
	listArg   string
}

func (f *fakeDebtRepo) List(ctx context.Context, page, limit int, status string) ([]*domain.Debt, int64, error) {
	f.listArg = status
	return nil, 0, nil
}

func (f *fakeDebtRepo) Get(ctx context.Context, id uuid.UUID) (*domain.Debt, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getResult, nil
}

func (f *fakeDebtRepo) Create(ctx context.Context, d *domain.Debt) error {
	f.created = d
	return f.createErr
}

func (f *fakeDebtRepo) Update(ctx context.Context, d *domain.Debt) error {
	f.updated = d
	return f.updateErr
}

func (f *fakeDebtRepo) Delete(ctx context.Context, id uuid.UUID) error {
	f.deleted = id
	return f.deleteErr
}

func strPtr(s string) *string { return &s }

func TestDebtCreate_RejectsEmptyCustomerName(t *testing.T) {
	repo := &fakeDebtRepo{}
	uc := NewDebtUsecase(repo)

	_, err := uc.Create(context.Background(), &domain.CreateDebtRequest{TotalDebt: 1000})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
	if repo.created != nil {
		t.Fatal("repository must not be called when validation fails")
	}
}

func TestDebtCreate_RejectsNegativeAmounts(t *testing.T) {
	for _, req := range []*domain.CreateDebtRequest{
		{CustomerName: "Budi", TotalDebt: -1},
		{CustomerName: "Budi", TotalDebt: 1000, PaidAmount: -1},
	} {
		uc := NewDebtUsecase(&fakeDebtRepo{})
		if _, err := uc.Create(context.Background(), req); !errors.Is(err, domain.ErrBadRequest) {
			t.Fatalf("expected ErrBadRequest for %+v, got %v", req, err)
		}
	}
}

// The client cannot open a debt already marked paid: status is derived.
func TestDebtCreate_DerivesStatus(t *testing.T) {
	cases := []struct {
		name   string
		total  int64
		paid   int64
		expect string
	}{
		{"unpaid", 50000, 0, domain.DebtStatusActive},
		{"partial", 50000, 10000, domain.DebtStatusPartial},
		{"settled on creation", 50000, 50000, domain.DebtStatusPaid},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeDebtRepo{}
			uc := NewDebtUsecase(repo)

			d, err := uc.Create(context.Background(), &domain.CreateDebtRequest{
				CustomerName: "Budi",
				TotalDebt:    tc.total,
				PaidAmount:   tc.paid,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d.Status != tc.expect {
				t.Fatalf("expected status %q, got %q", tc.expect, d.Status)
			}
		})
	}
}

func TestDebtPatch_AllowsBadDebtWriteOff(t *testing.T) {
	existing := &domain.Debt{
		ID:           uuid.New(),
		CustomerName: "Budi",
		TotalDebt:    100000,
		PaidAmount:   0,
		Status:       domain.DebtStatusActive,
	}
	repo := &fakeDebtRepo{getResult: existing}
	uc := NewDebtUsecase(repo)

	got, err := uc.Patch(context.Background(), existing.ID, &domain.PatchDebtRequest{
		Status: strPtr(domain.DebtStatusBadDebt),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != domain.DebtStatusBadDebt {
		t.Fatalf("expected bad_debt, got %q", got.Status)
	}
}

func TestDebtPatch_RejectsInvalidStatus(t *testing.T) {
	existing := &domain.Debt{ID: uuid.New(), CustomerName: "Budi", Status: domain.DebtStatusActive}
	repo := &fakeDebtRepo{getResult: existing}
	uc := NewDebtUsecase(repo)

	_, err := uc.Patch(context.Background(), existing.ID, &domain.PatchDebtRequest{
		Status: strPtr("cancelled"),
	})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
	if repo.updated != nil {
		t.Fatal("repository must not be updated when validation fails")
	}
}

// Changing an amount without an explicit status must re-derive it, otherwise
// status silently drifts from paid_amount.
func TestDebtPatch_RederivesStatusWhenAmountsChange(t *testing.T) {
	existing := &domain.Debt{
		ID:           uuid.New(),
		CustomerName: "Budi",
		TotalDebt:    100000,
		PaidAmount:   0,
		Status:       domain.DebtStatusActive,
	}
	repo := &fakeDebtRepo{getResult: existing}
	uc := NewDebtUsecase(repo)

	got, err := uc.Patch(context.Background(), existing.ID, &domain.PatchDebtRequest{
		PaidAmount: int64Ptr(100000),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != domain.DebtStatusPaid {
		t.Fatalf("expected paid after full payment, got %q", got.Status)
	}
}

// An untouched status must survive a PATCH that only changes the note.
func TestDebtPatch_KeepsStatusWhenNothingRelevantChanges(t *testing.T) {
	existing := &domain.Debt{
		ID:           uuid.New(),
		CustomerName: "Budi",
		TotalDebt:    100000,
		PaidAmount:   0,
		Status:       domain.DebtStatusBadDebt,
	}
	repo := &fakeDebtRepo{getResult: existing}
	uc := NewDebtUsecase(repo)

	got, err := uc.Patch(context.Background(), existing.ID, &domain.PatchDebtRequest{
		Note: strPtr("written off"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != domain.DebtStatusBadDebt {
		t.Fatalf("expected bad_debt preserved, got %q", got.Status)
	}
}

func TestDebtList_RejectsUnknownStatusFilter(t *testing.T) {
	uc := NewDebtUsecase(&fakeDebtRepo{})

	_, err := uc.List(context.Background(), 1, 20, "nonsense")
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestDebtList_PassesStatusFilter(t *testing.T) {
	repo := &fakeDebtRepo{}
	uc := NewDebtUsecase(repo)

	if _, err := uc.List(context.Background(), 1, 20, domain.DebtStatusPartial); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.listArg != domain.DebtStatusPartial {
		t.Fatalf("expected filter %q, got %q", domain.DebtStatusPartial, repo.listArg)
	}
}
