package rag

import (
	"os"
	"path/filepath"
)

func LoadDir(dir string) ([]Chunk, error) {
	var all []Chunk
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		all = append(all, ChunkMarkdown(e.Name(), string(b))...)
	}
	return all, nil
}
