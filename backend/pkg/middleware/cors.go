package middleware

import (
	"net/http"
	"strings"
)

// CORS middleware xử lý CORS headers với configurable allowed origins
func CORS(allowedOrigins []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowedOrigin := ""

		// Nếu không có allowedOrigins config, fallback to * (backward compatible)
		if len(allowedOrigins) == 0 {
			allowedOrigin = "*"
		} else {
			// Check if request origin matches any allowed origin
			if origin != "" {
				for _, allowed := range allowedOrigins {
					if matchesOrigin(origin, allowed) {
						allowedOrigin = origin
						break
					}
				}
			}
			// If no match and origins list is not empty, don't set header (reject)
			if allowedOrigin == "" {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Correlation-ID")
		w.Header().Set("Access-Control-Expose-Headers", "X-Correlation-ID")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "3600")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// matchesOrigin checks if request origin matches allowed origin pattern
// Supports exact match and wildcard patterns like "https://*.example.com"
func matchesOrigin(origin, pattern string) bool {
	// Exact match
	if origin == pattern {
		return true
	}

	// Wildcard pattern matching (e.g., "https://*.example.com")
	if strings.Contains(pattern, "*") {
		// Replace * with .* for regex-like matching
		// Simple implementation: check prefix and suffix
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]
			return strings.HasPrefix(origin, prefix) && strings.HasSuffix(origin, suffix)
		}
	}

	return false
}
