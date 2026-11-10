# Agent Spec — AI-buddy

## Роль
AI-buddy — помощник стажёра Авито (Backend). Отвечает по базе знаний, выполняет действия через MCP-инструменты, ведёт по чек-листу.

## Инварианты
1. Нет источника — нет ответа. RAG не нашёл чанк → refusal + buddy_name.
2. Нет права — нет действия. auth.Can перед каждым инструментом.
3. Deny-by-default по audience. Чанк без audience невидим.
4. Draft → Confirm. Агент не создаёт заявку сам, только draft.
5. LLM не блокирует UI. POST /messages → 202 + SSE.
6. Таймауты: run 60с, инструмент 10с.
7. Формат ответа — JSON (response_format: json_object).

## Формат ответа
{"type":"answer","text":"...","sources":[...]}
{"type":"tool_call","tool":"...","args":{...}}
{"type":"draft","action":"absence","args":{...}}
{"type":"refusal","text":"...","buddy_name":"Иван Петров"}

## Инструменты
| Имя | Право | Результат |
|---|---|---|
| checklist_get | checklist.read | items, next_step |
| absence_get_policy | checklist.read | policy, manager_name, buddy_name |
| absence_create_request | absence.create | absence_id ABS-…, status |
| access_request | checklist.read | request_id REQ-…, owner |
| directory_find_owner | directory.read | owner |

## RAG
- Источник: data/knowledge/*.md
- Чанкинг по #, метаданные stage/team/audience
- BM25 + префиксный (5 рун), minScore 0.5
- Фильтр: teamID + audience (deny-by-default)

## Асинхронность
POST /api/v1/chat/sessions → 201
POST /api/v1/chat/sessions/{id}/messages → 202
GET /api/v1/chat/sessions/{id}/events (SSE): status → tool_result → message_done

## Ошибки
JSON {"error":{"code","message"}}. Коды: bad_request, unauthorized, forbidden, not_found, conflict, agent_error.