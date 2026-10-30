package main

import (
	"context"
	"fmt"
	"jaga-warung-go/internal/config"
	httpDelivery "jaga-warung-go/internal/delivery/http"
	"jaga-warung-go/internal/repository"
	"jaga-warung-go/pkg/logger"

	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// Init Logger & Load Config (.env)
	cfg := config.LoadConfig()
	logger.InitLogger(cfg)

	// Inisialisasi Database via GORM menggunakan cfg.DatabaseURL
	db, err := config.InitDB(cfg)
	if err != nil {
		log.Fatalf("Gagal inisialisasi database: %v", err)
	}

	// Sync Database Schema
	if err := repository.SyncDatabase(db); err != nil {
		log.Fatalf("Gagal sync database: %v", err)
	}

	// Seed Data
	repository.RunSeeders(db)

	// Ambil instance sql.DB untuk kebutuhan Graceful Shutdown / close connection
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Gagal mendapatkan sql.DB instance: %v", err)
	}
	defer sqlDB.Close()

	// Setup Echo Engine & Middleware
	e := echo.New()

	e.HTTPErrorHandler = httpDelivery.CustomHTTPErrorHandler
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	// Register Router & Pass DB ke SetupRouter
	httpDelivery.SetupRouter(e, db, cfg)

	// Setup net/http Server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      e,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	// Run Server di goroutine
	go func() {
		log.Printf("Server berjalan di port %s...", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Mematikan server secara bertahap...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Shutdown forced: %v", err)
	}

	log.Println("Server berhasil dihentikan secara aman")
}
