package sft

import (
	"testing"
	"time"

	"github.com/babarkhan811/sospf/internal/semantic"
)

func TestPutLookupExpirySweep(t *testing.T) {
	tab := New()
	now := time.Unix(1000, 0)
	tab.now = func() time.Time { return now }

	k := Key{IntentCategory: "segment", Prefix: "medical/imaging"}
	tab.Put(k, Entry{NextHop: 2, Cost: 3.5, ExpiresAt: now.Add(10 * time.Second)})

	if e, ok := tab.Lookup(k); !ok || e.NextHop != 2 {
		t.Fatalf("live entry lookup failed: %+v ok=%v", e, ok)
	}

	now = now.Add(11 * time.Second) // past expiry
	if _, ok := tab.Lookup(k); ok {
		t.Error("expired entry must not be returned")
	}
	if n := tab.Sweep(); n != 1 {
		t.Errorf("sweep should drop 1 expired entry, dropped %d", n)
	}
	if tab.Len() != 0 {
		t.Errorf("table should be empty after sweep, has %d", tab.Len())
	}
}

func TestInstallDecision(t *testing.T) {
	tab := New()
	r := semantic.NewResolver()
	iv := semantic.IntentVector{RequiredCapabilities: []string{"ont:x"}, Constraints: semantic.Constraints{DisclosureLimit: 3}}
	var id semantic.AgentID
	copy(id[:], "agent-x")
	p, _ := semantic.ParsePrefix("d/svc/agent-x")
	c := &semantic.CapabilityLSA{AgentID: id, Prefix: p, Domain: "d", Status: semantic.StatusActive, Capabilities: []string{"ont:x"}}
	d := r.Resolve(iv, []*semantic.Candidate{{Cap: c, Trust: 0.9, NextHop: 7}})

	if !tab.InstallDecision("ont:x", p.String(), iv, d) {
		t.Fatal("install should succeed for a decision with a selection")
	}
	e, ok := tab.Lookup(Key{IntentCategory: "ont:x", Prefix: p.String()})
	if !ok || e.NextHop != 7 {
		t.Errorf("installed entry wrong: %+v ok=%v", e, ok)
	}

	// A decision with no selection does not install.
	empty := semantic.Decision{}
	if tab.InstallDecision("ont:y", "d", iv, empty) {
		t.Error("installing an empty decision should return false")
	}
}
