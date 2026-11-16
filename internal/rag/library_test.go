package rag

import "testing"

func TestLibraryFiltersSectionsByRoleAndTeam(t *testing.T) {
	chunks, err := LoadDir("../../data/knowledge")
	if err != nil {
		t.Fatal(err)
	}
	l := NewLibrary(chunks, "../../data/knowledge")
	in := map[string]bool{}
	for _, d := range l.List("backend", "intern") {
		in[d.ID] = true
		if d.Title == "" || d.Title == d.ID || len(d.Stages) == 0 {
			t.Fatalf("title/stages from file: %+v", d)
		}
	}
	if !in["access.md"] || in["mentors.md"] || in["frontend.md"] {
		t.Fatalf("intern backend: %v", in)
	}
	full, _ := l.Get("tools.md", "backend", "buddy")
	part, _ := l.Get("tools.md", "backend", "intern")
	fe, _ := l.Get("tools.md", "frontend", "intern")
	if len(full.Sections) <= len(part.Sections) || len(fe.Sections) == 0 || len(fe.Sections) >= len(full.Sections) {
		t.Fatalf("sections: mentor=%d intern=%d frontend=%d", len(full.Sections), len(part.Sections), len(fe.Sections))
	}
	if _, ok := l.Get("mentors.md", "backend", "intern"); ok {
		t.Fatal("closed article")
	}
	if _, ok := l.Get("nope.md", "backend", "intern"); ok {
		t.Fatal("missing article")
	}
	// без файлов на диске — заголовок = имя файла
	l2 := NewLibrary([]Chunk{{Source: "x.md", Title: "Раздел", Stage: "week1", Audience: []string{"all"}, Text: "t"}}, "/nonexistent")
	if d, ok := l2.Get("x.md", "", "intern"); !ok || d.Title != "x.md" || !Visible(Chunk{Audience: []string{"all"}}, "a", "intern") {
		t.Fatalf("fallback title: %+v", d)
	}
}
