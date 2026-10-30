package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitDB menginisialisasi koneksi GORM PostgreSQL menggunakan DatabaseURL
func InitDB(cfg *Config) (*gorm.DB, error) {

	gormLogLevel := logger.Warn

	if cfg.isDebug {
		gormLogLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		TranslateError: true,
		Logger: logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold:             200 * time.Millisecond, // Log kueri jika berjalan lebih dari 200ms
				LogLevel:                  gormLogLevel,           // Gunakan logger.Warn atau logger.Error
				IgnoreRecordNotFoundError: true,                   // Abaikan log error jika gorm.ErrRecordNotFound
				ParameterizedQueries:      true,                   // Sembunyikan parameter sensitif di log
				Colorful:                  true,
			},
		),
	})
	if err != nil {
		return nil, fmt.Errorf("gagal terhubung ke database via GORM: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("gagal mendapatkan sql.DB dari GORM: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(1 * time.Hour)

	log.Println("Berhasil terhubung ke PostgreSQL menggunakan GORM")
	return db, nil
}
