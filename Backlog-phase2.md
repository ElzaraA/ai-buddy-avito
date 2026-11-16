# Backlog-phase2

## Основание и границы

Этот backlog описывает подтверждённый локальным кодом Phase-2 scope. Контракт API — `specs/contracts/openapi.yaml`; серверные обработчики находятся в `internal/api/`, клиент и пользовательские сценарии — в `frontend/src/`. Приложенная папка `Telegram Desktop/docs` не доступна по указанным путям в текущей среде, поэтому её CJM и RecSys-исследования нельзя проверить. В репозитории отсутствуют `internal/recsys`, ML-артефакты и endpoint рекомендаций. Не заменять их фиктивным scoring; интеграцию RecSys считать заблокированной до получения проверяемой модели и backend-контракта.

Авторизация в данной версии — демонстрационная: API принимает `X-Demo-User`; это не production SSO. Не добавлять имитацию production-входа. API истории чатов отсутствует; доступна история только текущей сессии из `GET /chat/sessions/{sessionId}/messages`.

## Задачи

### P2-01 — Подтвердить текущий API/RAG и role flows
- **Приоритет:** P0
- **Сценарий:** Стажёр получает ответ с подтверждёнными источниками или отказ; создаёт черновик; владелец подтверждает его; руководитель команды принимает решение.
- **Результат:** Существующие UI/API-интеграции и security-инварианты проверены узкими frontend/backend тестами; дефекты исправлены, без замены серверной бизнес-логики.
- **Frontend:** `frontend/src/api/client.ts`, `frontend/src/features/chat/useChat.ts`, `frontend/src/features/chat/ChatPanel.tsx`, `frontend/src/features/progress/MyActions.tsx`, `frontend/src/features/team/TeamPanel.tsx`.
- **Backend endpoints:** `POST /chat/sessions`, `POST/GET /chat/sessions/{id}/messages`, `GET /chat/sessions/{id}/events`, `GET/POST /actions`, `POST /actions/{id}/{confirm,cancel,approve,reject}`.
- **Модель:** `AgentResponse`, `Action`, `ChatSession`, `ChatMessage`; см. OpenAPI и `frontend/src/api/schemas.ts`.
- **Роли/безопасность:** серверный RAG фильтрует доступную аудиторию и команду; автор подтверждает черновик; только руководитель своей команды согласует; frontend не меняет статус сам.
- **Приёмка:** источник показывается из ответа API; нет источника — отказ; draft/pending/approved/rejected/cancelled отображаются из API; 401/403/409 не маскируются как успех.
- **Тесты:** `frontend/src/features/chat/ChatPanel.test.tsx`, `frontend/src/features/progress/MyActions.test.tsx`, `internal/agent/llm_contract_test.go`, `internal/rag/relevance_test.go`, `internal/api/lifecycle_test.go`.
- **Зависимости:** нет.
- **Статус:** Выполнена; frontend feature suite и `go test ./...` подтверждают RAG, SSE, действия и server-side RBAC.

### P2-02 — Маршруты и история браузера
- **Приоритет:** P0
- **Сценарий:** Пользователь открывает раздел ссылкой, обновляет страницу, использует Back/Forward; неизвестный URL получает 404, запрещённый по роли URL — 403.
- **Результат:** URL является источником активного раздела; маршруты прямого открытия и role guards работают без новой routing-библиотеки.
- **Frontend:** `frontend/src/App.tsx`, `frontend/src/App.test.tsx`, `frontend/src/features/shell/views.ts`, `frontend/src/features/shell/Sidebar.tsx`.
- **Backend endpoints:** запрос текущей роли через `GET /me`; данные разделов используют свои существующие API.
- **Модель:** `ViewId`, `User.role`; список экранов — из `viewsFor`.
- **Роли/безопасность:** защищённые разделы проверяются по роли из `/me` (при загрузке — временно безопасная роль из выбранной demo-учётки); сервер остаётся источником истины. Не предоставлять frontend-ссылкой доступ к чужим данным.
- **Приёмка:** `/`, `/desk`, `/chat`, `/plan`, `/schedule`, `/actions`, `/skills`, `/team`, `/profile`, `/plan/:stepId` разрешаются; запрещённые разделы дают 403, неизвестные — 404; refresh и Back/Forward сохраняют ожидаемое состояние.
- **Тесты:** расширить `frontend/src/App.test.tsx` на прямые URL, переходы, refresh-состояние, popstate, 403/404 и смену роли.
- **Зависимости:** P2-01.
- **Статус:** Выполнена; прямой URL, Back, 403/404 подтверждены `frontend/src/App.test.tsx`.

