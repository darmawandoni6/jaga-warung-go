package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"jaga-warung-go/internal/domain"
)

type fakeProductRepo struct {
	created    *domain.Product
	getResult  *domain.Product
	getErr     error
	updated    *domain.Product
	updateErr  error
	listCat    *int64
	listAll    bool
	restored   string
	rebuilt    uuid.UUID
	rebuildErr error
}

func (f *fakeProductRepo) List(ctx context.Context, page, limit int, search string, categoryID *int64, includeInactive bool) ([]*domain.Product, int64, error) {
	f.listCat = categoryID
	f.listAll = includeInactive
	return nil, 0, nil
}

func (f *fakeProductRepo) Get(ctx context.Context, barcode string) (*domain.Product, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getResult, nil
}

func (f *fakeProductRepo) Create(ctx context.Context, p *domain.Product) error {
	f.created = p
	return nil
}

func (f *fakeProductRepo) Update(ctx context.Context, p *domain.Product) error {
	f.updated = p
	return f.updateErr
}

func (f *fakeProductRepo) Delete(ctx context.Context, barcode string) error { return nil }

func (f *fakeProductRepo) Restore(ctx context.Context, barcode string) (*domain.Product, error) {
	f.restored = barcode
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getResult, nil
}

func (f *fakeProductRepo) CountBelow(ctx context.Context, minStock int64) ([]*domain.Product, int64, error) {
	return nil, 0, nil
}

func (f *fakeProductRepo) All(ctx context.Context) []*domain.Product { return nil }

func (f *fakeProductRepo) RebuildStockForTransaction(ctx context.Context, transactionID uuid.UUID) error {
	f.rebuilt = transactionID
	return f.rebuildErr
}

type fakeCategoryRepo struct {
	domain.CategoryRepository
	result *domain.Category
	err    error
}

func (f *fakeCategoryRepo) Get(ctx context.Context, id int64) (*domain.Category, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func TestProductCreate_RejectsMissingCategory(t *testing.T) {
	repo := &fakeProductRepo{}
	uc := NewProductUsecase(repo, &fakeCategoryRepo{})

	_, err := uc.Create(context.Background(), &domain.CreateProductRequest{Barcode: "899"})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
	if repo.created != nil {
		t.Fatal("repository must not be called when category_id is absent")
	}
}

func TestProductCreate_RejectsUnknownCategory(t *testing.T) {
	repo := &fakeProductRepo{}
	uc := NewProductUsecase(repo, &fakeCategoryRepo{err: domain.ErrNotFound})

	_, err := uc.Create(context.Background(), &domain.CreateProductRequest{
		Barcode:    "899",
		CategoryID: int64Ptr(99),
	})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
	if repo.created != nil {
		t.Fatal("repository must not be called when validation fails")
	}
}

func TestProductCreate_StoresValidCategory(t *testing.T) {
	repo := &fakeProductRepo{}
	uc := NewProductUsecase(repo, &fakeCategoryRepo{result: &domain.Category{ID: 7, Name: "Minuman"}})

	p, err := uc.Create(context.Background(), &domain.CreateProductRequest{
		Barcode:    "899",
		CategoryID: int64Ptr(7),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.CategoryID == nil || *p.CategoryID != 7 {
		t.Fatalf("expected category_id 7, got %v", p.CategoryID)
	}
	if p.Category == nil || p.Category.ID != 7 {
		t.Fatalf("expected response to carry the fetched category, got %v", p.Category)
	}
}

func TestProductPatch_RejectsUnknownCategory(t *testing.T) {
	repo := &fakeProductRepo{getResult: &domain.Product{Barcode: "899"}}
	uc := NewProductUsecase(repo, &fakeCategoryRepo{err: domain.ErrNotFound})

	_, err := uc.Patch(context.Background(), "899", &domain.PatchProductRequest{CategoryID: int64Ptr(99)})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
	if repo.updated != nil {
		t.Fatal("repository must not be updated when validation fails")
	}
}

func TestProductPatch_MovesProductBetweenCategories(t *testing.T) {
	repo := &fakeProductRepo{getResult: &domain.Product{Barcode: "899"}}
	uc := NewProductUsecase(repo, &fakeCategoryRepo{result: &domain.Category{ID: 7}})

	got, err := uc.Patch(context.Background(), "899", &domain.PatchProductRequest{CategoryID: int64Ptr(7)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CategoryID == nil || *got.CategoryID != 7 {
		t.Fatalf("expected category_id 7, got %v", got.CategoryID)
	}
	// The embedded category must be reloaded, not left at the pre-patch value.
	if got.Category == nil || got.Category.ID != 7 {
		t.Fatalf("expected refreshed category, got %v", got.Category)
	}
	if repo.updated == nil {
		t.Fatal("expected repository update")
	}
}

func TestProductList_PassesCategoryFilter(t *testing.T) {
	repo := &fakeProductRepo{}
	uc := NewProductUsecase(repo, &fakeCategoryRepo{})

	if _, err := uc.List(context.Background(), 1, 20, "", int64Ptr(7), false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.listCat == nil || *repo.listCat != 7 {
		t.Fatalf("expected category filter 7, got %v", repo.listCat)
	}
	if repo.listAll {
		t.Fatal("inactive rows must be excluded by default")
	}
}

func TestProductList_ForwardsIncludeInactive(t *testing.T) {
	repo := &fakeProductRepo{}
	uc := NewProductUsecase(repo, &fakeCategoryRepo{})

	if _, err := uc.List(context.Background(), 1, 20, "", nil, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.listAll {
		t.Fatal("expected include_inactive to reach the repository")
	}
}

func TestProductRestore_Delegates(t *testing.T) {
	repo := &fakeProductRepo{getResult: &domain.Product{Barcode: "899"}}
	uc := NewProductUsecase(repo, &fakeCategoryRepo{})

	p, err := uc.Restore(context.Background(), "899")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Barcode != "899" || repo.restored != "899" {
		t.Fatalf("expected restore of 899, got %v / %q", p, repo.restored)
	}
}

func TestProductRestore_MissingProduct(t *testing.T) {
	repo := &fakeProductRepo{getErr: domain.ErrNotFound}
	uc := NewProductUsecase(repo, &fakeCategoryRepo{})

	if _, err := uc.Restore(context.Background(), "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
