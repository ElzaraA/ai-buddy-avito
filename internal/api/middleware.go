package api

import (
	"net/http"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
)

// DemoUser берёт пользователя из заголовка X-Demo-User или дефолтного стажёра.
func DemoUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Demo-User")
		u, ok := auth.DemoUsers[key]
		if !ok {
			u = auth.DemoUsers["intern.backend"]
		}
		next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), u)))
	})
}
