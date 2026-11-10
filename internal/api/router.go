package api

import (
"github.com/go-chi/chi/v5"
"github.com/go-chi/cors"
)

func NewRouter(h *Handlers) *chi.Mux {
r := chi.NewRouter()
r.Use(cors.Handler(cors.Options{
AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:3000"},
AllowedMethods:   []string{"GET", "POST", "PATCH", "OPTIONS"},
AllowedHeaders:   []string{"Content-Type", "X-Demo-User"},
AllowCredentials: true,
MaxAge:           300,
}))
r.Use(DemoUser)

r.Get("/health", h.Health)

r.Route("/api/v1", func(r chi.Router) {
r.Get("/me", h.Me)
r.Post("/auth/login", h.Me) // упрощённо: логин = /me по X-Demo-User

r.Get("/onboarding/progress", h.OnboardingProgress)
r.Patch("/onboarding/steps/{id}", h.OnboardingStepUpdate)

r.Post("/actions", h.CreateAction)
r.Post("/actions/{id}/confirm", h.ConfirmAction)
r.Get("/actions/{id}", h.GetAction)

r.Get("/events", h.Events)

// обратная совместимость со старым путём
r.Post("/ask", h.Ask)
})

// старый путь без версии
r.Post("/api/ask", h.Ask)

return r
}