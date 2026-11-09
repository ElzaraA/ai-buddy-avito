package tools

import (
"context"
"errors"
"time"

"github.com/ElzaraA/ai-buddy-avito/internal/auth"
"github.com/ElzaraA/ai-buddy-avito/internal/store"
"github.com/google/uuid"
)

type AbsenceCreate struct{ S *store.Store }

func (t *AbsenceCreate) Name() string               { return "absence.create" }
func (t *AbsenceCreate) RequiredPermission() string { return "absence.create" }

func (t *AbsenceCreate) Run(ctx context.Context, u auth.User, args map[string]string) (Result, error) {
if !auth.Can(u, t.RequiredPermission()) {
return Result{}, ErrForbidden
}
from, to := args["date_from"], args["date_to"]
if from == "" || to == "" {
return Result{}, errors.New("date_from and date_to required")
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
Data:    map[string]any{"absence_id": a.ID, "status": a.Status, "date_from": from, "date_to": to},
}, nil
}
