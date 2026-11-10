package tools

import (
"context"
"strings"

"github.com/ElzaraA/ai-buddy-avito/internal/auth"
)

type DirectoryFind struct{}

func (t *DirectoryFind) Name() string               { return "directory.find_owner" }
func (t *DirectoryFind) RequiredPermission() string { return "directory.read" }

var owners = map[string]string{
"доступ":     "Иван Петров (IT Support)",
"инструмент": "Мария Сидорова (Backend Lead)",
"чек-лист":   "Наставник команды",
"отгул":      "Руководитель команды",
}

func (t *DirectoryFind) Run(ctx context.Context, u auth.User, args map[string]string) (Result, error) {
if !auth.Can(u, t.RequiredPermission()) {
return Result{}, ErrForbidden
}
q := strings.ToLower(args["query"])
for k, v := range owners {
if strings.Contains(q, k) {
return Result{OK: true, Message: "Ответственный найден", Data: map[string]any{"owner": v}}, nil
}
}
return Result{OK: false, Message: "Ответственный не найден"}, nil
}
