package store

import "strings"

// Шаг «Получить доступы (Git, Jira, Grafana)» закрывается, только когда одобрены доступы ко ВСЕМ
// перечисленным системам. Пока шаг выполнен частично, он показывает прогресс (sub_done из sub_total)
// и список систем, которые ещё нужно запросить (missing).

// AccessSystems — системы шага «Получить доступы», в порядке показа. Git и GitLab — одна система.
var AccessSystems = []string{"gitlab", "jira", "grafana"}

// AccessSystemTitle — имя системы для текста.
func AccessSystemTitle(r string) string {
	switch r {
	case "gitlab":
		return "GitLab"
	case "jira":
		return "Jira"
	case "grafana":
		return "Grafana"
	case "git":
		return "Git"
	}
	return r
}

// accessSystem приводит ресурс заявки к системе шага: git → gitlab.
func accessSystem(r string) string {
	r = strings.ToLower(strings.TrimSpace(r))
	if r == "git" {
		return "gitlab"
	}
	return r
}

// AccessStatus — состояние заявок пользователя на доступ по системам шага: approved или pending
// (одобренная заявка важнее ждущей). Системы без живой заявки в карте отсутствуют.
func (s *Store) AccessStatus(userID string) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, _, _ := s.accessStatusLocked(userID)
	return st
}

// accessStatusLocked — состояние по системам плюс номер последней ждущей и последней одобренной заявки.
func (s *Store) accessStatusLocked(userID string) (map[string]string, string, string) {
	out := map[string]string{}
	var pendingID, approvedID string
	var pendingAt, approvedAt int64
	for _, a := range s.absences {
		if a.UserID != userID || a.Type != ActionAccess {
			continue
		}
		sys := accessSystem(a.Resource)
		switch a.Status {
		case StatusApproved:
			out[sys] = StatusApproved
			if t := a.UpdatedAt.UnixNano(); approvedID == "" || t > approvedAt || (t == approvedAt && a.ID > approvedID) {
				approvedID, approvedAt = a.ID, t
			}
		case StatusPending:
			if out[sys] != StatusApproved {
				out[sys] = StatusPending
			}
			if t := a.UpdatedAt.UnixNano(); pendingID == "" || t > pendingAt || (t == pendingAt && a.ID > pendingID) {
				pendingID, pendingAt = a.ID, t
			}
		}
	}
	return out, pendingID, approvedID
}

// applyAccess выставляет шагу доступа прогресс по системам и статус.
// Все системы одобрены → done; есть система без заявки → todo (стажёру есть что сделать);
// иначе (всё запрошено, часть ждёт решения) → waiting.
func applyAccess(it *ChecklistItem, status map[string]string, pendingID, approvedID string) {
	total := len(AccessSystems)
	if it.Done {
		done := total
		it.SubDone, it.SubTotal, it.Missing = &done, total, nil
		it.Status = StepDone
		return
	}
	approved := 0
	var missing []string
	for _, sys := range AccessSystems {
		switch status[sys] {
		case StatusApproved:
			approved++
		case StatusPending:
		default:
			missing = append(missing, sys)
		}
	}
	it.SubDone, it.SubTotal, it.Missing = &approved, total, missing
	switch {
	case approved == total:
		it.Done, it.Status, it.ActionID = true, StepDone, approvedID
	case len(missing) > 0:
		it.Status, it.ActionID = StepTodo, pendingID
	default:
		it.Status, it.ActionID = StepWaiting, pendingID
	}
}

// SyncAccessStep пересчитывает шаг доступа пользователя по его заявкам (после подтверждения,
// одобрения, отклонения или отмены заявки на доступ). Возвращает шаг и true, если он есть.
func (s *Store) SyncAccessStep(userID string) (ChecklistItem, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.checklist[userID]
	status, pendingID, approvedID := s.accessStatusLocked(userID)
	for i := range items {
		if items[i].Kind != ActionAccess {
			continue
		}
		applyAccess(&items[i], status, pendingID, approvedID)
		s.touch()
		return items[i], true
	}
	return ChecklistItem{}, false
}
