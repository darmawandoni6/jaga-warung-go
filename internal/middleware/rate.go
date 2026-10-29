package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// RateLimit limits auth routes to 5 req/min per IP, burst 10.
func RateLimit() echo.MiddlewareFunc {
	return middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{
			Rate:      5.0 / 60.0,
			Burst:     10,
			ExpiresIn: time.Minute,
		},
	))
}
