package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/Maheesh09/AI-gateway/internal/repository"
)

const TaskAnalyzeRequest = "request:analyze"

// AnalyzePayload is serialised and enqueued by the gateway after every
// successfully proxied request.
type AnalyzePayload struct {
	APIKeyID   string `json:"api_key_id"`
	RouteID    string `json:"route_id"`
	RequestID  string `json:"request_id"`
	Path       string `json:"path"`
	Method     string `json:"method"`
	StatusCode int    `json:"status_code"`
	LatencyMs  int    `json:"latency_ms"`
	IPAddress  string `json:"ip_address"`
}

// Worker processes analysis jobs from the asynq queue.
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

// HandleAnalyzeTask is the asynq handler for TaskAnalyzeRequest jobs.
func (w *Worker) HandleAnalyzeTask(ctx context.Context, t *asynq.Task) error {
	var p AnalyzePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	// Persist request log to PostgreSQL
	if err := w.logRepo.Insert(ctx, repository.LogEntry{
		APIKeyID:   p.APIKeyID,
		RouteID:    p.RouteID,
		Method:     p.Method,
		Path:       p.Path,
		StatusCode: p.StatusCode,
		LatencyMs:  p.LatencyMs,
		IPAddress:  p.IPAddress,
	}); err != nil {
		return fmt.Errorf("insert log: %w", err)
	}

	// Rule-based detector — no external API call
	signal, err := w.detector.Evaluate(ctx, p.APIKeyID)
	if err != nil {
		return fmt.Errorf("detector: %w", err)
	}

	if !signal.ShouldAnalyze {
		return nil
	}

	// Escalate to Claude only when rules fire
	result, err := w.analyzer.Analyze(ctx, p.APIKeyID, signal)
	if err != nil {
		// Fault-tolerant fallback: store a rule-only alert so nothing is lost
		return w.alertRepo.Insert(ctx, p.APIKeyID, signal.TriggerType, "MEDIUM",
			"AI analysis unavailable — triggered by rule: "+signal.TriggerType, false)
	}

	return w.alertRepo.Insert(ctx, p.APIKeyID, signal.TriggerType,
		result.Severity, result.Explanation, result.AutoBlock)
}

// EnqueueAnalysis is called by the gateway to dispatch an async analysis job.
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
