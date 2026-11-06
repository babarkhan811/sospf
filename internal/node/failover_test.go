package node_test

import (
	"context"
	"testing"
	"time"

	"github.com/babarkhan811/sospf/internal/emu"
	"github.com/babarkhan811/sospf/internal/node"
	"github.com/babarkhan811/sospf/internal/spf"
)

// protectedTopo: 1-2:1, 1-3:1, 2-4:1, 3-4:2. Primary 1->4 is via 2; router 3 is
// a loop-free backup.
func protectedTopo() *emu.Topo {
	return &emu.Topo{
		Routers: []emu.TopoRouter{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}},
		Links: []emu.TopoLink{
			{A: 1, B: 2, Metric: 1},
			{A: 1, B: 3, Metric: 1},
			{A: 2, B: 4, Metric: 1},
			{A: 3, B: 4, Metric: 2},
		},
	}
}

// A health signal (not a metric change, not a dead timer) flips forwarding to
// the precomputed backup immediately, while the primary table is unchanged.
func TestPredictiveFailoverUsesBackup(t *testing.T) {
	e := emu.Build(node.DefaultConfig(), protectedTopo())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.Start(ctx)
	if !e.WaitStable(4, 3*time.Second) {
		t.Fatal("initial convergence failed")
	}
	r1 := e.Routers()[1]
	dest4 := spf.Dest{Kind: spf.DestRouter, ID: 4}

	// Baseline: primary and effective both go via neighbor 2.
	waitEffNextHop(t, r1, dest4, 2, 2*time.Second)
	if r1.Table()[dest4].NextHop != 2 {
		t.Fatalf("primary to 4 should be via 2, got %d", r1.Table()[dest4].NextHop)
	}

	// Predict neighbor 2 is failing. Forwarding must switch to the backup (via 3)
	// quickly — well under the 1s dead interval — without the primary changing.
	r1.InjectHealth(2, true)
	waitEffNextHop(t, r1, dest4, 3, 500*time.Millisecond)

	if r1.Table()[dest4].NextHop != 2 {
		t.Errorf("primary table must be unchanged (still via 2); got %d — failover should be local/effective only",
			r1.Table()[dest4].NextHop)
	}

	// Recovery: when the neighbor is healthy again, forwarding returns to primary.
	r1.InjectHealth(2, false)
	waitEffNextHop(t, r1, dest4, 2, 1*time.Second)
}

func waitEffNextHop(t *testing.T, r *node.Router, dest spf.Dest, want uint32, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last spf.Route
	for time.Now().Before(deadline) {
		last = r.EffectiveTable()[dest]
		if last.NextHop == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("effective route to %+v did not reach next-hop %d (last: %+v)", dest, want, last)
}
