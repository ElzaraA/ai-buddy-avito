# Traceability — сценарий → тест

| ID | Сценарий | Тест |
|---|---|---|
| S1-absence | draft, даты, confirm | agent_test.go:TestAskAbsenceDraftWithDate |
| S1-policy | порядок, имена | tools_test.go:TestAbsenceCreateRequest |
| S2-access | REQ-… | tools_test.go:TestDirectoryFindOwner |
| S3-refusal | отказ без источника | chunker_test.go:TestSearchRefusal, agent_test.go:TestAskRefusalNoSources |
| S4-403 | чужой шаг | app_test.go:TestBuildApp |
| S5-async | 202 + SSE | handlers_test.go:TestRouter |
| S6-audience | deny-by-default | audience_test.go:TestDenyByDefault, TestAudienceMentorOnly |
| S7-injection | refusal | agent_test.go:TestAskGuardInjection, mock_test.go:TestMockAbsenceGuard |
| S8-team-filter | фильтр по команде | chunker_test.go:TestSearchTeamFilter |
| S9-store | absence, checklist | store_test.go |
| S10-auth | RBAC | auth_test.go:TestCan |