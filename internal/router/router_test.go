package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"jaga-warung-go/internal/config"
	"jaga-warung-go/internal/middleware"
)

func TestHealth(t *testing.T) {
	e := echo.New()
	cfg := &config.Config{JWTSecret: "test"}
	Register(e, nil, cfg)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestAuthGate(t *testing.T) {
	e := echo.New()
	cfg := &config.Config{JWTSecret: "test"}
	Register(e, nil, cfg)

	// Auth routes must be reachable without a token (they sit outside JWT middleware).
	authPaths := []string{"/auth/login", "/auth/refresh", "/auth/logout"}
	for _, path := range authPaths {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		e.ServeHTTP(rec, req)
		if rec.Code == http.StatusUnauthorized {
			t.Errorf("%s: got 401, must be reachable without a token", path)
		}
	}

	// Protected routes must 401 without a token.
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("/products: status = %d, want 401", rec.Code)
	}
}

func TestAuthSkipper(t *testing.T) {
	cases := map[string]bool{
		"/health":       true,
		"/ready":        true,
		"/auth/login":   true,
		"/auth/refresh": true,
		"/auth/logout":  true,
		"/products":     false,
		"/transactions": false,
	}
	for path, want := range cases {
		c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, path, nil), nil)
		if got := middleware.AuthSkipper(c); got != want {
			t.Errorf("%s: skip = %v, want %v", path, got, want)
		}
	}
}
