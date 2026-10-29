package middleware

import (
	"fmt"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

type ctxKey string

const accountKey ctxKey = "account_id"

func SetAccount(c echo.Context, id string) { c.Set(string(accountKey), id) }
func GetAccount(c echo.Context) string {
	v, _ := c.Get(string(accountKey)).(string)
	return v
}

// AuthJWT validates Bearer access tokens and writes account ID into context.
func AuthJWT(secret []byte, skipper func(echo.Context) bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if skipper != nil && skipper(c) {
				return next(c)
			}
			token := extractBearer(c.Request().Header.Get("Authorization"))
			if token == "" {
				return c.NoContent(http.StatusUnauthorized)
			}
			tok, err := jwt.ParseWithClaims(token, jwt.MapClaims{}, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("bad method")
				}
				return secret, nil
			}, jwt.WithExpirationRequired())
			if err != nil || !tok.Valid {
				return c.NoContent(http.StatusUnauthorized)
			}
			claims, ok := tok.Claims.(jwt.MapClaims)
			if !ok || claims["type"] != "access" {
				return c.NoContent(http.StatusUnauthorized)
			}
			sub, _ := claims["sub"].(string)
			if sub == "" {
				return c.NoContent(http.StatusUnauthorized)
			}
			SetAccount(c, sub)
			return next(c)
		}
	}
}

func AuthSkipper(c echo.Context) bool {
	p := c.Request().URL.Path
	return p == "/health" || p == "/ready" || len(p) >= 6 && p[:6] == "/auth/"
}

func extractBearer(s string) string {
	if len(s) < 7 || s[:7] != "Bearer " {
		return ""
	}
	return s[7:]
}
