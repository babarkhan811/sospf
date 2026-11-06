package semantic

import (
	"sort"
	"time"
)

// Weights parameterise the composite cost (brief §5.4 stage 3):
//
//	cost = w_path·igp + w_lat·latency + w_trust·(1−trust) + w_load·load + w_sem·(1−sim)
type Weights struct {
	Path  float64
	Lat   float64
	Trust float64
	Load  float64
	Sem   float64
}

// DefaultWeights is a balanced starting point; operators tune these.
func DefaultWeights() Weights {
	return Weights{Path: 1.0, Lat: 0.01, Trust: 50.0, Load: 10.0, Sem: 30.0}
}

// Candidate is one possible target agent with the live metrics the resolver
// needs. IGPCost/NextHop come from SPF (Stage 1); Trust/Latency/Load come from
// the Trust and Telemetry subsystems (Phase 5).
type Candidate struct {
	Cap       *CapabilityLSA
	Trust     float64 // [0,1]
	IGPCost   int
	LatencyMs int
	Load      float64 // [0,1]
	NextHop   uint32

	Result MatchResult
}

func (w Weights) cost(c *Candidate) float64 {
	return w.Path*float64(c.IGPCost) +
		w.Lat*float64(c.LatencyMs) +
		w.Trust*(1-c.Trust) +
		w.Load*c.Load +
		w.Sem*(1-c.Result.Similarity)
}

// Ranked is a surviving candidate with its composite cost.
type Ranked struct {
	Cand *Candidate
	Cost float64
}

// Decision is the resolver's output (draft-cui §5.3 Routing Decision).
type Decision struct {
	Selected      *Candidate // nil if nothing eligible
	Cost          float64
	ECMP          []*Candidate // equal-cost alternates to Selected
	Ranked        []Ranked     // all eligible, best first
	Indeterminate []*CapabilityLSA
	Reason        ReasonCode // why nothing was selected (ReasonNone on success)
	ExpiresAt     time.Time
}

// Resolver performs constrained semantic selection.
type Resolver struct {
	Weights  Weights
	Subclass map[string][]string
	TTL      time.Duration
	Now      func() time.Time
}

// NewResolver returns a resolver with default weights and a 30s decision TTL.
func NewResolver() *Resolver {
	return &Resolver{Weights: DefaultWeights(), TTL: 30 * time.Second, Now: time.Now}
}

// Resolve applies CSPF-style hard pruning, then ranks survivors by composite
// cost and returns the best (with equal-cost alternates). On failure it returns
// the most salient reason code.
func (r *Resolver) Resolve(iv IntentVector, cands []*Candidate) Decision {
	now := r.Now()
	dec := Decision{Reason: ReasonNone, ExpiresAt: now.Add(r.TTL)}

	var (
		eligible                                          []Ranked
		nCapMismatch, nTrust, nPolicy, nIndeterminate, nStatus int
	)

	for _, c := range cands {
		if !c.Cap.Eligible() { // status != Active
			nStatus++
			continue
		}
		c.Result = Match(iv, c.Cap, r.Subclass)
		switch c.Result.Outcome {
		case Incompatible:
			nCapMismatch++
			continue
		case Indeterminate:
			dec.Indeterminate = append(dec.Indeterminate, c.Cap)
			nIndeterminate++
			continue
		}
		// Hard constraints.
		if c.Trust < iv.Constraints.MinTrust {
			nTrust++
			continue
		}
		if iv.Constraints.MaxLatencyMs > 0 && c.LatencyMs > iv.Constraints.MaxLatencyMs {
			nPolicy++
			continue
		}
		if len(iv.Constraints.AllowedDomains) > 0 && !contains(iv.Constraints.AllowedDomains, c.Cap.Domain) {
			nPolicy++
			continue
		}
		if c.Cap.Disclosure > iv.Constraints.DisclosureLimit {
			nPolicy++
			continue
		}
		cost := r.Weights.cost(c)
		if iv.Constraints.CostCeiling > 0 && cost > iv.Constraints.CostCeiling {
			nPolicy++
			continue
		}
		eligible = append(eligible, Ranked{Cand: c, Cost: cost})
	}

	if len(eligible) == 0 {
		dec.Reason = pickReason(nTrust, nCapMismatch, nPolicy, nIndeterminate, nStatus)
		return dec
	}

	sort.SliceStable(eligible, func(i, j int) bool { return eligible[i].Cost < eligible[j].Cost })
	dec.Ranked = eligible
	dec.Selected = eligible[0].Cand
	dec.Cost = eligible[0].Cost
	for _, e := range eligible[1:] {
		if e.Cost == dec.Cost {
			dec.ECMP = append(dec.ECMP, e.Cand)
		}
	}
	return dec
}

// pickReason chooses the most salient failure reason (brief §5.6 reason codes).
func pickReason(nTrust, nCap, nPolicy, nIndet, nStatus int) ReasonCode {
	switch {
	case nTrust > 0:
		return TrustInsufficiency
	case nPolicy > 0:
		return PolicyConstraintFailure
	case nCap > 0:
		return CapabilityMismatch
	case nIndet > 0:
		return UnavailableContext
	case nStatus > 0:
		return StaleCapabilityInfo
	default:
		return CapabilityMismatch
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
