package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"jaga-warung-go/pkg/auth"
)

type ctxKey string

const accountKey ctxKey = "account_id"

// SetAccount stores the authenticated account ID on the echo context.
func SetAccount(c echo.Context, id string) { c.Set(string(accountKey), id) }

// GetAccount returns the authenticated account ID, or "" when the route was not
// behind AuthJWT.
func GetAccount(c echo.Context) string {
	v, _ := c.Get(string(accountKey)).(string)
	return v
}

// AuthJWT validates Bearer access tokens and writes the account ID into the
// context. It returns a JSON error through echo's HTTPErrorHandler so clients
// always see the {"status","message"} envelope instead of an empty 401 body.
//
// Skipper may be nil. Prefer mounting this per route group over using a path
// skipper: group mounting cannot drift out of sync with the routes.
func AuthJWT(secret []byte, skipper func(echo.Context) bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if skipper != nil && skipper(c) {
				return next(c)
			}
			token := extractBearer(c.Request().Header.Get("Authorization"))
			if token == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
			}
			sub, err := auth.ParseAccess(secret, token)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired token")
			}
			SetAccount(c, sub)
			return next(c)
		}
	}
}

func extractBearer(s string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(s, prefix) {
		return ""
	}
	return strings.TrimSpace(s[len(prefix):])
}
