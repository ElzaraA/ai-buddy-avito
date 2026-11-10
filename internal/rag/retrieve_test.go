package rag

import (
"os"
"path/filepath"
"testing"
)

func TestLoadDir(t *testing.T) {
dir := t.TempDir()
if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A\n<!-- stage: week1 team: backend -->\ntext a"), 0644); err != nil {
t.Fatal(err)
}
if err := os.WriteFile(filepath.Join(dir, "skip.txt"), []byte("x"), 0644); err != nil {
t.Fatal(err)
}

chunks, err := LoadDir(dir)
if err != nil {
t.Fatal(err)
}
if len(chunks) == 0 {
t.Fatal("must load chunks")
}
if chunks[0].Source != "a.md" {
t.Fatalf("wrong source: %s", chunks[0].Source)
}
}

func TestLoadDirMissing(t *testing.T) {
if _, err := LoadDir("no-such-dir"); err == nil {
t.Fatal("must error")
}
}