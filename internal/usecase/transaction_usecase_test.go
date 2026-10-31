package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"jaga-warung-go/internal/domain"
)

// fakeTransactionRepo records what the usecase handed down so the tests can
// assert on validation without a database.
type fakeTransactionRepo struct {
	created   *domain.Transaction
	createdIt []*domain.TransactionItem
	createErr error
	getResult *domain.Transaction
	getErr    error
	updated   *domain.Transaction
	updateErr error
	deleted   uuid.UUID
	deleteErr error
}

func (f *fakeTransactionRepo) List(ctx context.Context, page, limit int) ([]*domain.Transaction, int64, error) {
	return nil, 0, nil
}

func (f *fakeTransactionRepo) Get(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getResult, nil
}

func (f *fakeTransactionRepo) Create(ctx context.Context, t *domain.Transaction, items []*domain.TransactionItem) error {
	f.created, f.createdIt = t, items
	return f.createErr
}

func (f *fakeTransactionRepo) Update(ctx context.Context, t *domain.Transaction) error {
	f.updated = t
	return f.updateErr
}

func (f *fakeTransactionRepo) Delete(ctx context.Context, id uuid.UUID) error {
	f.deleted = id
	return f.deleteErr
}

func int64Ptr(v int64) *int64 { return &v }

func TestTransactionCreate_RejectsEmptyItems(t *testing.T) {
	repo := &fakeTransactionRepo{}
	uc := NewTransactionUsecase(repo)

	_, err := uc.Create(context.Background(), &domain.CreateTransactionRequest{
		TotalAmount: 1000, PaymentAmount: 1000,
	})

	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
	if repo.created != nil {
		t.Fatal("repository must not be called when validation fails")
	}
}

func TestTransactionCreate_RejectsUnderpayment(t *testing.T) {
	repo := &fakeTransactionRepo{}
	uc := NewTransactionUsecase(repo)

	_, err := uc.Create(context.Background(), &domain.CreateTransactionRequest{
		TotalAmount:   5000,
		PaymentAmount: 4000,
		Items: []domain.CreateTransactionItemRequest{
			{ProductBarcode: "A", SellPrice: 5000, Quantity: 1},
		},
	})

	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest for payment < total, got %v", err)
	}
}

func TestTransactionCreate_RejectsNonPositiveQuantity(t *testing.T) {
	repo := &fakeTransactionRepo{}
	uc := NewTransactionUsecase(repo)

	_, err := uc.Create(context.Background(), &domain.CreateTransactionRequest{
		TotalAmount: 0, PaymentAmount: 0,
		Items: []domain.CreateTransactionItemRequest{
			{ProductBarcode: "A", SellPrice: 1000, Quantity: 0},
		},
	})

	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest for quantity 0, got %v", err)
	}
}

// A line without an explicit subtotal must derive price × quantity, so a client
// cannot persist a subtotal that disagrees with the line.
func TestTransactionCreate_DerivesSubtotal(t *testing.T) {
	repo := &fakeTransactionRepo{}
	uc := NewTransactionUsecase(repo)

	_, err := uc.Create(context.Background(), &domain.CreateTransactionRequest{
		TotalAmount: 4000, PaymentAmount: 5000, ChangeAmount: 1000,
		Items: []domain.CreateTransactionItemRequest{
			{ProductBarcode: "899", ProductName: "Kopi", SellPrice: 2000, Quantity: 2},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.createdIt) != 1 {
		t.Fatalf("expected 1 item, got %d", len(repo.createdIt))
	}
	if got := repo.createdIt[0].Subtotal; got != 4000 {
		t.Fatalf("expected derived subtotal 4000, got %d", got)
	}
}

// An explicit subtotal is respected, since a client may legitimately apply a
// per-line discount.
func TestTransactionCreate_KeepsExplicitSubtotal(t *testing.T) {
	repo := &fakeTransactionRepo{}
	uc := NewTransactionUsecase(repo)

	_, err := uc.Create(context.Background(), &domain.CreateTransactionRequest{
		TotalAmount: 3500, PaymentAmount: 3500,
		Items: []domain.CreateTransactionItemRequest{
			{ProductBarcode: "899", SellPrice: 2000, Quantity: 2, Subtotal: int64Ptr(3500)},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := repo.createdIt[0].Subtotal; got != 3500 {
		t.Fatalf("expected explicit subtotal 3500, got %d", got)
	}
}

// Insufficient stock comes from the repository; the usecase must not swallow it.
func TestTransactionCreate_PropagatesInsufficientStock(t *testing.T) {
	repo := &fakeTransactionRepo{createErr: domain.ErrInsufficientStock}
	uc := NewTransactionUsecase(repo)

	_, err := uc.Create(context.Background(), &domain.CreateTransactionRequest{
		TotalAmount: 1000, PaymentAmount: 1000,
		Items: []domain.CreateTransactionItemRequest{
			{ProductBarcode: "A", SellPrice: 1000, Quantity: 1},
		},
	})

	if !errors.Is(err, domain.ErrInsufficientStock) {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}
}

func TestTransactionPatch_AppliesOnlyPresentFields(t *testing.T) {
	existing := &domain.Transaction{
		ID:            uuid.New(),
		TotalAmount:   1000,
		PaymentAmount: 2000,
		ChangeAmount:  1000,
	}
	repo := &fakeTransactionRepo{getResult: existing}
	uc := NewTransactionUsecase(repo)

	got, err := uc.Patch(context.Background(), existing.ID, &domain.PatchTransactionRequest{
		TotalAmount: int64Ptr(5000),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.TotalAmount != 5000 {
		t.Fatalf("expected total_amount 5000, got %d", got.TotalAmount)
	}
	// Absent keys must keep their stored value.
	if got.PaymentAmount != 2000 || got.ChangeAmount != 1000 {
		t.Fatalf("absent fields were overwritten: %+v", got)
	}
}

func TestTransactionPatch_MissingRowIsNotFound(t *testing.T) {
	repo := &fakeTransactionRepo{getErr: domain.ErrNotFound}
	uc := NewTransactionUsecase(repo)

	_, err := uc.Patch(context.Background(), uuid.New(), &domain.PatchTransactionRequest{})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTransactionList_ClampsPagination(t *testing.T) {
	cases := []struct {
		name              string
		page, limit       int
		wantPage, wantLim int
	}{
		{"zero values", 0, 0, 1, 20},
		{"negative", -3, -1, 1, 20},
		{"limit too large", 1, 500, 1, 20},
		{"valid", 2, 50, 2, 50},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc := NewTransactionUsecase(&fakeTransactionRepo{})
			res, err := uc.List(context.Background(), tc.page, tc.limit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Meta.Page != tc.wantPage || res.Meta.Limit != tc.wantLim {
				t.Fatalf("got page=%d limit=%d, want page=%d limit=%d",
					res.Meta.Page, res.Meta.Limit, tc.wantPage, tc.wantLim)
			}
		})
	}
}
