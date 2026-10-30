package http

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"jaga-warung-go/internal/config"
	"jaga-warung-go/internal/delivery/http/middleware"
	"jaga-warung-go/internal/repository"
	"jaga-warung-go/internal/usecase"
)

// SetupRouter mendaftarkan seluruh route dan menghubungkan layer dependency per domain
func SetupRouter(e *echo.Echo, db *gorm.DB, cfg *config.Config) {
	// Health Check
	e.GET("/health", func(c echo.Context) error {

		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]any{
				"status":  "unhealthy",
				"message": "database connection failed",
			})
		}

		return c.JSON(http.StatusOK, map[string]any{
			"status":  "healthy",
			"message": "service is running smoothly",
		})

	})

	// Root Group API. Auth is mounted per group below, NOT globally, so a new
	// group must opt in explicitly.
	v1 := e.Group("/api/v1")

	// -------------------------------------------------------------
	// 1. Auth Domain Wiring & Routing
	// -------------------------------------------------------------
	authRepo := repository.NewAccountRepo(db)
	refreshRepo := repository.NewRefreshTokenRepo(db)
	authUsecase := usecase.NewAuthUsecase(authRepo, refreshRepo, cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL)
	authHandler := NewAuthHandler(authUsecase)

	// Public: login/refresh/logout must work without a valid access token.
	authGroup := v1.Group("/auth")
	authGroup.Use(middleware.RateLimit())
	authGroup.Use(middleware.TimeoutMiddleware(5 * time.Second))

	authGroup.POST("/login", authHandler.Login)
	authGroup.POST("/refresh", authHandler.Refresh)
	authGroup.POST("/logout", authHandler.Logout)

	// Protected groups. AuthJWT runs before Timeout so an unauthenticated
	// request never reaches the database.
	requireAuth := middleware.AuthJWT([]byte(cfg.JWTSecret), nil)

	// -------------------------------------------------------------
	// 2. Category Domain Wiring & Routing
	// -------------------------------------------------------------
	categoryRepo := repository.NewCategoryRepository(db)
	categoryUsecase := usecase.NewCategoryUsecase(categoryRepo)
	categoryHandler := NewCategoryHandler(categoryUsecase)

	categoryGroup := v1.Group("/categories")
	categoryGroup.Use(requireAuth)
	categoryGroup.Use(middleware.TimeoutMiddleware(5 * time.Second))

	categoryGroup.GET("", categoryHandler.List)
	categoryGroup.POST("", categoryHandler.Create)
	categoryGroup.GET("/:id", categoryHandler.Get)
	categoryGroup.PATCH("/:id", categoryHandler.Patch)
	categoryGroup.DELETE("/:id", categoryHandler.Delete)

	// -------------------------------------------------------------
	// 3. Product Domain Wiring & Routing
	// -------------------------------------------------------------
	productRepo := repository.NewProductRepo(db)
	productUsecase := usecase.NewProductUsecase(productRepo)
	productHandler := NewProductHandler(productUsecase)

	productGroup := v1.Group("/products")
	productGroup.Use(requireAuth)
	productGroup.Use(middleware.TimeoutMiddleware(5 * time.Second))

	productGroup.GET("", productHandler.List)
	productGroup.POST("", productHandler.Create)
	// Static routes MUST be registered before /:id, otherwise echo matches
	// "low-stock" as a barcode and returns 404.
	productGroup.GET("/low-stock", productHandler.LowStock)
	productGroup.GET("/:id", productHandler.Get)
	productGroup.PATCH("/:id", productHandler.Patch)
	productGroup.DELETE("/:id", productHandler.Delete)
}
