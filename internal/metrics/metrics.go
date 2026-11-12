// Package metrics — минимальный реестр метрик в формате Prometheus без внешних зависимостей.
// Покрывает технические метрики (HTTP, LLM) и метрики пользы (ответы, отказы, действия).
package metrics

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

type series struct {
	help   string
	kind   string // counter | gauge
	values map[string]float64
}

type Registry struct {
	mu     sync.Mutex
	series map[string]*series
}

func New() *Registry { return &Registry{series: map[string]*series{}} }

func labelKey(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i+1 < len(labels); i += 2 {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%s=%q", labels[i], labels[i+1])
	}
	return b.String()
}

func (r *Registry) add(name, kind, help string, v float64, labels []string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.series[name]
	if s == nil {
		s = &series{help: help, kind: kind, values: map[string]float64{}}
		r.series[name] = s
	}
	s.values[labelKey(labels)] += v
}

// Inc увеличивает счётчик на 1. labels — пары ключ, значение.
func (r *Registry) Inc(name, help string, labels ...string) { r.add(name, "counter", help, 1, labels) }

// Add увеличивает счётчик на v (например, суммарные секунды).
func (r *Registry) Add(name, help string, v float64, labels ...string) {
	r.add(name, "counter", help, v, labels)
}

// Gauge изменяет текущее значение на delta (например, число открытых SSE-потоков).
func (r *Registry) Gauge(name, help string, delta float64, labels ...string) {
	r.add(name, "gauge", help, delta, labels)
}

// Value возвращает текущее значение серии (для тестов и сводки).
func (r *Registry) Value(name string, labels ...string) float64 {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if s := r.series[name]; s != nil {
		return s.values[labelKey(labels)]
	}
	return 0
}

// Sum — сумма по всем наборам меток серии.
func (r *Registry) Sum(name string) float64 {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	total := 0.0
	if s := r.series[name]; s != nil {
		for _, v := range s.values {
			total += v
		}
	}
	return total
}

// Write выводит метрики в текстовом формате Prometheus (детерминированный порядок).
func (r *Registry) Write(w io.Writer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.series))
	for n := range r.series {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		s := r.series[n]
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", n, s.help, n, s.kind)
		keys := make([]string, 0, len(s.values))
		for k := range s.values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if k == "" {
				fmt.Fprintf(w, "%s %g\n", n, s.values[k])
			} else {
				fmt.Fprintf(w, "%s{%s} %g\n", n, k, s.values[k])
			}
		}
	}
}
