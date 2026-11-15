package api

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

// «Пульс», «Сэкономлено наставнику» и «Достижения».

// MinutesPerQuestion — допущение для оценки экономии: столько минут наставник тратит
// на типовой вопрос стажёра (прочитать, переключиться, ответить). Используется только для оценки.
const MinutesPerQuestion = 5

// pulseHistoryDays — сколько последних отметок отдаёт GET /pulse/me.
const pulseHistoryDays = 14

type pulseReq struct {
	Mood int    `json:"mood"`
	Note string `json:"note"`
}

func (h *Handlers) internOnly(w http.ResponseWriter, r *http.Request, perm string) (auth.User, bool) {
	u, _ := auth.FromCtx(r.Context())
	if !auth.Can(u, perm) {
		writeError(w, http.StatusForbidden, "forbidden", "only interns can use this")
		return u, false
	}
	return u, true
}

// SubmitPulse — отметка самочувствия за сегодня (повторная отметка перезаписывает сегодняшнюю).
func (h *Handlers) SubmitPulse(w http.ResponseWriter, r *http.Request) {
	u, ok := h.internOnly(w, r, "pulse.write")
	if !ok {
		return
	}
	var body pulseReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badBody(w, err)
		return
	}
	if body.Mood < store.MinMood || body.Mood > store.MaxMood {
		writeError(w, http.StatusBadRequest, "bad_request", "mood must be 1..5")
		return
	}
	if utf8.RuneCountInString(body.Note) > store.MaxPulseNoteLen {
		writeError(w, http.StatusBadRequest, "bad_request", "note too long (max 300 chars)")
		return
	}
	e := h.Store.SetPulse(u.ID, body.Mood, body.Note)
	h.Metrics.Inc("buddy_pulse_checkins_total", "Отметки «Пульса» стажёров", "mood", strconv.Itoa(body.Mood))
	writeJSON(w, http.StatusOK, e)
}

// MyPulse — последние 14 отметок стажёра (старые первыми).
func (h *Handlers) MyPulse(w http.ResponseWriter, r *http.Request) {
	u, ok := h.internOnly(w, r, "pulse.write")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"today":   h.Store.Today(),
		"entries": h.Store.Pulse(u.ID, pulseHistoryDays),
	})
}

type internPulse struct {
	User auth.User `json:"user"`
	store.PulseStatus
}

// TeamPulse — «Пульс команды»: стажёры своей команды, сначала те, кому нужно внимание.
// Заметки стажёров видят только наставник и руководитель этой команды.
func (h *Handlers) TeamPulse(w http.ResponseWriter, r *http.Request) {
	u, ok := h.teamUser(w, r)
	if !ok {
		return
	}
	today := h.Store.Today()
	out := make([]internPulse, 0)
	for _, intern := range auth.TeamInterns(u.TeamID) {
		out = append(out, internPulse{User: intern, PulseStatus: store.AssessPulse(h.Store.Pulse(intern.ID, 0), today)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Risk && !out[j].Risk })
	writeJSON(w, http.StatusOK, out)
}

// MyAchievements — достижения стажёра.
func (h *Handlers) MyAchievements(w http.ResponseWriter, r *http.Request) {
	u, ok := h.internOnly(w, r, "achievements.read")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.Store.Achievements(u.TeamID, u.ID))
}

// savedStats — поля сводки «Сэкономлено наставнику».
type savedStats struct {
	QuestionsClosed    int64   `json:"questions_closed"`
	SignalsTotal       int64   `json:"signals_total"`
	RefusalsTotal      int64   `json:"refusals_total"`
	MentorMinutesSaved int64   `json:"mentor_minutes_saved"`
	MinutesPerQuestion int     `json:"minutes_per_question"`
	ClosedShare        float64 `json:"closed_share"` // closed / (closed + signals + refusals), 0..1
}

func savedOf(ts store.TeamStats) savedStats {
	s := savedStats{
		QuestionsClosed: ts.Closed, SignalsTotal: ts.Signals, RefusalsTotal: ts.Refusals,
		MentorMinutesSaved: ts.Closed * MinutesPerQuestion, MinutesPerQuestion: MinutesPerQuestion,
	}
	if total := ts.Closed + ts.Signals + ts.Refusals; total > 0 {
		s.ClosedShare = math.Round(float64(ts.Closed)/float64(total)*100) / 100
	}
	return s
}
