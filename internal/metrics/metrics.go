// Package metrics is a tiny, dependency-free metrics registry that exposes
// counters and gauges in Prometheus text exposition format (brief §4
// observability). It avoids pulling the Prometheus client library into the
// reference implementation; a production build can swap in the real client
// behind the same call sites.
package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Counter is a monotonically increasing metric.
type Counter struct {
	name string
	help string
	v    uint64
}

// Inc adds one.
func (c *Counter) Inc() { atomic.AddUint64(&c.v, 1) }

// Add adds n.
func (c *Counter) Add(n uint64) { atomic.AddUint64(&c.v, n) }

// Value returns the current count.
func (c *Counter) Value() uint64 { return atomic.LoadUint64(&c.v) }

// Gauge is a value that can go up or down.
type Gauge struct {
	name string
	help string
	mu   sync.Mutex
	v    float64
}

// Set sets the gauge value.
func (g *Gauge) Set(f float64) {
	g.mu.Lock()
	g.v = f
	g.mu.Unlock()
}

// Value returns the current value.
func (g *Gauge) Value() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.v
}

// Registry holds named metrics.
type Registry struct {
	mu       sync.Mutex
	counters map[string]*Counter
	gauges   map[string]*Gauge
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{counters: map[string]*Counter{}, gauges: map[string]*Gauge{}}
}

// Counter returns (creating if needed) a counter.
func (r *Registry) Counter(name, help string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c := r.counters[name]; c != nil {
		return c
	}
	c := &Counter{name: name, help: help}
	r.counters[name] = c
	return c
}

// Gauge returns (creating if needed) a gauge.
func (r *Registry) Gauge(name, help string) *Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g := r.gauges[name]; g != nil {
		return g
	}
	g := &Gauge{name: name, help: help}
	r.gauges[name] = g
	return g
}

// Expose renders all metrics in Prometheus text exposition format.
func (r *Registry) Expose() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var b strings.Builder
	names := make([]string, 0, len(r.counters))
	for n := range r.counters {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		c := r.counters[n]
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", n, c.help, n, n, c.Value())
	}
	gnames := make([]string, 0, len(r.gauges))
	for n := range r.gauges {
		gnames = append(gnames, n)
	}
	sort.Strings(gnames)
	for _, n := range gnames {
		g := r.gauges[n]
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s gauge\n%s %g\n", n, g.help, n, n, g.Value())
	}
	return b.String()
}
