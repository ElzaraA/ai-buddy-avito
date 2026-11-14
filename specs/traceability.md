# Traceability — сценарий / инвариант → тест

Все тесты запускаются `make test`; golden-набор — `data/golden/golden.json` (фразы из README и сценариев).

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
| INV-6 | сбой/мусор от LLM не ломает ответ | `TestLLMOutputs`, `TestLLMError` |
| INV-7 | prompt injection | `TestGolden/S7-injection`, `TestGolden/S7-injection-ru`, `TestToolCallDoesNotMutate` |
| SEC-IDOR | чужие сессия/сообщения/SSE/действия — 403 | `TestChatForeignSessionForbidden`, `TestActionsOwnershipAndRoles` |
| SEC-input | лимит тела 64 КБ, вопрос ≤ 2000 символов, валидация дат | `TestBodyLimit`, `TestChatBadInput`, `TestCreateActionValidation` |
| SEC-auth | неизвестный X-Demo-User — 401 | `TestAuthAndMe`, `actions_test.go: TestUnauthorized` |
| OPS | старт, /health, graceful shutdown, ошибки старта | `cmd/server/main_test.go` |
| S1-approve | руководитель согласует/отклоняет; автор, наставник и чужая команда — 403; история заявки | `internal/api/lifecycle_test.go: TestAbsenceApprovalFlow`, `TestRejectAndCancel` |
| S2-waiting | заявка на доступ двигает шаг чек-листа: waiting → done / todo | `lifecycle_test.go: TestAccessRequestDrivesChecklistStep`, `internal/store/persist_test.go: TestLinkAndUnlinkStep` |
| TEAM | кабинет наставника: прогресс своей команды, входящие, сводка; стажёру — 403 | `lifecycle_test.go: TestTeamViews` |
| GAPS | вопросы без ответа копятся обезличенно | `TestTeamViews`, `persist_test.go: TestGaps`, `internal/agent/resilience_test.go: TestNoSourceFlag` |
| INV-8 | повторы при 429/5xx/сети, без повтора при 4xx; резервная модель и `degraded` | `internal/llm/retry_test.go`, `resilience_test.go: TestFallbackModel`, `internal/api/resilience_test.go: TestFallbackWhenLLMDown` |
| INV-8b | LLM недоступна без резерва: 502 / событие error, внутренние детали не утекают; паника не роняет сервер | `TestLLMDownWithoutFallback`, `TestAgentPanicIsContained` |
| INV-9 | MCP: handshake, tools/list, права, мутация → черновик, ошибки JSON-RPC | `internal/api/mcp_test.go` |
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
