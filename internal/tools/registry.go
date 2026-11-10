package tools

import (
"context"
"errors"
"time"

"github.com/ElzaraA/ai-buddy-avito/internal/auth"
)

type Result struct {
OK      bool           `json:"ok"`
Message string         `json:"message"`
Data    map[string]any `json:"data,omitempty"`
}

type Tool interface {
Name() string
RequiredPermission() string
Run(ctx context.Context, user auth.User, args map[string]string) (Result, error)
}

type Registry struct {
tools map[string]Tool
}

func NewRegistry() *Registry { return &Registry{tools: map[string]Tool{}} }

func (r *Registry) Register(t Tool) { r.tools[t.Name()] = t }

func (r *Registry) Get(name string) (Tool, bool) {
t, ok := r.tools[name]
return t, ok
}

// Run выполняет инструмент с таймаутом 10с.
func Run(ctx context.Context, t Tool, u auth.User, args map[string]string) (Result, error) {
ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
defer cancel()
return t.Run(ctx, u, args)
}

var ErrForbidden = errors.New("forbidden")