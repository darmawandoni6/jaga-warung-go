package router

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Register wires the middleware chain and every route.
func Register(e *echo.Echo) {
	e.Use(middleware.Recover(), middleware.RequestID(), middleware.RequestLogger())

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
}
