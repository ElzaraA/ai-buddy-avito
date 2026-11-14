# Traceability — сценарий / инвариант → тест

Тесты бэкенда запускаются `make test`; golden-набор — `data/golden/golden.json` (фразы из README и сценариев).
Тесты интерфейса (строки `UI-*`) — `cd frontend && npm test`; в колонке «Тест» указано имя `it(...)` в «кавычках».
Номера `INV-n` совпадают с нумерацией инвариантов в `specs/agent/spec.md`.

| ID | Что проверяем | Тест |
|---|---|---|
| S1-draft | «Хочу отгул 2026-11-20» → черновик с датами и action_id | `internal/agent/golden_test.go: TestGolden/S1-draft-date`, `internal/api/chat_test.go: TestChatS1DraftThenConfirm` |
| S1-weekday | «в Пятницу» → ближайшая пятница | `TestGolden/S1-draft-weekday`, `internal/agent/llm_contract_test.go: TestParseWeekday` |
| S1-clarify | уточнение даты вторым сообщением | `internal/api/chat_test.go: TestChatS1DateClarification` |
| S1-confirm | без подтверждения — draft; confirm → pending; видно в GET /actions | `TestChatS1DraftThenConfirm` |
| S1-policy | «Как оформить отгул?» → правила + имена | `TestGolden/S1-policy` |
| S2-checklist | «Что мне сделать на этой неделе?» → чек-лист и next_step | `TestGolden/S2-checklist`, `TestChatToolResultEvent` |
| S2-access | «Запроси доступ в GitLab» → черновик REQ-… для gitlab | `TestGolden/S2-access-gitlab`, `TestChatS2AccessDraft` |
| S2-403 | чужой шаг чек-листа — 403 | `internal/api/actions_api_test.go: TestProgressAndSteps`, `actions_test.go: TestStepUpdate` |
| S3-answer | ответ по базе с источником | `TestGolden/S3-answer-stack`, `TestGolden/S3-answer-logs`, `TestChatS3AnswerAndRefusal` |
| S3-refusal | вне базы — отказ с именем наставника | `TestGolden/S3-refusal-*`, `internal/rag/relevance_test.go: TestStopwordsDoNotMatch` |
| INV-1 | нет источника — нет ответа (даже если LLM ответила); текст модели в отказе — только короткая отсылка к наставнику без цифр | `llm_contract_test.go: TestLLMOutputs/hallucination_without_sources`, `internal/agent/hardening_test.go: TestRefusalTextWithoutSources` |
| INV-2 | право проверяется в инструменте и при подтверждении | `internal/tools/perm_test.go: TestToolsForbidden`, `TestToolPermissionDenied`, `TestActionsOwnershipAndRoles` |
| INV-3 | deny-by-default по audience, mentors_only, фильтр команды | `relevance_test.go: TestAudienceDenyByDefault`, `TestGolden/S6-*`, `TestGolden/S8-*` |
| INV-4 | мутирующий tool_call → только черновик, хранилище не меняется | `llm_contract_test.go: TestToolCallDoesNotMutate`, `TestToolMeta` |
| INV-4b | двойное подтверждение под нагрузкой — ровно один 200 | `actions_api_test.go: TestConfirmConcurrent` |
| INV-5 | 202 + SSE: status → tool_result → message_done; replay; heartbeat | `TestChatToolResultEvent`, `TestChatReplayOnConnect`, `TestEventsHeartbeat` |
| INV-6 | таймауты (run 60 с, инструмент 10 с, HTTP-клиент LLM 20 с): отмена по контексту прерывает повторы; сбой или мусор от LLM → понятная ошибка без внутренних деталей | `TestLLMOutputs`, `TestLLMError`, `internal/llm/retry_test.go: TestRetryStopsOnContextCancel`, `TestLLMDownWithoutFallback`; в UI — строка UI-S1-nohang |
| INV-7 | prompt injection | `TestGolden/S7-injection`, `TestGolden/S7-injection-ru`, `TestToolCallDoesNotMutate` |
| SEC-IDOR | чужие сессия/сообщения/SSE/действия — 403 | `TestChatForeignSessionForbidden`, `TestActionsOwnershipAndRoles` |
| SEC-input | лимит тела 64 КБ, вопрос ≤ 2000 символов, валидация дат | `TestBodyLimit`, `TestChatBadInput`, `TestCreateActionValidation` |
| SEC-auth | неизвестный X-Demo-User — 401 | `TestAuthAndMe`, `actions_test.go: TestUnauthorized` |
| OPS | старт, /health, graceful shutdown, ошибки старта | `cmd/server/main_test.go` |
| S1-approve | руководитель согласует/отклоняет; автор, наставник и чужая команда — 403; история заявки | `internal/api/lifecycle_test.go: TestAbsenceApprovalFlow`, `TestRejectAndCancel` |
| S2-waiting | заявка на доступ двигает шаг чек-листа: waiting → done / todo | `lifecycle_test.go: TestAccessRequestDrivesChecklistStep`, `internal/store/persist_test.go: TestLinkAndUnlinkStep` |
| TEAM | кабинет наставника: прогресс своей команды, входящие, сводка; стажёру — 403 | `lifecycle_test.go: TestTeamViews` |
| GAPS | вопросы без ответа копятся без автора | `TestTeamViews`, `persist_test.go: TestGaps`, `internal/agent/resilience_test.go: TestNoSourceFlag` |
| INV-8 | повторы при 429/5xx/сети, без повтора при 4xx; резервная модель и `degraded` | `internal/llm/retry_test.go`, `resilience_test.go: TestFallbackModel`, `internal/api/resilience_test.go: TestFallbackWhenLLMDown` |
| INV-8b | LLM недоступна без резерва: 502 / событие error, внутренние детали не утекают; паника не роняет сервер | `TestLLMDownWithoutFallback`, `TestAgentPanicIsContained` |
| INV-9 | MCP: handshake, tools/list, права, мутация → черновик, ошибки JSON-RPC | `internal/api/mcp_test.go` |
| INV-10 | заявку согласует только руководитель своей команды: не автор, не наставник, не другая команда; решение окончательное и в истории | `lifecycle_test.go: TestAbsenceApprovalFlow`, `TestRejectAndCancel`, `actions_api_test.go: TestActionsOwnershipAndRoles` |
| SPEC-sync | `mcp-tools.json` совпадает с кодом (имена, права, подтверждение, аргументы) | `internal/tools/spec_sync_test.go: TestSpecMatchesCode` |
| HISTORY | модель получает последние реплики; не более 3 ожидающих ответов | `TestHistoryAndPendingCap`, `TestHistoryIsTrimmedAndOrdered` |
| LOAD | 100 параллельных диалогов без потерь и перемешивания | `internal/api/resilience_test.go: TestHundredParallelChats` |
| LIMIT | 429 + Retry-After, лимит персональный | `TestRateLimit`, `TestRateLimiterWindow` |
| OBS | /metrics, метки по шаблону маршрута, X-Request-Id | `TestMetricsAndRequestID`, `internal/metrics/metrics_test.go` |
| STATE | снимок на диск: round-trip, повреждённый файл, сохранение при остановке, перезапуск сервера | `internal/store/persist_test.go`, `cmd/server/main_test.go: TestStateSurvivesRestart` |
| DATES | прошедшая дата не оформляется; мусорные даты от LLM игнорируются; перевод строки не ломает промпт | `resilience_test.go: TestPastDateAndLLMDates`, `TestPromptInjectionViaNewline`, `TestPastAndLongAbsenceRejected` |
| DATES-human | «20 ноября», «20.11», «с 20 по 22 ноября», «послезавтра», «сегодня»; несуществующая дата отклоняется | `internal/agent/hardening_test.go: TestParseDatesAt`, `TestAbsenceDraftHumanDates`, `TestGolden/AI3-*` |
| SEC-validate | единая валидация черновика: формат и диапазон дат, белый список ресурсов, длина причины; повторная проверка при confirm | `internal/store/actions_test.go: TestValidateAction`, `internal/api/hardening_test.go: TestMCPDraftValidation`, `TestConfirmRevalidatesDraft`, `TestCreateActionResourceWhitelist`, `TestSaveDraftRejectsInvalid` |
| SEC-quota | лимит записи, не более 50 открытых черновиков, не более 200 сообщений в сессии | `TestWriteRateLimit`, `TestDraftQuota`, `TestSessionMessageCap`, `store/actions_test.go: TestCreateDraftLimit` |
| S2-lock | шаг kind=access / status=waiting вручную не закрывается (409), только согласованием | `store/actions_test.go: TestAccessStepIsLocked`, `TestProgressAndSteps`, `TestAccessRequestDrivesChecklistStep` |
| SMALLTALK | «привет/спасибо/ок» — вежливый ответ без LLM и без записи в вопросы без ответа | `hardening_test.go: TestSmallTalkAndGaps`, `internal/api/hardening_test.go: TestSmallTalkIsNotAGap` |
| RAG-ru | стемминг, синонимы и транслитерация; вопросы про доступ — ответ, не черновик | `relevance_test.go: TestStemmingAndSynonyms`, `TestCoverageRequiresTopicWords`, `internal/llm/mock_test.go: TestMockAccessQuestionsAreAnswers` |
| SEC-query-user | `?user=` принимается только для SSE `/chat/sessions/{id}/events`; `approve?user=lead.backend` без заголовка — 403, заявка остаётся pending | `internal/api/r2_test.go: TestQueryUserOnlyForEventStream`, `TestIsEventStream`, `actions_api_test.go: TestAuthAndMe` |
| SEC-csrf | POST/PATCH/PUT/DELETE с `Origin` не из `CORS_ORIGINS` — 403; без `Origin` и GET не затрагиваются; `*` пропускает всё | `r2_test.go: TestCrossOriginWritesBlocked`, `TestSameOriginWritesWildcard` |
| S2-unknown-resource | доступ к системе вне белого списка (Confluence, Slack, Kubernetes) или без системы — честный отказ, черновика нет | `TestGolden/R2-access-*`, `internal/agent/r2_test.go: TestAccessUnknownResourceRefusal`, `internal/llm/mock_test.go: TestMockAccessUnknownResource`, `r2_test.go: TestAccessToUnknownSystem` |
| GAPS-pii | e-mail и номера из 6+ цифр в пробелах базы маскируются до сохранения; короткие числа остаются | `internal/store/gaps_test.go: TestRecordGapMasksPII`, `TestMaskPIIKeepsShortNumbers` |
| TEAM-contacts | наставник и руководитель берутся по команде пользователя (frontend — Анна Козлова / Дмитрий Орлов); `lead.frontend` согласует заявку frontend, `lead.backend` — 403 | `internal/auth/auth_test.go: TestTeamContacts`, `internal/agent/r2_test.go: TestNamesByTeam`, `internal/tools/extra_test.go: TestAbsenceNamesByTeam`, `r2_test.go: TestFrontendTeamContactsAndLead`, `TestGolden/R2-frontend-mentor` |
| STAFF-wording | наставнику и руководителю не советуют «обратиться к наставнику»: HR BP / IT Support | `internal/agent/r2_test.go: TestStaffWording`, `r2_test.go: TestStaffTextsDoNotReferToMentor`, `TestGolden/R2-mentor-no-self-advice` |
| RAG-restricted | материал своей команды для другой роли — отказ «доступен наставникам», не пробел базы | `internal/rag/relevance_test.go: TestRestricted`, `internal/agent/r2_test.go: TestRestrictedIsNotAGap`, `r2_test.go: TestMentorOnlyMaterialIsNotAGap`, `TestGolden/S6-mentor-only` |
| SUPPORT | «мне плохо», «не справляюсь», «выгораю» — поддержка с именем наставника без вызова LLM и без записи в пробелы | `internal/agent/r2_test.go: TestEmotionalSupport`, `internal/api/r2_test.go: TestEmotionalSupport` |
| OBS-r2 | метка MCP-метода ограничена (initialize, ping, tools/*, notifications, other); `/api/ask` считается в `buddy_agent_runs_total` | `r2_test.go: TestMCPMethodLabelCardinality`, `TestAskCountsAgentRuns` |
| STATE-now | confirm/approve/reject/cancel сохраняют снимок сразу, не дожидаясь таймера | `internal/store/gaps_test.go: TestPersisterSavesOnRequest`, `TestRequestSaveWithoutPersister`, `r2_test.go: TestDecisionRequestsSave`, `cmd/server/main_test.go: TestConfirmSavedImmediately` |
| OPS-exit | код выхода при ошибке старта; зависший SSE-поток не мешает остановке | `cmd/server/main_test.go: TestMainExitCode`, `TestShutdownClosesHangingStreams` |
| SPEC-openapi | каждый путь `specs/contracts/openapi.yaml` есть в `internal/api/router.go` | CI, задача `backend`, шаг `openapi paths exist in router.go` (`.github/workflows/ci.yml`) |

## Интерфейс (frontend/src)

| ID | Что проверяем | Тест |
|---|---|---|
| UI-S1-chat | S1/S3: результат виден в чате — «думает», затем ответ по SSE; ответ с источниками, отказ с отсылкой к наставнику | `features/chat/ChatPanel.test.tsx`: «пустой чат показывает подсказки, отправка — «думает», затем ответ по SSE», «клик по подсказке отправляет её; сессия создаётся один раз»; `components/AssistantMessage.test.tsx`: «answer: текст и уникальные источники», «refusal: предлагает обратиться к наставнику» |
| UI-S1-draft | S1: заявка создаётся только после «Отправить заявку»: черновик → confirm → карточка | `components/DraftCard.test.tsx`: «заявка создаётся только после подтверждения: черновик → confirm → карточка», «без дат просит указать дату и не даёт отправить», «черновик уже сохранён бэком (action_id): только confirm, без второго POST /actions» |
| UI-S1-nohang | S1/INV-5/INV-6: UI не зависает — нет ответа агента → понятная ошибка; сбой агента и сети — русский текст | `features/chat/ChatPanel.test.tsx`: «таймаут: агент не ответил — ошибка, интерфейс не висит», «событие action обновляет прогресс, ошибка агента — понятное сообщение», «сбой создания сессии не залипает: следующая попытка создаёт сессию заново»; `store/chat.test.ts`: «повторное завершение не перетирает результат (таймаут после ответа)»; `api/client.test.ts`: «сервер недоступен (fetch бросает TypeError) — «Нет связи с сервером»» |
| UI-S1-degraded | INV-8: ответ резервной модели помечен в чате | `components/AssistantMessage.test.tsx`: «degraded: резервная модель — честная пометка под ответом» |
| UI-S2 | S2: прогресс по этапам, шаг «ждёт заявку» без кнопки «готово», шаг доступа без заявки — подсказка | `features/progress/ProgressPanel.test.tsx`: «показывает этапы, процент и следующий шаг; отмечает шаг выполненным», «бэк v2: percent_done от сервера и шаг «ждёт заявку»», «шаг kind=access без заявки: вместо «готово» — подсказка запросить доступ», «ошибка изменения шага (чужой шаг → 403) видна пользователю» |
| UI-MY | «Мои заявки»: статус и комментарий руководителя, черновики отдельно, подтверждение черновика MCP, отмена | `features/progress/MyActions.test.tsx`: «показывает отправленные заявки со статусом и комментарием; черновики — отдельным блоком», «черновик от внешнего агента (MCP) подтверждается в интерфейсе», «отмена заявки вызывает cancel и перечитывает список», «ошибка отмены (409) — понятный текст и перечитывание списка; без заявок блок не рисуется» |
| UI-S4 | S4: кабинет руководителя — заявки, прогресс стажёров, пробелы базы, метрики; согласование с комментарием у каждой заявки | `features/team/TeamPanel.test.tsx`: «руководитель видит заявки, прогресс стажёров и пробелы базы; согласует с комментарием», «комментарий уходит только с той заявкой, у которой он написан», «руководитель отклоняет заявку», ««метрики пользы»: плитки сводки по команде из /team/summary»; `App.test.tsx`: «руководителю вместо плана адаптации показывается кабинет команды» |
| UI-S4-roles | S4/INV-10: наставник видит кабинет без кнопок решения; уже решённая заявка (409) — понятный текст | `features/team/TeamPanel.test.tsx`: «наставник видит заявки без кнопок решения», «ошибка согласования показывается, ошибка загрузки — с кнопкой «Повторить»» |
| UI-S4-errors | ошибка загрузки пробелов базы — ошибка, а не «помощник ответил на все вопросы» | `features/team/TeamPanel.test.tsx`: «ошибка загрузки пробелов базы — ошибка с «Повторить», а не «ответил на все вопросы»», «пока пробелы базы грузятся — «Загружаю», а не «ответил на все вопросы»», «пустые списки: нет заявок, стажёров и пробелов» |
| UI-users | переключатель тестовых пользователей, в т. ч. «Руководитель Frontend» | `App.test.tsx`: «показывает пользователя, чат и прогресс; переключение тестового пользователя», «в переключателе есть руководитель команды frontend» |
