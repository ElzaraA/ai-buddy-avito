package rag

import "testing"

func TestChunkMarkdown(t *testing.T) {
	md := "# A\n<!-- stage: week1 team: backend audience: intern -->\ntext a\n# B\n<!-- stage: week2 team: backend audience: intern -->\ntext b"
	chunks := ChunkMarkdown("src.md", md)
	if len(chunks) != 2 {
		t.Fatalf("want 2, got %d", len(chunks))
	}
	if chunks[0].Stage != "week1" || chunks[1].Stage != "week2" {
		t.Fatal("wrong stages")
	}
	if len(chunks[0].Audience) == 0 || chunks[0].Audience[0] != "intern" {
		t.Fatal("wrong audience")
	}
}

func TestSearchPrefix(t *testing.T) {
	chunks := ChunkMarkdown("k.md", "# Отгул\n<!-- stage: week1 team: backend audience: intern -->\nПорядок оформления отгула")
	idx := NewIndex(chunks)
	if len(idx.Search("оформить отгул", "backend", "intern", 3)) == 0 {
		t.Fatal("must find by prefix")
	}
}

func TestSearchRefusal(t *testing.T) {
	chunks := ChunkMarkdown("k.md", "# Отгул\n<!-- stage: week1 team: backend audience: intern -->\nПорядок оформления отгула")
	idx := NewIndex(chunks)
	for _, q := range []string{"Сколько платят в Авито?", "Какая погода?"} {
		if len(idx.Search(q, "backend", "intern", 3)) != 0 {
			t.Fatalf("must refuse: %s", q)
		}
	}
}

func TestSearchTeamFilter(t *testing.T) {
	chunks := ChunkMarkdown("k.md", "# T\n<!-- stage: week1 team: backend audience: intern -->\nтекст")
	idx := NewIndex(chunks)
	if len(idx.Search("текст", "frontend", "intern", 3)) != 0 {
		t.Fatal("must filter")
	}
}
