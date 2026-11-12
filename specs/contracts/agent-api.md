> **Статус: целевая архитектура (roadmap).** В прототипе агент работает in-process (`internal/agent`), а не отдельным сервисом. Реализованный контракт: `openapi.yaml` и `specs/agent/spec.md`.

# Контракт backend (Go) <-> agent

Внутренний, наружу не торчит. Backend — единственная точка входа и источник истины о пользователе.

## POST /internal/agent/run   (ответ — SSE-поток)

```json
{
  "requestId": "uuid",
  "sessionId": "uuid",
  "user": {
    "id": "u_intern_1",
    "role": "intern",
    "teamId": "team_payments",
    "mentorId": "u_mentor_1",
    "permissions": ["absence:create_own", "access:request_own", "checklist:update_own", "kb:read_team"]
  },
  "onboarding": { "currentStage": "week_1", "nextStepId": "step_gitlab_access" },
  "history": [ { "role": "user", "text": "..." }, { "role": "assistant", "text": "..." } ],
  "message": "Хочу взять отгул в пятницу"
}
```

События потока совпадают с `ChatEvent` из openapi.yaml (`status`, `token`, `tool_result`, `message_done`, `error`).
Backend сохраняет `tool_result` в таблицу `actions`, сообщения — в `chat_messages`, и ретранслирует события фронту.

## Правила
- Таймаут на весь run — 60 с; на вызов одного инструмента — 10 с. При превышении — событие `error` с понятным текстом.
- Идентичность пользователя агент **не берёт из текста и не передаёт через аргументы LLM**.
  В MCP-серверы она передаётся заголовком `X-User-Context` (подписанный backend), см. mcp-tools.json.
- LLM: провайдер/модель/ключ — только из env (`LLM_PROVIDER`, `LLM_MODEL`, `LLM_API_KEY`). В тестах — фейковая LLM.
