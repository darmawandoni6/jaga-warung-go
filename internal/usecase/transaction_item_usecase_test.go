package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"jaga-warung-go/internal/domain"
)

type fakeItemRepo struct {
	created   *domain.TransactionItem
	createErr error
	getResult *domain.TransactionItem
	getErr    error
	updated   *domain.TransactionItem
	updateErr error
	deleted   uuid.UUID
	deleteErr error
}

func (f *fakeItemRepo) List(ctx context.Context, page, limit int, txID *uuid.UUID) ([]*domain.TransactionItem, int64, error) {
	return nil, 0, nil
}

func (f *fakeItemRepo) Get(ctx context.Context, id uuid.UUID) (*domain.TransactionItem, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getResult, nil
}

func (f *fakeItemRepo) Create(ctx context.Context, item *domain.TransactionItem) error {
	f.created = item
	return f.createErr
}

func (f *fakeItemRepo) Update(ctx context.Context, item *domain.TransactionItem) error {
	f.updated = item
	return f.updateErr
}

func (f *fakeItemRepo) Delete(ctx context.Context, id uuid.UUID) error {
	f.deleted = id
	return f.deleteErr
}

func TestTransactionItemCreate_RejectsMissingBarcode(t *testing.T) {
	repo := &fakeItemRepo{}
	uc := NewTransactionItemUsecase(repo)

	_, err := uc.Create(context.Background(), &domain.CreateTransactionItemRequest{
		TransactionID: uuid.New(), Quantity: 1,
	})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
	if repo.created != nil {
		t.Fatal("repository must not be called when validation fails")
	}
}

func TestTransactionItemCreate_RejectsNonPositiveQuantity(t *testing.T) {
	repo := &fakeItemRepo{}
	uc := NewTransactionItemUsecase(repo)

	_, err := uc.Create(context.Background(), &domain.CreateTransactionItemRequest{
		TransactionID: uuid.New(), ProductBarcode: "899", Quantity: -2,
	})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestTransactionItemCreate_DerivesSubtotal(t *testing.T) {
	repo := &fakeItemRepo{}
	uc := NewTransactionItemUsecase(repo)
	txID := uuid.New()

	item, err := uc.Create(context.Background(), &domain.CreateTransactionItemRequest{
		TransactionID:  txID,
		ProductBarcode: "899",
		SellPrice:      1500,
		Quantity:       3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if item.Subtotal != 4500 {
		t.Fatalf("expected subtotal 4500, got %d", item.Subtotal)
	}
	if item.TransactionID != txID {
		t.Fatalf("transaction_id was not passed through: %v", item.TransactionID)
	}
}

func TestTransactionItemPatch_RejectsNonPositiveQuantity(t *testing.T) {
	existing := &domain.TransactionItem{ID: uuid.New(), Quantity: 2}
	repo := &fakeItemRepo{getResult: existing}
	uc := NewTransactionItemUsecase(repo)

	_, err := uc.Patch(context.Background(), existing.ID, &domain.PatchTransactionItemRequest{
		Quantity: int64Ptr(0),
	})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest for quantity 0, got %v", err)
	}
	if repo.updated != nil {
		t.Fatal("repository must not be updated when validation fails")
	}
}

func TestTransactionItemPatch_AppliesOnlyPresentFields(t *testing.T) {
	existing := &domain.TransactionItem{
		ID:          uuid.New(),
		ProductName: "Kopi",
		SellPrice:   2000,
		Quantity:    2,
		Subtotal:    4000,
	}
	repo := &fakeItemRepo{getResult: existing}
	uc := NewTransactionItemUsecase(repo)

	got, err := uc.Patch(context.Background(), existing.ID, &domain.PatchTransactionItemRequest{
		Quantity: int64Ptr(5),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Quantity != 5 {
		t.Fatalf("expected quantity 5, got %d", got.Quantity)
	}
	if got.ProductName != "Kopi" || got.SellPrice != 2000 {
		t.Fatalf("absent fields were overwritten: %+v", got)
	}
}

// Editing a line changes how much was sold, so the repository must be the one
// that rebuilds stock — the usecase must not swallow its error.
func TestTransactionItemPatch_PropagatesInsufficientStock(t *testing.T) {
	existing := &domain.TransactionItem{ID: uuid.New(), Quantity: 2}
	repo := &fakeItemRepo{getResult: existing, updateErr: domain.ErrInsufficientStock}
	uc := NewTransactionItemUsecase(repo)

	_, err := uc.Patch(context.Background(), existing.ID, &domain.PatchTransactionItemRequest{
		Quantity: int64Ptr(100),
	})
	if !errors.Is(err, domain.ErrInsufficientStock) {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}
}
