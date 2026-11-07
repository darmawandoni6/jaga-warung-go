package domain

import (
	"time"

	"github.com/google/uuid"
)

// Account is the single pre-provisioned login, seeded from env at boot.
type Account struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Phone        string    `gorm:"not null;uniqueIndex;type:varchar(32)" json:"phone"`
	PasswordHash string    `gorm:"not null" json:"-"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamptz" json:"updated_at"`
}

// RefreshToken backs /auth/logout
type RefreshToken struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	AccountID uuid.UUID  `gorm:"type:uuid;not null;index" json:"account_id"`
	TokenHash string     `gorm:"not null;uniqueIndex" json:"-"`
	ExpiresAt time.Time  `gorm:"not null;type:timestamptz" json:"expires_at"`
	RevokedAt *time.Time `gorm:"type:timestamptz" json:"revoked_at"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`
}

// Product (ID tetap Barcode bertipe string)
type Product struct {
	Barcode   string  `gorm:"primaryKey;type:varchar(100)" json:"barcode"`
	Name      string  `gorm:"not null" json:"name"`
	BuyPrice  int64   `gorm:"not null;default:0" json:"buy_price"`
	SellPrice int64   `gorm:"not null;default:0" json:"sell_price"`
	Stock     int64   `gorm:"not null;default:0" json:"stock"`
	MinStock  int64   `gorm:"not null;default:0" json:"min_stock"`
	Image     *string `json:"image"`

	// 1 category → many products: the FK lives on products. NULL = uncategorized
	// (existing rows keep working), so the column is a pointer.
	CategoryID *int64 `gorm:"index" json:"category_id"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamptz" json:"updated_at"`

	SoftDelete `gorm:"embedded"`

	Movements []StockMovement `gorm:"foreignKey:ProductBarcode;references:Barcode;constraint:OnDelete:RESTRICT" json:"-"`
	Category  *Category       `gorm:"foreignKey:CategoryID;references:ID;constraint:OnDelete:RESTRICT" json:"category,omitempty"`
}

// Category
type Category struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"name"`
	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamptz" json:"updated_at"`

	SoftDelete `gorm:"embedded"`
}

// Payment methods. Transactions only use cash or non_cash: the actual gateway
// channel (qris, gopay, shopeepay, ...) lives on payments.payment_type, because
// Snap picks it, not us. Debt payments still record qris explicitly.
const (
	PaymentMethodCash    = "cash"
	PaymentMethodNonCash = "non_cash"
	PaymentMethodQRIS    = "qris"
)

// Transaction status values, matching the chk_transactions_status CHECK
// constraint. paid is terminal; expired means the QRIS attempt failed, not that
// the sale was voided (the goods already left and stock is not reverted).
const (
	TransactionStatusPending = "pending"
	TransactionStatusPaid    = "paid"
	TransactionStatusExpired = "expired"
)

// DeriveTransactionStatus is the one place the paid transition is decided at
// create time and on the non_cash→cash switch. It never returns expired: that
// state is written only when a payment attempt fails, like bad_debt on a debt.
func DeriveTransactionStatus(method string, total, paid int64) string {
	if method == PaymentMethodCash || paid >= total {
		return TransactionStatusPaid
	}
	return TransactionStatusPending
}

type Transaction struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TotalAmount   int64      `gorm:"not null" json:"total_amount"`
	PaymentAmount int64      `gorm:"not null" json:"payment_amount"`
	ChangeAmount  int64      `gorm:"not null;default:0" json:"change_amount"`
	Method        string     `gorm:"not null;default:cash;type:varchar(10)" json:"method"`
	Status        string     `gorm:"not null;default:pending;type:varchar(20)" json:"status"`
	PaidAt        *time.Time `gorm:"type:timestamptz" json:"paid_at"`
	Note          *string    `json:"note"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`

	SoftDelete `gorm:"embedded"`

	Items []TransactionItem `gorm:"foreignKey:TransactionID;constraint:OnDelete:CASCADE" json:"items"`
}

// TransactionItem
type TransactionItem struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TransactionID  uuid.UUID `gorm:"type:uuid;not null;index" json:"transaction_id"`
	ProductBarcode string    `gorm:"not null;index;type:varchar(100)" json:"product_barcode"`
	ProductName    string    `gorm:"not null" json:"product_name"`
	SellPrice      int64     `gorm:"not null" json:"sell_price"`
	Quantity       int64     `gorm:"not null" json:"quantity"`
	Subtotal       int64     `gorm:"not null" json:"subtotal"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`

	SoftDelete `gorm:"embedded"`

	Product Product `gorm:"foreignKey:ProductBarcode;references:Barcode;constraint:OnDelete:RESTRICT" json:"-"`
}

// Debt status values, matching the chk_debt_status CHECK constraint.
const (
	DebtStatusActive  = "active"
	DebtStatusPartial = "partial"
	DebtStatusPaid    = "paid"
	DebtStatusBadDebt = "bad_debt"
)

// DeriveDebtStatus is the one place the active/partial/paid transition is
// decided. docs/schema.md: status is derived by PayDebt, never trusted from the
// client. bad_debt is not reachable here — it is a manual write-off, so it only
// arrives through an explicit PATCH.
func DeriveDebtStatus(totalDebt, paidAmount int64) string {
	switch {
	case paidAmount > 0 && paidAmount >= totalDebt:
		return DebtStatusPaid
	case paidAmount > 0:
		return DebtStatusPartial
	default:
		return DebtStatusActive
	}
}

