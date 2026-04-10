package service

// ratelimit.go — per-key rate limit helpers.
// The sliding-window enforcement lives in internal/middleware/ratelimit.go.
// This file is reserved for any business-logic wrappers (e.g. per-key overrides)
// that may be needed in the future.
