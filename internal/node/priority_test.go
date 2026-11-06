package node

import (
	"testing"

	"github.com/babarkhan811/sospf/internal/packet"
	"github.com/babarkhan811/sospf/internal/semantic"
)

// TestCoreRoutingIsNotStarvedBySemanticFlood proves the ADR-0005 guarantee: a
// large backlog of semantic updates does not delay a core routing message, and
// semantic processing is bounded per cycle. It drives the priority helpers
// directly (no goroutine, no timing), so the result is deterministic.
func TestCoreRoutingIsNotStarvedBySemanticFlood(t *testing.T) {
	r := NewRouter(1, 0, DefaultConfig(), nil)
	r.AddLink(2, 1)

	var sent []any
	r.SetSend(func(to uint32, msg any) { sent = append(sent, msg) })

	// Flood 100k DISTINCT semantic updates into the low-priority queue (distinct
	// opaque IDs, so each is a genuinely new LSA that would be applied).
	for i := 0; i < 100_000; i++ {
		r.semInbox <- packet.SemanticUpdate{Blobs: [][]byte{makeCapBlob(2, 10, uint32(i))}}
	}
	// Queue one core routing message: a Hello that names us, which should drive
	// the neighbor to ExStart and make us emit a Database Description.
	r.inbox <- envelope{from: 2, msg: packet.Hello{Neighbors: []uint32{1}}}

	// Core-first: draining core handles the Hello WITHOUT touching the 100k
	// semantic backlog.
	r.drainCore()

	if r.SemProcessed() != 0 {
		t.Fatalf("draining core must not process any semantic updates, got %d", r.SemProcessed())
	}
	if !sentDD(sent) {
		t.Fatal("core Hello was not processed promptly (no DD emitted) despite the semantic flood")
	}

	// Semantic work is strictly budgeted.
	r.processSemanticBatch(r.cfg.SemanticBudget)
	if got := r.SemProcessed(); got != uint64(r.cfg.SemanticBudget) {
		t.Fatalf("semantic batch should process exactly the budget (%d), got %d", r.cfg.SemanticBudget, got)
	}
	// The rest remain queued, not dropped (to be processed over later cycles).
	if len(r.semInbox) != 100_000-r.cfg.SemanticBudget {
		t.Fatalf("remaining semantic backlog wrong: %d", len(r.semInbox))
	}
}

func sentDD(msgs []any) bool {
	for _, m := range msgs {
		if _, ok := m.(packet.DatabaseDescription); ok {
			return true
		}
	}
	return false
}

// makeCapBlob builds a minimal encoded Agent Capability LSA with a given opaque
// ID so callers can generate distinct instances.
func makeCapBlob(adv uint32, seq int32, oid uint32) []byte {
	var id semantic.AgentID
	copy(id[:], "agent")
	p, _ := semantic.ParsePrefix("medical/x/agent")
	c := &semantic.CapabilityLSA{
		AdvRouter: adv, OpaqueID: oid, Seq: seq, AgentID: id, Prefix: p,
		Domain: "medical", Status: semantic.StatusActive, Capabilities: []string{"ont:x"},
	}
	return c.Encode()
}
