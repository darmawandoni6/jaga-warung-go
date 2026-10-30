package http

import (
	"errors"
	"net/http"

	"jaga-warung-go/internal/domain"

	"github.com/labstack/echo/v4"
)

// CustomHTTPErrorHandler menangani error global dari Echo (termasuk 404, 405, dan 500)
func CustomHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	code := http.StatusInternalServerError
	message := "internal server error"

	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		if msg, ok := he.Message.(string); ok {
			message = msg
		} else {
			message = http.StatusText(code)
		}
	}

	// Tangani kasus endpoint/URL tidak ditemukan (404) secara otomatis
	if code == http.StatusNotFound && message == "Not Found" {
		message = "endpoint not found"
	}

	// Tangani domain error jika ada yang terlewat dari handler
	switch {
	case errors.Is(err, domain.ErrNotFound):
		code = http.StatusNotFound
		message = "resource not found"
	case errors.Is(err, domain.ErrConflict):
		code = http.StatusConflict
		message = "data conflict"
	case errors.Is(err, domain.ErrBadRequest):
		code = http.StatusBadRequest
		message = "bad request"
	}

	// Kirim response JSON yang terstruktur
	_ = c.JSON(code, APIResponse{
		Status:  code,
		Message: message,
	})
}
