package domain

import "time"

// Single-tenant server: no owner_id, no seq. Sync is deferred, so nothing here
// carries a cursor or a tenant column. Login proves "the one account", not
// "which account's rows".
//
// Every data table soft-deletes. Facts (transactions, items, stock movements,
// debt payments) are editable now, so they need deleted_at too.
type SoftDelete struct {
	DeletedAt *time.Time `gorm:"index" json:"deleted_at"`
}

// Errors returned by repositories to the usecase layer. Mapped to HTTP status
// at the delivery boundary (handler → 404/409, etc.).
var (
	ErrNotFound   = &DomainError{code: "not_found", http: 404}
	ErrConflict   = &DomainError{code: "conflict", http: 409}
	ErrBadRequest = &DomainError{code: "bad_request", http: 400}

	// ErrInsufficientStock is a 409 like ErrConflict but a distinct code so the
	// delivery layer can say "insufficient stock" instead of "conflict".
	ErrInsufficientStock = &DomainError{code: "insufficient_stock", http: 409}
)

// DomainError carries a stable machine-readable code and the HTTP status the
// delivery layer should emit. Do not add gorm-specific errors here.
type DomainError struct {
	code string
	http int
}

func (e *DomainError) Error() string   { return e.code }
func (e *DomainError) HTTPStatus() int { return e.http }

// NewDomainError constructs a domain error with the given code and HTTP status.
func NewDomainError(code string, httpStatus int) *DomainError {
	return &DomainError{code: code, http: httpStatus}
}
