package llm

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Просьба добавить встречу: глагол-просьба и слово о встрече.
var (
	addVerbs = []string{"добав", "запланир", "поставь", "поставить", "назначь", "назначить", "создай", "создать",
		"запиши", "записать", "внеси", "внести", "забронир", "закинь", "впиши"}
	meetingWords = []string{"встреч", "созвон", "синк", "звонок", "колл", "митинг", "расписан", "1 1"}
)

// wantsAddMeeting — «добавь в расписание завтра встречу в 15.00», «поставь созвон с Катей на 17:00».
func wantsAddMeeting(w string) bool {
	return hasWord(w, addVerbs...) && hasWord(w, meetingWords...)
}

var (
	// время: «15:00», «15.00», «в 15», «на 9 утра», «к 3 дня»
	reTimeColon = regexp.MustCompile(`(?:^|[^\d.:])(\d{1,2}):(\d{2})(?:$|[^\d])`)
	reTimePrep  = regexp.MustCompile(`(?:^|\s)(?:в|на|к|с)\s+(\d{1,2})(?:\.(\d{2}))?(?:\s*(?:час\S*))?(?:\s+(утра|дня|вечера|ночи))?(?:$|[^\d.])`)
	reDayMonth  = regexp.MustCompile(`(?:^|[^\d.:])(\d{1,2})\.(\d{1,2})(?:$|[^\d])`)
	reMinutes   = regexp.MustCompile(`(?:^|\s)на\s+(\d{1,3})\s*мин`)
	reIn        = regexp.MustCompile(`через\s+(\d{1,3})\s*мин`)
	// с кем: «с Катей», «со Светой»; служебные слова после «с» — не имена
	reWith = regexp.MustCompile(`(?:^|\s)(?:с|со)\s+([a-zа-я-]{2,})`)
)

// после «с» — не имя: «с утра», «с понедельника»
var notNames = map[string]bool{"утра": true, "обеда": true, "понедельника": true, "завтра": true, "сегодня": true}

// роли пишем со строчной: «Созвон с наставником», а имена — с заглавной: «Созвон с Катей»
var roleWords = []string{"наставник", "ментор", "руководител", "тимлид", "лид", "команд", "бадди", "hr", "эйчар"}

var weekdayStems = []struct{ stem, day string }{
	{"понедельник", "monday"}, {"вторник", "tuesday"}, {"сред", "wednesday"}, {"четверг", "thursday"},
	{"пятниц", "friday"}, {"суббот", "saturday"}, {"воскресень", "sunday"},
}

// scheduleAddArgs разбирает просьбу о встрече в аргументы инструмента schedule_add.
// Строгую проверку (прошедшее время, лимиты) делает сам инструмент.
func scheduleAddArgs(q, w string) map[string]string {
	args := map[string]string{}

	// «через 10 минут», «через полчаса», «через час» — от текущего момента
	switch m := reIn.FindStringSubmatch(q); {
	case m != nil:
		args["in_min"] = m[1]
	case hasWord(w, "через полчаса"):
		args["in_min"] = "30"
	case hasWord(w, "через час"):
		args["in_min"] = "60"
	}

	// время
	timeToken := ""
	if args["in_min"] != "" {
		// время уже задано
	} else if m := reTimeColon.FindStringSubmatch(q); m != nil {
		args["time"] = m[1] + ":" + m[2]
		timeToken = m[1] + ":" + m[2]
	} else if m := reTimePrep.FindStringSubmatch(q); m != nil {
		h, _ := strconv.Atoi(m[1])
		if (m[3] == "дня" || m[3] == "вечера") && h < 12 {
			h += 12
		}
		min := "00"
		if m[2] != "" {
			min = m[2]
			timeToken = m[1] + "." + m[2]
		}
		args["time"] = strconv.Itoa(h) + ":" + min
	}

	// дата
	switch {
	case hasWord(w, "послезавтра"):
		args["date"] = "day_after_tomorrow"
	case hasWord(w, "завтра"):
		args["date"] = "tomorrow"
	case hasWord(w, "сегодня"):
		args["date"] = "today"
	default:
		for _, d := range weekdayStems {
			if hasWord(w, d.stem) {
				args["date"] = d.day
				break
			}
		}
	}
	if args["date"] == "" {
		for _, m := range reDayMonth.FindAllStringSubmatch(q, -1) {
			if m[1]+"."+m[2] == timeToken {
				continue
			}
			if mon, _ := strconv.Atoi(m[2]); mon >= 1 && mon <= 12 {
				args["date"] = m[1] + "." + m[2]
				break
			}
		}
	}

	// длительность
	switch {
	case hasWord(w, "полтора часа"):
		args["duration_min"] = "90"
	case hasWord(w, "на полчаса"):
		args["duration_min"] = "30"
	case hasWord(w, "на час", "на 1 час"):
		args["duration_min"] = "60"
	default:
		if m := reMinutes.FindStringSubmatch(q); m != nil {
			args["duration_min"] = m[1]
		}
	}

	// название
	kind := "Созвон"
	switch {
	case hasWord(w, "синк"):
		kind = "Синк"
	case hasWord(w, "1 1"):
		kind = "1:1"
	case hasWord(w, "встреч", "митинг"):
		kind = "Встреча"
	}
	title := kind
	for _, m := range reWith.FindAllStringSubmatch(q, -1) {
		name := m[1]
		if notNames[name] {
			continue
		}
		role := false
		for _, rw := range roleWords {
			role = role || strings.HasPrefix(name, rw)
		}
		if !role {
			r := []rune(name)
			r[0] = unicode.ToUpper(r[0])
			name = string(r)
		}
		title = kind + " с " + name
		break
	}
	args["title"] = title
	return args
}
