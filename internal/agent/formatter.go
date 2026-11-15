package agent

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Форматирование ответа из базы знаний для стажёра первой недели:
//
//	Коротко: <одно предложение>.
//	<остальной текст фрагмента>
//
//	<подпись списка из фрагмента, например «Что сделать:»>
//	1. шаг
//	2. шаг
//
//	Кого спросить: <ответственный>
//
// Форматтер ничего не придумывает: он только переставляет и нумерует текст найденного фрагмента.
// Единственное, что он может добавить, — строку «Кого спросить» с наставником из контактов команды,
// если во фрагменте есть пошаговая инструкция, но ответственный не назван.

var stepRe = regexp.MustCompile(`^\d{1,2}[.)]\s+(.+)$`)

const (
	leadPrefix = "Коротко: "
	whoPrefix  = "Кого спросить: "
)

type kbBlock struct {
	label    string   // подпись списка («Что сделать:»)
	prose    string   // абзац текста
	items    []string // пункты списка
	numbered bool     // нумерованный список (порядок действий), иначе — маркированный
}

// parseChunk разбирает текст фрагмента на абзацы, списки и строку «Кого спросить».
func parseChunk(text string) (blocks []kbBlock, who string) {
	var cur *kbBlock
	for _, raw := range strings.Split(text, "\n") {
		ln := strings.TrimSpace(raw)
		if ln == "" {
			cur = nil
			continue
		}
		if low := strings.ToLower(ln); strings.HasPrefix(low, strings.ToLower(whoPrefix)) {
			who = strings.TrimSpace(ln[len(whoPrefix):])
			cur = nil
			continue
		}
		item, numbered, isItem := listItem(ln)
		if isItem {
			if cur == nil || cur.prose != "" && !strings.HasSuffix(cur.prose, ":") || len(cur.items) > 0 && cur.numbered != numbered {
				blocks = append(blocks, kbBlock{numbered: numbered})
				cur = &blocks[len(blocks)-1]
			} else if cur.prose != "" {
				// абзац, который кончается двоеточием, — подпись этого списка
				cur.label, cur.prose, cur.numbered = cur.prose, "", numbered
			}
			cur.items = append(cur.items, item)
			continue
		}
		// строка, которая кончается двоеточием, — подпись следующего списка: отдельный блок
		if cur != nil && cur.prose != "" && len(cur.items) == 0 && !strings.HasSuffix(cur.prose, ":") && !strings.HasSuffix(ln, ":") {
			cur.prose += " " + ln
			continue
		}
		blocks = append(blocks, kbBlock{prose: ln})
		cur = &blocks[len(blocks)-1]
	}
	return blocks, who
}

func listItem(ln string) (text string, numbered, ok bool) {
	if m := stepRe.FindStringSubmatch(ln); m != nil {
		return strings.TrimSpace(m[1]), true, true
	}
	for _, p := range []string{"- ", "• ", "* ", "— "} {
		if strings.HasPrefix(ln, p) {
			return strings.TrimSpace(ln[len(p):]), false, true
		}
	}
	return "", false, false
}

// splitSentences делит абзац на предложения: точка, «!», «?» или «…», затем пробел и заглавная буква,
// цифра или кавычка. «Go 1.22» и «(Олег Смирнов).» не режутся посередине.
func splitSentences(s string) []string {
	rs := []rune(strings.TrimSpace(s))
	var out []string
	start := 0
	for i := 0; i < len(rs); i++ {
		if !strings.ContainsRune(".!?…", rs[i]) || i+2 >= len(rs) || rs[i+1] != ' ' {
			continue
		}
		next := rs[i+2]
		if unicode.IsUpper(next) || unicode.IsDigit(next) || next == '«' || next == '"' {
			out = append(out, strings.TrimSpace(string(rs[start:i+1])))
			start = i + 2
		}
	}
	if tail := strings.TrimSpace(string(rs[start:])); tail != "" {
		out = append(out, tail)
	}
	return out
}

// formatKB превращает текст фрагмента базы знаний в ответ «Коротко → шаги → кого спросить».
func (c contacts) formatKB(text string) string {
	blocks, who := parseChunk(text)
	var parts []string
	led, procedure := false, false
	for _, b := range blocks {
		if b.prose != "" {
			if !led {
				led = true
				ss := splitSentences(b.prose)
				p := ss[0]
				if !strings.HasPrefix(p, "Коротко") {
					p = leadPrefix + p
				}
				if len(ss) > 1 {
					p += "\n" + strings.Join(ss[1:], " ")
				}
				parts = append(parts, p)
				continue
			}
			parts = append(parts, b.prose)
			continue
		}
		var sb strings.Builder
		label := b.label
		if label == "" && b.numbered {
			label = "По шагам:"
		}
		if label != "" {
			sb.WriteString(label)
			sb.WriteString("\n")
		}
		for i, it := range b.items {
			if i > 0 {
				sb.WriteString("\n")
			}
			if b.numbered {
				sb.WriteString(strconv.Itoa(i+1) + ". " + it)
			} else {
				sb.WriteString("• " + it)
			}
		}
		procedure = procedure || b.numbered
		parts = append(parts, sb.String())
	}
	switch {
	case who != "":
		parts = append(parts, whoPrefix+who)
	case procedure && !c.staff && c.Buddy != "":
		parts = append(parts, whoPrefix+"наставник "+c.Buddy+" — если на каком-то шаге застряли.")
	}
	return strings.Join(parts, "\n\n")
}

// sentences — все предложения фрагмента, включая пункты списков (без маркеров).
func sentences(text string) []string {
	var out []string
	for _, raw := range strings.Split(text, "\n") {
		ln := strings.TrimSpace(raw)
		if ln == "" {
			continue
		}
		if item, _, ok := listItem(ln); ok {
			ln = item
		}
		out = append(out, splitSentences(ln)...)
	}
	return out
}
