package semantic

import (
	"fmt"
	"testing"
)

func idxCand(agent, domain string, trust float64, caps ...string) *Candidate {
	var id AgentID
	copy(id[:], agent)
	p, _ := ParsePrefix(domain + "/svc/" + agent)
	return &Candidate{
		Cap: &CapabilityLSA{
			AgentID: id, Prefix: p, Domain: domain, Status: StatusActive,
			Disclosure: DisclosureDomainInternal, Capabilities: caps,
		},
		Trust: trust, IGPCost: 1, LatencyMs: 10, Load: 0.1, NextHop: 2,
	}
}

func medicalIntent() IntentVector {
	return IntentVector{
		TaskTypeIRI: "ont:segment", Domain: "medical",
		RequiredCapabilities: []string{"ont:segment"},
		Constraints:          Constraints{MinTrust: 0.7, DisclosureLimit: 2, AllowedDomains: []string{"medical"}},
	}
}

// Stage A must narrow by domain and Stage B must still pick the exact same best
// agent a full scan would.
func TestIndexedResolveMatchesBruteForce(t *testing.T) {
	cands := []*Candidate{
		idxCand("med-best", "medical", 0.95, "ont:segment"),
		idxCand("med-ok", "medical", 0.80, "ont:segment"),
		idxCand("med-lowtrust", "medical", 0.50, "ont:segment"), // fails trust floor
		idxCand("fin-1", "finance", 0.99, "ont:segment"),        // wrong domain
		idxCand("log-1", "logistics", 0.99, "ont:ocr"),          // wrong domain + cap
	}
	idx := NewIndex()
	for _, c := range cands {
		idx.Insert(c)
	}
	r := NewResolver()
	iv := medicalIntent()

	brute := r.Resolve(iv, cands)
	indexed, examined := idx.ResolveIndexed(r, iv)

	if brute.Selected == nil || indexed.Selected == nil {
		t.Fatalf("both should select something (brute=%v indexed=%v)", brute.Selected, indexed.Selected)
	}
	if string(brute.Selected.Cap.AgentID[:]) != string(indexed.Selected.Cap.AgentID[:]) {
		t.Fatalf("indexed picked a different agent than brute force")
	}
	if trim(indexed.Selected.Cap.AgentID) != "med-best" {
		t.Errorf("should pick med-best, got %q", trim(indexed.Selected.Cap.AgentID))
	}
	// Stage A examined only the medical subtree (3 agents), not all 5.
	if examined != 3 {
		t.Errorf("indexed resolve should examine only the 3 medical agents, examined %d", examined)
	}
}

// Grouping must never bypass hard constraints: a tempting agent in the right
// group with too-low trust is shortlisted but NOT selected.
func TestIndexDoesNotBypassConstraints(t *testing.T) {
	idx := NewIndex()
	idx.Insert(idxCand("med-lowtrust", "medical", 0.50, "ont:segment"))
	r := NewResolver()
	iv := medicalIntent() // MinTrust 0.7
	dec, examined := idx.ResolveIndexed(r, iv)
	if examined != 1 {
		t.Fatalf("expected the low-trust agent to be shortlisted, examined %d", examined)
	}
	if dec.Selected != nil {
		t.Fatal("exact stage must reject the low-trust agent despite it being in the right group")
	}
	if dec.Reason != TrustInsufficiency {
		t.Errorf("reason: got %v want trust-insufficiency", dec.Reason)
	}
}

func TestShortlistExactSkipsEmptySubtrees(t *testing.T) {
	idx := NewIndex()
	idx.Insert(idxCand("a", "medical", 0.9, "ont:segment"))
	// A capability no medical agent has: the subtree is skipped entirely.
	if got := idx.ShortlistExact([]string{"medical"}, "ont:speech"); got != nil {
		t.Errorf("subtree lacking the capability should be skipped, got %d", len(got))
	}
	if got := idx.ShortlistExact([]string{"medical"}, "ont:segment"); len(got) != 1 {
		t.Errorf("present capability should return the agent, got %d", len(got))
	}
}

func trimID(id AgentID) string {
	n := len(id)
	for n > 0 && id[n-1] == 0 {
		n--
	}
	return string(id[:n])
}

func trim(id AgentID) string { return trimID(id) }

// Benchmark: 100k agents across 100 domains. Indexed resolve narrows to the one
// target domain before ranking; brute force ranks all 100k.
func buildBig(n, domains int) ([]*Candidate, *CapabilityIndex) {
	cands := make([]*Candidate, 0, n)
	idx := NewIndex()
	for i := 0; i < n; i++ {
		dom := fmt.Sprintf("dom%03d", i%domains)
		c := idxCand(fmt.Sprintf("a%06d", i), dom, 0.6+float64(i%40)/100.0, "ont:inference")
		cands = append(cands, c)
		idx.Insert(c)
	}
	return cands, idx
}

func BenchmarkBruteForce100k(b *testing.B) {
	cands, _ := buildBig(100_000, 100)
	r := NewResolver()
	iv := IntentVector{TaskTypeIRI: "ont:inference", Domain: "dom042",
		RequiredCapabilities: []string{"ont:inference"},
		Constraints:          Constraints{MinTrust: 0.6, DisclosureLimit: 2, AllowedDomains: []string{"dom042"}}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Resolve(iv, cands)
	}
}

func BenchmarkIndexed100k(b *testing.B) {
	_, idx := buildBig(100_000, 100)
	r := NewResolver()
	iv := IntentVector{TaskTypeIRI: "ont:inference", Domain: "dom042",
		RequiredCapabilities: []string{"ont:inference"},
		Constraints:          Constraints{MinTrust: 0.6, DisclosureLimit: 2, AllowedDomains: []string{"dom042"}}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = idx.ResolveIndexed(r, iv)
	}
}