// ValidDebtStatus reports whether s is one of the four allowed values, so a
// PATCH cannot write something the CHECK constraint would reject at the DB.
func ValidDebtStatus(s string) bool {
	switch s {
	case DebtStatusActive, DebtStatusPartial, DebtStatusPaid, DebtStatusBadDebt:
		return true
	default:
		return false
	}
}

// Debt
type Debt struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	CustomerName string    `gorm:"not null" json:"customer_name"`
	Phone        *string   `json:"phone"`
	TotalDebt    int64     `gorm:"not null;default:0" json:"total_debt"`
	PaidAmount   int64     `gorm:"not null;default:0" json:"paid_amount"`
	Status       string    `gorm:"not null;default:active;type:varchar(20);check:chk_debt_status,status IN ('active','partial','paid','bad_debt')" json:"status"`
	Note         *string   `json:"note"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamptz" json:"updated_at"`

	SoftDelete `gorm:"embedded"`

	Payments []DebtPayment `gorm:"foreignKey:DebtID;constraint:OnDelete:CASCADE" json:"-"`
}

// Payment is one Midtrans QRIS attempt against a transaction or a debt.
//
// OrderID is generated by us (uuid.New in CreateSnap) and sent to Midtrans as
// the order id; the Midtrans side never invents it. The Midtrans* fields below
// are filled from the CheckTransaction response, never from the webhook body, so
// a row can be matched against the Midtrans dashboard without guessing:
//
//	transactions.id    → payments.target_id (target_type = 'transaction')
//	debt_payments.id   → payments.debt_payment_id
//	orders in dashboard→ payments.order_id / payments.midtrans_transaction_id
type Payment struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	OrderID       uuid.UUID  `gorm:"type:uuid;uniqueIndex;not null" json:"order_id"`
	TargetType    string     `gorm:"not null;type:varchar(20)" json:"target_type"`
	TargetID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"target_id"`
	DebtPaymentID *uuid.UUID `gorm:"type:uuid;uniqueIndex" json:"debt_payment_id"`
	Amount        int64      `gorm:"not null" json:"amount"`
	Status        string     `gorm:"not null;type:varchar(20)" json:"status"`
	SnapToken     string     `json:"snap_token"`
	RedirectURL   string     `json:"redirect_url"`

	// Proof, straight from CheckTransaction: Midtrans' own transaction id, the
	// channel used, the raw status code, when it settled, and the full response.
	MidtransTransactionID *string    `gorm:"type:varchar(64);index" json:"midtrans_transaction_id"`
	PaymentType           *string    `gorm:"type:varchar(32)" json:"payment_type"`
	StatusCode            *string    `gorm:"type:varchar(8)" json:"status_code"`
	SettledAt             *time.Time `gorm:"type:timestamptz" json:"settled_at"`
	VerifiedAt            *time.Time `gorm:"type:timestamptz" json:"verified_at"`
	RawStatus             []byte     `gorm:"type:jsonb" json:"-"`

	CreatedAt  time.Time `gorm:"type:timestamptz" json:"created_at"`
	UpdatedAt  time.Time `gorm:"type:timestamptz" json:"updated_at"`
	SoftDelete `gorm:"embedded"`
}

type DebtPayment struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	DebtID uuid.UUID `gorm:"type:uuid;not null;index" json:"debt_id"`
	Amount int64     `gorm:"not null" json:"amount"`
	Method string    `gorm:"not null;default:cash;type:varchar(10)" json:"method"`
	Note   *string   `json:"note"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`

	SoftDelete `gorm:"embedded"`
}

// CashFlow
type CashFlow struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Type   string    `gorm:"not null;type:text;check:chk_cash_flows_type,type IN ('income','expense')" json:"type"`
	Amount int64     `gorm:"not null" json:"amount"`
	Note   string    `gorm:"not null;default:'';type:text" json:"note"`
	Date   string    `gorm:"not null;type:text" json:"date"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamptz" json:"updated_at"`

	SoftDelete `gorm:"embedded"`
}

// StockMovementType values. The CHECK constraint in docs/schema.md allows only
// these four; quantity is always stored positive and the sign comes from the
// type (see repository.stockSign).
const (
	PaymentStatusPending   = "pending"
	PaymentStatusPaid      = "paid"
	PaymentStatusExpired   = "expired"
	PaymentStatusCanceled  = "canceled"
	MovementRestock        = "restock"
	MovementSale           = "sale"
	MovementAdjustmentLoss = "adjustment_loss"
	MovementAdjustmentGain = "adjustment_gain"
)

// ValidMovementType reports whether s is one of the four allowed types, so a
// POST/PATCH cannot write something the chk_stock_movements_type CHECK
// constraint would reject at the DB.
func ValidMovementType(s string) bool {
	switch s {
	case MovementRestock, MovementSale, MovementAdjustmentLoss, MovementAdjustmentGain:
		return true
	default:
		return false
	}
}

// StockMovement
type StockMovement struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	ProductBarcode string    `gorm:"not null;index;type:varchar(100)" json:"product_barcode"`
	Type           string    `gorm:"not null;type:text;check:chk_stock_movements_type,type IN ('restock','sale','adjustment_loss','adjustment_gain')" json:"type"`
	Quantity       int64     `gorm:"not null" json:"quantity"`
	PreviousStock  int64     `gorm:"not null" json:"previous_stock"`
	FinalStock     int64     `gorm:"not null" json:"final_stock"`
	TotalCost      int64     `gorm:"default:0" json:"total_cost"`
	Note           *string   `gorm:"type:text" json:"note"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`

	SoftDelete `gorm:"embedded"`
}
