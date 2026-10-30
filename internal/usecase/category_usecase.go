package usecase

import (
	"context"
	"fmt"

	"jaga-warung-go/internal/domain"
)

// CategoryUsecase implements domain.CategoryUseCase. The delivery layer calls
// it after parsing the request; validation lives here. No echo dependency.
type CategoryUsecase struct {
	repo domain.CategoryRepository
}

func NewCategoryUsecase(repo domain.CategoryRepository) domain.CategoryUseCase {
	return &CategoryUsecase{repo: repo}
}

func (uc *CategoryUsecase) List(ctx context.Context) ([]*domain.Category, error) {
	return uc.repo.List(ctx)
}

func (uc *CategoryUsecase) Create(ctx context.Context, name string) (*domain.Category, error) {
	if name == "" {
		return nil, domain.ErrBadRequest
	}
	p := &domain.Category{Name: name}
	if err := uc.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (uc *CategoryUsecase) Get(ctx context.Context, id int64) (*domain.Category, error) {
	c, err := uc.repo.Get(ctx, id)
	if err != nil {
		if err == domain.ErrNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("category get: %w", err)
	}
	return c, nil
}

func (uc *CategoryUsecase) Patch(ctx context.Context, id int64, name string) (*domain.Category, error) {
	if name == "" {
		return nil, domain.ErrBadRequest
	}
	cur, err := uc.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("category patch get: %w", err)
	}
	cur.Name = name
	if err := uc.repo.Update(ctx, cur); err != nil {
		return nil, fmt.Errorf("category patch update: %w", err)
	}
	return cur, nil
}

func (uc *CategoryUsecase) Delete(ctx context.Context, id int64) error {
	return uc.repo.Delete(ctx, id)
}
