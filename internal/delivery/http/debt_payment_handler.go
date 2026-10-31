package http

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"jaga-warung-go/internal/domain"
)

// DebtPaymentHandler implements /debt-payments. POST is the payment flow from
// docs/api.md § Business endpoints, not a plain insert: it updates the parent
// debt's paid_amount and derived status and can emit a cash-flow income.
type DebtPaymentHandler struct {
	paymentUsecase domain.DebtPaymentUseCase
}

func NewDebtPaymentHandler(uc domain.DebtPaymentUseCase) *DebtPaymentHandler {
	return &DebtPaymentHandler{paymentUsecase: uc}
}

func (h *DebtPaymentHandler) List(c echo.Context) error {
	ctx := c.Request().Context()

	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	var debtID *uuid.UUID
	if raw := c.QueryParam("debt_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid debt_id")
		}
		debtID = &id
	}

	result, err := h.paymentUsecase.List(ctx, page, limit, debtID)
	if err != nil {
		return dbError(err)
	}

	meta := NewMeta(result.Meta.Page, result.Meta.Limit, result.Meta.Total)
	return SuccessResponse(c, http.StatusOK, "debt payments retrieved", result.Data, meta)
}

func (h *DebtPaymentHandler) Create(c echo.Context) error {
	ctx := c.Request().Context()

	var req struct {
		DebtID         string  `json:"debt_id"`
		Amount         int64   `json:"amount"`
		Note           *string `json:"note"`
		CreateCashFlow bool    `json:"create_cash_flow"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	debtID, err := uuid.Parse(req.DebtID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid debt_id")
	}

	result, err := h.paymentUsecase.Create(ctx, &domain.CreateDebtPaymentRequest{
		DebtID:         debtID,
		Amount:         req.Amount,
		Note:           req.Note,
		CreateCashFlow: req.CreateCashFlow,
	})
	if err != nil {
		return dbError(err)
	}

	// Echo both rows: the payment and the parent debt with its new paid_amount
	// and derived status, so the client does not need a follow-up GET.
	return SuccessResponse(c, http.StatusCreated, "debt payment created", map[string]any{
		"payment": result.Payment,
		"debt":    result.Debt,
	}, nil)
}

func (h *DebtPaymentHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseUUIDParam(c)
	if err != nil {
		return err
	}

	payment, err := h.paymentUsecase.Get(ctx, id)
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusOK, "debt payment retrieved", payment, nil)
}

func (h *DebtPaymentHandler) Patch(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseUUIDParam(c)
	if err != nil {
		return err
	}

	var req struct {
		Amount *int64  `json:"amount"`
		Note   *string `json:"note"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	payment, err := h.paymentUsecase.Patch(ctx, id, &domain.PatchDebtPaymentRequest{
		Amount: req.Amount,
		Note:   req.Note,
	})
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusOK, "debt payment updated", payment, nil)
}

func (h *DebtPaymentHandler) Delete(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseUUIDParam(c)
	if err != nil {
		return err
	}

	if err := h.paymentUsecase.Delete(ctx, id); err != nil {
		return dbError(err)
	}

	return c.NoContent(http.StatusNoContent)
}