### P2-03 — Профиль текущего пользователя
- **Приоритет:** P1
- **Сценарий:** Стажёр, наставник или руководитель просматривает сведения о себе и роль/команду.
- **Результат:** Непустой экран профиля текущего пользователя и прогресса стажёра; не отображать выдуманные поля и чужой профиль.
- **Frontend:** `frontend/src/App.tsx`, `frontend/src/features/shell/views.ts`, новый компонент в `frontend/src/features/home/` или близкой существующей feature-папке, тесты.
- **Backend endpoints:** `GET /me`; для роли intern — `GET /onboarding/progress`, `GET /pulse/me`; для staff дополнительный API не требуется.
- **Модель:** `User`, `Progress`, `MyPulse` из `frontend/src/api/schemas.ts`.
- **Роли/безопасность:** показывать только данные текущего `X-Demo-User`; team API остаётся staff-only.
- **Приёмка:** профиль доступен по маршруту и меню; ошибки и загрузка объяснены; данные соответствуют `/me`; отсутствующие данные не подменены mock-значениями.
- **Тесты:** профиль intern/staff, API error, отсутствие progress для staff.
- **Зависимости:** P2-02.
- **Статус:** Выполнена; `/me`, progress/Pulse, ошибки и retry проверены `frontend/src/App.test.tsx`.

### P2-04 — Детали шага адаптации
- **Приоритет:** P1
- **Сценарий:** Стажёр открывает конкретный шаг из чек-листа, проверяет статус и завершает его, если он не требует одобрения заявки.
- **Результат:** Детальный URL использует фактическую модель чек-листа; approval-gated шаг нельзя завершить вручную.
- **Frontend:** `frontend/src/App.tsx`, `frontend/src/features/progress/ProgressPanel.tsx`, `frontend/src/features/progress/` тесты.
- **Backend endpoints:** `GET /onboarding/progress`, `PATCH /onboarding/steps/{stepId}`.
- **Модель:** `ChecklistItem` (`id`, `title`, `stage`, `done`, `status`, `kind`, `related_action_id`). Дополнительного API описания задач нет.
- **Роли/безопасность:** только intern видит собственный план; ownership и waiting/access ограничения проверяются backend.
- **Приёмка:** ссылка на существующий шаг открывает детали; неизвестный id даёт понятный not-found; complete обновляет API-данные; waiting/access не предлагает ручное завершение.
- **Тесты:** прямое открытие детали, неизвестный шаг, обычный шаг и approval-gated шаг.
- **Зависимости:** P2-02.
- **Статус:** Выполнена; API-данные и approval-gated запрет проверены `frontend/src/App.test.tsx`.

### P2-05 — Заявки стажёра: подтверждение и статусы
- **Приоритет:** P1
- **Сценарий:** Стажёр формирует через Buddy черновик доступа/отгула, проверяет, отправляет или отменяет его и видит статус решения.
- **Результат:** Полный сценарий подключён к API; draft требует явного подтверждения; заявки и статусы загружаются с сервера.
- **Frontend:** `frontend/src/features/chat/ChatPanel.tsx`, `frontend/src/components/DraftCard.tsx`, `frontend/src/features/progress/MyActions.tsx`, `frontend/src/features/home/ActionsView.tsx`, API и тесты.
- **Backend endpoints:** `POST /chat/sessions/{id}/messages`, `GET /actions`, `POST /actions/{id}/confirm`, `POST /actions/{id}/cancel`.
- **Модель:** `Action` (тип, статус, dates/resource, comment).
- **Роли/безопасность:** автор заявки подтверждает/отменяет свою; запрет подтверждать за другого enforced backend.
- **Приёмка:** в UI нет фиктивного auto-submit; ошибки валидации/устаревшего черновика/409 видимы; данные после изменения обновляются.
- **Тесты:** существующие `MyActions.test.tsx`, `ChatPanel.test.tsx`, backend `actions_api_test.go`, `lifecycle_test.go`.
- **Зависимости:** P2-01.
- **Статус:** Выполнена; chat/action frontend tests и backend lifecycle/action tests прошли.

