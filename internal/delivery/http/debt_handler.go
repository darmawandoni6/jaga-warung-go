package http

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"jaga-warung-go/internal/domain"
)

// DebtHandler implements /debts. Status is derived server-side on create, so the
// POST body carries no status field.
type DebtHandler struct {
	debtUsecase domain.DebtUseCase
}

func NewDebtHandler(uc domain.DebtUseCase) *DebtHandler {
	return &DebtHandler{debtUsecase: uc}
}

func (h *DebtHandler) List(c echo.Context) error {
	ctx := c.Request().Context()

	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	result, err := h.debtUsecase.List(ctx, page, limit, c.QueryParam("status"))
	if err != nil {
		return dbError(err)
	}

	meta := NewMeta(result.Meta.Page, result.Meta.Limit, result.Meta.Total)
	return SuccessResponse(c, http.StatusOK, "debts retrieved", result.Data, meta)
}

func (h *DebtHandler) Create(c echo.Context) error {
	ctx := c.Request().Context()

	var req struct {
		CustomerName string  `json:"customer_name"`
		Phone        *string `json:"phone"`
		TotalDebt    int64   `json:"total_debt"`
		PaidAmount   int64   `json:"paid_amount"`
		Note         *string `json:"note"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	debt, err := h.debtUsecase.Create(ctx, &domain.CreateDebtRequest{
		CustomerName: req.CustomerName,
		Phone:        req.Phone,
		TotalDebt:    req.TotalDebt,
		PaidAmount:   req.PaidAmount,
		Note:         req.Note,
	})
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusCreated, "debt created", debt, nil)
}

func (h *DebtHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseUUIDParam(c)
	if err != nil {
		return err
	}

	debt, err := h.debtUsecase.Get(ctx, id)
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusOK, "debt retrieved", debt, nil)
}

func (h *DebtHandler) Patch(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseUUIDParam(c)
	if err != nil {
		return err
	}

	var req struct {
		CustomerName *string `json:"customer_name"`
		Phone        *string `json:"phone"`
		TotalDebt    *int64  `json:"total_debt"`
		PaidAmount   *int64  `json:"paid_amount"`
		Status       *string `json:"status"`
		Note         *string `json:"note"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	debt, err := h.debtUsecase.Patch(ctx, id, &domain.PatchDebtRequest{
		CustomerName: req.CustomerName,
		Phone:        req.Phone,
		TotalDebt:    req.TotalDebt,
		PaidAmount:   req.PaidAmount,
		Status:       req.Status,
		Note:         req.Note,
	})
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusOK, "debt updated", debt, nil)
}

func (h *DebtHandler) Delete(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseUUIDParam(c)
	if err != nil {
		return err
	}

	if err := h.debtUsecase.Delete(ctx, id); err != nil {
		return dbError(err)
	}

	// 204 must not carry a body, so this cannot use SuccessResponse.
	return c.NoContent(http.StatusNoContent)
}
