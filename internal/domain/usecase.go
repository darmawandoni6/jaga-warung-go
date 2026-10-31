package domain

import (
	"context"

	"github.com/google/uuid"
)

// AuthUseCase is the port for authentication flows. Delivery/http calls it;
// the implementation in usecase/auth_usecase.go drives the repository.
//
// Keep the shape tight: login returns a token pair, refresh returns a new pair
// (and rotates the old refresh token), logout revokes.
type AuthUseCase interface {
	Login(ctx context.Context, phone, password string) (*AuthTokens, error)
	Refresh(ctx context.Context, rawToken string) (*AuthTokens, error)
	Logout(ctx context.Context, rawToken string) error
}

// AuthTokens is the shape the delivery layer serializes into the /auth/login
// and /auth/refresh response envelope.
type AuthTokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
}

// ProductUseCase is the port for product CRUD. The handler talks to
// the repository through this port; product_usecase.go implements it
// after validation.
type ProductUseCase interface {
	List(ctx context.Context, page, limit int, search string) (*ProductListResult, error)
	Create(ctx context.Context, req *CreateProductRequest) (*Product, error)
	Get(ctx context.Context, barcode string) (*Product, error)
	Patch(ctx context.Context, barcode string, req *PatchProductRequest) (*Product, error)
	Delete(ctx context.Context, barcode string) error
	LowStock(ctx context.Context) ([]*Product, int64, error)
}

// CategoryUseCase is the port for category CRUD. category_usecase.go
// implements it; the delivery layer talks only to this port.
type CategoryUseCase interface {
	List(ctx context.Context) ([]*Category, error)
	Create(ctx context.Context, name string) (*Category, error)
	Get(ctx context.Context, id int64) (*Category, error)
	Patch(ctx context.Context, id int64, name string) (*Category, error)
	Delete(ctx context.Context, id int64) error
}

type ProductListResult struct {
	Data []*Product
	Meta ProductListMeta
}

type ProductListMeta struct {
	Page  int
	Limit int
	Total int64
}

type CreateProductRequest struct {
	Barcode   string
	Name      string
	BuyPrice  int64
	SellPrice int64
	Stock     int64
	MinStock  int64
	Type      *string
	Image     *string
}

type PatchProductRequest struct {
	Name      *string
	BuyPrice  *int64
	SellPrice *int64
	Stock     *int64
	MinStock  *int64
	Type      *string
	Image     *string
}

// TransactionUseCase is the port for /transactions. Create is the sale flow
// (docs/api.md § Business endpoints), not a plain insert: it validates stock,
// writes items and movements, and rebuilds products.stock.
type TransactionUseCase interface {
	List(ctx context.Context, page, limit int) (*TransactionListResult, error)
	Create(ctx context.Context, req *CreateTransactionRequest) (*Transaction, error)
	Get(ctx context.Context, id uuid.UUID) (*Transaction, error)
	Patch(ctx context.Context, id uuid.UUID, req *PatchTransactionRequest) (*Transaction, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// TransactionItemUseCase is the port for /transaction-items. List filters by
// transaction_id so a client can load the items of one sale without preload.
type TransactionItemUseCase interface {
	List(ctx context.Context, page, limit int, transactionID *uuid.UUID) (*TransactionItemListResult, error)
	Create(ctx context.Context, req *CreateTransactionItemRequest) (*TransactionItem, error)
	Get(ctx context.Context, id uuid.UUID) (*TransactionItem, error)
	Patch(ctx context.Context, id uuid.UUID, req *PatchTransactionItemRequest) (*TransactionItem, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type TransactionListResult struct {
	Data []*Transaction
	Meta ProductListMeta
}

type TransactionItemListResult struct {
	Data []*TransactionItem
	Meta ProductListMeta
}

// CreateTransactionItemRequest is one line of a sale. Subtotal is optional: when
// nil the usecase computes sell_price * quantity, so a client cannot send a
// subtotal that disagrees with the line. TransactionID is required by
// POST /transaction-items and ignored when the item is created as part of a sale
// (the parent transaction sets it).
type CreateTransactionItemRequest struct {
	TransactionID  uuid.UUID
	ProductBarcode string
	ProductName    string
	SellPrice      int64
	Quantity       int64
	Subtotal       *int64
}

type CreateTransactionRequest struct {
	TotalAmount   int64
	PaymentAmount int64
	ChangeAmount  int64
	Note          *string
	Items         []CreateTransactionItemRequest
}

// PatchTransactionRequest is partial: a nil field keeps the stored value. Items
// are not patchable here — edit them through /transaction-items.
type PatchTransactionRequest struct {
	TotalAmount   *int64
	PaymentAmount *int64
	ChangeAmount  *int64
	Note          *string
}

type PatchTransactionItemRequest struct {
	ProductName *string
	SellPrice   *int64
	Quantity    *int64
	Subtotal    *int64
}
