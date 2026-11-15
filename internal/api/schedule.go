package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/schedule"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

// Ограничения личных созвонов: защищают хранилище и интерфейс от мусора.
const (
	maxUpcomingMeetings = 50
	maxMeetingAheadDays = 60
	defaultScheduleDays = 7
	maxScheduleDays     = 31
)

// DefaultReminderLead — за сколько до начала встречи помощник напоминает о ней в чате.
const DefaultReminderLead = 15 * time.Minute

func (h *Handlers) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *Handlers) reminderLead() time.Duration {
	if h.ReminderLead > 0 {
		return h.ReminderLead
	}
	return DefaultReminderLead
}

// occurrences — встречи пользователя на ближайшие days дней, начиная с текущего момента.
func (h *Handlers) occurrences(u auth.User, days int) (time.Time, time.Time, []schedule.Occurrence) {
	from := h.now()
	to := from.Add(time.Duration(days) * 24 * time.Hour)
	return from, to, schedule.Expand(schedule.TeamRituals(u), h.Store.MeetingsByOwner(u.ID), from, to)
}

type scheduleResponse struct {
	From            time.Time             `json:"from"`
	To              time.Time             `json:"to"`
	ReminderLeadMin int                   `json:"reminder_lead_min"`
	Items           []schedule.Occurrence `json:"items"`
}

// Schedule — встречи команды и личные созвоны пользователя (?days=1..31, по умолчанию 7).
func (h *Handlers) Schedule(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	days := defaultScheduleDays
	if raw := r.URL.Query().Get("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxScheduleDays {
			writeError(w, http.StatusBadRequest, "bad_request", "days must be 1..31")
			return
		}
		days = n
	}
	from, to, items := h.occurrences(u, days)
	writeJSON(w, http.StatusOK, scheduleResponse{
		From: from, To: to, ReminderLeadMin: int(h.reminderLead() / time.Minute), Items: items,
	})
}

type createMeetingBody struct {
	Title       string `json:"title"`
	Start       string `json:"start"` // RFC3339 или «2026-11-16T15:00» по Москве
	DurationMin int    `json:"duration_min"`
	With        string `json:"with"`
}

// parseStart принимает время с часовым поясом или локальное московское из поля datetime-local.
func parseStart(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, true
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05"} {
		if t, err := time.ParseInLocation(layout, raw, schedule.MSK); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// CreateMeeting — добавить личный созвон в своё расписание.
func (h *Handlers) CreateMeeting(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	var body createMeetingBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	title := strings.TrimSpace(body.Title)
	with := strings.TrimSpace(body.With)
	switch {
	case title == "" || utf8.RuneCountInString(title) > 120:
		writeError(w, http.StatusBadRequest, "bad_request", "Название созвона — от 1 до 120 символов.")
		return
	case utf8.RuneCountInString(with) > 120:
		writeError(w, http.StatusBadRequest, "bad_request", "Поле «С кем» — не длиннее 120 символов.")
		return
	}
	start, ok := parseStart(body.Start)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_request", "Укажите дату и время начала.")
		return
	}
	now := h.now()
	if !start.After(now) {
		writeError(w, http.StatusBadRequest, "bad_request", "Созвон можно добавить только на будущее время.")
		return
	}
	if start.After(now.AddDate(0, 0, maxMeetingAheadDays)) {
		writeError(w, http.StatusBadRequest, "bad_request", "Созвон можно запланировать не дальше чем на 60 дней вперёд.")
		return
	}
	dur := body.DurationMin
	if dur == 0 {
		dur = 30
	}
	if dur < 5 || dur > 240 {
		writeError(w, http.StatusBadRequest, "bad_request", "Длительность — от 5 до 240 минут.")
		return
	}
	if h.Store.CountUpcomingMeetings(u.ID, now) >= maxUpcomingMeetings {
		writeError(w, http.StatusConflict, "too_many_meetings", "В расписании уже 50 будущих созвонов — отмените ненужные.")
		return
	}
	m := h.Store.CreateMeeting(store.Meeting{
		OwnerID: u.ID, Title: title, Start: start.In(schedule.MSK), DurationMin: dur, With: with,
	})
	h.Store.RequestSave()
	occ := schedule.Expand(nil, []store.Meeting{m}, m.Start, m.Start.Add(time.Minute))
	writeJSON(w, http.StatusCreated, occ[0])
}

// CancelMeeting — отменить свой созвон. Встречи команды из интерфейса не отменяются.
func (h *Handlers) CancelMeeting(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	if found, _ := h.Store.CancelMeeting(chi.URLParam(r, "id"), u.ID); !found {
		writeError(w, http.StatusNotFound, "not_found", "meeting not found")
		return
	}
	h.Store.RequestSave()
	w.WriteHeader(http.StatusNoContent)
}

type reminder struct {
	schedule.Occurrence
	Text string `json:"text"`
}

// Reminders — встречи, которые начнутся в ближайшие минуты и о которых пользователю ещё не напоминали.
// Интерфейс опрашивает этот адрес, показывает напоминание в чате и подтверждает показ (ack).
func (h *Handlers) Reminders(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	now := h.now()
	lead := h.reminderLead()
	occ := schedule.Expand(schedule.TeamRituals(u), h.Store.MeetingsByOwner(u.ID), now, now.Add(lead+time.Minute))
	out := make([]reminder, 0)
	for _, o := range schedule.Due(occ, now, lead) {
		if h.Store.ReminderAcked(u.ID, o.ID) {
			continue
		}
		out = append(out, reminder{Occurrence: o, Text: schedule.ReminderText(o, now)})
	}
	writeJSON(w, http.StatusOK, out)
}

// AckReminder — напоминание показано, больше его не присылать.
func (h *Handlers) AckReminder(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	id := chi.URLParam(r, "id")
	if id == "" || utf8.RuneCountInString(id) > 80 {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid reminder id")
		return
	}
	h.Store.AckReminder(u.ID, id)
	w.WriteHeader(http.StatusNoContent)
}
