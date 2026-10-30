package http

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"jaga-warung-go/internal/domain"
)

type ProductHandler struct {
	productUsecase domain.ProductUseCase
}

func NewProductHandler(uc domain.ProductUseCase) *ProductHandler {
	return &ProductHandler{
		productUsecase: uc,
	}
}

func (h *ProductHandler) List(c echo.Context) error {
	ctx := c.Request().Context()

	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	result, err := h.productUsecase.List(ctx, page, limit, c.QueryParam("search"))
	if err != nil {
		return dbError(err)
	}

	// Normalize through the usecase-resolved page/limit so meta matches the
	// rows actually returned (the usecase clamps defaults).
	meta := NewMeta(result.Meta.Page, result.Meta.Limit, result.Meta.Total)
	return SuccessResponse(c, http.StatusOK, "products retrieved", result.Data, meta)
}

func (h *ProductHandler) LowStock(c echo.Context) error {
	ctx := c.Request().Context()

	products, _, err := h.productUsecase.LowStock(ctx)
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusOK, "low stock products retrieved", products, nil)
}

func (h *ProductHandler) Create(c echo.Context) error {
	ctx := c.Request().Context()

	var req struct {
		Barcode   string  `json:"barcode"`
		Name      string  `json:"name"`
		BuyPrice  int64   `json:"buy_price"`
		SellPrice int64   `json:"sell_price"`
		Stock     int64   `json:"stock"`
		MinStock  int64   `json:"min_stock"`
		Type      *string `json:"type"`
		Image     *string `json:"image"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	product, err := h.productUsecase.Create(ctx, &domain.CreateProductRequest{
		Barcode:   req.Barcode,
		Name:      req.Name,
		BuyPrice:  req.BuyPrice,
		SellPrice: req.SellPrice,
		Stock:     req.Stock,
		MinStock:  req.MinStock,
		Type:      req.Type,
		Image:     req.Image,
	})
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusCreated, "product created", product, nil)
}

func (h *ProductHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()

	product, err := h.productUsecase.Get(ctx, c.Param("id"))
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusOK, "product retrieved", product, nil)
}

func (h *ProductHandler) Patch(c echo.Context) error {
	ctx := c.Request().Context()

	var req struct {
		Name      *string `json:"name"`
		BuyPrice  *int64  `json:"buy_price"`
		SellPrice *int64  `json:"sell_price"`
		Stock     *int64  `json:"stock"`
		MinStock  *int64  `json:"min_stock"`
		Type      *string `json:"type"`
		Image     *string `json:"image"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	product, err := h.productUsecase.Patch(ctx, c.Param("id"), &domain.PatchProductRequest{
		Name:      req.Name,
		BuyPrice:  req.BuyPrice,
		SellPrice: req.SellPrice,
		Stock:     req.Stock,
		MinStock:  req.MinStock,
		Type:      req.Type,
		Image:     req.Image,
	})
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusOK, "product updated", product, nil)
}

func (h *ProductHandler) Delete(c echo.Context) error {
	ctx := c.Request().Context()

	if err := h.productUsecase.Delete(ctx, c.Param("id")); err != nil {
		return dbError(err)
	}

	return c.NoContent(http.StatusNoContent)
}
