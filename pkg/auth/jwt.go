package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"jaga-warung-go/internal/domain"
)

// SignAccess issues a short-lived access JWT carrying accountID as subject.
func SignAccess(secret []byte, account *domain.Account, ttlSec int64) (string, error) {
	claims := jwt.MapClaims{
		"sub":  account.ID,
		"type": "access",
		"iat":  jwt.NewNumericDate(time.Now()),
		"exp":  jwt.NewNumericDate(time.Now().Add(time.Duration(ttlSec) * time.Second)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

// ParseAccess validates an access token and returns the account ID.
func ParseAccess(secret []byte, tokenString string) (string, error) {
	tok, err := jwt.ParseWithClaims(tokenString, jwt.MapClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("bad method")
		}
		return secret, nil
	}, jwt.WithExpirationRequired())
	if err != nil || !tok.Valid {
		return "", fmt.Errorf("invalid token")
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok || claims["type"] != "access" {
		return "", fmt.Errorf("not access")
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", fmt.Errorf("missing sub")
	}
	return sub, nil
}

// IssueRefresh returns a raw opaque refresh token and its SHA-256 hash. Only the
// hash is persisted; the raw value goes to the client.
func IssueRefresh() (raw string, hashed string) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	raw = hex.EncodeToString(nonce)
	return raw, HashRefresh(raw)
}

// HashRefresh is SHA-256 hex. The token is 256 bits of entropy, so bcrypt would
// only add latency.
func HashRefresh(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
