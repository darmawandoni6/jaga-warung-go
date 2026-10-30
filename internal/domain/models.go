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
	Type      *string `json:"type"`
	Image     *string `json:"image"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamptz" json:"updated_at"`

	SoftDelete `gorm:"embedded"`

	Movements []StockMovement `gorm:"foreignKey:ProductBarcode;references:Barcode;constraint:OnDelete:RESTRICT" json:"-"`
}

// Category
type Category struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"name"`
	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamptz" json:"updated_at"`

	SoftDelete `gorm:"embedded"`
}

// Transaction
type Transaction struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TotalAmount   int64     `gorm:"not null" json:"total_amount"`
	PaymentAmount int64     `gorm:"not null" json:"payment_amount"`
	ChangeAmount  int64     `gorm:"not null;default:0" json:"change_amount"`
	Note          *string   `json:"note"`

	CreatedAt time.Time `gorm:"type:timestamptz" json:"created_at"`

	SoftDelete `gorm:"embedded"`

	Items []TransactionItem `gorm:"foreignKey:TransactionID;constraint:OnDelete:CASCADE" json:"-"`
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

// DebtPayment
type DebtPayment struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	DebtID uuid.UUID `gorm:"type:uuid;not null;index" json:"debt_id"`
	Amount int64     `gorm:"not null" json:"amount"`
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
