package repository

import "context"

// AlertRepo provides persistence for alerts triggered by AI analysis or rules.
type AlertRepo struct{}

// Insert stores an alert. Keep implementation minimal for now.
func (r *AlertRepo) Insert(ctx context.Context, apiKeyID, triggerType, severity, message string, autoBlock bool) error {
	// TODO: persist alert to alerts table
	return nil
}
