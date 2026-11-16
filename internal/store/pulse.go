package store

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// «Пульс» — ежедневная отметка самочувствия стажёра (1 — тяжело … 5 — отлично).
// Одна запись на пользователя в день: повторная отметка за сегодня перезаписывает её.
// Заметку видят только наставник и руководитель команды стажёра; персональные данные в ней маскируются.

type PulseEntry struct {
	Date string    `json:"date"` // YYYY-MM-DD в поясе продукта (TZ_NAME, по умолчанию Europe/Moscow)
	Mood int       `json:"mood"`
	Note string    `json:"note,omitempty"`
	At   time.Time `json:"at"`
}

const (
	MinMood         = 1
	MaxMood         = 5
	MaxPulseNoteLen = 300
	// maxPulsePerUser — сколько дней истории храним (с запасом на «3 дня подряд» и графики).
	maxPulsePerUser = 120
	// PulseWindowDays — окно для среднего, тренда и правил риска.
	PulseWindowDays = 7
)

const dateLayout = "2006-01-02"

// Today — сегодняшняя дата в формате записей «Пульса».
func (s *Store) Today() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return LocalDate(s.now())
}

// SetClock подменяет часы хранилища (тесты и демо-сценарии).
func (s *Store) SetClock(now func() time.Time) {
	s.mu.Lock()
	s.now = now
	s.mu.Unlock()
}

// SetPulse сохраняет отметку за сегодня (перезаписывая сегодняшнюю, если была).
// mood и длина заметки проверяются вызывающим кодом; здесь заметка маскируется и обрезается.
func (s *Store) SetPulse(userID string, mood int, note string) PulseEntry {
	note = strings.TrimSpace(MaskPII(note))
	if r := []rune(note); len(r) > MaxPulseNoteLen {
		note = string(r[:MaxPulseNoteLen])
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	e := PulseEntry{Date: LocalDate(now), Mood: mood, Note: note, At: now}
	list := s.pulse[userID]
	if n := len(list); n > 0 && list[n-1].Date == e.Date {
		list = append(append([]PulseEntry(nil), list[:n-1]...), e)
	} else {
		list = append(append([]PulseEntry(nil), list...), e)
	}
	if len(list) > maxPulsePerUser {
		list = list[len(list)-maxPulsePerUser:]
	}
	s.pulse[userID] = list
	s.touch()
	return e
}

// Pulse — последние n отметок пользователя в хронологическом порядке (старые первыми).
func (s *Store) Pulse(userID string, n int) []PulseEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src := s.pulse[userID]
	if n > 0 && len(src) > n {
		src = src[len(src)-n:]
	}
	return append(make([]PulseEntry, 0, len(src)), src...)
}

// PulseDay — отметка за конкретный день окна (Mood == nil — отметки не было).
type PulseDay struct {
	Date string `json:"date"`
	Mood *int   `json:"mood"`
}

// PulseStatus — сводка «Пульса» стажёра для наставника.
type PulseStatus struct {
	Today      *int       `json:"today"`
	Avg7       *float64   `json:"avg7"`
	Trend      string     `json:"trend"` // up | down | flat
	Risk       bool       `json:"risk"`
	RiskReason string     `json:"risk_reason,omitempty"`
	Days       []PulseDay `json:"days"`           // последние 7 дней, старые первыми
	Note       string     `json:"note,omitempty"` // последняя заметка за 7 дней
	NoteDate   string     `json:"note_date,omitempty"`
}

// MoodLabels — подписи шкалы «Пульса» (как на кнопках у стажёра).
var MoodLabels = map[int]string{1: "Тяжело", 2: "Так себе", 3: "Нормально", 4: "Хорошо", 5: "Отлично"}

// AssessPulse считает сводку по отметкам (в хронологическом порядке) на дату today.
//
// Правила риска (любое из):
//   - сегодняшняя отметка ≤ 2;
//   - две из трёх последних отметок за 7 дней ≤ 2;
//   - среднее за 7 дней ≤ 2.5.
//
// Тренд: последняя отметка окна сравнивается со средним предыдущих отметок окна
// (разница ≥ 0.5 — up/down, иначе flat; меньше двух отметок — flat).
func AssessPulse(entries []PulseEntry, today string) PulseStatus {
	st := PulseStatus{Trend: "flat", Days: make([]PulseDay, 0, PulseWindowDays)}
	end, err := time.Parse(dateLayout, today)
	if err != nil {
		return st
	}
	byDate := map[string]PulseEntry{}
	for _, e := range entries {
		byDate[e.Date] = e
	}
	var window []PulseEntry // отметки окна, старые первыми
	for i := PulseWindowDays - 1; i >= 0; i-- {
		d := end.AddDate(0, 0, -i).Format(dateLayout)
		day := PulseDay{Date: d}
		if e, ok := byDate[d]; ok {
			m := e.Mood
			day.Mood = &m
			window = append(window, e)
		}
		st.Days = append(st.Days, day)
	}
	if e, ok := byDate[today]; ok {
		m := e.Mood
		st.Today = &m
	}
	if len(window) == 0 {
		return st
	}
	sum := 0
	for _, e := range window {
		sum += e.Mood
		if e.Note != "" {
			st.Note, st.NoteDate = e.Note, e.Date
		}
	}
	avg := math.Round(float64(sum)/float64(len(window))*10) / 10
	st.Avg7 = &avg

	if n := len(window); n >= 2 {
		prev := 0
		for _, e := range window[:n-1] {
			prev += e.Mood
		}
		diff := float64(window[n-1].Mood) - float64(prev)/float64(n-1)
		switch {
		case diff >= 0.5:
			st.Trend = "up"
		case diff <= -0.5:
			st.Trend = "down"
		}
	}

	last3 := window
	if len(last3) > 3 {
		last3 = last3[len(last3)-3:]
	}
	low := 0
	for _, e := range last3 {
		if e.Mood <= 2 {
			low++
		}
	}
	switch {
	case st.Today != nil && *st.Today <= 2:
		st.Risk, st.RiskReason = true, fmt.Sprintf("сегодня отметил(а) «%s»", MoodLabels[*st.Today])
	case low >= 2:
		st.Risk, st.RiskReason = true, fmt.Sprintf("%d из %d последних отметок — «Тяжело» или «Так себе»", low, len(last3))
	case avg <= 2.5:
		st.Risk, st.RiskReason = true, fmt.Sprintf("среднее за неделю — %s из 5", strings.Replace(fmt.Sprintf("%.1f", avg), ".", ",", 1))
	}
	return st
}

// PulseStreak — самая длинная серия отметок подряд (по дням) и дата, когда серия впервые достигла want дней.
func PulseStreak(entries []PulseEntry, want int) (best int, reachedAt *time.Time) {
	run := 0
	var prev time.Time
	for i, e := range entries {
		d, err := time.Parse(dateLayout, e.Date)
		if err != nil {
			run = 0
			continue
		}
		if i > 0 && run > 0 && d.Equal(prev.AddDate(0, 0, 1)) {
			run++
		} else {
			run = 1
		}
		prev = d
		if run > best {
			best = run
		}
		if run >= want && reachedAt == nil {
			at := e.At
			reachedAt = &at
		}
	}
	return best, reachedAt
}
