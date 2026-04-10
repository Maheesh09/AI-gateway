package repository

import "time"

// RequestStats represents aggregated request metrics returned by LogRepo.
type RequestStats struct {
	TotalRequests  int
	ErrorCount     int
	ErrorRate      float64
	UniqueIPs      int
	RequestsPerMin float64
	Window         time.Duration
}
