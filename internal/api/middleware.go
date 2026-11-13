package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/metrics"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// maxBodyBytes — лимит тела запроса: вопрос до 2000 символов с запасом.
const maxBodyBytes = 64 << 10

// DefaultDemoUser — учётка по умолчанию, если заголовок не передан (упрощённая авторизация прототипа).
const DefaultDemoUser = "intern.backend"

// DemoUser имитирует SSO: пользователь берётся из заголовка X-Demo-User
// или из query ?user= (EventSource не умеет передавать заголовки). Неизвестный логин — 401.
func DemoUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Demo-User")
		if key == "" {
			key = r.URL.Query().Get("user")
		}
		if key == "" {
			key = DefaultDemoUser
		}
		u, ok := auth.DemoUsers[key]
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "unknown demo user")
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), u)))
	})
}

// LimitBody ограничивает размер тела запроса.
func LimitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders — базовые защитные заголовки для API.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// Observe пишет структурированный лог запроса (без тела — там могут быть вопросы стажёра),
// возвращает X-Request-Id и считает метрики по шаблону маршрута (а не по сырому пути).
func Observe(m *metrics.Registry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			reqID := middleware.GetReqID(r.Context())
			if reqID != "" {
				ww.Header().Set("X-Request-Id", reqID)
			}
			start := time.Now()
			next.ServeHTTP(ww, r)

			route := "unmatched"
			if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
				route = rc.RoutePattern()
			}
			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			elapsed := time.Since(start)
			m.Inc("buddy_http_requests_total", "HTTP-запросы", "method", r.Method, "route", route, "status", strconv.Itoa(status))
			m.Add("buddy_http_request_duration_seconds_sum", "Суммарное время обработки HTTP-запросов", elapsed.Seconds(), "route", route)
			slog.Info("http", "method", r.Method, "path", r.URL.Path, "status", status, "ms", elapsed.Milliseconds(), "req_id", reqID)
		})
	}
}

// RateLimiter — ограничение числа запросов пользователя в минуту (фиксированное окно).
// Защищает LLM-бюджет и сервер от «залипшей» кнопки и простого флуда.
type RateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	now    func() time.Time
	hits   map[string]*bucket
}

type bucket struct {
	start time.Time
	n     int
}

// NewRateLimiter: limit запросов за window на пользователя. limit <= 0 — без ограничения (nil).
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit <= 0 {
		return nil
	}
	return &RateLimiter{limit: limit, window: window, now: time.Now, hits: map[string]*bucket{}}
}

// allow сообщает, можно ли выполнить запрос, и через сколько секунд повторить, если нельзя.
func (l *RateLimiter) allow(key string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b := l.hits[key]
	if b == nil || now.Sub(b.start) >= l.window {
		l.hits[key] = &bucket{start: now, n: 1}
		return true, 0
	}
	if b.n >= l.limit {
		retry := int(l.window.Seconds()-now.Sub(b.start).Seconds()) + 1
		return false, retry
	}
	b.n++
	return true, 0
}

// Middleware отвечает 429 с Retry-After, если пользователь превысил лимит. nil-лимитер ничего не ограничивает.
func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	if l == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := auth.FromCtx(r.Context())
		if ok, retry := l.allow(u.ID); !ok {
			w.Header().Set("Retry-After", strconv.Itoa(retry))
			writeError(w, http.StatusTooManyRequests, "rate_limited", "Слишком много запросов, попробуйте через "+strconv.Itoa(retry)+" с")
			return
		}
		next.ServeHTTP(w, r)
	})
}
