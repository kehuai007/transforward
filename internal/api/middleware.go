package api

import (
	"net/http"
)

var validTokens = make(map[string]bool)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for login, ws, and static resources
		path := r.URL.Path
		if path == "/api/login" || path == "/ws" || path == "/" || path == "/dist/" {
			next.ServeHTTP(w, r)
			return
		}

		// Skip auth for static resources under /dist/
		if len(path) > 6 && path[:6] == "/dist/" {
			next.ServeHTTP(w, r)
			return
		}

		token := r.Header.Get("Authorization")
		if token == "" {
			token = r.URL.Query().Get("token")
		}

		if token == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// Remove "Bearer " prefix if present
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		if !validTokens[token] {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