### P2-06 — Кабинет наставника и руководителя
- **Приоритет:** P1
- **Сценарий:** Staff просматривает только свою команду, входящие, прогресс, вопросы без ответа и сигналы; lead принимает решения.
- **Результат:** Реальные API-данные, role-appropriate actions, подтверждения/ошибки и повторная загрузка после mutation.
- **Frontend:** `frontend/src/features/team/TeamPanel.tsx`, `frontend/src/features/team/TeamPulse.tsx`, тесты.
- **Backend endpoints:** `GET /team/progress`, `GET /team/actions?status=pending`, `GET /team/knowledge-gaps`, `GET /team/signals`, `POST /team/signals/{id}/resolve`, `GET /team/summary`, `GET /team/pulse`, `POST /actions/{id}/{approve,reject}`.
- **Модель:** `TeamProgress`, `Action`, `KnowledgeGap`, `Signal`, `TeamSummary`, `TeamPulse`.
- **Роли/безопасность:** mentor — просмотр и разбор сигналов, lead — решения; фильтрация команды и запреты enforced backend.
- **Приёмка:** mentor не видит controls решения; lead видит только разрешённые действия; empty/error/loading состояния не пропускают данные другой команды.
- **Тесты:** `TeamPanel.test.tsx`, backend `lifecycle_test.go`, `actions_api_test.go`, `pulse_test.go`.
- **Зависимости:** P2-01.
- **Статус:** Выполнена; `TeamPanel` frontend tests и backend team/lifecycle tests прошли.

### P2-07 — Расписание и уведомления
- **Приоритет:** P1
- **Сценарий:** Пользователь видит недельные события, стажёр создаёт/отменяет личный созвон; напоминания появляются в чате.
- **Результат:** Все операции используют schedule/reminder API, с валидацией и конфликтными состояниями.
- **Frontend:** `frontend/src/features/schedule/ScheduleView.tsx`, `frontend/src/features/schedule/useReminders.ts`, `frontend/src/App.tsx`, связанные тесты.
- **Backend endpoints:** `GET/POST /schedule`, `POST /schedule/{id}/cancel`, `GET /reminders`, `POST /reminders/{id}/ack`.
- **Модель:** `Occurrence`, `Schedule`, `Reminder`.
- **Роли/безопасность:** пользователь меняет только собственные встречи; отмена командных событий запрещена backend.
- **Приёмка:** loading/empty/error/success, проверка времени и конфликта, reminder ack исключает повторы.
- **Тесты:** schedule feature tests и `internal/api/schedule_test.go`, `internal/api/sse_test.go`.
- **Зависимости:** P2-01.
- **Статус:** Выполнена; расписание/reminder frontend tests и backend schedule tests прошли.

### P2-08 — Pulse стажёра и Pulse команды
- **Приоритет:** P1
- **Сценарий:** Стажёр отправляет добровольную отметку и видит историю; staff видит агрегат своей команды и сигналы риска.
- **Результат:** Реальные `pulse` API, приватность заметок и корректные empty/error states.
- **Frontend:** `frontend/src/features/pulse/`, `frontend/src/features/home/Desk.tsx`, `frontend/src/features/team/TeamPulse.tsx`, тесты.
- **Backend endpoints:** `POST /pulse`, `GET /pulse/me`, `GET /team/pulse`.
- **Модель:** `PulseEntry`, `MyPulse`, `TeamPulse`.
- **Роли/безопасность:** intern видит только свои отметки; staff — данные только своей команды; заметки не расширяют доступ.
- **Приёмка:** значения 1–5 валидируются; повторная отправка отражается сервером; UI не заявляет медицинский диагноз.
- **Тесты:** Pulse components, `internal/api/pulse_test.go`.
- **Зависимости:** P2-01.
- **Статус:** Выполнена; pulse frontend components и backend pulse tests прошли.

### P2-09 — Обработка ошибок, загрузки и пустых данных
- **Приоритет:** P1
- **Сценарий:** Пользователь получает понятное состояние при network/401/403/404/409/429/5xx на каждом разделе.
- **Результат:** Нет вечных spinner, пустых белых областей и успешного вида после ошибки; предусмотрены retry там, где он безопасен.
- **Frontend:** `frontend/src/api/client.ts`, `frontend/src/App.tsx`, все активные feature-компоненты и тесты.
- **Backend endpoints:** все задействованные endpoint из P2-01—P2-08.
- **Модель:** `ApiError`, `ApiErrorSchema`.
- **Роли/безопасность:** ошибка 403 не обходится повтором под чужой учёткой; никаких клиентских решений вместо backend.
- **Приёмка:** ошибки и состояния покрыты на основных экранах; mutating actions не повторяются автоматически при конфликте.
- **Тесты:** API client tests и компонентные негативные сценарии.
- **Зависимости:** P2-01—P2-08.
- **Статус:** Выполнена; 401 → экран входа, 403/404 — отдельные экраны, 409 (задачи, заявки) и 429 (лимит черновиков) показывают сообщение сервера, 5xx — «Повторить» (`App.test.tsx`, `Tasks.test.tsx`, `NewRequest.test.tsx`).

