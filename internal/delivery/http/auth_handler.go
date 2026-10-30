package http

import (
	"errors"
	"net/http"

	"jaga-warung-go/internal/domain"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
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

	return SuccessResponse(c, http.StatusOK, "login successful", map[string]any{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"expires_in":    tokens.ExpiresIn,
	}, nil)
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

	return SuccessResponse(c, http.StatusOK, "token refreshed", map[string]any{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"expires_in":    tokens.ExpiresIn,
	}, nil)
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
	// GORM is opened with TranslateError: true, so unique/FK violations arrive
	// as these sentinels instead of raw pgx errors. Without this they would
	// surface as 500.
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return echo.NewHTTPError(http.StatusConflict, "duplicate value")
	case errors.Is(err, gorm.ErrForeignKeyViolated):
		return echo.NewHTTPError(http.StatusConflict, "referenced record does not exist or is in use")
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
}
