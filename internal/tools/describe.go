package tools

import "sort"

// Spec — описание инструмента для MCP (tools/list) и для промпта LLM.
type Spec struct {
	Name                 string         `json:"name"`
	Description          string         `json:"description"`
	Permission           string         `json:"permission"`
	RequiresConfirmation bool           `json:"requiresConfirmation"`
	InputSchema          map[string]any `json:"inputSchema"`
}

func object(required []string, props map[string]any) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

func date(desc string) map[string]any {
	return map[string]any{"type": "string", "format": "date", "description": desc}
}

// specs — единый источник описаний; синхронизирован со specs/contracts/mcp-tools.json (проверяется тестом).
var specs = map[string]Spec{
	"checklist_get": {
		Description: "Чек-лист адаптации текущего пользователя и следующий шаг",
		InputSchema: object(nil, map[string]any{}),
	},
	"absence_get_policy": {
		Description: "ТЕСТОВЫЕ правила оформления отгула, имена руководителя и наставника",
		InputSchema: object(nil, map[string]any{}),
	},
	"absence_create_request": {
		Description: "Заявка на отгул от имени текущего пользователя. Только через черновик и подтверждение.",
		InputSchema: object([]string{"date_from", "date_to"}, map[string]any{
			"date_from": date("первый день отгула"),
			"date_to":   date("последний день отгула"),
			"reason":    str("причина"),
		}),
	},
	"access_request": {
		Description: "Заявка на доступ к системе (Jira, GitLab, Grafana, Git). Только через черновик и подтверждение.",
		InputSchema: object([]string{"resource"}, map[string]any{
			"resource": map[string]any{"type": "string", "enum": []string{"jira", "gitlab", "grafana", "git"}},
			"reason":   str("зачем нужен доступ"),
		}),
	},
	"schedule_get": {
		Description: "Встречи пользователя: стендапы, 1:1, планирование, ретро и личные созвоны",
		InputSchema: object(nil, map[string]any{
			"when": map[string]any{"type": "string", "enum": []string{"today", "tomorrow", "week"}, "description": "период, по умолчанию неделя"},
			"kind": map[string]any{"type": "string", "enum": []string{"standup", "one_on_one", "planning", "retro"}, "description": "только встречи одного вида"},
		}),
	},
	"schedule_add": {
		Description: "Добавить личный созвон в расписание пользователя; Бадди напомнит о нём в чате",
		InputSchema: object(nil, map[string]any{
			"time":         str("время начала, ЧЧ:ММ по Москве"),
			"in_min":       str("через сколько минут от текущего момента — вместо date и time"),
			"date":         str("today, tomorrow, day_after_tomorrow, monday…sunday, ДД.ММ или ГГГГ-ММ-ДД; по умолчанию сегодня, если время прошло — завтра"),
			"title":        str("название, например «Созвон с Катей»"),
			"duration_min": str("длительность в минутах, 5–240, по умолчанию 30"),
			"with":         str("с кем"),
		}),
	},
	"directory_find_owner": {
		Description: "Кто отвечает за тему (доступы, инструменты, чек-лист, отгулы)",
		InputSchema: object([]string{"query"}, map[string]any{"query": str("тема")}),
	},
}

// Describe возвращает описание зарегистрированного инструмента.
func Describe(t Tool) Spec {
	s := specs[t.Name()]
	s.Name = t.Name()
	s.Permission = t.RequiredPermission()
	s.RequiresConfirmation = t.RequiresConfirmation()
	if s.InputSchema == nil {
		s.InputSchema = object(nil, map[string]any{})
	}
	return s
}

// List — описания всех зарегистрированных инструментов по алфавиту.
func (r *Registry) List() []Spec {
	out := make([]Spec, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, Describe(t))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
