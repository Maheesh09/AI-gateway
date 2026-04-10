package middleware

import (
	"log"
	"net/http"
	"time"
)

// Logger is a minimal structured request logger that wraps the response writer
// to capture the status code after the handler chain completes.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		lw := &logWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(lw, r)

		log.Printf(
			"method=%s path=%s status=%d latency=%s ip=%s",
			r.Method,
			r.URL.Path,
			lw.status,
			time.Since(start).Round(time.Millisecond),
			r.RemoteAddr,
		)
	})
}

// logWriter captures the HTTP status code written by a downstream handler.
type logWriter struct {
	http.ResponseWriter
	status int
}

func (lw *logWriter) WriteHeader(status int) {
	lw.status = status
	lw.ResponseWriter.WriteHeader(status)
}
