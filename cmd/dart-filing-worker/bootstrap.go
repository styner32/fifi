package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fifi/internal/dartfiling/tasks"
	"github.com/hibiken/asynq"
)

type taskEnqueuer interface {
	Enqueue(*asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
}

// Restarting a worker must preserve queued work and must not trigger a new paid
// fetch/analyze batch implicitly. Bootstrap remains available as an explicit opt-in.
func bootstrapFetchReports(client taskEnqueuer, setting string) error {
	setting = strings.TrimSpace(setting)
	if setting == "" {
		return nil
	}
	enabled, err := strconv.ParseBool(setting)
	if err != nil {
		return fmt.Errorf("DART_WORKER_BOOTSTRAP_FETCH_REPORTS must be a boolean: %w", err)
	}
	if !enabled {
		return nil
	}
	task, err := tasks.NewFetchReportsTask(nil, nil)
	if err != nil {
		return err
	}
	_, err = client.Enqueue(task, asynq.Queue("default"), asynq.Timeout(2*time.Hour))
	return err
}
