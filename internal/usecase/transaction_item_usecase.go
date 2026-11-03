package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"jaga-warung-go/internal/domain"
	"jaga-warung-go/pkg/kafka"
)

// TransactionItemUsecase implements domain.TransactionItemUseCase. Every write
// here changes how much of a product was sold; the repository only touches the
// line rows, and the Kafka worker rebuilds products.stock from the event.
type TransactionItemUsecase struct {
	repo domain.TransactionItemRepository
	pub  *kafka.Producer
}

func NewTransactionItemUsecase(repo domain.TransactionItemRepository, pub *kafka.Producer) domain.TransactionItemUseCase {
	return &TransactionItemUsecase{repo: repo, pub: pub}
}

func (uc *TransactionItemUsecase) List(ctx context.Context, page, limit int, transactionID *uuid.UUID) (*domain.TransactionItemListResult, error) {
	page, limit = normalizePage(page, limit)
	data, total, err := uc.repo.List(ctx, page, limit, transactionID)
	if err != nil {
		return nil, fmt.Errorf("transaction item list: %w", err)
	}
	return &domain.TransactionItemListResult{
		Data: data,
		Meta: domain.ProductListMeta{Page: page, Limit: limit, Total: total},
	}, nil
}

func (uc *TransactionItemUsecase) Create(ctx context.Context, req *domain.CreateTransactionItemRequest) (*domain.TransactionItem, error) {
	if req.ProductBarcode == "" || req.Quantity <= 0 {
		return nil, domain.ErrBadRequest
	}

	subtotal := req.SellPrice * req.Quantity
	if req.Subtotal != nil {
		subtotal = *req.Subtotal
	}

	item := &domain.TransactionItem{
		TransactionID:  req.TransactionID,
		ProductBarcode: req.ProductBarcode,
		ProductName:    req.ProductName,
		SellPrice:      req.SellPrice,
		Quantity:       req.Quantity,
		Subtotal:       subtotal,
	}
	if err := uc.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	uc.pub.Publish(ctx, kafka.TopicEvents, item.ID.String(),
		domain.NewEvent(domain.EventTypeTransactionItemCreated, "transaction.item", item.ID.String(), domain.NewTransactionItemEvent(item)))
	return item, nil
}

func (uc *TransactionItemUsecase) Get(ctx context.Context, id uuid.UUID) (*domain.TransactionItem, error) {
	item, err := uc.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("transaction item get: %w", err)
	}
	return item, nil
}

func (uc *TransactionItemUsecase) Patch(ctx context.Context, id uuid.UUID, req *domain.PatchTransactionItemRequest) (*domain.TransactionItem, error) {
	cur, err := uc.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("transaction item patch get: %w", err)
	}

	if req.ProductName != nil {
		cur.ProductName = *req.ProductName
	}
	if req.SellPrice != nil {
		cur.SellPrice = *req.SellPrice
	}
	if req.Quantity != nil {
		if *req.Quantity <= 0 {
			return nil, domain.ErrBadRequest
		}
		cur.Quantity = *req.Quantity
	}
	if req.Subtotal != nil {
		cur.Subtotal = *req.Subtotal
	}

	if err := uc.repo.Update(ctx, cur); err != nil {
		return nil, err
	}
	uc.pub.Publish(ctx, kafka.TopicEvents, cur.ID.String(),
		domain.NewEvent(domain.EventTypeTransactionItemUpdated, "transaction.item", cur.ID.String(), domain.NewTransactionItemEvent(cur)))
	return cur, nil
}

func (uc *TransactionItemUsecase) Delete(ctx context.Context, id uuid.UUID) error {
	// Fetch first: the worker needs the parent sale to rebuild that product's
	// stock, and a deleted row is the only place to learn it.
	item, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}
	uc.pub.Publish(ctx, kafka.TopicEvents, id.String(),
		domain.NewEvent(domain.EventTypeTransactionItemDeleted, "transaction.item", id.String(),
			domain.TransactionItemEvent{
				ID:             item.ID,
				TransactionID:  item.TransactionID,
				ProductBarcode: item.ProductBarcode,
			}))
	return nil
}
