package http

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"jaga-warung-go/internal/domain"
)

// TransactionItemHandler implements /transaction-items. GET supports
// ?transaction_id= so a client can load the items of one sale without the
// transaction payload preloading them (docs/api.md § Semantics).
type TransactionItemHandler struct {
	itemUsecase domain.TransactionItemUseCase
}

func NewTransactionItemHandler(uc domain.TransactionItemUseCase) *TransactionItemHandler {
	return &TransactionItemHandler{itemUsecase: uc}
}

func (h *TransactionItemHandler) List(c echo.Context) error {
	ctx := c.Request().Context()

	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	var transactionID *uuid.UUID
	if raw := c.QueryParam("transaction_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid transaction_id")
		}
		transactionID = &id
	}

	result, err := h.itemUsecase.List(ctx, page, limit, transactionID)
	if err != nil {
		return dbError(err)
	}

	meta := NewMeta(result.Meta.Page, result.Meta.Limit, result.Meta.Total)
	return SuccessResponse(c, http.StatusOK, "transaction items retrieved", result.Data, meta)
}

func (h *TransactionItemHandler) Create(c echo.Context) error {
	ctx := c.Request().Context()

	var req struct {
		TransactionID  string `json:"transaction_id"`
		ProductBarcode string `json:"product_barcode"`
		ProductName    string `json:"product_name"`
		SellPrice      int64  `json:"sell_price"`
		Quantity       int64  `json:"quantity"`
		Subtotal       *int64 `json:"subtotal"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	// transaction_id is the parent FK: the client must send it, but the server
	// still generates the item's own id.
	transactionID, err := uuid.Parse(req.TransactionID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid transaction_id")
	}

	item, err := h.itemUsecase.Create(ctx, &domain.CreateTransactionItemRequest{
		TransactionID:  transactionID,
		ProductBarcode: req.ProductBarcode,
		ProductName:    req.ProductName,
		SellPrice:      req.SellPrice,
		Quantity:       req.Quantity,
		Subtotal:       req.Subtotal,
	})
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusCreated, "transaction item created", item, nil)
}

func (h *TransactionItemHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseUUIDParam(c)
	if err != nil {
		return err
	}

	item, err := h.itemUsecase.Get(ctx, id)
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusOK, "transaction item retrieved", item, nil)
}

func (h *TransactionItemHandler) Patch(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseUUIDParam(c)
	if err != nil {
		return err
	}

	var req struct {
		ProductName *string `json:"product_name"`
		SellPrice   *int64  `json:"sell_price"`
		Quantity    *int64  `json:"quantity"`
		Subtotal    *int64  `json:"subtotal"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	item, err := h.itemUsecase.Patch(ctx, id, &domain.PatchTransactionItemRequest{
		ProductName: req.ProductName,
		SellPrice:   req.SellPrice,
		Quantity:    req.Quantity,
		Subtotal:    req.Subtotal,
	})
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusOK, "transaction item updated", item, nil)
}

func (h *TransactionItemHandler) Delete(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseUUIDParam(c)
	if err != nil {
		return err
	}

	if err := h.itemUsecase.Delete(ctx, id); err != nil {
		return dbError(err)
	}

	return c.NoContent(http.StatusNoContent)
}
