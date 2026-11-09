package api

import (
"net/http"

"github.com/ElzaraA/ai-buddy-avito/internal/auth"
)

func DemoUser(next http.Handler) http.Handler {
return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
u := auth.User{ID: "intern-1", Name: "Стажёр", Role: auth.RoleIntern, TeamID: "backend"}
next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), u)))
})
}
