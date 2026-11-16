# Report-phase2

## Срез и источники

Это аудит исходного состояния до исполнения Backlog-phase2. Основание: `README.md`, `frontend/README.md`, `specs/contracts/openapi.yaml`, `internal/api/router.go`, обработчики в `internal/api/`, frontend `App.tsx` и feature-компоненты, а также тесты рядом с этими реализациями. Приложение CJM/ML-документов указывает на внешнюю папку `Telegram Desktop/docs`, но эти пути в текущей среде не открываются; их содержимое не считается проверенным.

## Архитектура

- Backend — Go. Основные модули: `internal/api` (HTTP/SSE/MCP), `internal/agent` (intent, context, actions), `internal/rag` (индекс и retrieval), `internal/llm` (mock и OpenAI-compatible client), `internal/auth` (demo identity/RBAC), `internal/store` (состояние), `internal/tools`, `internal/schedule`, `internal/metrics`, `internal/config`.
- Frontend — React 18, TypeScript, Vite, Zustand, TanStack Query и Zod. API-клиент и схемы — `frontend/src/api/`; feature-компоненты — `frontend/src/features/`; оболочка — `frontend/src/App.tsx` и `features/shell/`.
- Контракт — OpenAPI 3.0.3 в `specs/contracts/openapi.yaml`. Реализация маршрутов — `internal/api/router.go`; frontend проверяет ответы Zod-схемами.
- В исходном срезе навигация была внутренним `useState`; URL-маршрутов, прямого открытия экранов и 404/403 страниц не было.

## Back-end API

Все пути ниже имеют префикс `/api/v1`, если не указано иное.

| Endpoint | Назначение и методы |
|---|---|
| `/me` | GET текущий пользователь |
| `/onboarding/progress` | GET план/прогресс; `/onboarding/steps/{stepId}` PATCH завершение шага |
| `/chat/sessions` | POST сессия; `/chat/sessions/{sessionId}/messages` GET история сессии, POST вопрос; `/events` GET SSE |
| `/actions` | GET мои заявки, POST черновик; `/actions/{actionId}` GET; `/confirm`, `/cancel`, `/approve`, `/reject` POST |
| `/team/progress` | GET прогресс стажёров команды |
| `/team/actions` | GET заявки команды, фильтр `status` |
| `/team/signals`, `/team/signals/{signalId}/resolve` | GET сигналы; POST разобрать сигнал |
| `/team/knowledge-gaps` | GET обезличенные пробелы базы знаний |
| `/team/summary`, `/team/pulse` | GET командные метрики и pulse |
| `/schedule`, `/schedule/{id}/cancel` | GET/POST расписание; POST отменить личную встречу |
| `/reminders`, `/reminders/{id}/ack` | GET напоминания; POST подтвердить показ |
| `/pulse`, `/pulse/me`, `/achievements` | POST pulse; GET собственная история/достижения |
| `/ask` | POST синхронный ответ для проверки |
| Вне `/api/v1` | `/health`, `/metrics`, `/mcp`, `/api/ask` (см. OpenAPI/README) |

В OpenAPI и роутере нет endpoint истории между сессиями, чужого профиля или рекомендаций RecSys. `/chat/sessions/{id}/messages` — только история конкретной сессии.

## Исходные Front-end экраны и интеграции

Внутренние экраны (не отдельные URL в исходном срезе):

| Экран/сценарий | Компоненты | API |
|---|---|---|
| Рабочий стол стажёра | `Desk`, `PulseStrip`, `Achievements` | progress, actions, pulse, achievements, reminders |
| AI Buddy chat | `ChatPanel`, `useChat`, `AssistantMessage`, `DraftCard` | sessions, messages, SSE events |
| План/checklist | `ProgressPanel` | progress, steps PATCH |
| Мои заявки | `ActionsView`, `MyActions` | actions list, confirm, cancel |
| Кабинет команды | `TeamPanel`, `TeamPulse` | team progress/actions/gaps/signals/summary/pulse, action decisions |
| Расписание | `ScheduleView`, `useReminders` | schedule, cancel, reminders/ack |
| Подсказки о Buddy | `Skills` | переход в чат; отдельный endpoint не нужен |
| Общая оболочка и тестовый пользователь | `App`, `Sidebar` | `/me`, заголовок `X-Demo-User` |

