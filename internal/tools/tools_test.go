package tools

import (
"context"
"testing"

"github.com/ElzaraA/ai-buddy-avito/internal/auth"
"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

func TestChecklistStatus(t *testing.T) {
st := store.New()
st.SeedChecklist("u1", []store.ChecklistItem{{ID: "1", Title: "a", Done: false}})
tool := &ChecklistStatus{S: st}
res, err := tool.Run(context.Background(), auth.User{ID: "u1", Role: auth.RoleIntern}, nil)
if err != nil || !res.OK {
t.Fatal("must succeed")
}
if res.Data["next_step"] != "a" {
t.Fatal("wrong next step")
}
if tool.Name() != "checklist.status" || tool.RequiredPermission() != "checklist.read" {
t.Fatal("wrong meta")
}
}

func TestAbsenceMeta(t *testing.T) {
tool := &AbsenceCreate{S: store.New()}
if tool.Name() != "absence.create" {
t.Fatal("wrong name")
}
if tool.RequiredPermission() != "absence.create" {
t.Fatal("wrong permission")
}
}

func TestAbsenceForbidden(t *testing.T) {
tool := &AbsenceCreate{S: store.New()}
_, err := tool.Run(context.Background(), auth.User{Role: auth.RoleHRBP}, map[string]string{"date_from": "a", "date_to": "b"})
if err != ErrForbidden {
t.Fatal("must be forbidden")
}
}

func TestAbsenceMissingDates(t *testing.T) {
tool := &AbsenceCreate{S: store.New()}
_, err := tool.Run(context.Background(), auth.User{ID: "u1", Role: auth.RoleIntern}, map[string]string{})
if err == nil {
t.Fatal("must require dates")
}
}

func TestAbsenceOK(t *testing.T) {
st := store.New()
tool := &AbsenceCreate{S: st}
res, err := tool.Run(context.Background(), auth.User{ID: "u1", Role: auth.RoleIntern}, map[string]string{"date_from": "2026-02-10", "date_to": "2026-02-10"})
if err != nil || !res.OK {
t.Fatal("must create")
}
if res.Data["status"] != "pending" {
t.Fatal("wrong status")
}
}

func TestDirectoryFind(t *testing.T) {
tool := &DirectoryFind{}
res, err := tool.Run(context.Background(), auth.User{Role: auth.RoleIntern}, map[string]string{"query": "отгул"})
if err != nil || !res.OK {
t.Fatal("must find")
}
res, _ = tool.Run(context.Background(), auth.User{Role: auth.RoleIntern}, map[string]string{"query": "неведомо"})
if res.OK {
t.Fatal("must not find")
}
}

func TestDirectoryForbidden(t *testing.T) {
tool := &DirectoryFind{}
_, err := tool.Run(context.Background(), auth.User{Role: auth.RoleHRBP}, map[string]string{"query": "отгул"})
if err != ErrForbidden {
t.Fatal("must forbid")
}
}

func TestRegistry(t *testing.T) {
r := NewRegistry()
r.Register(&DirectoryFind{})
if _, ok := r.Get("directory.find_owner"); !ok {
t.Fatal("must get")
}
if _, ok := r.Get("unknown"); ok {
t.Fatal("must not get unknown")
}
}