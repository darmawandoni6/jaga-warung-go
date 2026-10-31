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

	// -------------------------------------------------------------
	// 4. Transaction Domain Wiring & Routing
	// -------------------------------------------------------------
	transactionRepo := repository.NewTransactionRepo(db)
	transactionItemRepo := repository.NewTransactionItemRepo(db)

	transactionUsecase := usecase.NewTransactionUsecase(transactionRepo)
	transactionItemUsecase := usecase.NewTransactionItemUsecase(transactionItemRepo)

	transactionHandler := NewTransactionHandler(transactionUsecase)
	transactionItemHandler := NewTransactionItemHandler(transactionItemUsecase)

	transactionGroup := v1.Group("/transactions")
	transactionGroup.Use(requireAuth)
	transactionGroup.Use(middleware.TimeoutMiddleware(10 * time.Second))

	// POST is the sale flow, not a plain insert (docs/api.md § Business endpoints).
	transactionGroup.GET("", transactionHandler.List)
	transactionGroup.POST("", transactionHandler.Create)
	transactionGroup.GET("/:id", transactionHandler.Get)
	transactionGroup.PATCH("/:id", transactionHandler.Patch)
	transactionGroup.DELETE("/:id", transactionHandler.Delete)

	transactionItemGroup := v1.Group("/transaction-items")
	transactionItemGroup.Use(requireAuth)
	transactionItemGroup.Use(middleware.TimeoutMiddleware(10 * time.Second))

	transactionItemGroup.GET("", transactionItemHandler.List)
	transactionItemGroup.POST("", transactionItemHandler.Create)
	transactionItemGroup.GET("/:id", transactionItemHandler.Get)
	transactionItemGroup.PATCH("/:id", transactionItemHandler.Patch)
	transactionItemGroup.DELETE("/:id", transactionItemHandler.Delete)

	// -------------------------------------------------------------
	// 5. Debt Domain Wiring & Routing
	// -------------------------------------------------------------
	debtRepo := repository.NewDebtRepository(db)
	debtPaymentRepo := repository.NewDebtPaymentRepository(db)

	debtUsecase := usecase.NewDebtUsecase(debtRepo)
	debtPaymentUsecase := usecase.NewDebtPaymentUsecase(debtPaymentRepo)

	debtHandler := NewDebtHandler(debtUsecase)
	debtPaymentHandler := NewDebtPaymentHandler(debtPaymentUsecase)

	debtGroup := v1.Group("/debts")
	debtGroup.Use(requireAuth)
	debtGroup.Use(middleware.TimeoutMiddleware(10 * time.Second))

	debtGroup.GET("", debtHandler.List)
	debtGroup.POST("", debtHandler.Create)
	debtGroup.GET("/:id", debtHandler.Get)
	debtGroup.PATCH("/:id", debtHandler.Patch)
	debtGroup.DELETE("/:id", debtHandler.Delete)

	debtPaymentGroup := v1.Group("/debt-payments")
	debtPaymentGroup.Use(requireAuth)
	debtPaymentGroup.Use(middleware.TimeoutMiddleware(10 * time.Second))

	// POST is the payment flow, not a plain insert (docs/api.md § Business endpoints).
	debtPaymentGroup.GET("", debtPaymentHandler.List)
	debtPaymentGroup.POST("", debtPaymentHandler.Create)
	debtPaymentGroup.GET("/:id", debtPaymentHandler.Get)
	debtPaymentGroup.PATCH("/:id", debtPaymentHandler.Patch)
	debtPaymentGroup.DELETE("/:id", debtPaymentHandler.Delete)

	// -------------------------------------------------------------
	// 6. Stock Movement Domain Wiring & Routing
	// -------------------------------------------------------------
	stockMovementRepo := repository.NewStockMovementRepo(db)
	stockMovementUsecase := usecase.NewStockMovementUsecase(stockMovementRepo)
	stockMovementHandler := NewStockMovementHandler(stockMovementUsecase)

	stockMovementGroup := v1.Group("/stock-movements")
	stockMovementGroup.Use(requireAuth)
	stockMovementGroup.Use(middleware.TimeoutMiddleware(10 * time.Second))

	// POST applies the delta and appends the movement atomically
	// (docs/api.md § Business endpoints).
	stockMovementGroup.GET("", stockMovementHandler.List)
	stockMovementGroup.POST("", stockMovementHandler.Create)
	stockMovementGroup.GET("/:id", stockMovementHandler.Get)
	stockMovementGroup.PATCH("/:id", stockMovementHandler.Patch)
	stockMovementGroup.DELETE("/:id", stockMovementHandler.Delete)
}
