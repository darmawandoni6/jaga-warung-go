package http

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"jaga-warung-go/internal/domain"
)

// TransactionHandler implements /transactions. POST is the sale flow from
// docs/api.md § Business endpoints, not a plain insert: it overrides the generic
// CRUD POST.
type TransactionHandler struct {
	transactionUsecase domain.TransactionUseCase
}

func NewTransactionHandler(uc domain.TransactionUseCase) *TransactionHandler {
	return &TransactionHandler{transactionUsecase: uc}
}

func (h *TransactionHandler) List(c echo.Context) error {
	ctx := c.Request().Context()

	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	result, err := h.transactionUsecase.List(ctx, page, limit)
	if err != nil {
		return dbError(err)
	}

	meta := NewMeta(result.Meta.Page, result.Meta.Limit, result.Meta.Total)
	return SuccessResponse(c, http.StatusOK, "transactions retrieved", result.Data, meta)
}

func (h *TransactionHandler) Create(c echo.Context) error {
	ctx := c.Request().Context()

	var req struct {
		TotalAmount   int64   `json:"total_amount"`
		PaymentAmount int64   `json:"payment_amount"`
		ChangeAmount  int64   `json:"change_amount"`
		Note          *string `json:"note"`
		Items         []struct {
			ProductBarcode string `json:"product_barcode"`
			ProductName    string `json:"product_name"`
			SellPrice      int64  `json:"sell_price"`
			Quantity       int64  `json:"quantity"`
			Subtotal       *int64 `json:"subtotal"`
		} `json:"items"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	items := make([]domain.CreateTransactionItemRequest, 0, len(req.Items))
	for _, in := range req.Items {
		items = append(items, domain.CreateTransactionItemRequest{
			ProductBarcode: in.ProductBarcode,
			ProductName:    in.ProductName,
			SellPrice:      in.SellPrice,
			Quantity:       in.Quantity,
			Subtotal:       in.Subtotal,
		})
	}

	transaction, err := h.transactionUsecase.Create(ctx, &domain.CreateTransactionRequest{
		TotalAmount:   req.TotalAmount,
		PaymentAmount: req.PaymentAmount,
		ChangeAmount:  req.ChangeAmount,
		Note:          req.Note,
		Items:         items,
	})
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusCreated, "transaction created", transaction, nil)
}

func (h *TransactionHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseUUIDParam(c)
	if err != nil {
		return err
	}

	transaction, err := h.transactionUsecase.Get(ctx, id)
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusOK, "transaction retrieved", transaction, nil)
}

func (h *TransactionHandler) Patch(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseUUIDParam(c)
	if err != nil {
		return err
	}

	var req struct {
		TotalAmount   *int64  `json:"total_amount"`
		PaymentAmount *int64  `json:"payment_amount"`
		ChangeAmount  *int64  `json:"change_amount"`
		Note          *string `json:"note"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	transaction, err := h.transactionUsecase.Patch(ctx, id, &domain.PatchTransactionRequest{
		TotalAmount:   req.TotalAmount,
		PaymentAmount: req.PaymentAmount,
		ChangeAmount:  req.ChangeAmount,
		Note:          req.Note,
	})
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusOK, "transaction updated", transaction, nil)
}

func (h *TransactionHandler) Delete(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseUUIDParam(c)
	if err != nil {
		return err
	}

	if err := h.transactionUsecase.Delete(ctx, id); err != nil {
		return dbError(err)
	}

	// 204 must not carry a body, so this cannot use SuccessResponse.
	return c.NoContent(http.StatusNoContent)
}

// parseUUIDParam reads the :id path param as a UUID. Transactions, items, debts,
// cash flows and stock movements use UUID PKs, unlike categories (int64).
func parseUUIDParam(c echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	return id, nil
}
