package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"jaga-warung-go/internal/domain"
	"jaga-warung-go/pkg/auth"
)

var testSecret = []byte("test-secret")

// issueAccess mints a real access token via pkg/auth so the test exercises the
// same signing path the server uses.
func issueAccess(t *testing.T, ttl time.Duration) string {
	t.Helper()
	acc := &domain.Account{Phone: "081234567890"}
	acc.ID = uuid.New()
	tok, err := auth.SignAccess(testSecret, acc, int64(ttl.Seconds()))
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	return tok
}

// newServer mounts AuthJWT on a /api/v1 group, mirroring SetupRouter.
func newServer(skipper func(echo.Context) bool) *echo.Echo {
	e := echo.New()
	g := e.Group("/api/v1")
	g.Use(AuthJWT(testSecret, skipper))
	g.GET("/protected", func(c echo.Context) error {
		return c.String(http.StatusOK, GetAccount(c))
	})
	return e
}

func doRequest(t *testing.T, e *echo.Echo, authHeader string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestAuthJWT(t *testing.T) {
	allowAll := func(echo.Context) bool { return true }

	tests := []struct {
		name       string
		skipper    func(echo.Context) bool
		authHeader string
		wantStatus int
	}{
		{"missing header", nil, "", http.StatusUnauthorized},
		{"wrong scheme", nil, "Token abc", http.StatusUnauthorized},
		{"garbage token", nil, "Bearer not-a-jwt", http.StatusUnauthorized},
		{"valid token", nil, "Bearer " + issueAccess(t, time.Minute), http.StatusOK},
		{"skipper bypasses", allowAll, "", http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(t, newServer(tc.skipper), tc.authHeader)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestAuthJWTExpiredToken(t *testing.T) {
	rec := doRequest(t, newServer(nil), "Bearer "+issueAccess(t, -time.Minute))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expired token status = %d, want 401", rec.Code)
	}
}

// A refresh token must never be accepted as an access token. This guards the
// "type" claim check in pkg/auth.ParseAccess.
func TestAuthJWTRejectsNonAccessToken(t *testing.T) {
	claims := jwt.MapClaims{
		"sub":  uuid.NewString(),
		"type": "refresh",
		"iat":  jwt.NewNumericDate(time.Now()),
		"exp":  jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(testSecret)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	rec := doRequest(t, newServer(nil), "Bearer "+raw)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("refresh token accepted as access token: status = %d", rec.Code)
	}
}

// A token signed with a different secret must be rejected.
func TestAuthJWTWrongSecret(t *testing.T) {
	acc := &domain.Account{Phone: "081234567890"}
	acc.ID = uuid.New()
	raw, err := auth.SignAccess([]byte("other-secret"), acc, 60)
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}

	rec := doRequest(t, newServer(nil), "Bearer "+raw)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("token signed with wrong secret accepted: status = %d", rec.Code)
	}
}
