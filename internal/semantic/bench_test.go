package semantic

import (
	"fmt"
	"testing"
)

func benchCandidates(n int) []*Candidate {
	cands := make([]*Candidate, 0, n)
	for i := 0; i < n; i++ {
		var id AgentID
		copy(id[:], fmt.Sprintf("agent-%06d", i))
		p, _ := ParsePrefix(fmt.Sprintf("medical/inference/agent-%d", i))
		cands = append(cands, &Candidate{
			Cap: &CapabilityLSA{
				AgentID: id, Prefix: p, Domain: "medical", Status: StatusActive,
				Disclosure: DisclosureDomainInternal, Capabilities: []string{"ont:inference"},
			},
			Trust: 0.5 + float64(i%50)/100.0, IGPCost: 1 + i%5, LatencyMs: 5 + i%40, Load: 0.1, NextHop: uint32(2 + i%8),
		})
	}
	return cands
}

func BenchmarkResolve100(b *testing.B) {
	r := NewResolver()
	iv := IntentVector{TaskTypeIRI: "ont:inference", Domain: "medical",
		RequiredCapabilities: []string{"ont:inference"},
		Constraints:          Constraints{MinTrust: 0.6, MaxLatencyMs: 100, DisclosureLimit: 2}}
	cands := benchCandidates(100)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Resolve(iv, cands)
	}
}

func BenchmarkResolve1000(b *testing.B) {
	r := NewResolver()
	iv := IntentVector{TaskTypeIRI: "ont:inference", Domain: "medical",
		RequiredCapabilities: []string{"ont:inference"},
		Constraints:          Constraints{MinTrust: 0.6, MaxLatencyMs: 100, DisclosureLimit: 2}}
	cands := benchCandidates(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Resolve(iv, cands)
	}
}
