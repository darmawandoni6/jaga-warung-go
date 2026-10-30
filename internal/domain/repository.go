package domain

import (
	"context"

	"github.com/google/uuid"
)

// AccountRepository persists the single tenant account. AuthUseCase is the
// only caller.
type AccountRepository interface {
	FindByPhone(ctx context.Context, phone string) (*Account, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Account, error)
	Create(ctx context.Context, acc *Account) error
}

// RefreshTokenRepository persists auth sessions. AuthUseCase calls it to
// rotate on refresh and revoke on logout.
type RefreshTokenRepository interface {
	Create(ctx context.Context, rt *RefreshToken) error
	FindByHash(ctx context.Context, tokenHash string) (*RefreshToken, error)
	Revoke(ctx context.Context, hash string) error
	ListByAccount(ctx context.Context, accountID string) ([]*RefreshToken, error)
}

// ProductRepository is the read/write port for /products. The handler only talks
// through this port; the repository implements it against GORM/Postgres.
type ProductRepository interface {
	List(ctx context.Context, page, limit int, search string) ([]*Product, int64, error)
	Get(ctx context.Context, barcode string) (*Product, error)
	Create(ctx context.Context, p *Product) error
	Update(ctx context.Context, p *Product) error
	Delete(ctx context.Context, barcode string) error
	CountBelow(ctx context.Context, minStock int64) ([]*Product, int64, error)
	All(ctx context.Context) []*Product // for migration seed, not a business query
}

// CategoryRepository persists categories. No handler calls it yet — exposed so
// the usecase layer (Phase 5) can consume it without depending on infrastructure.
type CategoryRepository interface {
	SeedDefaultCategories(ctx context.Context) error
	Create(ctx context.Context, c *Category) error
	FindAll(ctx context.Context) ([]*Category, error)
	FindByID(ctx context.Context, id int64) (*Category, error)
}
