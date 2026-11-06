package metrics

import (
	"strings"
	"testing"
)

func TestCounterGaugeAndExposition(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("sospf_resolve_total", "resolves")
	c.Inc()
	c.Add(4)
	if c.Value() != 5 {
		t.Fatalf("counter: got %d want 5", c.Value())
	}
	// Same name returns the same counter.
	if r.Counter("sospf_resolve_total", "x") != c {
		t.Error("counter lookup should be idempotent")
	}
	g := r.Gauge("sospf_lsdb_size", "lsdb size")
	g.Set(42)

	out := r.Expose()
	for _, want := range []string{
		"# TYPE sospf_resolve_total counter",
		"sospf_resolve_total 5",
		"# TYPE sospf_lsdb_size gauge",
		"sospf_lsdb_size 42",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("exposition missing %q in:\n%s", want, out)
		}
	}
}