### P2-10 — Адаптивность и доступность
- **Приоритет:** P1
- **Сценарий:** Основные пользовательские потоки доступны с клавиатуры и узкого экрана.
- **Результат:** Сохранены существующая дизайн-система и адаптивные layout; интерактивные элементы имеют label/focus/disabled и подтверждение критических действий.
- **Frontend:** `frontend/src/styles.css`, Sidebar и компоненты форм/действий, тесты.
- **Backend endpoints:** без изменений; проверяются UI flows P2-02—P2-08.
- **Модель:** существующие API-модели.
- **Роли/безопасность:** доступность не меняет RBAC.
- **Приёмка:** ключевые маршруты не переполняются на mobile; keyboard navigation и aria names работают; нет горизонтального переполнения в основных формах.
- **Тесты:** frontend tests и ручная проверка в доступных браузерных инструментах; недоступные проверки явно отмечаются.
- **Зависимости:** P2-02—P2-09.
- **Статус:** Выполнена; прогон в Chromium на 1440, 1180 и 390 px: без горизонтальной прокрутки на `/desk`, `/tasks`, `/tasks/:id`, `/knowledge/:id`, `/actions/new`; формы с подписями, ссылки — настоящие `<a href>`.

### P2-11 — RecSys как следующий шаг адаптации
- **Приоритет:** P2, блокирована внешними артефактами/контрактом
- **Сценарий:** Стажёр получает ранжированный разрешённый следующий шаг с объяснением и переходом к нему.
- **Результат:** Не реализовывать scoring или API-фейк. Сначала получить исследование/обученную модель из доступной копии и утверждённый контракт; затем серверный endpoint с бизнес-фильтрацией и тестами.
- **Frontend:** после появления API — `frontend/src/features/progress/` и интеграция с `Desk`/`ProgressPanel`.
- **Backend endpoints:** отсутствуют в `specs/contracts/openapi.yaml` и `internal/api/router.go`.
- **Модель:** отсутствует в текущей копии (`internal/recsys/` не найден).
- **Роли/безопасность:** только допустимые задачи текущего стажёра; backend является источником профиля/прогресса/прав.
- **Приёмка:** только обученная/проверяемая модель, серверное ранжирование, объяснение, переход к задаче; не маркетплейс Avito leads.
- **Тесты:** backend model/API tests, frontend integration tests и воспроизводимый evaluation baseline.
- **Зависимости:** ML/model artifact, research, бизнес-контракт; блокер внешний.
- **Статус:** Выполнена после получения модели команды (`ml/recsys`): `internal/recsys` (скоринг совпадает со sklearn до 1e-6), каталог `data/tasks/catalog.json`, `GET /recommendations`, раздел «Задачи», карточка «Следующая задача», инструмент чата `tasks_recommend`. Без весов — правило с честной пометкой.

### P2-12 — Итоговая проверка, актуализация backlog и Report-phase2.1
- **Приоритет:** P0
- **Сценарий:** Разработчик воспроизводит продукт локально и проверяет основные роли и сценарии.
- **Результат:** Все выполнимые задачи проверены; отчёт содержит фактические экраны, маршруты, API, изменения, тестовые результаты и ограничения.
- **Frontend:** `frontend/` и тесты; документация корня.
- **Backend endpoints:** проверка OpenAPI против маршрутов и тестов.
- **Модель:** по фактическим интеграциям.
- **Роли/безопасность:** описывать только проверенные ограничения.
- **Приёмка:** frontend tests/typecheck/build, backend tests, доступные route/API checks выполнены; цифры берутся из вывода команд, RecSys блокер раскрыт.
- **Тесты:** `npm test`, `npm run typecheck`, `npm run build`, `go test ./...`.
- **Зависимости:** остальные выполнимые задачи.
- **Статус:** Выполнена; см. `Report-phase2.1.md`: go test 13/13 (95.3%), vitest 175/175 (98.3%), build, обход ролей в браузере.

## Порядок выполнения

P2-01 → P2-02 → P2-03/P2-04 → P2-05/P2-06/P2-07/P2-08 → P2-09 → P2-10 → P2-12. P2-11 остаётся явно заблокированной до появления проверяемых ML-артефактов и API-контракта; демо-ответы или статические рекомендации не считаются выполнением.
### P2-13 — Экраны, которых не хватало в исходном срезе
- **Приоритет:** P1
- **Результат:** демо-вход `/login` (с возвратом на `?next=` и «Выйти»), «База знаний» (поиск и статьи, права как у поиска Бадди), «История диалогов», форма заявки без чата `/actions/new`, карточка стажёра `/team/interns/:id`, ссылки на статьи из источников ответа.
- **Backend endpoints:** `GET /knowledge`, `GET /knowledge/{id}`, `GET /chat/sessions`, `GET /team/interns/{id}` — добавлены с тестами и в OpenAPI.
- **Тесты:** `internal/api/phase2_test.go`, `internal/rag/library_test.go`, `features/*/*.test.tsx`, `App.test.tsx`.
- **Статус:** Выполнена.
