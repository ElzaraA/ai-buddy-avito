package agent

import (
	"regexp"
	"time"
)

var dateRe = regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2})\b`)

// ParseDates вытаскивает ISO-даты из запроса. Если дат нет — возвращает пусто.
func ParseDates(q string) (string, string) {
	m := dateRe.FindAllString(q, -1)
	switch len(m) {
	case 0:
		return "", ""
	case 1:
		return m[0], m[0]
	default:
		return m[0], m[1]
	}
}

// ParseWeekday — простой парсер «в пятницу» → ближайшая пятница от today.
func ParseWeekday(q string, today time.Time) (string, bool) {
	days := map[string]time.Weekday{
		"понедельник": time.Monday, "вторник": time.Tuesday, "среду": time.Wednesday,
		"четверг": time.Thursday, "пятниц": time.Friday, "суббот": time.Saturday, "воскресень": time.Sunday,
	}
	for k, wd := range days {
		if regexp.MustCompile(k).MatchString(q) {
			delta := (int(wd) - int(today.Weekday()) + 7) % 7
			if delta == 0 {
				delta = 7
			}
			d := today.AddDate(0, 0, delta)
			return d.Format("2006-01-02"), true
		}
	}
	return "", false
}
