package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Maheesh09/AI-gateway/internal/repository"
	"github.com/hibiken/asynq"
)

const TaskAnalyzeRequest = "request:analyze"

// AnalyzePayload is the data enqueued after each proxied request
type AnalyzePayload struct {
	APIKeyID   string `json:"api_key_id"`
	RequestID  string `json:"request_id"`
	Path       string `json:"path"`
	Method     string `json:"method"`
	StatusCode int    `json:"status_code"`
	LatencyMs  int    `json:"latency_ms"`
	IPAddress  string `json:"ip_address"`
}

type Worker struct {
	detector  *Detector
	analyzer  *Analyzer
	alertRepo *repository.AlertRepo
	logRepo   *repository.LogRepo
}

func NewWorker(d *Detector, a *Analyzer, alertRepo *repository.AlertRepo, logRepo *repository.LogRepo) *Worker {
	return &Worker{
		detector:  d,
		analyzer:  a,
		alertRepo: alertRepo,
		logRepo:   logRepo,
	}
}

func (w *Worker) HandleAnalyzeTask(ctx context.Context, t *asynq.Task) error {
	var p AnalyzePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	// Log the request to PostgreSQL
	if err := w.logRepo.Insert(ctx, p); err != nil {
		return fmt.Errorf("insert log: %w", err)
	}

	// Run rule-based detector — fast, no external API call
	signal, err := w.detector.Evaluate(ctx, p.APIKeyID)
	if err != nil {
		return fmt.Errorf("detector: %w", err)
	}

	// Only call Claude if rules were triggered
	if !signal.ShouldAnalyze {
		return nil
	}

	result, err := w.analyzer.Analyze(ctx, p.APIKeyID, signal)
	if err != nil {
		// Don't fail the job — store a fallback alert without AI explanation
		return w.alertRepo.Insert(ctx, p.APIKeyID, signal.TriggerType, "MEDIUM",
			"AI analysis unavailable — triggered by rule: "+signal.TriggerType, false)
	}

	return w.alertRepo.Insert(ctx, p.APIKeyID, signal.TriggerType,
		result.Severity, result.Explanation, result.AutoBlock)
}

// EnqueueAnalysis is called from the gateway after each proxied request.
func EnqueueAnalysis(client *asynq.Client, p AnalyzePayload) error {
	payload, err := json.Marshal(p)
	if err != nil {
		return err
	}

	task := asynq.NewTask(TaskAnalyzeRequest, payload,
		asynq.MaxRetry(3),
		asynq.Queue("default"),
	)

	_, err = client.Enqueue(task)
	return err
}
