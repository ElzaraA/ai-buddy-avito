package store

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Ограничения действий едины для всех входов: POST /actions, чат, MCP — и повторно при подтверждении.
const (
	MaxReasonRunes = 500
	MaxAbsenceDays = 30
	// MaxOpenDrafts — сколько неподтверждённых черновиков может держать один пользователь.
	MaxOpenDrafts = 50
)

// ErrTooManyDrafts — у пользователя слишком много неподтверждённых черновиков.
var ErrTooManyDrafts = errors.New("too many drafts")

// accessResources — системы, доступ к которым можно запросить (whitelist).
var accessResources = map[string]bool{"jira": true, "gitlab": true, "grafana": true, "git": true}

// NormalizeResource приводит имя системы к виду из whitelist («GitLab » → «gitlab»).
func NormalizeResource(r string) string {
	return strings.ToLower(strings.TrimSpace(r))
}

// ValidResource — можно ли запросить доступ к этой системе.
func ValidResource(r string) bool { return accessResources[r] }

// ValidateAbsenceDates: формат YYYY-MM-DD, конец не раньше начала, начало не в прошлом, не дольше 30 дней.
func ValidateAbsenceDates(from, to string, now time.Time) error {
	f, err1 := time.Parse("2006-01-02", from)
	t, err2 := time.Parse("2006-01-02", to)
	switch {
	case err1 != nil || err2 != nil:
		return errors.New("date_from/date_to must be YYYY-MM-DD")
	case t.Before(f):
		return errors.New("date_to must be >= date_from")
	case from < now.Format("2006-01-02"):
		return errors.New("date_from is in the past")
	case t.Sub(f) > MaxAbsenceDays*24*time.Hour:
		return errors.New("absence longer than 30 days is not supported")
	}
	return nil
}

// ValidateAction проверяет действие перед созданием черновика и перед подтверждением.
func ValidateAction(a Absence, now time.Time) error {
	if utf8.RuneCountInString(a.Reason) > MaxReasonRunes {
		return errors.New("reason too long (max 500 chars)")
	}
	switch a.Type {
	case ActionAbsence:
		return ValidateAbsenceDates(a.DateFrom, a.DateTo, now)
	case ActionAccess:
		if a.Resource == "" {
			return errors.New("resource required")
		}
		if !ValidResource(a.Resource) {
			return errors.New("resource must be one of: jira, gitlab, grafana, git")
		}
		return nil
	}
	return errors.New("type must be absence or access")
}

// CreateDraft атомарно проверяет лимит открытых черновиков пользователя и сохраняет черновик.
func (s *Store) CreateDraft(a Absence) (Absence, error) {
	a.Status = StatusDraft
	a = s.prepare(a)
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, x := range s.absences {
		if x.UserID == a.UserID && x.Status == StatusDraft {
			n++
		}
	}
	if n >= MaxOpenDrafts {
		return Absence{}, ErrTooManyDrafts
	}
	s.absences[a.ID] = a
	s.touch()
	return cloneAction(a), nil
}
