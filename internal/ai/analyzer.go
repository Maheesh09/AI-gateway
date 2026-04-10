package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type AnalysisResult struct {
	Severity    string `json:"severity"`    // LOW, MEDIUM, HIGH, CRITICAL
	Explanation string `json:"explanation"` // human-readable summary
	AutoBlock   bool   `json:"auto_block"`  // true if CRITICAL
}

type Analyzer struct {
	apiKey string
	model  string
	client *http.Client
}

func NewAnalyzer(apiKey, model string) *Analyzer {
	return &Analyzer{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *Analyzer) Analyze(ctx context.Context, apiKeyID string, signal *AnomalySignal) (*AnalysisResult, error) {
	stats := signal.RecentStats

	prompt := fmt.Sprintf(`You are a security analyst reviewing API gateway traffic anomalies.

Analyze the following traffic data for API key ID: %s
Trigger type: %s

Traffic stats (last %s):
- Total requests: %d
- Error count: %d  
- Error rate: %.1f%%
- Unique IP addresses: %d
- Requests per minute: %.1f

Respond ONLY with a JSON object in this exact format (no other text):
{
  "severity": "LOW|MEDIUM|HIGH|CRITICAL",
  "explanation": "2-3 sentence plain English explanation of what you see and why it is or isn't a concern",
  "auto_block": false
}

Set auto_block to true only if severity is CRITICAL.`,
		apiKeyID,
		signal.TriggerType,
		stats.Window,
		stats.TotalRequests,
		stats.ErrorCount,
		stats.ErrorRate*100,
		stats.UniqueIPs,
		stats.RequestsPerMin,
	)

	body, err := json.Marshal(map[string]any{
		"model":      a.model,
		"max_tokens": 300,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.anthropic.com/v1/messages",
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()

	var claudeResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&claudeResp); err != nil {
		return nil, err
	}

	if len(claudeResp.Content) == 0 {
		return nil, fmt.Errorf("empty response from Claude")
	}

	var result AnalysisResult
	if err := json.Unmarshal([]byte(claudeResp.Content[0].Text), &result); err != nil {
		return nil, fmt.Errorf("parse claude json: %w", err)
	}

	return &result, nil
}
