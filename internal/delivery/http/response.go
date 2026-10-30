package http

import "github.com/labstack/echo/v4"

// Meta is the pagination block of APIResponse. Omitted entirely when nil.
type Meta struct {
	Page      int `json:"page,omitempty"`
	PerPage   int `json:"per_page,omitempty"`
	TotalRows int `json:"total_rows,omitempty"`
	TotalPage int `json:"total_page,omitempty"`
}

// APIResponse is the single envelope every endpoint returns, success or error.
// Handlers must build responses with SuccessResponse / ErrorResponse rather
// than calling c.JSON directly, so the shape cannot drift between endpoints.
type APIResponse struct {
	Status  int         `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
	Errors  interface{} `json:"errors,omitempty"`
}

// NewMeta builds pagination meta. TotalPage is derived, not passed in, so it
// cannot disagree with TotalRows/PerPage.
func NewMeta(page, limit int, total int64) *Meta {
	totalPage := 0
	if limit > 0 {
		totalPage = int((total + int64(limit) - 1) / int64(limit))
	}
	return &Meta{
		Page:      page,
		PerPage:   limit,
		TotalRows: int(total),
		TotalPage: totalPage,
	}
}

// SuccessResponse mengirimkan response sukses standar (200, 201, dll.)
func SuccessResponse(c echo.Context, statusCode int, message string, data interface{}, meta *Meta) error {
	return c.JSON(statusCode, APIResponse{
		Status:  statusCode,
		Message: message,
		Data:    data,
		Meta:    meta,
	})
}

// ErrorResponse mengirimkan response error standar (400, 401, 404, 500, dll.)
func ErrorResponse(c echo.Context, statusCode int, message string, errs any) error {
	return c.JSON(statusCode, APIResponse{
		Status:  statusCode,
		Message: message,
		Errors:  errs,
	})
}
