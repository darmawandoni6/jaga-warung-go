package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"jaga-warung-go/internal/auth"
	"jaga-warung-go/internal/config"
	"jaga-warung-go/internal/model"
)

// RegisterAuth wires POST /auth/login, /auth/refresh, /auth/logout on the given
// group. No /auth/register: the one account is seeded from env (B5).
// Panics if db is nil — this is a developer mistake, not a runtime case.
func RegisterAuth(g *echo.Group, db *gorm.DB, cfg *config.Config) {
	if db == nil {
		panic("handler.RegisterAuth: db must not be nil")
	}
	g.POST("/login", func(c echo.Context) error {
		var req struct {
			Phone    string `json:"phone" validate:"required"`
			Password string `json:"password" validate:"required"`
		}
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "bad request")
		}
		var acc model.Account
		if err := db.Where("phone = ?", req.Phone).First(&acc).Error; err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
		}
		if err := auth.CheckPassword(req.Password, acc.PasswordHash); err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
		}
		return issueTokens(c, db, &acc, cfg)
	})

	g.POST("/refresh", func(c echo.Context) error {
		var req struct {
			RefreshToken string `json:"refresh_token" validate:"required"`
		}
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "bad request")
		}

		hash := auth.HashRefresh(req.RefreshToken)
		var rt model.RefreshToken
		err := db.Where("token_hash = ? AND revoked_at IS NULL", hash).First(&rt).Error
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid refresh token")
		}
		if time.Now().After(rt.ExpiresAt) {
			return echo.NewHTTPError(http.StatusUnauthorized, "refresh token expired")
		}

		var acc model.Account
		if err := db.First(&acc, "id = ?", rt.AccountID).Error; err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid refresh token")
		}

		// Rotate: revoke the used token before issuing a new one.
		now := time.Now()
		db.Model(&rt).Update("revoked_at", now)
		return issueTokens(c, db, &acc, cfg)
	})

	g.POST("/logout", func(c echo.Context) error {
		var req struct {
			RefreshToken string `json:"refresh_token" validate:"required"`
		}
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "bad request")
		}
		hash := auth.HashRefresh(req.RefreshToken)
		now := time.Now()
		db.Model(&model.RefreshToken{}).
			Where("token_hash = ? AND revoked_at IS NULL", hash).
			Update("revoked_at", now)
		return c.NoContent(http.StatusNoContent)
	})
}

func issueTokens(c echo.Context, db *gorm.DB, acc *model.Account, cfg *config.Config) error {
	accessTTL := int64(cfg.AccessTTL.Seconds())
	access, err := auth.SignAccess([]byte(cfg.JWTSecret), acc, accessTTL)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "sign failed")
	}

	raw, hash := auth.IssueRefresh()
	rt := model.RefreshToken{
		ID:        uuid.New().String(),
		AccountID: acc.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(cfg.RefreshTTL),
		CreatedAt: time.Now(),
	}
	if err := db.Create(&rt).Error; err != nil {
		return dbError(err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"access_token":  access,
		"refresh_token": raw,
		"expires_in":    accessTTL,
	})
}

// dbError maps gorm errors to HTTP status. Extended by Phase 4's generic
// Resource[T]; the mapping lives here so it has one owner.
func dbError(err error) *echo.HTTPError {
	switch {
	case errors.Is(err, gorm.ErrDuplicatedKey), errors.Is(err, gorm.ErrForeignKeyViolated):
		return echo.NewHTTPError(http.StatusConflict, "conflict")
	case errors.Is(err, gorm.ErrRecordNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
}
