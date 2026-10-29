package model

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

// Account is the single pre-provisioned login, seeded from env at boot.
// No registration endpoint exists.
type Account struct {
	ID           string `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Phone        string `gorm:"not null;uniqueIndex;type:varchar(32)" json:"phone"`
	PasswordHash string `gorm:"not null" json:"-"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamptz" json:"updated_at"`
}

// RefreshToken backs /auth/logout: revoked_at is set on logout and checked on
// refresh. Without it a stolen refresh token stays valid for its full TTL.
type RefreshToken struct {
	ID        string     `gorm:"primaryKey;type:varchar(36)" json:"id"`
	AccountID string     `gorm:"not null;index;type:varchar(36)" json:"account_id"`
	TokenHash string     `gorm:"not null;uniqueIndex" json:"-"`
	ExpiresAt time.Time  `gorm:"not null;type:timestamptz" json:"expires_at"`
	RevokedAt *time.Time `gorm:"type:timestamptz" json:"revoked_at"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`
}

// Product mirrors products. Stock is a cache: the truth is the sum of
// stock_movements deltas (B6). Only internal/service writes it, and every edit
// that touches a movement rebuilds it.
type Product struct {
	Barcode   string  `gorm:"primaryKey;type:varchar(100)" json:"barcode"`
	Name      string  `gorm:"not null" json:"name"`
	BuyPrice  int64   `gorm:"not null;default:0" json:"buy_price"`
	SellPrice int64   `gorm:"not null;default:0" json:"sell_price"`
	Stock     int64   `gorm:"not null;default:0" json:"stock"`
	MinStock  int64   `gorm:"not null;default:0" json:"min_stock"`
	Type      *string `json:"type"`
	Image     *string `json:"image"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamptz" json:"updated_at"`

	SoftDelete `gorm:"embedded"`

	// Has-many for Preload and the FK constraint. Deleting a product with
	// movements would erase the sum stock is derived from, so RESTRICT (B6).
	Movements []StockMovement `gorm:"foreignKey:ProductBarcode;references:Barcode;constraint:OnDelete:RESTRICT" json:"-"`
}

// Category mirrors categories. Name is unique on lower(name) — explicit SQL,
// see database.Migrate (D6).
type Category struct {
	ID   string `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Name string `gorm:"not null;type:text" json:"name"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamptz" json:"updated_at"`

	SoftDelete `gorm:"embedded"`
}

// Transaction mirrors transactions. Editable, so it carries deleted_at; no
// updated_at, matching schema.go and the client.
type Transaction struct {
	ID            string  `gorm:"primaryKey;type:varchar(36)" json:"id"`
	TotalAmount   int64   `gorm:"not null" json:"total_amount"`
	PaymentAmount int64   `gorm:"not null" json:"payment_amount"`
	ChangeAmount  int64   `gorm:"not null;default:0" json:"change_amount"`
	Note          *string `json:"note"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`

	SoftDelete `gorm:"embedded"`

	// Has-many for Preload. GET /transactions does not preload it by default;
	// use GET /transaction-items?transaction_id=.
	Items []TransactionItem `gorm:"foreignKey:TransactionID;constraint:OnDelete:CASCADE" json:"-"`
}

// TransactionItem mirrors transaction_items.
type TransactionItem struct {
	ID             string `gorm:"primaryKey;type:varchar(36)" json:"id"`
	TransactionID  string `gorm:"not null;index;type:varchar(36)" json:"transaction_id"`
	ProductBarcode string `gorm:"not null;index;type:varchar(100)" json:"product_barcode"`
	ProductName    string `gorm:"not null" json:"product_name"`
	SellPrice      int64  `gorm:"not null" json:"sell_price"`
	Quantity       int64  `gorm:"not null" json:"quantity"`
	Subtotal       int64  `gorm:"not null" json:"subtotal"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`

	SoftDelete `gorm:"embedded"`

	// Belongs-to so the FK on product_barcode is declared here: a product with
	// sales cannot vanish (RESTRICT).
	Product Product `gorm:"foreignKey:ProductBarcode;references:Barcode;constraint:OnDelete:RESTRICT" json:"-"`
}

// Debt mirrors debts. Status is derived by service.PayDebt, never trusted from
// a client write.
type Debt struct {
	ID           string  `gorm:"primaryKey;type:varchar(36)" json:"id"`
	CustomerName string  `gorm:"not null" json:"customer_name"`
	Phone        *string `json:"phone"`
	TotalDebt    int64   `gorm:"not null;default:0" json:"total_debt"`
	PaidAmount   int64   `gorm:"not null;default:0" json:"paid_amount"`
	Status       string  `gorm:"not null;default:active;type:varchar(20);check:chk_debt_status,status IN ('active','partial','paid','bad_debt')" json:"status"`
	Note         *string `json:"note"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamptz" json:"updated_at"`

	SoftDelete `gorm:"embedded"`

	// Has-many for Preload and the FK constraint; payments belong to the debt.
	Payments []DebtPayment `gorm:"foreignKey:DebtID;constraint:OnDelete:CASCADE" json:"-"`
}

// DebtPayment mirrors debt_payments.
type DebtPayment struct {
	ID     string  `gorm:"primaryKey;type:varchar(36)" json:"id"`
	DebtID string  `gorm:"not null;index;type:varchar(36)" json:"debt_id"`
	Amount int64   `gorm:"not null" json:"amount"`
	Note   *string `json:"note"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`

	SoftDelete `gorm:"embedded"`
}

// CashFlow mirrors cash_flows. Date stays text (YYYY-MM-DD) as the client
// stores it (D10). updated_at is server-added: the client has none.
type CashFlow struct {
	ID     string `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Type   string `gorm:"not null;type:text;check:chk_cash_flows_type,type IN ('income','expense')" json:"type"`
	Amount int64  `gorm:"not null" json:"amount"`
	Note   string `gorm:"not null;default:'';type:text" json:"note"`
	Date   string `gorm:"not null;type:text" json:"date"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamptz" json:"updated_at"`

	SoftDelete `gorm:"embedded"`
}

// StockMovement mirrors stock_movements and is the ONLY source of truth for
// stock (B6). Quantity is always positive; the sign comes from Type.
// PreviousStock/FinalStock are advisory audit fields, never read as current
// stock.
type StockMovement struct {
	ID             string  `gorm:"primaryKey;type:varchar(36)" json:"id"`
	ProductBarcode string  `gorm:"not null;index;type:varchar(100)" json:"product_barcode"`
	Type           string  `gorm:"not null;type:text;check:chk_stock_movements_type,type IN ('restock','sale','adjustment_loss','adjustment_gain')" json:"type"`
	Quantity       int64   `gorm:"not null" json:"quantity"`
	PreviousStock  int64   `gorm:"not null" json:"previous_stock"`
	FinalStock     int64   `gorm:"not null" json:"final_stock"`
	TotalCost      int64   `gorm:"default:0" json:"total_cost"`
	Note           *string `gorm:"type:text" json:"note"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`

	SoftDelete `gorm:"embedded"`
}

// AllModels is the migration list, ordered so parents precede children.
func AllModels() []any {
	return []any{
		&Account{},
		&RefreshToken{},
		&Product{},
		&Category{},
		&Transaction{},
		&TransactionItem{},
		&Debt{},
		&DebtPayment{},
		&CashFlow{},
		&StockMovement{},
	}
}
