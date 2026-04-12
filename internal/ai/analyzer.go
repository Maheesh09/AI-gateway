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
	if a.model == "mock" {
		return a.mockAnalyze(signal)
	}

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

	if resp.StatusCode >= 400 {
		var errData struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errData); err != nil {
			return nil, fmt.Errorf("anthropic error status %d (could not decode error body)", resp.StatusCode)
		}
		return nil, fmt.Errorf("anthropic error (%s): %s", errData.Error.Type, errData.Error.Message)
	}

	var claudeResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&claudeResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(claudeResp.Content) == 0 {
		return nil, fmt.Errorf("empty content from Claude response")
	}

	var result AnalysisResult
	if err := json.Unmarshal([]byte(claudeResp.Content[0].Text), &result); err != nil {
		return nil, fmt.Errorf("parse claude json: %w", err)
	}
	return &result, nil
}

func (a *Analyzer) mockAnalyze(signal *AnomalySignal) (*AnalysisResult, error) {
	// Simulate the thinking process
	time.Sleep(500 * time.Millisecond)

	switch signal.TriggerType {
	case "burst_traffic":
		return &AnalysisResult{
			Severity:    "HIGH",
			Explanation: "I am observing a sudden spike in traffic from a single API key. This pattern is often associated with automated scrapers or potential DoS attempts. The rate has exceeded the 40 RPS threshold in a 1-minute window.",
			AutoBlock:   false,
		}, nil
	case "error_spike":
		return &AnalysisResult{
			Severity:    "MEDIUM",
			Explanation: "The API key is producing an unusually high volume of 4xx/5xx responses. This could indicate a client-side integration bug or an attempt to probe for non-existent resources (fuzzing).",
			AutoBlock:   false,
		}, nil
	case "scan_pattern":
		return &AnalysisResult{
			Severity:    "CRITICAL",
			Explanation: "CRITICAL: Detected requests for this API key arriving from over 20 unique IP addresses in a very short window. This strongly suggests a distributed scanning tool or compromised key. Recommending immediate suspension.",
			AutoBlock:   true,
		}, nil
	default:
		return &AnalysisResult{
			Severity:    "LOW",
			Explanation: "Minor traffic anomaly detected by rules, but the pattern does not match known malicious signatures. Monitoring recommended.",
			AutoBlock:   false,
		}, nil
	}
}
