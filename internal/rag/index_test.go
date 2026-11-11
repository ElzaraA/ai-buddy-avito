package rag

import "testing"

func TestChunkAndSearch(t *testing.T) {
	md := "# Absence\n<!-- stage: week1 team: backend audience: intern -->\nAbsence rules"
	chunks := ChunkMarkdown("k.md", md)
	if len(chunks) == 0 {
		t.Fatal("no chunks")
	}
	idx := NewIndex(chunks)
	if len(idx.Search("absence rules", "backend", "intern", 3)) == 0 {
		t.Fatal("expected hits")
	}
}
