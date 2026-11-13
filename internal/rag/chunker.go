package rag

import (
	"regexp"
	"strings"
)

type Chunk struct {
	ID         string
	Source     string
	Stage      string
	Title      string // заголовок раздела: индексируется вместе с текстом
	TeamID     string
	Audience   []string
	Text       string
	Disclaimer string
}

var headerRe = regexp.MustCompile(`^#+\s+(.*)$`)

func ChunkMarkdown(source, content string) []Chunk {
	lines := strings.Split(content, "\n")
	var chunks []Chunk
	var cur Chunk
	var buf []string

	flush := func() {
		text := strings.TrimSpace(strings.Join(buf, "\n"))
		if text == "" {
			return
		}
		cur.Text = text
		cur.Source = source
		cur.ID = source + "#" + cur.Stage + "#" + itoa(len(chunks))
		chunks = append(chunks, cur)
		buf = buf[:0]
	}

	for _, ln := range lines {
		if m := headerRe.FindStringSubmatch(ln); m != nil {
			flush()
			cur = Chunk{Stage: m[1], Title: m[1]}
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(ln), "<!--") {
			// Метаданные раздела; прочие комментарии (например, дисклеймер файла) не индексируются.
			if strings.Contains(ln, "stage:") {
				cur.Stage = extract(ln, "stage")
				cur.TeamID = extract(ln, "team")
				cur.Audience = splitCSV(extract(ln, "audience"))
			}
			continue
		}
		buf = append(buf, ln)
	}
	flush()
	return chunks
}

func extract(s, key string) string {
	re := regexp.MustCompile(key + `:\s*([^\s]+)`)
	if m := re.FindStringSubmatch(s); len(m) == 2 {
		return m[1]
	}
	return ""
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
