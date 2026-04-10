package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AlertRepo provides persistence for anomaly alerts.
type AlertRepo struct {
	pool *pgxpool.Pool
}

func NewAlertRepo(pool *pgxpool.Pool) *AlertRepo {
	return &AlertRepo{pool: pool}
}

// Insert stores a new anomaly alert. If autoBlock is true the triggering key
// is also disabled in the same transaction.
func (r *AlertRepo) Insert(ctx context.Context, apiKeyID, triggerType, severity, explanation string, autoBlock bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, `
		INSERT INTO anomaly_alerts
			(api_key_id, severity, trigger_type, ai_explanation, auto_blocked)
		VALUES
			($1::uuid, $2::alert_severity, $3, $4, $5)
	`, apiKeyID, severity, triggerType, explanation, autoBlock)
	if err != nil {
		return err
	}

	if autoBlock {
		if _, err = tx.Exec(ctx,
			`UPDATE api_keys SET is_active = false WHERE id = $1::uuid`, apiKeyID,
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// List returns alerts, optionally filtered by severity and resolved status.
// severity = "" means all severities; resolved = nil means all statuses.
func (r *AlertRepo) List(ctx context.Context, severity string, resolved *bool) ([]AnomalyAlert, error) {
	args := []interface{}{}
	conditions := []string{}
	idx := 1

	if severity != "" {
		conditions = append(conditions, fmt.Sprintf("severity::text = $%d", idx))
		args = append(args, severity)
		idx++
	}

	if resolved != nil {
		if *resolved {
			conditions = append(conditions, "resolved_at IS NOT NULL")
		} else {
			conditions = append(conditions, "resolved_at IS NULL")
		}
	}

	query := `
		SELECT
			id::text, api_key_id::text, severity::text,
			trigger_type, COALESCE(ai_explanation, ''),
			auto_blocked, resolved_at, created_at
		FROM anomaly_alerts`

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC LIMIT 200"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []AnomalyAlert
	for rows.Next() {
		var a AnomalyAlert
		if err := rows.Scan(
			&a.ID, &a.APIKeyID, &a.Severity,
			&a.TriggerType, &a.AIExplanation,
			&a.AutoBlocked, &a.ResolvedAt, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

// GetByID fetches a single alert.
func (r *AlertRepo) GetByID(ctx context.Context, id string) (*AnomalyAlert, error) {
	var a AnomalyAlert
	err := r.pool.QueryRow(ctx, `
		SELECT
			id::text, api_key_id::text, severity::text,
			trigger_type, COALESCE(ai_explanation, ''),
			auto_blocked, resolved_at, created_at
		FROM anomaly_alerts WHERE id = $1::uuid
	`, id).Scan(
		&a.ID, &a.APIKeyID, &a.Severity,
		&a.TriggerType, &a.AIExplanation,
		&a.AutoBlocked, &a.ResolvedAt, &a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// Resolve marks an alert as resolved and optionally re-enables its API key.
func (r *AlertRepo) Resolve(ctx context.Context, id string, reEnableKey bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var apiKeyID string
	err = tx.QueryRow(ctx, `
		UPDATE anomaly_alerts
		SET resolved_at = NOW()
		WHERE id = $1::uuid AND resolved_at IS NULL
		RETURNING api_key_id::text
	`, id).Scan(&apiKeyID)
	if err != nil {
		return err
	}

	if reEnableKey && apiKeyID != "" {
		if _, err = tx.Exec(ctx,
			`UPDATE api_keys SET is_active = true WHERE id = $1::uuid`, apiKeyID,
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
