package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Event types.
const (
	EventTypeTransactionCreated = "transaction.created"
	EventTypeTransactionUpdated = "transaction.updated"
	EventTypeTransactionDeleted = "transaction.deleted"

	EventTypeTransactionItemCreated = "transaction.item.created"
	EventTypeTransactionItemUpdated = "transaction.item.updated"
	EventTypeTransactionItemDeleted = "transaction.item.deleted"

	EventTypeStockMovementCreated = "stock.movement.created"
	EventTypeStockMovementUpdated = "stock.movement.updated"
	EventTypeStockMovementDeleted = "stock.movement.deleted"

	EventTypeEventProcessingFailed = "event.processing.failed"
)

// Event is the envelope every Kafka message carries. Data holds the marshalled
// payload and is absent for id-only events (soft deletes). One type is used on
// both sides of the wire: the producer marshals it, the consumer unmarshals it
// and then decodes Data into the typed payload below.
type Event struct {
	EventID string          `json:"event_id"`
	Type    string          `json:"type"`
	Entity  string          `json:"entity"`
	ID      string          `json:"id"`
	At      time.Time       `json:"at"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// NewEvent builds an envelope, filling the per-message identity and timestamp.
func NewEvent(eventType, entity, id string, data any) Event {
	ev := Event{
		EventID: uuid.NewString(),
		Type:    eventType,
		Entity:  entity,
		ID:      id,
		At:      time.Now().UTC(),
	}
	if data != nil {
		// Payloads are plain structs, so marshalling cannot fail; a failed
		// marshal would leave Data nil, which the wire already allows.
		ev.Data, _ = json.Marshal(data)
	}
	return ev
}

// TransactionEvent is the payload of transaction.created / transaction.updated.
// It mirrors the row plus its line items, decoupled from the GORM model.
type TransactionEvent struct {
	ID            uuid.UUID              `json:"id"`
	TotalAmount   int64                  `json:"total_amount"`
	PaymentAmount int64                  `json:"payment_amount"`
	ChangeAmount  int64                  `json:"change_amount"`
	Method        string                 `json:"method"`
	Status        string                 `json:"status"`
	PaidAt        *time.Time             `json:"paid_at"`
	Note          *string                `json:"note"`
	CreatedAt     time.Time              `json:"created_at"`
	Items         []TransactionItemEvent `json:"items"`
}

// TransactionItemEvent is the payload of transaction.item.created / updated.
type TransactionItemEvent struct {
	ID             uuid.UUID `json:"id"`
	TransactionID  uuid.UUID `json:"transaction_id"`
	ProductBarcode string    `json:"product_barcode"`
	ProductName    string    `json:"product_name"`
	SellPrice      int64     `json:"sell_price"`
	Quantity       int64     `json:"quantity"`
	Subtotal       int64     `json:"subtotal"`
	CreatedAt      time.Time `json:"created_at"`
}

// StockMovementEvent is the payload of stock.movement.created / updated.
type StockMovementEvent struct {
	ID             uuid.UUID `json:"id"`
	ProductBarcode string    `json:"product_barcode"`
	Type           string    `json:"type"`
	Quantity       int64     `json:"quantity"`
	PreviousStock  int64     `json:"previous_stock"`
	FinalStock     int64     `json:"final_stock"`
	TotalCost      int64     `json:"total_cost"`
	Note           *string   `json:"note"`
	CreatedAt      time.Time `json:"created_at"`
}

// ConsumerFailureEvent is the payload published to the DLQ when the consumer
// cannot apply an event. OriginalData is copied verbatim so the message can be
// replayed by hand once the cause is fixed.
type ConsumerFailureEvent struct {
	OriginalEventID string          `json:"original_event_id"`
	OriginalType    string          `json:"original_type"`
	EntityID        string          `json:"entity_id"`
	OriginalData    json.RawMessage `json:"original_data,omitempty"`
	Error           string          `json:"error"`
	At              time.Time       `json:"at"`
}

// NewTransactionEvent maps a transaction row (with Items already populated)
// onto the event payload.
func NewTransactionEvent(t *Transaction) TransactionEvent {
	ev := TransactionEvent{
		ID:            t.ID,
		TotalAmount:   t.TotalAmount,
		PaymentAmount: t.PaymentAmount,
		ChangeAmount:  t.ChangeAmount,
		Method:        t.Method,
		Status:        t.Status,
		PaidAt:        t.PaidAt,
		Note:          t.Note,
		CreatedAt:     t.CreatedAt,
		Items:         make([]TransactionItemEvent, len(t.Items)),
	}
	for i, item := range t.Items {
		ev.Items[i] = NewTransactionItemEvent(&item)
	}
	return ev
}

// NewTransactionItemEvent maps a transaction item row onto the event payload.
func NewTransactionItemEvent(it *TransactionItem) TransactionItemEvent {
	return TransactionItemEvent{
		ID:             it.ID,
		TransactionID:  it.TransactionID,
		ProductBarcode: it.ProductBarcode,
		ProductName:    it.ProductName,
		SellPrice:      it.SellPrice,
		Quantity:       it.Quantity,
		Subtotal:       it.Subtotal,
		CreatedAt:      it.CreatedAt,
	}
}

// NewStockMovementEvent maps a stock movement row onto the event payload.
func NewStockMovementEvent(m *StockMovement) StockMovementEvent {
	return StockMovementEvent{
		ID:             m.ID,
		ProductBarcode: m.ProductBarcode,
		Type:           m.Type,
		Quantity:       m.Quantity,
		PreviousStock:  m.PreviousStock,
		FinalStock:     m.FinalStock,
		TotalCost:      m.TotalCost,
		Note:           m.Note,
		CreatedAt:      m.CreatedAt,
	}
}
