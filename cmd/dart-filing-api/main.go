package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/fifi/internal/dartfiling/config"
	"github.com/fifi/internal/dartfiling/routes"
	"github.com/fifi/internal/db"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := db.InitDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	var result map[string]interface{}
	if err := db.Raw("SELECT 1").Scan(&result).Error; err != nil {
		log.Fatalf("Failed to check database connection: %v", err)
	}

	router := routes.SetupRouter(db, cfg)

	serverAddr := fmt.Sprintf(":%s", "8080")
	log.Printf("Starting server on %s", serverAddr)

	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v", err)
	}
}
