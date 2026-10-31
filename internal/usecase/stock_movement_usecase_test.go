package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"jaga-warung-go/internal/domain"
)

type fakeStockMovementRepo struct {
	created   *domain.StockMovement
	createErr error
	getResult *domain.StockMovement
	getErr    error
	updated   *domain.StockMovement
	updateErr error
	deleted   uuid.UUID
	deleteErr error
	listArg   string
}

func (f *fakeStockMovementRepo) List(ctx context.Context, page, limit int, productBarcode string) ([]*domain.StockMovement, int64, error) {
	f.listArg = productBarcode
	return nil, 0, nil
}

func (f *fakeStockMovementRepo) Get(ctx context.Context, id uuid.UUID) (*domain.StockMovement, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getResult, nil
}

func (f *fakeStockMovementRepo) Create(ctx context.Context, m *domain.StockMovement) error {
	f.created = m
	return f.createErr
}

func (f *fakeStockMovementRepo) Update(ctx context.Context, m *domain.StockMovement) error {
	f.updated = m
	return f.updateErr
}

func (f *fakeStockMovementRepo) Delete(ctx context.Context, id uuid.UUID) error {
	f.deleted = id
	return f.deleteErr
}

func TestStockMovementCreate_RejectsMissingBarcode(t *testing.T) {
	repo := &fakeStockMovementRepo{}
	uc := NewStockMovementUsecase(repo)

	_, err := uc.Create(context.Background(), &domain.CreateStockMovementRequest{
		Type: domain.MovementRestock, Quantity: 10,
	})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
	if repo.created != nil {
		t.Fatal("repository must not be called when validation fails")
	}
}

func TestStockMovementCreate_RejectsUnknownType(t *testing.T) {
	uc := NewStockMovementUsecase(&fakeStockMovementRepo{})

	_, err := uc.Create(context.Background(), &domain.CreateStockMovementRequest{
		ProductBarcode: "899",
		Type:           "magic",
		Quantity:       10,
	})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestStockMovementCreate_RejectsNonPositiveQuantity(t *testing.T) {
	uc := NewStockMovementUsecase(&fakeStockMovementRepo{})

	_, err := uc.Create(context.Background(), &domain.CreateStockMovementRequest{
		ProductBarcode: "899",
		Type:           domain.MovementRestock,
		Quantity:       0,
	})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

// Quantity is always positive; the sign comes from the type. A "loss of 5" is
// type=adjustment_loss, quantity=5 — never a negative quantity.
func TestStockMovementCreate_AcceptsAllValidTypes(t *testing.T) {
	for _, movementType := range []string{
		domain.MovementRestock,
		domain.MovementSale,
		domain.MovementAdjustmentLoss,
		domain.MovementAdjustmentGain,
	} {
		repo := &fakeStockMovementRepo{}
		uc := NewStockMovementUsecase(repo)

		m, err := uc.Create(context.Background(), &domain.CreateStockMovementRequest{
			ProductBarcode: "899",
			Type:           movementType,
			Quantity:       5,
		})
		if err != nil {
			t.Fatalf("type %q: unexpected error %v", movementType, err)
		}
		if repo.created.Type != movementType || repo.created.Quantity != 5 {
			t.Fatalf("type %q: repo got %+v", movementType, repo.created)
		}
		if m == nil {
			t.Fatal("created movement must be returned")
		}
	}
}

func TestStockMovementCreate_PropagatesInsufficientStock(t *testing.T) {
	repo := &fakeStockMovementRepo{createErr: domain.ErrInsufficientStock}
	uc := NewStockMovementUsecase(repo)

	_, err := uc.Create(context.Background(), &domain.CreateStockMovementRequest{
		ProductBarcode: "899",
		Type:           domain.MovementAdjustmentLoss,
		Quantity:       100,
	})
	if !errors.Is(err, domain.ErrInsufficientStock) {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}
}

func TestStockMovementPatch_RejectsUnknownType(t *testing.T) {
	existing := &domain.StockMovement{ID: uuid.New(), Type: domain.MovementRestock, Quantity: 10}
	repo := &fakeStockMovementRepo{getResult: existing}
	uc := NewStockMovementUsecase(repo)

	_, err := uc.Patch(context.Background(), existing.ID, &domain.PatchStockMovementRequest{
		Type: strPtr("magic"),
	})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
	if repo.updated != nil {
		t.Fatal("repository must not be updated when validation fails")
	}
}

func TestStockMovementPatch_RejectsNonPositiveQuantity(t *testing.T) {
	existing := &domain.StockMovement{ID: uuid.New(), Type: domain.MovementRestock, Quantity: 10}
	repo := &fakeStockMovementRepo{getResult: existing}
	uc := NewStockMovementUsecase(repo)

	_, err := uc.Patch(context.Background(), existing.ID, &domain.PatchStockMovementRequest{
		Quantity: int64Ptr(-1),
	})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestStockMovementPatch_AppliesOnlyPresentFields(t *testing.T) {
	existing := &domain.StockMovement{
		ID:        uuid.New(),
		Type:      domain.MovementRestock,
		Quantity:  10,
		TotalCost: 12000,
		Note:      strPtr("restock"),
	}
	repo := &fakeStockMovementRepo{getResult: existing}
	uc := NewStockMovementUsecase(repo)

	got, err := uc.Patch(context.Background(), existing.ID, &domain.PatchStockMovementRequest{
		Note: strPtr("restock big"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *got.Note != "restock big" {
		t.Fatalf("expected updated note, got %q", *got.Note)
	}
	if got.Type != domain.MovementRestock || got.Quantity != 10 || got.TotalCost != 12000 {
		t.Fatalf("absent fields were overwritten: %+v", got)
	}
}

func TestStockMovementList_PassesBarcodeFilter(t *testing.T) {
	repo := &fakeStockMovementRepo{}
	uc := NewStockMovementUsecase(repo)

	if _, err := uc.List(context.Background(), 1, 20, "899"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.listArg != "899" {
		t.Fatalf("expected filter %q, got %q", "899", repo.listArg)
	}
}
