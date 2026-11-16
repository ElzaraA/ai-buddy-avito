# Покрытие бэкенда (Go)

Пересчитано 16.11.2026 командой:

```bash
go test -count=1 -coverprofile=coverage/coverage.out ./...
go tool cover -func=coverage/coverage.out     # итог — последняя строка total
go tool cover -html=coverage/coverage.out -o coverage/index.html
```

**Итого: 89,6% операторов** (`total: (statements) 89.6%`). Порог CI — 80%.

| Пакет | Покрытие |
|---|---|
| `cmd/server` | 92,3% |
| `internal/agent` | 90,5% |
| `internal/api` | 92,2% |
| `internal/auth` | 97,1% |
| `internal/config` | 96,7% |
| `internal/llm` | 95,7% |
| `internal/metrics` | 100,0% |
| `internal/rag` | 96,7% |
| `internal/store` | 80,9% |
| `internal/tools` | 87,0% |

Все 10 пакетов — `ok`. Замер сделан во время параллельной доработки кода (новые функции «Сбросить демо», ответ наставника на сигнал и др.), поэтому цифра ниже прежней 97,2%: новый код добавлен раньше, чем тесты к нему. Перед финальной сдачей пересчитать той же командой.

Файлы: `coverage.out` — профиль, `index.html` — построчный отчёт.
