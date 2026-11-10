package rag

import "testing"

func TestChunkMarkdown(t *testing.T) {
md := "# A\n<!-- stage: week1 team: backend -->\ntext a\n# B\n<!-- stage: week2 team: backend -->\ntext b"
chunks := ChunkMarkdown("src.md", md)
if len(chunks) != 2 {
t.Fatalf("want 2, got %d", len(chunks))
}
if chunks[0].Stage != "week1" || chunks[1].Stage != "week2" {
t.Fatal("wrong stages")
}
if chunks[0].TeamID != "backend" {
t.Fatal("wrong team")
}
}

func TestChunkEmpty(t *testing.T) {
chunks := ChunkMarkdown("x.md", "")
if len(chunks) != 0 {
t.Fatal("must be empty")
}
}

func TestSearchPrefix(t *testing.T) {
chunks := ChunkMarkdown("k.md", "# Отгул\n<!-- stage: week1 team: backend -->\nПорядок оформления отгула")
idx := NewIndex(chunks)
hits := idx.Search("оформить отгул", "backend", 3)
if len(hits) == 0 {
t.Fatal("must find by prefix")
}
}

func TestSearchTeamFilter(t *testing.T) {
chunks := ChunkMarkdown("k.md", "# T\n<!-- stage: week1 team: backend -->\nтекст")
idx := NewIndex(chunks)
if len(idx.Search("текст", "frontend", 3)) != 0 {
t.Fatal("must filter")
}
}

func TestSearchEmptyIndex(t *testing.T) {
idx := NewIndex(nil)
if len(idx.Search("x", "", 3)) != 0 {
t.Fatal("empty index must return none")
}
}