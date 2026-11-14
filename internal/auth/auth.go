package auth

import (
	"context"
	"sort"
)

type Role string

const (
	RoleIntern Role = "intern"
	RoleBuddy  Role = "buddy"
	RoleLead   Role = "lead"
	RoleHRBP   Role = "hr_bp"
)

type User struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Role   Role   `json:"role"`
	TeamID string `json:"team_id"`
}

type ctxKey struct{}

func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

func FromCtx(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxKey{}).(User)
	return u, ok
}

// DemoUsers — тестовые учётки для проверки прав (X-Demo-User).
var DemoUsers = map[string]User{
	"intern.backend": {ID: "intern-1", Name: "Стажёр Backend", Role: RoleIntern, TeamID: "backend"},
	"mentor.backend": {ID: "mentor-1", Name: "Наставник Backend", Role: RoleBuddy, TeamID: "backend"},
	"intern.other":   {ID: "intern-2", Name: "Стажёр другой команды", Role: RoleIntern, TeamID: "frontend"},
	"lead.backend":   {ID: "lead-1", Name: "Руководитель Backend", Role: RoleLead, TeamID: "backend"},
	"intern.junior":  {ID: "intern-3", Name: "Второй стажёр Backend", Role: RoleIntern, TeamID: "backend"},
	"lead.frontend":  {ID: "lead-2", Name: "Руководитель Frontend", Role: RoleLead, TeamID: "frontend"},
}

// Contacts — наставник и руководитель команды (синтетические имена прототипа) в нужных падежах.
type Contacts struct {
	Buddy           string // «Иван Петров»
	BuddyDative     string // «обратитесь к наставнику Ивану Петрову»
	BuddyAccusative string // «предупредите наставника Ивана Петрова»
	Manager         string // «согласует руководитель — Мария Сидорова»
	// Stems — основы фамилий для поиска имени в тексте в любом падеже («Петров» ловит «Петрову»).
	Stems []string
}

// DefaultTeam — команда, контакты которой берутся для неизвестной команды.
const DefaultTeam = "backend"

var teamContacts = map[string]Contacts{
	"backend": {
		Buddy: "Иван Петров", BuddyDative: "Ивану Петрову", BuddyAccusative: "Ивана Петрова",
		Manager: "Мария Сидорова", Stems: []string{"Петров", "Сидоров"},
	},
	"frontend": {
		Buddy: "Анна Козлова", BuddyDative: "Анне Козловой", BuddyAccusative: "Анну Козлову",
		Manager: "Дмитрий Орлов", Stems: []string{"Козлов", "Орлов"},
	},
}

// TeamContacts — наставник и руководитель команды; для неизвестной команды — контакты DefaultTeam.
func TeamContacts(teamID string) Contacts {
	if c, ok := teamContacts[teamID]; ok {
		return c
	}
	return teamContacts[DefaultTeam]
}

// ForeignContactStems — основы фамилий контактов других команд (чтобы не назвать стажёру чужого наставника).
func ForeignContactStems(teamID string) []string {
	own := TeamContacts(teamID)
	var out []string
	for _, c := range teamContacts {
		if c.Buddy == own.Buddy {
			continue
		}
		out = append(out, c.Stems...)
	}
	sort.Strings(out)
	return out
}

// TeamInterns — стажёры команды (для кабинета наставника и руководителя).
func TeamInterns(teamID string) []User {
	var out []User
	for _, u := range DemoUsers {
		if u.Role == RoleIntern && u.TeamID == teamID {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ByID ищет тестового пользователя по id.
func ByID(id string) (User, bool) {
	for _, u := range DemoUsers {
		if u.ID == id {
			return u, true
		}
	}
	return User{}, false
}

func Can(u User, action string) bool {
	switch action {
	case "checklist.read":
		return true
	case "absence.create":
		return u.Role == RoleIntern || u.Role == RoleBuddy
	case "access.request":
		return u.Role == RoleIntern || u.Role == RoleBuddy
	case "directory.read":
		return u.Role == RoleIntern || u.Role == RoleBuddy || u.Role == RoleLead
	case "action.approve":
		// Согласует руководитель команды.
		return u.Role == RoleLead
	case "team.read":
		// Прогресс стажёров, заявки команды и пробелы базы знаний видят наставник и руководитель.
		return u.Role == RoleBuddy || u.Role == RoleLead
	case "absence.read.all":
		return u.Role == RoleLead || u.Role == RoleHRBP
	default:
		return false
	}
}
