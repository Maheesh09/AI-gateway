package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	appdb "github.com/Maheesh09/AI-gateway/internal/db"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	redis         *redis.Client // Redis Client
	defaultRPM    int           // Default requests per minute
	windowSeconds int           // Window size in seconds (e.g., 60 for 1 minute)
}

func NewRateLimiter(r *redis.Client, defaultRPM, windowSec int) *RateLimiter {
	return &RateLimiter{
		redis:         r,
		defaultRPM:    defaultRPM,
		windowSeconds: windowSec,
	}
}

func (rl *RateLimiter) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keyID, _ := r.Context().Value(ContextKeyAPIKeyID).(string)
		if keyID == "" {
			next.ServeHTTP(w, r)
			return
		}

		allowed, count, err := rl.check(r.Context(), keyID, rl.defaultRPM)
		if err != nil {
			// Fail open — don't block requests if Redis is down
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.defaultRPM))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(max(0, rl.defaultRPM-count-1)))
		w.Header().Set("X-RateLimit-Window", strconv.Itoa(rl.windowSeconds))

		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(rl.windowSeconds))
			http.Error(w, `{"error":"rate limit exceeded","retry_after":`+strconv.Itoa(rl.windowSeconds)+`}`,
				http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) check(ctx context.Context, keyID string, limit int) (allowed bool, count int, err error) {
	redisKey := fmt.Sprintf("rate_limit:%s", keyID)
	nowMs := time.Now().UnixMilli()
	reqID := uuid.New().String()

	result, err := appdb.SlidingWindowScript.Run(
		ctx, rl.redis,
		[]string{redisKey},
		nowMs,
		rl.windowSeconds,
		limit,
		reqID,
	).Int()

	if err != nil {
		return false, 0, err
	}

	if result == -1 {
		return false, limit, nil
	}

	return true, result, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
