package middleware

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"
)

func TimeoutMiddleware(timeout time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 1. Context baru + cancel diciptakan DI SINI (Layer Middleware)
			ctx, cancel := context.WithTimeout(c.Request().Context(), timeout)

			// 2. defer cancel() dipasang DI SINI!
			//    Akan otomatis dieksekusi begitu HTTP Request Selesai/Selesai Diproses.
			defer cancel()

			// 3. Masukkan ctx yang sudah bertimeout ke Request
			c.SetRequest(c.Request().WithContext(ctx))

			// 4. Teruskan ke Handler -> UseCase -> Repository
			return next(c)
		}
	}
}
