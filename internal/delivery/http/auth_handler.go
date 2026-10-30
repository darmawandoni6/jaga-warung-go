package http

import (
	"errors"
	"net/http"

	"jaga-warung-go/internal/domain"

	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	authUsecase domain.AuthUseCase
}

func NewAuthHandler(uc domain.AuthUseCase) *AuthHandler {
	return &AuthHandler{
		authUsecase: uc,
	}
}

func (h *AuthHandler) Login(c echo.Context) error {
	ctx := c.Request().Context()
	var req struct {
		Phone    string `json:"phone" validate:"required"`
		Password string `json:"password" validate:"required"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	tokens, err := h.authUsecase.Login(ctx, req.Phone, req.Password)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"expires_in":    tokens.ExpiresIn,
	})
}

func (h *AuthHandler) Refresh(c echo.Context) error {
	ctx := c.Request().Context()

	var req struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	tokens, err := h.authUsecase.Refresh(ctx, req.RefreshToken)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid refresh token")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"expires_in":    tokens.ExpiresIn,
	})
}

func (h *AuthHandler) Logout(c echo.Context) error {
	ctx := c.Request().Context()

	var req struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	if err := h.authUsecase.Logout(ctx, req.RefreshToken); err != nil {
		return dbError(err)
	}

	return c.NoContent(http.StatusNoContent)
}

// dbError maps domain errors to HTTP status.
func dbError(err error) *echo.HTTPError {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	case errors.Is(err, domain.ErrConflict):
		return echo.NewHTTPError(http.StatusConflict, "conflict")
	case errors.Is(err, domain.ErrBadRequest):
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
}
