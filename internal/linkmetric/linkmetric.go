// Package linkmetric turns live telemetry (latency, load) into a dynamic IGP
// link cost, so SPF computes paths that reflect real conditions rather than only
// a static admin cost (prototype of EXPERIMENTAL idea #1: the semantic/telemetry
// layer improving real routing).
//
// Adaptive metrics are prone to oscillation ("everyone floods the fast link,
// which then becomes slow"). Two mechanisms prevent that here:
//   - EWMA smoothing of the raw samples, and
//   - a hysteresis band: the advertised cost only changes when the computed cost
//     moves at least `Band` units from the last advertised value.
package linkmetric

// Weights control how much telemetry influences the cost.
type Weights struct {
	Alpha      float64 // EWMA weight on the newest sample (0..1)
	PerMs      float64 // cost units added per millisecond of smoothed latency
	PerLoad    float64 // cost units added at full load (load in [0,1])
	Band       uint16  // minimum change from the advertised cost to re-advertise
}

// DefaultWeights is a balanced, flap-resistant starting point.
func DefaultWeights() Weights {
	return Weights{Alpha: 0.3, PerMs: 0.1, PerLoad: 20, Band: 2}
}

// Metric computes a dynamic cost for one link.
type Metric struct {
	base     uint16
	w        Weights
	ewmaLat  float64
	primed   bool
	reported uint16
}

// New creates a dynamic metric starting at the static base cost.
func New(base uint16, w Weights) *Metric {
	if base == 0 {
		base = 1
	}
	return &Metric{base: base, w: w, reported: base}
}

// Observe folds one telemetry sample in and returns the effective cost plus
// whether it changed enough to re-advertise.
func (m *Metric) Observe(latencyMs, load float64) (uint16, bool) {
	if !m.primed {
		m.ewmaLat = latencyMs
		m.primed = true
	} else {
		m.ewmaLat = m.w.Alpha*latencyMs + (1-m.w.Alpha)*m.ewmaLat
	}
	raw := float64(m.base) + m.w.PerMs*m.ewmaLat + m.w.PerLoad*load
	eff := quantize(raw)
	if diff16(eff, m.reported) >= m.w.Band {
		m.reported = eff
		return eff, true
	}
	return m.reported, false
}

// Reported returns the currently advertised cost.
func (m *Metric) Reported() uint16 { return m.reported }

func quantize(f float64) uint16 {
	if f < 1 {
		return 1
	}
	if f > 65535 {
		return 65535
	}
	return uint16(f + 0.5)
}

func diff16(a, b uint16) uint16 {
	if a > b {
		return a - b
	}
	return b - a
}
