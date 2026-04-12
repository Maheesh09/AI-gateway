package ai

import (
	"context"
	"time"

	"github.com/Maheesh09/AI-gateway/internal/repository"
)

type AnomalySignal struct {
	ShouldAnalyze bool
	TriggerType   string
	RecentStats   *repository.RequestStats
}

type Detector struct {
	logRepo *repository.LogRepo
}

func NewDetector(repo *repository.LogRepo) *Detector {
	return &Detector{logRepo: repo}
}

// Evaluate inspects recent request history for a given API key.
// Returns a signal indicating whether the AI analyzer should run.
func (d *Detector) Evaluate(ctx context.Context, apiKeyID string) (*AnomalySignal, error) {
	window := 1 * time.Minute
	since := time.Now().Add(-window)

	stats, err := d.logRepo.GetStats(ctx, apiKeyID, since)
	if err != nil {
		return nil, err
	}

	signal := &AnomalySignal{RecentStats: stats}

	switch {
	// Sudden burst: more than 200 requests in 5 minutes
	case stats.RequestsPerMin > 40:
		signal.ShouldAnalyze = true
		signal.TriggerType = "burst_traffic"

	// High error rate: >30% 4xx/5xx in last 5 minutes (min 20 requests)
	case stats.TotalRequests >= 20 && stats.ErrorRate > 0.30:
		signal.ShouldAnalyze = true
		signal.TriggerType = "error_spike"

	// Scanning pattern: many unique IPs in short window (unusual for a single API key)
	case stats.UniqueIPs > 20 && stats.TotalRequests < 50:
		signal.ShouldAnalyze = true
		signal.TriggerType = "scan_pattern"
	}

	return signal, nil
}
