package http

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"jaga-warung-go/internal/domain"
)

// CategoryHandler implements the /categories CRUD endpoints.
type CategoryHandler struct {
	categoryUsecase domain.CategoryUseCase
}

func NewCategoryHandler(uc domain.CategoryUseCase) *CategoryHandler {
	return &CategoryHandler{
		categoryUsecase: uc,
	}
}

func (h *CategoryHandler) List(c echo.Context) error {
	ctx := c.Request().Context()

	categories, err := h.categoryUsecase.List(ctx)
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusOK, "categories retrieved", categories, nil)
}

func (h *CategoryHandler) Create(c echo.Context) error {
	ctx := c.Request().Context()

	var req struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	category, err := h.categoryUsecase.Create(ctx, req.Name)
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusCreated, "category created", category, nil)
}

func (h *CategoryHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseIDParam(c)
	if err != nil {
		return err
	}

	category, err := h.categoryUsecase.Get(ctx, id)
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusOK, "category retrieved", category, nil)
}

func (h *CategoryHandler) Patch(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseIDParam(c)
	if err != nil {
		return err
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	category, err := h.categoryUsecase.Patch(ctx, id, req.Name)
	if err != nil {
		return dbError(err)
	}

	return SuccessResponse(c, http.StatusOK, "category updated", category, nil)
}

func (h *CategoryHandler) Delete(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseIDParam(c)
	if err != nil {
		return err
	}

	if err := h.categoryUsecase.Delete(ctx, id); err != nil {
		return dbError(err)
	}

	// 204 must not carry a body, so this cannot use SuccessResponse.
	return c.NoContent(http.StatusNoContent)
}

// parseIDParam reads the :id path param as int64. Categories use an int64 PK,
// unlike every other model which uses a UUID.
func parseIDParam(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	return id, nil
}
