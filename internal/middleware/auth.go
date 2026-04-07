package middleware

import (
	"context"       // Used for passing authentication information (like API key ID and owner ID) down the request handling chain without modifying function signatures.
	"crypto/sha256" // Used for hashing API keys before storing/comparing them, ensuring that plaintext keys are never exposed in the database or logs.
	"encoding/hex"
	"net/http"
	"strings" // Used for string manipulation

	"github.com/Maheesh09/AI-gateway/internal/repository"
	"github.com/golang-jwt/jwt/v5" // 3rd party library for handling JWTs, used to validate Bearer tokens in the Authorization header.
)

type contextKey string

const (
	ContextKeyAPIKeyID contextKey = "api_key_id"
	ContextKeyOwnerID  contextKey = "owner_id"
)

type AuthMiddleware struct {
	jwtSecret  []byte
	apiKeyRepo *repository.APIKeyRepo
}

func NewAuth(jwtSecret string, repo *repository.APIKeyRepo) *AuthMiddleware {
	return &AuthMiddleware{
		jwtSecret:  []byte(jwtSecret),
		apiKeyRepo: repo,
	}
}

func (m *AuthMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Try Bearer JWT first (for user-facing clients)
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			if keyID, ownerID, ok := m.validateJWT(tokenStr); ok {
				ctx := context.WithValue(r.Context(), ContextKeyAPIKeyID, keyID)
				ctx = context.WithValue(ctx, ContextKeyOwnerID, ownerID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// Try X-API-Key header (servie to service)
		rawKey := r.Header.Get("X-API-Key")
		if rawKey != "" {
			if keyID, ownerID, ok := m.validateAPIKey(r.Context(), rawKey); ok {
				ctx := context.WithValue(r.Context(), ContextKeyAPIKeyID, keyID)
				ctx = context.WithValue(ctx, ContextKeyOwnerID, ownerID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	})
}

func (m *AuthMiddleware) validateJWT(tokenStr string) (keyID, ownerID string, ok bool) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return m.jwtSecret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if err != nil || !token.Valid {
		return "", "", false
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", false
	}

	return claims["key_id"].(string), claims["owner_id"].(string), true
}

func (m *AuthMiddleware) validateAPIKey(ctx context.Context, rawKey string) (keyID, ownerID string, ok bool) {
	// Hash the raw key — never compare plaintext
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	key, err := m.apiKeyRepo.FindByHash(ctx, keyHash)
	if err != nil || !key.IsActive {
		return "", "", false
	}

	return key.ID, key.OwnerID, true
}
