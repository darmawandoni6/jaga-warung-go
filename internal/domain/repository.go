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

// CategoryRepository persists categories. The handler talks through this
// port; repository/category_repository.go implements it against GORM/Postgres.
type CategoryRepository interface {
	SeedDefaultCategories(ctx context.Context) error
	Create(ctx context.Context, c *Category) error
	List(ctx context.Context) ([]*Category, error)
	Get(ctx context.Context, id int64) (*Category, error)
	Update(ctx context.Context, c *Category) error
	Delete(ctx context.Context, id int64) error
}

// TransactionRepository persists sales. Create is the transactional sale: the
// transaction, its items, the derived sale stock_movements and the rebuilt
// products.stock all land in one DB transaction or none of them do. Update and
// Delete touch only the header row; item changes go through
// TransactionItemRepository, which owns the stock side effects.
type TransactionRepository interface {
	List(ctx context.Context, page, limit int) ([]*Transaction, int64, error)
	Get(ctx context.Context, id uuid.UUID) (*Transaction, error)
	Create(ctx context.Context, t *Transaction, items []*TransactionItem) error
	Update(ctx context.Context, t *Transaction) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// TransactionItemRepository persists line items and owns the stock side effects
// of editing them: every write regenerates the product's sale movements and
// rebuilds products.stock from the movement sum.
type TransactionItemRepository interface {
	List(ctx context.Context, page, limit int, transactionID *uuid.UUID) ([]*TransactionItem, int64, error)
	Get(ctx context.Context, id uuid.UUID) (*TransactionItem, error)
	Create(ctx context.Context, item *TransactionItem) error
	Update(ctx context.Context, item *TransactionItem) error
	Delete(ctx context.Context, id uuid.UUID) error
}
