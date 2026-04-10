package middleware

import "net/http"

// AdminOnly returns a middleware that requires requests to carry the correct
// admin API key in the X-Admin-Key header. Use this to guard all /v1/admin routes.
func AdminOnly(adminKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-Admin-Key")
			if key == "" || key != adminKey {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
