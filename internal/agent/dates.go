package agent

import (
	"regexp"
	"strings"
	"time"
)

var dateRe = regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2})\b`)

// ParseDates вытаскивает корректные ISO-даты из запроса. Если дат нет — возвращает пусто.
func ParseDates(q string) (string, string) {
	var valid []string
	for _, m := range dateRe.FindAllString(q, -1) {
		if _, err := time.Parse("2006-01-02", m); err == nil {
			valid = append(valid, m)
		}
	}
	switch len(valid) {
	case 0:
		return "", ""
	case 1:
		return valid[0], valid[0]
	default:
		if valid[1] < valid[0] {
			return valid[1], valid[0]
		}
		return valid[0], valid[1]
	}
}

// weekdays — основы названий дней недели (порядок фиксирован, чтобы результат был детерминированным).
var weekdays = []struct {
	stem string
	day  time.Weekday
}{
	{"понедельник", time.Monday},
	{"вторник", time.Tuesday},
	{"среду", time.Wednesday}, // не «сред»: иначе «среда разработки» станет датой
	{"четверг", time.Thursday},
	{"пятниц", time.Friday},
	{"суббот", time.Saturday},
	{"воскресень", time.Sunday},
}

// ParseWeekday — «в пятницу» → ближайшая будущая пятница от today (без учёта регистра).
func ParseWeekday(q string, today time.Time) (string, bool) {
	q = strings.ToLower(q)
	if strings.Contains(q, "завтра") && !strings.Contains(q, "послезавтра") {
		return today.AddDate(0, 0, 1).Format("2006-01-02"), true
	}
	for _, w := range weekdays {
		if strings.Contains(q, w.stem) {
			delta := (int(w.day) - int(today.Weekday()) + 7) % 7
			if delta == 0 {
				delta = 7
			}
			return today.AddDate(0, 0, delta).Format("2006-01-02"), true
		}
	}
	return "", false
}
