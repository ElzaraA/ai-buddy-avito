package tools

import (
"context"
"errors"
"time"

"github.com/ElzaraA/ai-buddy-avito/internal/auth"
"github.com/ElzaraA/ai-buddy-avito/internal/store"
"github.com/google/uuid"
)

type AbsenceCreateRequest struct{ S *store.Store }

func (t *AbsenceCreateRequest) Name() string               { return "absence_create_request" }
func (t *AbsenceCreateRequest) RequiredPermission() string { return "absence.create" }

func (t *AbsenceCreateRequest) Run(ctx context.Context, u auth.User, args map[string]string) (Result, error) {
if !auth.Can(u, t.RequiredPermission()) {
return Result{}, ErrForbidden
}
from, to := args["dateFrom"], args["dateTo"]
if from == "" || to == "" {
from, to = args["date_from"], args["date_to"]
}
if from == "" || to == "" {
return Result{}, errors.New("dateFrom and dateTo required")
}
a := store.Absence{
ID: uuid.NewString(), UserID: u.ID,
DateFrom: from, DateTo: to, Reason: args["reason"],
Status: "pending", CreatedAt: time.Now(),
}
t.S.CreateAbsence(a)
return Result{
OK:      true,
Message: "Заявка на отгул создана и отправлена руководителю",
Data:    map[string]any{"absence_id": a.ID, "status": a.Status, "dateFrom": from, "dateTo": to},
}, nil
}