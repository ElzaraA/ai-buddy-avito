package auth

import (
"context"
"testing"
)

func TestCan(t *testing.T) {
intern := User{Role: RoleIntern}
if !Can(intern, "absence.create") {
t.Fatal("intern must create absence")
}
hr := User{Role: RoleHRBP}
if Can(hr, "absence.create") {
t.Fatal("hr must not create absence")
}
if !Can(hr, "absence.read.all") {
t.Fatal("hr must read all")
}
if Can(intern, "absence.read.all") {
t.Fatal("intern must not read all")
}
if !Can(intern, "checklist.read") {
t.Fatal("everyone reads checklist")
}
if !Can(User{Role: RoleLead}, "directory.read") {
t.Fatal("lead reads directory")
}
if Can(User{Role: RoleHRBP}, "directory.read") {
t.Fatal("hrbp not in directory")
}
if Can(intern, "unknown.action") {
t.Fatal("unknown must be forbidden")
}
}

func TestContext(t *testing.T) {
u := User{ID: "1", Role: RoleIntern}
ctx := WithUser(context.Background(), u)
got, ok := FromCtx(ctx)
if !ok || got.ID != "1" {
t.Fatal("must roundtrip")
}
if _, ok := FromCtx(context.Background()); ok {
t.Fatal("empty ctx must fail")
}
}