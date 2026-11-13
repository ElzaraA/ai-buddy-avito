package metrics

import (
	"strings"
	"sync"
	"testing"
)

func TestCountersGaugesAndOutput(t *testing.T) {
	r := New()
	r.Inc("requests_total", "Запросы", "method", "GET", "status", "200")
	r.Inc("requests_total", "Запросы", "method", "GET", "status", "200")
	r.Inc("requests_total", "Запросы", "method", "POST", "status", "400")
	r.Add("seconds_sum", "Секунды", 1.5)
	r.Gauge("streams", "Потоки", 2)
	r.Gauge("streams", "Потоки", -1)

	if v := r.Value("requests_total", "method", "GET", "status", "200"); v != 2 {
		t.Fatalf("value %v", v)
	}
	if r.Sum("requests_total") != 3 || r.Value("streams") != 1 || r.Value("missing") != 0 || r.Sum("missing") != 0 {
		t.Fatal("sum/gauge")
	}

	var b strings.Builder
	r.Write(&b)
	out := b.String()
	want := "# HELP requests_total Запросы\n# TYPE requests_total counter\n" +
		"requests_total{method=\"GET\",status=\"200\"} 2\n" +
		"requests_total{method=\"POST\",status=\"400\"} 1\n"
	if !strings.Contains(out, want) || !strings.Contains(out, "# TYPE streams gauge\nstreams 1\n") || !strings.Contains(out, "seconds_sum 1.5\n") {
		t.Fatalf("output:\n%s", out)
	}
	// Порядок детерминирован: одинаковый вывод при повторе.
	var b2 strings.Builder
	r.Write(&b2)
	if b2.String() != out {
		t.Fatal("output must be stable")
	}
}

// Значения меток экранируются — кавычка в значении не ломает формат.
func TestLabelEscaping(t *testing.T) {
	r := New()
	r.Inc("x_total", "x", "route", `a"b`)
	var b strings.Builder
	r.Write(&b)
	if !strings.Contains(b.String(), `x_total{route="a\"b"} 1`) {
		t.Fatalf("escaping: %s", b.String())
	}
}

// nil-реестр безопасен: код может не проверять, включены ли метрики.
func TestNilRegistryIsNoop(t *testing.T) {
	var r *Registry
	r.Inc("a", "a")
	r.Add("a", "a", 1)
	r.Gauge("a", "a", 1)
	if r.Value("a") != 0 || r.Sum("a") != 0 {
		t.Fatal("nil registry")
	}
}

func TestConcurrentInc(t *testing.T) {
	r := New()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r.Inc("c_total", "c") }()
	}
	wg.Wait()
	if r.Value("c_total") != 100 {
		t.Fatalf("lost increments: %v", r.Value("c_total"))
	}
}
