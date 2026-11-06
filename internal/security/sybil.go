package security

import "sync"

// SybilGuard bounds how many agents a single advertising router may register,
// limiting Sybil floods of fake agents (brief §5.5, §9). It is deliberately
// simple: a hard per-router cap.
type SybilGuard struct {
	mu    sync.Mutex
	max   int
	count map[uint32]int
}

// NewSybilGuard caps each router at max agents.
func NewSybilGuard(max int) *SybilGuard {
	return &SybilGuard{max: max, count: make(map[uint32]int)}
}

// Admit reports whether router may register one more agent, and records it if so.
func (g *SybilGuard) Admit(router uint32) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.count[router] >= g.max {
		return false
	}
	g.count[router]++
	return true
}

// Release decrements a router's agent count (deregistration).
func (g *SybilGuard) Release(router uint32) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.count[router] > 0 {
		g.count[router]--
	}
}

// Count returns the current agent count for a router.
func (g *SybilGuard) Count(router uint32) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.count[router]
}
