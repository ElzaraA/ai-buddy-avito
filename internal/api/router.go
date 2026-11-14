package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// DefaultOrigins — адреса фронта в dev-режиме (vite) и в docker.
var DefaultOrigins = []string{"http://localhost:5173", "http://127.0.0.1:5173", "http://localhost:3000"}

func NewRouter(h *Handlers) *chi.Mux {
	return NewRouterWithOrigins(h, DefaultOrigins)
}

func NewRouterWithOrigins(h *Handlers, origins []string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(Observe(h.Metrics))
	r.Use(SecurityHeaders)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: origins,
		AllowedMethods: []string{"GET", "POST", "PATCH", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "X-Demo-User", "Authorization", "Last-Event-ID"},
		ExposedHeaders: []string{"X-Request-Id", "Retry-After"},
		MaxAge:         300,
	}))
	r.Use(LimitBody)

	r.Get("/health", h.Health)
	if h.Metrics != nil {
		r.Get("/metrics", h.MetricsHandler)
	}

	r.Group(func(r chi.Router) {
		r.Use(DemoUser)

		r.Route("/api/v1", func(r chi.Router) {
			r.Get("/me", h.Me)

			r.Get("/onboarding/progress", h.OnboardingProgress)
			r.Patch("/onboarding/steps/{id}", h.OnboardingStepUpdate)

			// Действия: черновик → подтверждение → решение руководителя
			r.Get("/actions", h.ListActions)
			r.With(h.WriteLimiter.Middleware).Post("/actions", h.CreateAction)
			r.Get("/actions/{id}", h.GetAction)
			r.Post("/actions/{id}/confirm", h.ConfirmAction)
			r.Post("/actions/{id}/cancel", h.CancelAction)
			r.Post("/actions/{id}/approve", h.ApproveAction)
			r.Post("/actions/{id}/reject", h.RejectAction)

			// Кабинет наставника и руководителя
			r.Get("/team/progress", h.TeamProgress)
			r.Get("/team/actions", h.TeamActions)
			r.Get("/team/knowledge-gaps", h.TeamKnowledgeGaps)
			r.Get("/team/summary", h.TeamSummary)

			if h.Chat != nil {
				r.With(h.WriteLimiter.Middleware).Post("/chat/sessions", h.Chat.CreateSession)
				r.Get("/chat/sessions/{id}/messages", h.Chat.ListMessages)
				r.With(h.Limiter.Middleware).Post("/chat/sessions/{id}/messages", h.Chat.SendMessage)
				r.Get("/chat/sessions/{id}/events", h.Chat.Events)
			}

			r.With(h.Limiter.Middleware).Post("/ask", h.Ask)
		})
		// Синхронный эндпоинт для проверки curl'ом (README).
		r.With(h.Limiter.Middleware).Post("/api/ask", h.Ask)

		// MCP-сервер имитаций корпоративных сервисов (JSON-RPC 2.0).
		r.With(h.WriteLimiter.Middleware).Post("/mcp", h.MCP)
	})
	return r
}
