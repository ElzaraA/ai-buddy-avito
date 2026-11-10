# AI-buddy — фронтенд

React 18 + TypeScript, Vite, TanStack Query, Zustand, Zod.

## Экраны
- **Чат** — вопросы к помощнику. Сообщение уходит сразу (202), ответ агента приходит по SSE, пока что показывается «Помощник думает…». Ответы рисуются по типу:
  `answer` (текст + источники), `action` (карточка результата: номер заявки, статус), `draft` (черновик отгула с кнопкой «Отправить заявку»), `refusal` (отказ + имя наставника).
- **Моя адаптация** — этапы, шаги, % выполнения, следующий шаг, отметка шага выполненным.
- **Тестовый пользователь** — переключатель в шапке (заголовок `X-Demo-User`): стажёр backend, стажёр другой команды, наставник. Нужен для проверки прав.

## Запуск

Нужен запущенный бэкенд на `:8080` (из корня репозитория: `go run ./cmd/server`).

```bash
cd frontend
npm install
npm run dev        # http://localhost:5173, /api проксируется на :8080
```

Другой адрес бэкенда: `BACKEND_URL=http://host:port npm run dev`.

Через Docker (из корня): `docker compose up --build` → http://localhost:5173.

## Тесты и покрытие

```bash
npm test           # vitest
npm run coverage   # отчёт: frontend/coverage/index.html, порог строк 80%
npm run typecheck
```

## Структура
```
src/api/          Zod-схемы ответов бэкенда и API-клиент
src/store/        Zustand: пользователь, сессия, сообщения чата
src/features/chat/      чат + хук useChat (SSE, таймаут 65 с)
src/features/progress/  панель прогресса адаптации
src/components/   карточки ответов агента, форматирование
```

## Версии
Node 22, react 18.3, @tanstack/react-query 5, zustand 5, zod 3, vite 5, vitest 2.
