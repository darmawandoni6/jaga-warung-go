package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"jaga-warung-go/internal/domain"
)

// PaymentHandler serves /payments. POST is the checkout flow: it computes the
// amount from the target, asks Midtrans for a Snap redirect, and returns the
// checkout URL along with the token (the client opens the URL — no card UI).
type PaymentHandler struct {
	uc domain.PaymentUseCase
}

func NewPaymentHandler(uc domain.PaymentUseCase) *PaymentHandler {
	return &PaymentHandler{uc: uc}
}

func (h *PaymentHandler) Create(c echo.Context) error {
	ctx := c.Request().Context()

	var req struct {
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}
	if req.TargetType != "transaction" && req.TargetType != "debt" {
		return echo.NewHTTPError(http.StatusBadRequest, "target_type must be transaction or debt")
	}
	targetID, err := uuid.Parse(req.TargetID)
	if err != nil || targetID == uuid.Nil {
		return echo.NewHTTPError(http.StatusBadRequest, "target_id is required")
	}

	result, err := h.uc.CreateSnap(ctx, &domain.CreatePaymentRequest{
		TargetType: req.TargetType,
		TargetID:   targetID,
	})
	if err != nil {
		return dbError(err)
	}
	return SuccessResponse(c, http.StatusCreated, "payment created", map[string]interface{}{
		"id":           result.Payment.ID,
		"order_id":     result.Payment.OrderID,
		"target_type":  result.Payment.TargetType,
		"target_id":    result.Payment.TargetID,
		"amount":       result.Payment.Amount,
		"status":       result.Payment.Status,
		"redirect_url": result.RedirectURL,
		"snap_token":   result.SnapToken,
	}, nil)
}

func (h *PaymentHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseUUIDParam(c)
	if err != nil {
		return err
	}

	// ?verify=true re-queries Midtrans: the response is the proof that this
	// order_id belongs to this payment and settled for this amount.
	if c.QueryParam("verify") == "true" {
		out, err := h.uc.Verify(ctx, id)
		if err != nil {
			return dbError(err)
		}
		return SuccessResponse(c, http.StatusOK, "payment verified", out, nil)
	}

	p, err := h.uc.Get(ctx, id)
	if err != nil {
		return dbError(err)
	}
	return SuccessResponse(c, http.StatusOK, "payment retrieved", p, nil)
}

// Notification is public: Midtrans does not carry an auth token, so any
// authentication middleware must not be mounted here. See AGENTS.md § Auth
// wiring (every new group is public until you call group.Use(requireAuth)).
func (h *PaymentHandler) Notification(c echo.Context) error {
	ctx := c.Request().Context()
	var raw map[string]interface{}
	if err := c.Bind(&raw); err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{"message": "ignored"})
	}
	if err := h.uc.HandleNotification(ctx, raw); err != nil {
		// Return 200 for not_found and bad_request so Midtrans stops retrying.
		// Anything else is a transient server error and gets retried.
		switch {
		case errors.Is(err, domain.ErrNotFound), errors.Is(err, domain.ErrBadRequest):
			return c.JSON(http.StatusOK, map[string]interface{}{"message": "ignored"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{"message": "notification failed"})
		}
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"message": "ok"})
}

// List powers reconciliation: filter by target_type/target_id to see every QRIS
// attempt for one transaction or debt, newest first.
func (h *PaymentHandler) List(c echo.Context) error {
	ctx := c.Request().Context()

	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	targetType := c.QueryParam("target_type")
	if targetType != "" && targetType != "transaction" && targetType != "debt" {
		return echo.NewHTTPError(http.StatusBadRequest, "target_type must be transaction or debt")
	}
	var targetID *uuid.UUID
	if s := c.QueryParam("target_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid target_id")
		}
		targetID = &id
	}

	result, err := h.uc.List(ctx, page, limit, targetType, targetID)
	if err != nil {
		return dbError(err)
	}
	meta := NewMeta(result.Meta.Page, result.Meta.Limit, result.Meta.Total)
	return SuccessResponse(c, http.StatusOK, "payments retrieved", result.Data, meta)
}
