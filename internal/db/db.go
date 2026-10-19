package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitDB(dsn string) (*gorm.DB, error) {
	return InitDBContext(context.Background(), dsn)
}

func InitDBContext(ctx context.Context, dsn string) (*gorm.DB, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               logger.New(log.New(os.Stderr, "", log.LstdFlags), logger.Config{LogLevel: logger.Warn}),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("database handle: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}