Встроенные сценарии: draft отгула/доступа с подтверждением стажёром; согласование только lead; шаг доступа закрывается approval-ом; сигналы наставнику и обезличенные knowledge gaps; команды/пульс; командное расписание и личные созвоны.

## Отсутствовавшие экраны и маршруты на момент аудита

- URL-маршрутизация, deep links, Back/Forward, 404 и страница отказа по роли.
- Профиль пользователя; API позволяет показать только текущего пользователя через `/me` (нет API профиля другого человека).
- Деталь шага плана; API возвращает checklist-item поля, отдельной модели/task endpoint нет.
- Отдельный список/архив чатов между сессиями: соответствующий API отсутствует; доступна только история текущей сессии.
- RecSys UI нельзя сделать честно без сервиса/модели/контракта; отсутствуют `internal/recsys`, ML-артефакты и recommendation endpoint.
- Product Impact Lab и ALA не входят в подтверждённый кодом scope текущей копии; не добавлять автономные страницы без доступного контракта/требования.

## Авторизация и роли

- Demo auth через `X-Demo-User`; неизвестный пользователь — 401. SSE передаёт demo user через query, поскольку EventSource не поддерживает произвольные headers. См. OpenAPI description и `internal/auth/auth.go`.
- Поддерживаемые demo identities включают intern, buddy и lead backend/frontend (список UI в `frontend/src/store/chat.ts`). Это не production SSO.
- Ownership/RBAC/team isolation проверяются backend middleware/handlers/tools. Доказательства: `internal/api/actions_api_test.go`, `internal/api/lifecycle_test.go`, `internal/api/hardening_test.go`, `internal/rag/relevance_test.go`, `internal/auth/auth_test.go`.
- Frontend переключатель — только демо-механизм; он не является механизмом выдачи прав. Решение о статусе заявки остаётся за backend.

## LLM, RAG и RecSys

- Agent использует intent/context/tool pipeline (`internal/agent/agent.go`), retrieval из реального локального knowledge corpus через BM25 (`internal/rag/index.go`, `internal/rag/retrieve.go`), затем LLM contract. Доступ ограничивается audience/team по умолчанию deny; тесты — `internal/rag/relevance_test.go`.
- Без evidence агент отказывает и направляет к наставнику; проверка — `internal/agent/llm_contract_test.go`, `internal/agent/hardening_test.go`, golden tests.
- Backend поддерживает mock и OpenAI-compatible client (`internal/api/app.go`, `internal/llm/openai.go`); README указывает mock-режим по умолчанию. Реальные credentials остаются на сервере. Реальный провайдер не считался проверенным без доступного настроенного секрета/сервиса.
- RecSys в проверяемой локальной копии отсутствует. Никакая статическая выдача или случайный scoring не могут считаться интеграцией.

## Расхождения, ограничения и полный backlog

- Первоначальный краткий вариант этого отчёта содержал неверные имена `/team/pending` и `/team/gaps`, ошибочно называл 404 уже существующим экраном и недостаточно различал session history от архива. Точные endpoints и отсутствие соответствующих UI указаны выше; этот раздел supersedes эти записи.
- Прототип использует синтетические данные, demo identity, in-memory/file persistence и mock LLM по конфигурации; интеграции с корпоративными HR/IDM нет. Подробности и запуск — `README.md`, конфигурация — `internal/config/` и `docker-compose.yml`.
- Полный исполнимый список задач, критерии, файлы, модели, роли, тесты и статусы — `Backlog-phase2.md`. Подтверждённые в нём задачи исполняются; RecSys остаётся заблокированным, пока исследование/модель/API недоступны.