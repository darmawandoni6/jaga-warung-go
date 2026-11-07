package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"jaga-warung-go/internal/domain"
	"jaga-warung-go/pkg/kafka"
)

// TransactionUsecase implements domain.TransactionUseCase. The handler calls it after
// parsing the request body; the usecase owns validation and delegation to the
// repository. No echo / HTTP dependency lives here.
type TransactionUsecase struct {
	repo domain.TransactionRepository
	pub  *kafka.Producer
}

func NewTransactionUsecase(repo domain.TransactionRepository, pub *kafka.Producer) domain.TransactionUseCase {
	return &TransactionUsecase{repo: repo, pub: pub}
}

func (uc *TransactionUsecase) List(ctx context.Context, page, limit int, status string) (*domain.TransactionListResult, error) {
	page, limit = normalizePage(page, limit)
	data, total, err := uc.repo.List(ctx, page, limit, status)
	if err != nil {
		return nil, fmt.Errorf("transaction list: %w", err)
	}
	return &domain.TransactionListResult{
		Data: data,
		Meta: domain.ProductListMeta{Page: page, Limit: limit, Total: total},
	}, nil
}

// Create is the sale flow from docs/api.md § Business endpoints: it validates
// the body and hands the header plus items to the repository. Stock is applied
// afterwards by the Kafka worker from transaction.created.
func (uc *TransactionUsecase) Create(ctx context.Context, req *domain.CreateTransactionRequest) (*domain.Transaction, error) {
	if len(req.Items) == 0 {
		return nil, domain.ErrBadRequest
	}

	method := req.Method
	if method == "" {
		method = domain.PaymentMethodCash
	}
	if method != domain.PaymentMethodCash && method != domain.PaymentMethodNonCash {
		return nil, domain.ErrBadRequest
	}
	// non_cash is "record now, pay later": the amount is settled by the gateway,
	// so the client cannot claim it. Cash keeps the existing tender validation.
	if method == domain.PaymentMethodCash && req.PaymentAmount < req.TotalAmount {
		return nil, domain.ErrBadRequest
	}

	totalAmount := int64(0)
	changeAmount := int64(0)

	items := make([]*domain.TransactionItem, 0, len(req.Items))
	for _, in := range req.Items {
		if in.ProductBarcode == "" {
			return nil, domain.ErrBadRequest
		}
		if in.Quantity <= 0 {
			return nil, domain.ErrBadRequest
		}
		subtotal := in.SellPrice * in.Quantity
		totalAmount += subtotal

		items = append(items, &domain.TransactionItem{
			ProductBarcode: in.ProductBarcode,
			ProductName:    in.ProductName,
			SellPrice:      in.SellPrice,
			Quantity:       in.Quantity,
			Subtotal:       subtotal,
		})
	}

	paymentAmount := req.PaymentAmount
	if method == domain.PaymentMethodNonCash {
		paymentAmount = 0
	} else {
		changeAmount = req.PaymentAmount - totalAmount
	}

	if totalAmount != req.TotalAmount {
		return nil, domain.ErrBadRequest
	}

	if changeAmount > 0 && req.ChangeAmount != changeAmount {
		return nil, domain.ErrBadRequest
	}

	t := &domain.Transaction{
		TotalAmount:   totalAmount,
		PaymentAmount: paymentAmount,
		ChangeAmount:  changeAmount,
		Method:        method,
		Status:        domain.DeriveTransactionStatus(method, totalAmount, paymentAmount),
		Note:          req.Note,
	}
	if t.Status == domain.TransactionStatusPaid {
		now := time.Now().UTC()
		t.PaidAt = &now
	}

	if err := uc.repo.Create(ctx, t, items); err != nil {
		return nil, err
	}
	// The repo returns the header only; embed the items so the event carries a
	// complete receipt.
	row := *t
	row.Items = make([]domain.TransactionItem, len(items))
	for i, item := range items {
		row.Items[i] = *item
	}
	uc.pub.Publish(ctx, kafka.TopicEvents, t.ID.String(),
		domain.NewEvent(domain.EventTypeTransactionCreated, "transaction", t.ID.String(), domain.NewTransactionEvent(&row)))
	return t, nil
}

func (uc *TransactionUsecase) Get(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	t, err := uc.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("transaction get: %w", err)
	}
	return t, nil
}

func (uc *TransactionUsecase) Patch(ctx context.Context, id uuid.UUID, req *domain.PatchTransactionRequest) (*domain.Transaction, error) {
	cur, err := uc.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("transaction patch get: %w", err)
	}

	// paid is terminal: money already moved, so no amount and no method may
	// change afterwards. Corrections go through a new sale, not an edit.
	if cur.Status == domain.TransactionStatusPaid &&
		(req.Method != nil || req.PaymentAmount != nil || req.ChangeAmount != nil) {
		return nil, domain.ErrBadRequest
	}

	// Method only moves non_cash→cash. That is the recovery path when the gateway
	// attempt fails and the customer pays cash instead.
	if req.Method != nil {
		if *req.Method != domain.PaymentMethodCash || cur.Method != domain.PaymentMethodNonCash {
			return nil, domain.ErrBadRequest
		}
		if req.PaymentAmount == nil {
			return nil, domain.ErrBadRequest
		}
	}

	// While still non_cash, the amounts belong to the gateway or to the
	// non_cash→cash switch above — never to a plain amount patch.
	if cur.Method == domain.PaymentMethodNonCash && req.Method == nil &&
		(req.PaymentAmount != nil || req.ChangeAmount != nil) {
		return nil, domain.ErrBadRequest
	}

	// Apply explicit values; nil fields keep their current value.
	if req.TotalAmount != nil {
		cur.TotalAmount = *req.TotalAmount
	}
	if req.Note != nil {
		cur.Note = req.Note
	}

	switch {
	case req.Method != nil:
		tender := *req.PaymentAmount
		if tender < cur.TotalAmount {
			return nil, domain.ErrBadRequest
		}
		cur.Method = domain.PaymentMethodCash
		cur.PaymentAmount = tender
		cur.ChangeAmount = tender - cur.TotalAmount
		cur.Status = domain.DeriveTransactionStatus(domain.PaymentMethodCash, cur.TotalAmount, tender)
		now := time.Now().UTC()
		cur.PaidAt = &now
	default:
		if req.PaymentAmount != nil {
			cur.PaymentAmount = *req.PaymentAmount
		}
		if req.ChangeAmount != nil {
			cur.ChangeAmount = *req.ChangeAmount
		}
	}

	if err := uc.repo.Update(ctx, cur); err != nil {
		return nil, err
	}
	uc.pub.Publish(ctx, kafka.TopicEvents, cur.ID.String(),
		domain.NewEvent(domain.EventTypeTransactionUpdated, "transaction", cur.ID.String(), domain.NewTransactionEvent(cur)))
	return cur, nil
}

func (uc *TransactionUsecase) Delete(ctx context.Context, id uuid.UUID) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}
	// Id-only: the row is gone, and Delete returns no data.
	uc.pub.Publish(ctx, kafka.TopicEvents, id.String(),
		domain.NewEvent(domain.EventTypeTransactionDeleted, "transaction", id.String(), nil))
	return nil
}

// normalizePage clamps pagination to the same defaults as products so every
// paginated list reports a page/limit that matches the rows returned.
func normalizePage(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return page, limit
}
