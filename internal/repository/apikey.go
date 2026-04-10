package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// APIKey mirrors the api_keys table row.
type APIKey struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	OwnerID      string     `json:"owner_id"`
	KeyHash      string     `json:"-"` // never expose in JSON
	Scopes       []string   `json:"scopes"`
	RateLimitRPM int        `json:"rate_limit_rpm"`
	IsActive     bool       `json:"is_active"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// APIKeyRepo provides database access for API keys.
type APIKeyRepo struct {
	pool *pgxpool.Pool
}

func NewAPIKeyRepo(pool *pgxpool.Pool) *APIKeyRepo {
	return &APIKeyRepo{pool: pool}
}

const apiKeyColumns = `
	id::text, name, owner_id, key_hash, scopes,
	rate_limit_rpm, is_active, expires_at, created_at`

func scanAPIKey(row pgx.Row) (*APIKey, error) {
	var k APIKey
	err := row.Scan(
		&k.ID, &k.Name, &k.OwnerID, &k.KeyHash,
		&k.Scopes, &k.RateLimitRPM, &k.IsActive,
		&k.ExpiresAt, &k.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &k, nil
}

// FindByHash looks up an active, non-expired key by its SHA-256 hash.
func (r *APIKeyRepo) FindByHash(ctx context.Context, hash string) (*APIKey, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+apiKeyColumns+`
		FROM api_keys
		WHERE key_hash = $1 AND is_active = true
	`, hash)

	key, err := scanAPIKey(row)
	if err != nil || key == nil {
		return nil, err
	}

	// Honour expiry
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, nil
	}
	return key, nil
}

// Create inserts a new API key row and returns the persisted record.
func (r *APIKeyRepo) Create(ctx context.Context, name, ownerID, keyHash string, scopes []string, rateLimitRPM int, expiresAt *time.Time) (*APIKey, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO api_keys (name, owner_id, key_hash, scopes, rate_limit_rpm, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+apiKeyColumns,
		name, ownerID, keyHash, scopes, rateLimitRPM, expiresAt,
	)
	return scanAPIKey(row)
}

// List returns API keys paginated by page (1-indexed) and limit.
func (r *APIKeyRepo) List(ctx context.Context, page, limit int) ([]APIKey, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	rows, err := r.pool.Query(ctx, `
		SELECT `+apiKeyColumns+`
		FROM api_keys
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(
			&k.ID, &k.Name, &k.OwnerID, &k.KeyHash,
			&k.Scopes, &k.RateLimitRPM, &k.IsActive,
			&k.ExpiresAt, &k.CreatedAt,
		); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// GetByID fetches a single key by its UUID.
func (r *APIKeyRepo) GetByID(ctx context.Context, id string) (*APIKey, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+apiKeyColumns+`
		FROM api_keys WHERE id = $1::uuid
	`, id)
	return scanAPIKey(row)
}

// SetActive enables or disables (soft-deletes) a key.
func (r *APIKeyRepo) SetActive(ctx context.Context, id string, active bool) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE api_keys SET is_active = $1 WHERE id = $2::uuid`,
		active, id,
	)
	return err
}

// UpdateRateLimit changes the per-minute request limit for a key.
func (r *APIKeyRepo) UpdateRateLimit(ctx context.Context, id string, rpm int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE api_keys SET rate_limit_rpm = $1 WHERE id = $2::uuid`,
		rpm, id,
	)
	return err
}
