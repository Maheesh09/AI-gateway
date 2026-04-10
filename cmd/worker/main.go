package main

import (
	"log"

	"github.com/hibiken/asynq"

	"github.com/Maheesh09/AI-gateway/internal/ai"
	"github.com/Maheesh09/AI-gateway/internal/config"
	"github.com/Maheesh09/AI-gateway/internal/db"
	"github.com/Maheesh09/AI-gateway/internal/repository"
)

func main() {
	cfg := config.Load()

	pool, err := db.NewPool(cfg.DBUrl)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	logRepo := repository.NewLogRepo(pool)
	alertRepo := repository.NewAlertRepo(pool)

	detector := ai.NewDetector(logRepo)
	analyzer := ai.NewAnalyzer(cfg.AnthropicAPIKey, cfg.AnthropicModel)
	worker := ai.NewWorker(detector, analyzer, alertRepo, logRepo)

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr},
		asynq.Config{
			Concurrency: 5,
			Queues:      map[string]int{"default": 10},
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(ai.TaskAnalyzeRequest, worker.HandleAnalyzeTask)

	log.Println("AI worker started")
	if err := srv.Run(mux); err != nil {
		log.Fatalf("worker: %v", err)
	}
}
