package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"jaga-warung-go/internal/model"
)

// RegisterProducts wires CRUD routes for /products.
func RegisterProducts(e *echo.Echo, db *gorm.DB) {
	e.GET("/products", func(c echo.Context) error {
		page, _ := strconv.Atoi(c.QueryParam("page"))
		if page < 1 {
			page = 1
		}
		limit, _ := strconv.Atoi(c.QueryParam("limit"))
		if limit < 1 || limit > 100 {
			limit = 20
		}

		q := db.Model(&model.Product{})

		if search := c.QueryParam("search"); search != "" {
			search = "%" + search + "%"
			q = q.Where("name LIKE ? OR barcode LIKE ?", search, search)
		}

		var total int64
		if err := q.Count(&total).Error; err != nil {
			return dbError(err)
		}

		var products []model.Product
		offset := (page - 1) * limit
		if err := q.Offset(offset).Limit(limit).Find(&products).Error; err != nil {
			return dbError(err)
		}

		return c.JSON(http.StatusOK, map[string]any{
			"data": products,
			"meta": map[string]any{
				"page":  page,
				"limit": limit,
				"total": total,
			},
		})
	})

	e.GET("/products/low-stock", func(c echo.Context) error {
		var products []model.Product
		if err := db.Where("stock <= min_stock AND deleted_at IS NULL").Find(&products).Error; err != nil {
			return dbError(err)
		}
		return c.JSON(http.StatusOK, products)
	})

	e.POST("/products", func(c echo.Context) error {
		var product model.Product
		if err := c.Bind(&product); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "bad request")
		}
		if product.Barcode == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "barcode is required")
		}
		if err := db.Create(&product).Error; err != nil {
			return dbError(err)
		}
		return c.JSON(http.StatusCreated, product)
	})

	e.GET("/products/:id", func(c echo.Context) error {
		var product model.Product
		if err := db.Where("barcode = ?", c.Param("id")).First(&product).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, "not found")
			}
			return dbError(err)
		}
		return c.JSON(http.StatusOK, product)
	})

	e.PATCH("/products/:id", func(c echo.Context) error {
		var product model.Product
		if err := db.Where("barcode = ?", c.Param("id")).First(&product).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, "not found")
			}
			return dbError(err)
		}
		// PK and created_at are immutable: keep loaded values regardless of body.
		savedBarcode, savedCreatedAt := product.Barcode, product.CreatedAt
		if err := c.Bind(&product); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "bad request")
		}
		product.Barcode, product.CreatedAt = savedBarcode, savedCreatedAt
		// Select forces every listed column into the UPDATE, so explicit
		// 0/""/null in the body persist; barcode (PK) and created_at are
		// excluded — immutable.
		upd := db.Model(&product).
			Select("Name", "BuyPrice", "SellPrice", "Stock", "MinStock", "Type", "Image", "UpdatedAt").
			Save(&product)
		if upd.Error != nil {
			return dbError(upd.Error)
		}
		return c.JSON(http.StatusOK, product)
	})

	e.DELETE("/products/:id", func(c echo.Context) error {
		result := db.Where("barcode = ?", c.Param("id")).Delete(&model.Product{})
		if result.RowsAffected == 0 {
			return echo.NewHTTPError(http.StatusNotFound, "not found")
		}
		return c.NoContent(http.StatusNoContent)
	})
}
