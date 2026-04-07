package repository

import "context"

type APIKey struct {
	ID      string
	OwnerID string
	IsActive bool
}

type APIKeyRepo struct {
}

func (r *APIKeyRepo) FindByHash(ctx context.Context, hash string) (*APIKey, error) {
	return nil, nil // TODO: implement
}
