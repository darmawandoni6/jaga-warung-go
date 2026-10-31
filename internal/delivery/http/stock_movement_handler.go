package http

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"jaga-warung-go/internal/domain"
)

// StockMovementHandler implements /stock-movements. POST is the business flow
// from docs/api.md § Business endpoints: it applies the delta and appends the
// movement atomically, then the stock cache is rebuilt.
type StockMovementHandler struct {
	movementUsecase domain.StockMovementUseCase
}

func NewStockMovementHandler(uc domain.StockMovementUseCase) *StockMovementHandler {
	return &StockMovementHandler{movementUsecase: uc}
}

func (h *StockMovementHandler) List(c echo.Context) error {
	ctx := c.Request().Context()

	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	result, err := h.movementUsecase.List(ctx, page, limit, c.QueryParam("product_barcode"))
	if err != nil {
		return dbError(err)
	}

	meta := NewMeta(result.Meta.Page, result.Meta.Limit, result.Meta.Total)
	return SuccessResponse(c, http.StatusOK, "stock movements retrieved", result.Data, meta)
}

func (h *StockMovementHandler) Create(c echo.Context) error {
	ctx := c.Request().Context()

	var req struct {
		ProductBarcode string  `json:"product_barcode"`
		Type           string  `json:"type"`
		Quantity       int64   `json:"quantity"`
		TotalCost      int64   `json:"total_cost"`
		Note           *string `json:"note"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	movement, err := h.movementUsecase.Create(ctx, &domain.CreateStockMovementRequest{
		ProductBarcode: req.ProductBarcode,
		Type:           req.Type,
		Quantity:       req.Quantity,
		TotalCost:      req.TotalCost,
		Note:           req.Note,
	})
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusCreated, "stock movement created", movement, nil)
}

func (h *StockMovementHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseUUIDParam(c)
	if err != nil {
		return err
	}

	movement, err := h.movementUsecase.Get(ctx, id)
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusOK, "stock movement retrieved", movement, nil)
}

func (h *StockMovementHandler) Patch(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseUUIDParam(c)
	if err != nil {
		return err
	}

	var req struct {
		Type      *string `json:"type"`
		Quantity  *int64  `json:"quantity"`
		TotalCost *int64  `json:"total_cost"`
		Note      *string `json:"note"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	movement, err := h.movementUsecase.Patch(ctx, id, &domain.PatchStockMovementRequest{
		Type:      req.Type,
		Quantity:  req.Quantity,
		TotalCost: req.TotalCost,
		Note:      req.Note,
	})
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusOK, "stock movement updated", movement, nil)
}

func (h *StockMovementHandler) Delete(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseUUIDParam(c)
	if err != nil {
		return err
	}

	if err := h.movementUsecase.Delete(ctx, id); err != nil {
		return dbError(err)
	}

	return c.NoContent(http.StatusNoContent)
}
