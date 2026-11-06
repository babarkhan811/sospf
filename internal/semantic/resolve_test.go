package semantic

import (
	"testing"
	"time"
)

func cap(agent, domain string, status uint8, caps ...string) *CapabilityLSA {
	var id AgentID
	copy(id[:], agent)
	p, _ := ParsePrefix(domain + "/svc/" + agent)
	return &CapabilityLSA{AgentID: id, Prefix: p, Domain: domain, Status: status, Capabilities: caps}
}

func TestMatchExactSubclassEmbedding(t *testing.T) {
	iv := IntentVector{RequiredCapabilities: []string{"ont:segment"}}
	c := cap("a", "medical", StatusActive, "ont:segment")
	if Match(iv, c, nil).Outcome != Compatible {
		t.Error("exact match should be Compatible")
	}

	// Subclass: capability ont:ct-segment is a subclass of ont:segment.
	c2 := cap("b", "medical", StatusActive, "ont:ct-segment")
	sub := map[string][]string{"ont:ct-segment": {"ont:segment"}}
	if Match(iv, c2, sub).Outcome != Compatible {
		t.Error("subclass match should be Compatible")
	}
	if Match(iv, c2, nil).Outcome != Indeterminate {
		t.Error("no exact/subclass and no embedding should be Indeterminate")
	}

	// Embedding fallback.
	ivE := IntentVector{RequiredCapabilities: []string{"ont:x"}, Embedding: []float32{1, 0, 0}}
	cE := cap("c", "medical", StatusActive, "ont:y")
	cE.Embedding = []float32{0.9, 0.1, 0}
	if Match(ivE, cE, nil).Outcome != Compatible {
		t.Error("high cosine should be Compatible via fallback")
	}
	cE.Embedding = []float32{0, 1, 0}
	if Match(ivE, cE, nil).Outcome != Incompatible {
		t.Error("low cosine should be Incompatible")
	}
}

func TestResolveSelectsHighestTrustLowestCost(t *testing.T) {
	r := NewResolver()
	iv := IntentVector{RequiredCapabilities: []string{"ont:segment"}, Constraints: Constraints{MinTrust: 0.5, DisclosureLimit: 3}}
	cands := []*Candidate{
		{Cap: cap("low", "medical", StatusActive, "ont:segment"), Trust: 0.6, IGPCost: 1, LatencyMs: 10, Load: 0.1, NextHop: 2},
		{Cap: cap("high", "medical", StatusActive, "ont:segment"), Trust: 0.95, IGPCost: 1, LatencyMs: 10, Load: 0.1, NextHop: 3},
	}
	d := r.Resolve(iv, cands)
	if d.Selected == nil {
		t.Fatal("expected a selection")
	}
	if got := string(d.Selected.Cap.AgentID[:4]); got != "high" {
		t.Errorf("should pick the higher-trust agent, got %q", got)
	}
	if d.Reason != ReasonNone {
		t.Errorf("reason should be none on success, got %v", d.Reason)
	}
}

func TestResolveTrustInsufficiency(t *testing.T) {
	r := NewResolver()
	iv := IntentVector{RequiredCapabilities: []string{"ont:segment"}, Constraints: Constraints{MinTrust: 0.9, DisclosureLimit: 3}}
	cands := []*Candidate{
		{Cap: cap("a", "medical", StatusActive, "ont:segment"), Trust: 0.5, NextHop: 2},
	}
	d := r.Resolve(iv, cands)
	if d.Selected != nil {
		t.Fatal("no candidate meets the trust floor")
	}
	if d.Reason != TrustInsufficiency {
		t.Errorf("reason: got %v want trust-insufficiency", d.Reason)
	}
}

func TestResolveCapabilityMismatchAndIndeterminate(t *testing.T) {
	r := NewResolver()
	iv := IntentVector{RequiredCapabilities: []string{"ont:segment"}, Embedding: []float32{1, 0}, Constraints: Constraints{DisclosureLimit: 3}}
	bad := &Candidate{Cap: cap("a", "medical", StatusActive, "ont:ocr"), Trust: 1}
	bad.Cap.Embedding = []float32{0, 1} // cosine 0 -> incompatible
	d := r.Resolve(iv, []*Candidate{bad})
	if d.Reason != CapabilityMismatch {
		t.Errorf("reason: got %v want capability-mismatch", d.Reason)
	}

	// Indeterminate: no caps, no embedding on candidate.
	iv2 := IntentVector{RequiredCapabilities: []string{"ont:segment"}, Constraints: Constraints{DisclosureLimit: 3}}
	ind := &Candidate{Cap: cap("b", "medical", StatusActive, "ont:ocr"), Trust: 1}
	d2 := r.Resolve(iv2, []*Candidate{ind})
	if len(d2.Indeterminate) != 1 || d2.Reason != UnavailableContext {
		t.Errorf("expected 1 indeterminate + unavailable-context, got %d / %v", len(d2.Indeterminate), d2.Reason)
	}
}

func TestResolveECMP(t *testing.T) {
	r := NewResolver()
	iv := IntentVector{RequiredCapabilities: []string{"ont:x"}, Constraints: Constraints{DisclosureLimit: 3}}
	mk := func(a string, nh uint32) *Candidate {
		return &Candidate{Cap: cap(a, "d", StatusActive, "ont:x"), Trust: 0.8, IGPCost: 2, LatencyMs: 5, Load: 0.2, NextHop: nh}
	}
	d := r.Resolve(iv, []*Candidate{mk("a", 2), mk("b", 3)})
	if d.Selected == nil || len(d.ECMP) != 1 {
		t.Fatalf("expected a selection with 1 equal-cost alternate, got sel=%v ecmp=%d", d.Selected, len(d.ECMP))
	}
}

func TestDecisionExpiry(t *testing.T) {
	r := NewResolver()
	r.TTL = 25 * time.Second
	base := time.Unix(1000, 0)
	r.Now = func() time.Time { return base }
	iv := IntentVector{RequiredCapabilities: []string{"ont:x"}, Constraints: Constraints{DisclosureLimit: 3}}
	d := r.Resolve(iv, []*Candidate{{Cap: cap("a", "d", StatusActive, "ont:x"), Trust: 1, NextHop: 2}})
	if !d.ExpiresAt.Equal(base.Add(25 * time.Second)) {
		t.Errorf("expiry: got %v", d.ExpiresAt)
	}
}
