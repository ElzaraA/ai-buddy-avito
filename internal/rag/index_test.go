package rag

import "testing"

func TestChunkAndSearch(t *testing.T) {
md := "# Отгул\n<!-- stage: week1 team: backend -->\nПорядок оформления отгула"
chunks := ChunkMarkdown("k.md", md)
if len(chunks) == 0 {
t.Fatal("no chunks")
}
idx := NewIndex(chunks)
hits := idx.Search("как оформить отгул", "backend", 3)
if len(hits) == 0 {
t.Fatal("expected hits")
}
}