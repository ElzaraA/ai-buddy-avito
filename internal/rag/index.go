package rag

import (
"math"
"sort"
"strings"
)

type Index struct {
chunks []Chunk
df     map[string]int
tf     []map[string]int
lens   []int
}

func tokenize(s string) []string {
s = strings.ToLower(s)
return strings.FieldsFunc(s, func(r rune) bool {
return !(r >= 'a' && r <= 'z') && !(r >= 'а' && r <= 'я') && !(r >= '0' && r <= '9')
})
}

func NewIndex(chunks []Chunk) *Index {
idx := &Index{chunks: chunks, df: map[string]int{}}
for _, c := range chunks {
tf := map[string]int{}
toks := tokenize(c.Text + " " + c.Stage)
for _, t := range toks {
tf[t]++
}
for t := range tf {
idx.df[t]++
}
idx.tf = append(idx.tf, tf)
idx.lens = append(idx.lens, len(toks))
}
return idx
}

type Hit struct {
Chunk Chunk
Score float64
}

func prefixMatch(a, b string) bool {
if a == b {
return true
}
n := 5
if len(a) < n {
n = len(a)
}
if len(b) < n {
n = len(b)
}
if n < 3 {
return false
}
return a[:n] == b[:n]
}

func (idx *Index) Search(q string, teamID string, k int) []Hit {
qt := tokenize(q)
n := float64(len(idx.chunks))
avg := 0.0
for _, l := range idx.lens {
avg += float64(l)
}
if n > 0 {
avg /= n
}
if avg == 0 {
avg = 1
}

var hits []Hit
for i, c := range idx.chunks {
if teamID != "" && c.TeamID != "" && c.TeamID != teamID {
continue
}
score := 0.0
for _, t := range qt {
for ct, f := range idx.tf[i] {
if prefixMatch(t, ct) {
df := float64(idx.df[ct])
idf := math.Log(1 + (n-df+0.5)/(df+0.5))
score += idf * (float64(f) * 2.2) / (float64(f) + 1.2*(0.25+0.75*float64(idx.lens[i])/avg))
}
}
}
if score > 0 {
hits = append(hits, Hit{Chunk: c, Score: score})
}
}
sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
if len(hits) > k {
hits = hits[:k]
}
return hits
}