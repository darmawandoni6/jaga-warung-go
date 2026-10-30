package http

import "github.com/labstack/echo/v4"

type Meta struct {
	Page      int `json:"page,omitempty"`
	PerPage   int `json:"per_page,omitempty"`
	TotalRows int `json:"total_rows,omitempty"`
	TotalPage int `json:"total_page,omitempty"`
}

type APIResponse struct {
	Status  int         `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
	Errors  interface{} `json:"errors,omitempty"`
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
