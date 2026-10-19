package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/fifi/internal/dartfiling/config"
	"github.com/fifi/internal/dartfiling/tasks"
	"github.com/fifi/internal/db"

	"github.com/hibiken/asynq"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	db, err := db.InitDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Worker connected to database.")

	redisOpt, err := asynq.ParseRedisURI(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}

	asynqClient := asynq.NewClient(redisOpt)
	defer asynqClient.Close()

	scheduler := asynq.NewScheduler(redisOpt, &asynq.SchedulerOpts{})
	fetchCompaniesTask, err := tasks.NewFetchCompaniesTask()
	if err != nil {
		log.Fatalf("Failed to create fetch companies task: %v", err)
	}

	// Daily company refresh at midnight in the scheduler timezone
	entryID, err := scheduler.Register("0 0 * * *", fetchCompaniesTask, asynq.Queue("default"))
	if err != nil {
		log.Fatalf("Failed to register periodic task: %v", err)
	}
	log.Printf("Registered periodic task: %s (EntryID: %s)", fetchCompaniesTask.Type(), entryID)

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			// Specify different queues or priorities if needed
			Queues: map[string]int{
				"default": 1,
			},
			Concurrency: 1, // Max 10 concurrent jobs
		},
	)

	taskProcessor := tasks.NewTaskProcessor(db, cfg)

	mux := asynq.NewServeMux()
	mux.HandleFunc(
		tasks.TypeTaskFetchReports,
		taskProcessor.HandleFetchReportsTask,
	)

	mux.HandleFunc(
		tasks.TypeTaskFetchCompanies,
		taskProcessor.HandleFetchCompaniesTask,
	)

	if err := bootstrapFetchReports(asynqClient, os.Getenv("DART_WORKER_BOOTSTRAP_FETCH_REPORTS")); err != nil {
		log.Fatalf("Failed to enqueue bootstrap fetch: %v", err)
	}

	go func() {
		log.Println("Starting Asynq scheduler...")
		if err := scheduler.Run(); err != nil {
			log.Fatalf("Could not run Asynq scheduler: %v", err)
		}
	}()

	go func() {
		log.Println("Starting Asynq worker server...")
		if err := srv.Run(mux); err != nil {
			log.Fatalf("Could not run Asynq worker server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	log.Println("Shutdown signal received, shutting down gracefully...")

	scheduler.Shutdown()
	log.Println("Asynq scheduler shut down.")

	srv.Shutdown()
	log.Println("Asynq worker server shut down.")

	asynqClient.Close()
	log.Println("Asynq client closed.")

	log.Println("Worker process shut down complete.")
}
