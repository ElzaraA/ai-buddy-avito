# AI-buddy — ИИ-наставник для стажёров (Hack&Change 2026)

Прототип ИИ-помощника для стажёра Backend-команды Авито. Помогает разобраться в процессах и инструментах, отвечает на рабочие вопросы и выполняет действия через корпоративные сервисы (имитации).

## Пользователь

Основной пользователь — **Backend-стажёр** в первые недели онбординга. Вспомогательные роли: наставник (buddy), руководитель, HR BP.

## Структура

```
cmd/server/        entrypoint
internal/config/   конфиг + загрузка .env
internal/auth/     роли и права (RBAC)
internal/store/    in-memory хранилище (PG-совместимый контракт)
internal/rag/      чанкинг, BM25-поиск, загрузка знаний
internal/llm/      LLM-клиент (OpenAI-совместимый) + mock
internal/tools/    MCP-подобные инструменты с проверкой прав
internal/agent/    оркестратор: RAG -> LLM -> tool call
internal/api/      HTTP API (chi)
data/knowledge/    синтетическая база знаний
specs/             SDD-спецификации агента и инструментов
coverage/          профиль и HTML-отчёт покрытия
```

## Запуск

### Docker
```bash
cp .env.example .env
docker compose up --build
```
API: http://localhost:8080

### Локально (Go 1.22+)
```bash
cp .env.example .env
go mod tidy
go run ./cmd/server
```

## Переменные окружения

| Переменная | Назначение | По умолчанию |
|---|---|---|
| `ADDR` | адрес сервера | `:8080` |
| `LLM_MOCK` | `true` — mock-LLM без ключа | `true` |
| `LLM_BASE_URL` | адрес LLM API | `https://api.openai.com/v1` |
| `LLM_MODEL` | модель | `gpt-4o-mini` |
| `LLM_API_KEY` | ключ (не коммитить) | — |

## Тестовые учётные записи

В прототипе авторизация упрощена (middleware `DemoUser`). Роль задаётся в `internal/api/middleware.go`.

| Пользователь | Роль | Что проверить |
|---|---|---|
| `intern-1` | `intern` (backend) | S1, S2, S3 |

## Порядок проверки

### PowerShell

```powershell
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

# S1 — действие (отгул)
$body = @{ question = "Как оформить отгул?" } | ConvertTo-Json
Invoke-RestMethod -Uri http://localhost:8080/api/ask -Method Post -ContentType "application/json; charset=utf-8" -Body ([System.Text.Encoding]::UTF8.GetBytes($body))
# Ожидаемо: type = action, action.ok = true, absence_id

# S2 — чек-лист
$body = @{ question = "Какой следующий шаг по чек-листу?" } | ConvertTo-Json
Invoke-RestMethod -Uri http://localhost:8080/api/ask -Method Post -ContentType "application/json; charset=utf-8" -Body ([System.Text.Encoding]::UTF8.GetBytes($body))
# Ожидаемо: type = answer/action, sources непустой

# S3 — отказ
$body = @{ question = "Сколько платят в Авито?" } | ConvertTo-Json
Invoke-RestMethod -Uri http://localhost:8080/api/ask -Method Post -ContentType "application/json; charset=utf-8" -Body ([System.Text.Encoding]::UTF8.GetBytes($body))
# Ожидаемо: type = refusal
```

### bash

```bash
curl -s localhost:8080/api/ask -H "Content-Type: application/json" -d '{"question":"Как оформить отгул?"}'
```

## Тесты и покрытие

```bash
go test ./... -coverprofile=coverage/coverage.out
cmd /c "go tool cover -func=coverage/coverage.out"
go tool cover "-html=coverage/coverage.out" -o coverage/index.html
```

- Покрытие: **82.0%** (порог >80%).
- HTML-отчёт: `coverage/index.html`.

## Версии зависимостей

- Go 1.22
- `github.com/go-chi/chi/v5 v5.0.12`
- `github.com/google/uuid v1.6.0`

## Внешние API

- LLM: OpenAI-совместимый (`/chat/completions`), переменные `LLM_BASE_URL`, `LLM_MODEL`, `LLM_API_KEY`.
- Для демо по умолчанию `LLM_MOCK=true` — ключ не нужен.
- Пример конфигурации без секретов: `.env.example`.

## Архитектура и права

- RAG (BM25) по `data/knowledge/*.md`, фильтр по `team_id`.
- Ответ только по найденным источникам; нет источника → `refusal`.
- Каждый инструмент проверяет право (`auth.Can`) до выполнения.
- Действие считается выполненным, если инструмент вернул результат, агент его отобразил и сохранил.
- LLM-вызовы не блокируют UI; статус ожидания и ошибки обрабатываются на клиенте (при наличии UI).

## Ограничения

- Синтетические данные, имитации сервисов.
- Нет чувствительных ПД и финансовых операций.
- Тестовые инструкции не являются регламентами Авито.

## Видео

Ссылка: TODO (добавить перед сдачей)