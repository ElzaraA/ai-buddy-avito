package agent

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	dateRe = regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2})\b`)
	// dotDateRe — «20.11» или «20.11.2026».
	dotDateRe = regexp.MustCompile(`(?:^|[^\d.])(\d{1,2})\.(\d{1,2})(?:\.(\d{4}|\d{2}))?(?:$|[^\d.]|\.(?:$|\s))`)
	// monthDateRe — «20 ноября», «20-22 ноября», «с 20 по 22 ноября 2026».
	monthDateRe = regexp.MustCompile(`(?:^|[^\d])(\d{1,2})(?:\s*(?:-|–|—|по|до)\s*(\d{1,2}))?\s+(январ|феврал|март|апрел|ма[йя]|июн|июл|август|сентябр|октябр|ноябр|декабр)[а-я]*(?:\s+(\d{4}))?`)
)

// monthStems — основы названий месяцев в порядке месяцев.
var monthStems = []string{"январ", "феврал", "март", "апрел", "ма", "июн", "июл", "август", "сентябр", "октябр", "ноябр", "декабр"}

// monthGenitive — «20 ноября» для подсказок пользователю.
var monthGenitive = []string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}

// HumanDate — дата в виде «20 ноября».
func HumanDate(t time.Time) string {
	return strconv.Itoa(t.Day()) + " " + monthGenitive[t.Month()-1]
}

// ParseDates вытаскивает корректные ISO-даты из запроса. Если дат нет — возвращает пусто.
func ParseDates(q string) (string, string) {
	var valid []string
	for _, m := range dateRe.FindAllString(q, -1) {
		if _, err := time.Parse("2006-01-02", m); err == nil {
			valid = append(valid, m)
		}
	}
	return firstTwo(valid)
}

// ParseDatesAt понимает ISO-даты, «20.11[.2026]», «20 ноября» и диапазоны «с 20 по 22 ноября».
// Без года берётся ближайшая будущая дата относительно today.
func ParseDatesAt(q string, today time.Time) (string, string) {
	if f, t := ParseDates(q); f != "" {
		return f, t
	}
	q = strings.ToLower(q)
	type found struct {
		pos  int
		date string
	}
	var all []found
	add := func(pos int, day, month int, year string) {
		if d, ok := makeDate(day, month, year, today); ok {
			all = append(all, found{pos, d})
		}
	}
	for _, m := range monthDateRe.FindAllStringSubmatchIndex(q, -1) {
		month := monthIndex(q[m[6]:m[7]])
		year := ""
		if m[8] >= 0 {
			year = q[m[8]:m[9]]
		}
		add(m[2], atoi(q[m[2]:m[3]]), month, year)
		if m[4] >= 0 {
			add(m[4], atoi(q[m[4]:m[5]]), month, year)
		}
	}
	for _, m := range dotDateRe.FindAllStringSubmatchIndex(q, -1) {
		year := ""
		if m[6] >= 0 {
			year = q[m[6]:m[7]]
		}
		add(m[2], atoi(q[m[2]:m[3]]), atoi(q[m[4]:m[5]]), year)
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].pos < all[j].pos })
	dates := make([]string, 0, len(all))
	for _, f := range all {
		dates = append(dates, f.date)
	}
	return firstTwo(dates)
}

// InvalidISODate возвращает дату вида YYYY-MM-DD из запроса, которой не бывает (например, 2026-02-30).
func InvalidISODate(q string) string {
	for _, m := range dateRe.FindAllString(q, -1) {
		if _, err := time.Parse("2006-01-02", m); err != nil {
			return m
		}
	}
	return ""
}

func firstTwo(valid []string) (string, string) {
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

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func monthIndex(stem string) int {
	for i, s := range monthStems {
		if strings.HasPrefix(stem, s) {
			return i + 1
		}
	}
	return 0
}

// makeDate собирает дату и отбрасывает несуществующие (30 февраля). Без года — ближайшая будущая.
func makeDate(day, month int, year string, today time.Time) (string, bool) {
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return "", false
	}
	y := today.Year()
	explicit := year != ""
	if explicit {
		y = atoi(year)
		if y < 100 {
			y += 2000
		}
	}
	d := time.Date(y, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if d.Day() != day || int(d.Month()) != month {
		return "", false
	}
	if !explicit && d.Format("2006-01-02") < today.Format("2006-01-02") {
		d = d.AddDate(1, 0, 0)
		if d.Day() != day { // 29 февраля в невисокосный год
			return "", false
		}
	}
	return d.Format("2006-01-02"), true
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
// Понимает также «сегодня», «завтра» и «послезавтра».
func ParseWeekday(q string, today time.Time) (string, bool) {
	q = strings.ToLower(q)
	switch {
	case strings.Contains(q, "послезавтра"):
		return today.AddDate(0, 0, 2).Format("2006-01-02"), true
	case strings.Contains(q, "завтра"):
		return today.AddDate(0, 0, 1).Format("2006-01-02"), true
	case strings.Contains(q, "сегодня"):
		return today.Format("2006-01-02"), true
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
