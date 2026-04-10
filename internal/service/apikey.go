package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/Maheesh09/AI-gateway/internal/repository"
)

// APIKeyService handles business logic for API key lifecycle.
type APIKeyService struct {
	repo *repository.APIKeyRepo
}

func NewAPIKeyService(repo *repository.APIKeyRepo) *APIKeyService {
	return &APIKeyService{repo: repo}
}

// CreateKeyInput holds the desired properties for a new API key.
type CreateKeyInput struct {
	Name         string     `json:"name"`
	OwnerID      string     `json:"owner_id"`
	Scopes       []string   `json:"scopes"`
	RateLimitRPM int        `json:"rate_limit_rpm"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
}

// CreateKeyResult bundles the once-visible raw key with the stored record.
type CreateKeyResult struct {
	RawKey string            `json:"key"`   // shown only at creation; never stored
	Key    *repository.APIKey `json:"details"`
}

// Create generates a cryptographically-random key, hashes it, stores only
// the hash, and returns the raw key for one-time display to the caller.
func (s *APIKeyService) Create(ctx context.Context, input CreateKeyInput) (*CreateKeyResult, error) {
	// 32 random bytes → 64-char hex string
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	rawKey := "gw_" + hex.EncodeToString(raw) // "gw_" prefix makes gateway keys identifiable

	// SHA-256 hash — only this goes to the DB
	sum := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(sum[:])

	rpm := input.RateLimitRPM
	if rpm <= 0 {
		rpm = 60
	}

	key, err := s.repo.Create(ctx, input.Name, input.OwnerID, keyHash, input.Scopes, rpm, input.ExpiresAt)
	if err != nil {
		return nil, err
	}

	return &CreateKeyResult{RawKey: rawKey, Key: key}, nil
}
