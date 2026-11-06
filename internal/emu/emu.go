// Package emu is the in-process multi-router emulator (ADR-0002). Routers are
// goroutines; links are direct in-memory delivery that can be taken up or down
// to simulate failures. No sockets, no privileges — the basis for deterministic
// convergence tests (brief §7.2, §8).
package emu

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/babarkhan811/sospf/internal/node"
	"github.com/babarkhan811/sospf/internal/packet"
)

// Emulator owns a set of routers and the links between them.
type Emulator struct {
	mu      sync.RWMutex
	routers map[uint32]*node.Router
	links   map[linkKey]bool // true = up
	cfg     node.Config
}

type linkKey struct{ a, b uint32 }

func mk(a, b uint32) linkKey {
	if a > b {
		a, b = b, a
	}
	return linkKey{a, b}
}

// New returns an empty emulator using cfg for every router.
func New(cfg node.Config) *Emulator {
	return &Emulator{
		routers: make(map[uint32]*node.Router),
		links:   make(map[linkKey]bool),
		cfg:     cfg,
	}
}

// AddRouter creates a router with the given ID and optional stub prefixes.
func (e *Emulator) AddRouter(id, area uint32, stubs ...packet.RouterLink) *node.Router {
	r := node.NewRouter(id, area, e.cfg, stubs)
	id0 := id
	r.SetSend(func(to uint32, msg any) { e.route(id0, to, msg) })
	e.mu.Lock()
	e.routers[id] = r
	e.mu.Unlock()
	return r
}

// Connect adds a bidirectional point-to-point link a<->b with the given metric.
func (e *Emulator) Connect(a, b uint32, metric uint16) {
	e.mu.Lock()
	ra, rb := e.routers[a], e.routers[b]
	e.links[mk(a, b)] = true
	e.mu.Unlock()
	if ra != nil {
		ra.AddLink(b, metric)
	}
	if rb != nil {
		rb.AddLink(a, metric)
	}
}

// SetLinkUp brings the a<->b link up or down (failure injection).
func (e *Emulator) SetLinkUp(a, b uint32, up bool) {
	e.mu.Lock()
	e.links[mk(a, b)] = up
	e.mu.Unlock()
}

func (e *Emulator) route(from, to uint32, msg any) {
	e.mu.RLock()
	up := e.links[mk(from, to)]
	tgt := e.routers[to]
	e.mu.RUnlock()
	if up && tgt != nil {
		tgt.Deliver(from, msg)
	}
}

// Start launches every router.
func (e *Emulator) Start(ctx context.Context) {
	e.mu.RLock()
	rs := make([]*node.Router, 0, len(e.routers))
	for _, r := range e.routers {
		rs = append(rs, r)
	}
	e.mu.RUnlock()
	for _, r := range rs {
		r.Start(ctx)
	}
}

// Routers returns a snapshot of the routers, by ID.
func (e *Emulator) Routers() map[uint32]*node.Router {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make(map[uint32]*node.Router, len(e.routers))
	for k, v := range e.routers {
		out[k] = v
	}
	return out
}

// ---- topology files (JSON in Phase 2; a YAML loader is a later nicety) ----

// Topo is a declarative topology (brief §6 sample topologies).
type Topo struct {
	Routers []TopoRouter `json:"routers"`
	Links   []TopoLink   `json:"links"`
}

// TopoRouter is one router with an optional stub prefix.
type TopoRouter struct {
	ID     uint32 `json:"id"`
	Area   uint32 `json:"area"`
	Prefix uint32 `json:"prefix"`
	Metric uint16 `json:"metric"`
}

// TopoLink is one point-to-point link.
type TopoLink struct {
	A      uint32 `json:"a"`
	B      uint32 `json:"b"`
	Metric uint16 `json:"metric"`
}

// LoadTopo reads a JSON topology from disk.
func LoadTopo(path string) (*Topo, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t Topo
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// Build constructs an emulator from a topology.
func Build(cfg node.Config, t *Topo) *Emulator {
	e := New(cfg)
	for _, rt := range t.Routers {
		var stubs []packet.RouterLink
		if rt.Prefix != 0 {
			m := rt.Metric
			if m == 0 {
				m = 1
			}
			stubs = append(stubs, packet.RouterLink{Type: packet.LinkStub, ID: rt.Prefix, Metric: m})
		}
		e.AddRouter(rt.ID, rt.Area, stubs...)
	}
	for _, l := range t.Links {
		m := l.Metric
		if m == 0 {
			m = 1
		}
		e.Connect(l.A, l.B, m)
	}
	return e
}

// WaitStable blocks until every router holds exactly wantLSAs LSAs and routing
// tables are unchanged across two polls, or until timeout. It reports success.
func (e *Emulator) WaitStable(wantLSAs int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	prev := map[uint32]int{}
	stableRounds := 0
	for time.Now().Before(deadline) {
		ok := true
		cur := map[uint32]int{}
		for id, r := range e.Routers() {
			if r.DBLen() != wantLSAs {
				ok = false
			}
			cur[id] = len(r.Table())
		}
		if ok && sameCounts(prev, cur) {
			stableRounds++
			if stableRounds >= 3 {
				return true
			}
		} else {
			stableRounds = 0
		}
		prev = cur
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func sameCounts(a, b map[uint32]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
