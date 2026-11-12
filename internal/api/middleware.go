package api

import (
	"net/http"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
)

func DemoUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Demo-User")
		if key == "" {
			key = r.URL.Query().Get("user")
		}
		if key == "" {
			key = "intern.backend"
		}
		u, ok := auth.DemoUsers[key]
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "unknown demo user")
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), u)))
	})
}
