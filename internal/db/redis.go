package db

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	client := redis.NewClient(opts)

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return client, nil
}

// SlidingWindowScript atomically:
// 1. Remove entries older than window
// 2. Count remaining entries
// 3. Add current request if under limit
// 4. Set TTL on the key
// Returns the current count BEFORE adding this request.
var SlidingWindowScript = redis.NewScript(` // atomic read write for sliding window rate limiting
local key    = KEYS[1]
local now    = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit  = tonumber(ARGV[3])
local reqId  = ARGV[4]

local oldest = now - (window * 1000)

redis.call('ZREMRANGEBYSCORE', key, '-inf', oldest)

local count = redis.call('ZCARD', key)

if count < limit then
    redis.call('ZADD', key, now, reqId)
    redis.call('PEXPIRE', key, window * 1000)
    return count
else
    return -1
end
`)
