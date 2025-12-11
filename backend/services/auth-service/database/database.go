package database

import (
	cfgpkg "github.com/nexusdeploy/backend/pkg/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open mở kết nối GORM đến Postgres
func Open(cfg *cfgpkg.Config) (*gorm.DB, error) {
	dsn := cfg.GetDSN()
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
}

// AutoMigrate chạy migrate cho các models
func AutoMigrate(db *gorm.DB, models ...interface{}) error {
	return db.AutoMigrate(models...)
}
