package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

// The whole point of response.go is a single envelope shape. These tests pin it
// so a handler that calls c.JSON directly is caught.
func TestSuccessResponseEnvelope(t *testing.T) {
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)

	if err := SuccessResponse(c, http.StatusOK, "ok", map[string]any{"a": 1}, nil); err != nil {
		t.Fatalf("SuccessResponse: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["status"] != float64(http.StatusOK) {
		t.Fatalf("status field = %v, want 200 (numeric, not a string)", body["status"])
	}
	if body["message"] != "ok" {
		t.Fatalf("message = %v, want ok", body["message"])
	}
	if body["data"] == nil {
		t.Fatal("data missing")
	}
	// meta and errors are omitempty: they must not appear on a success without meta.
	if _, ok := body["meta"]; ok {
		t.Fatal("meta present, want omitted when nil")
	}
	if _, ok := body["errors"]; ok {
		t.Fatal("errors present, want omitted on success")
	}
}

func TestErrorResponseEnvelope(t *testing.T) {
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)

	if err := ErrorResponse(c, http.StatusNotFound, "not found", nil); err != nil {
		t.Fatalf("ErrorResponse: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["status"] != float64(http.StatusNotFound) {
		t.Fatalf("status = %v, want 404", body["status"])
	}
	if body["message"] != "not found" {
		t.Fatalf("message = %v", body["message"])
	}
	if _, ok := body["data"]; ok {
		t.Fatal("data present, want omitted on error")
	}
}

func TestNewMetaDerivesTotalPage(t *testing.T) {
	tests := []struct {
		name      string
		page      int
		limit     int
		total     int64
		wantPages int
	}{
		{"exact division", 1, 10, 30, 3},
		{"remainder rounds up", 1, 10, 31, 4},
		{"single partial page", 1, 10, 1, 1},
		{"empty result", 1, 10, 0, 0},
		{"zero limit guarded", 1, 0, 5, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			meta := NewMeta(tc.page, tc.limit, tc.total)
			if meta.TotalPage != tc.wantPages {
				t.Fatalf("TotalPage = %d, want %d", meta.TotalPage, tc.wantPages)
			}
			if meta.TotalRows != int(tc.total) {
				t.Fatalf("TotalRows = %d, want %d", meta.TotalRows, tc.total)
			}
			if meta.Page != tc.page || meta.PerPage != tc.limit {
				t.Fatalf("Page/PerPage = %d/%d, want %d/%d", meta.Page, meta.PerPage, tc.page, tc.limit)
			}
		})
	}
}
