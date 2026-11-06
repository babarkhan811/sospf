// Package trust implements S-OSPF's Trust Manager: an EWMA trust model over
// telemetry outcomes with decay and a new-agent floor, plus dampening so noisy
// telemetry does not flap routes (brief §5.5). The formula and defaults are
// S-OSPF's own choice — no source draft defines one (see docs/TRUST_MODEL.md).
package trust

import (
	"math"
	"sync"
	"time"

	"github.com/babarkhan811/sospf/internal/semantic"
)

// Params configure the trust model.
type Params struct {
	Alpha         float64       // EWMA weight on the newest sample (0..1)
	NewAgentFloor float64       // starting trust for an unseen agent
	AnomalyPenalty float64      // subtracted from a sample when an anomaly is flagged
	DecayHalfLife time.Duration // trust decays toward the floor with this half-life
	ReOriginDelta float64       // |new-old| above this re-originates a Trust LSA
}

// DefaultParams returns conservative defaults.
func DefaultParams() Params {
	return Params{
		Alpha:          0.3,
		NewAgentFloor:  0.5,
		AnomalyPenalty: 0.5,
		DecayHalfLife:  10 * time.Minute,
		ReOriginDelta:  0.05,
	}
}

// Outcome is a single telemetry observation about an interaction.
type Outcome struct {
	Success    bool
	SLAAdhered bool
	Anomaly    bool
}

// sample maps an outcome to a [0,1] target value.
func (p Params) sample(o Outcome) float64 {
	v := 0.0
	if o.Success {
		v += 0.6
	}
	if o.SLAAdhered {
		v += 0.4
	}
	if o.Anomaly {
		v -= p.AnomalyPenalty
	}
	return clamp(v)
}

type state struct {
	score    float64
	lastSeen time.Time
	advertised float64 // last value put into a Trust LSA
}

// Manager tracks trust per agent. Safe for concurrent use.
type Manager struct {
	p   Params
	now func() time.Time
	mu  sync.Mutex
	m   map[semantic.AgentID]*state
}

// New returns a Trust Manager.
func New(p Params) *Manager {
	return &Manager{p: p, now: time.Now, m: make(map[semantic.AgentID]*state)}
}

func (m *Manager) get(id semantic.AgentID) *state {
	s := m.m[id]
	if s == nil {
		s = &state{score: m.p.NewAgentFloor, advertised: m.p.NewAgentFloor, lastSeen: m.now()}
		m.m[id] = s
	}
	return s
}

// Update folds a telemetry outcome into the agent's trust via EWMA and returns
// the new score.
func (m *Manager) Update(id semantic.AgentID, o Outcome) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.get(id)
	m.applyDecay(s)
	sample := m.p.sample(o)
	s.score = clamp(m.p.Alpha*sample + (1-m.p.Alpha)*s.score)
	s.lastSeen = m.now()
	return s.score
}

// Score returns the agent's current trust (after applying decay).
func (m *Manager) Score(id semantic.AgentID) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.get(id)
	m.applyDecay(s)
	return s.score
}

// NeedsReOrigination reports whether the agent's trust has moved far enough from
// the last advertised value to warrant a new Trust LSA, and if so records the
// new advertised value. Respecting MinLSInterval is the caller's job.
func (m *Manager) NeedsReOrigination(id semantic.AgentID) (float64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.get(id)
	m.applyDecay(s)
	if abs(s.score-s.advertised) >= m.p.ReOriginDelta {
		s.advertised = s.score
		return s.score, true
	}
	return s.score, false
}

// applyDecay moves score toward the floor based on elapsed time (half-life).
func (m *Manager) applyDecay(s *state) {
	if m.p.DecayHalfLife <= 0 {
		return
	}
	elapsed := m.now().Sub(s.lastSeen)
	if elapsed <= 0 {
		return
	}
	// factor = 0.5 ^ (elapsed / halfLife)
	factor := math.Pow(2, -float64(elapsed)/float64(m.p.DecayHalfLife))
	s.score = m.p.NewAgentFloor + (s.score-m.p.NewAgentFloor)*factor
	s.lastSeen = m.now()
}

func clamp(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}
func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
