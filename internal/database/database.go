package database

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect opens the pool. TranslateError turns driver errors into gorm's
// ErrDuplicatedKey / ErrForeignKeyViolated so handler.DBError can map them.
func Connect(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		TranslateError: true,
		Logger:         logger.Default.LogMode(logger.Warn),
	})
}

// Migrate is AutoMigrate until the schema freezes; then versioned SQL.
func Migrate(db *gorm.DB, models ...any) error {
	return db.AutoMigrate(models...)
}
