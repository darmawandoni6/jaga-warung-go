package router

import (
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"gorm.io/gorm"

	"jaga-warung-go/internal/config"
	"jaga-warung-go/internal/handler"
	"jaga-warung-go/internal/middleware"
)

// Register wires the middleware chain and every route.
func Register(e *echo.Echo, db *gorm.DB, cfg *config.Config) {
	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	e.Use(echomw.RequestLogger())

	// Unauthenticated.
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})
	e.GET("/ready", func(c echo.Context) error {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			return c.JSON(503, map[string]string{"status": "not ready"})
		}
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// Auth: rate-limited, no JWT. Nil db = skip auth routes (testing only).
	if db != nil {
		authGroup := e.Group("/auth", middleware.RateLimit())
		handler.RegisterAuth(authGroup, db, cfg)
	} else {
		// Expose unauthenticated health/read-only stubs so auth gate tests can
		// assert 401 on protected routes without needing a live db.
		e.POST("/auth/login", func(c echo.Context) error { return c.NoContent(501) })
		e.POST("/auth/refresh", func(c echo.Context) error { return c.NoContent(501) })
		e.POST("/auth/logout", func(c echo.Context) error { return c.NoContent(501) })
	}

	// Everything else requires a bearer token.
	e.Use(middleware.AuthJWT([]byte(cfg.JWTSecret), middleware.AuthSkipper))
}
