package domain

import "context"

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
