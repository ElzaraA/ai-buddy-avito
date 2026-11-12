package tools

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
)

var accessSeq int64

type AccessRequest struct{}

func (t *AccessRequest) Name() string               { return "access_request" }
func (t *AccessRequest) RequiredPermission() string { return "access.request" }
func (t *AccessRequest) RequiresConfirmation() bool { return true }

func (t *AccessRequest) Run(ctx context.Context, u auth.User, args map[string]string) (Result, error) {
	if !auth.Can(u, t.RequiredPermission()) {
		return Result{}, ErrForbidden
	}
	resource := args["resource"]
	if resource == "" {
		resource = "unknown"
	}
	id := fmt.Sprintf("REQ-%06d", atomic.AddInt64(&accessSeq, 1))
	return Result{
		OK:      true,
		Message: "Заявка на доступ создана",
		Data: map[string]any{
			"request_id": id,
			"resource":   resource,
			"status":     "pending",
			"owner":      "Олег Смирнов (IT Support)",
			"created_at": time.Now().Format(time.RFC3339),
		},
	}, nil
}
