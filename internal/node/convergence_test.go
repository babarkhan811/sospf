package node_test

import (
	"context"
	"testing"
	"time"

	"github.com/babarkhan811/sospf/internal/emu"
	"github.com/babarkhan811/sospf/internal/node"
	"github.com/babarkhan811/sospf/internal/spf"
)

func triangle() *emu.Topo {
	return &emu.Topo{
		Routers: []emu.TopoRouter{
			{ID: 1, Prefix: 0x01010100, Metric: 1},
			{ID: 2, Prefix: 0x01010200, Metric: 1},
			{ID: 3, Prefix: 0x01010300, Metric: 1},
		},
		Links: []emu.TopoLink{
			{A: 1, B: 2, Metric: 1},
			{A: 2, B: 3, Metric: 1},
			{A: 1, B: 3, Metric: 5},
		},
	}
}

// waitRoute polls r until the route to dest matches (cost, nh), or fails.
func waitRoute(t *testing.T, r *node.Router, dest spf.Dest, cost int, nh uint32, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last spf.Route
	for time.Now().Before(deadline) {
		last = r.Table()[dest]
		if last.Cost == cost && last.NextHop == nh {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("route to %+v did not converge: got %+v want cost=%d nh=%d", dest, last, cost, nh)
}

func TestThreeRouterConvergence(t *testing.T) {
	e := emu.Build(node.DefaultConfig(), triangle())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.Start(ctx)

	if !e.WaitStable(3, 2*time.Second) {
		t.Fatal("topology did not converge: not every router holds 3 LSAs")
	}

	r1 := e.Routers()[1]
	// From router 1: reach 2 directly (cost 1); reach 3 via 2 (cost 2), NOT over
	// the direct cost-5 link.
	waitRoute(t, r1, spf.Dest{Kind: spf.DestRouter, ID: 2}, 1, 2, time.Second)
	waitRoute(t, r1, spf.Dest{Kind: spf.DestRouter, ID: 3}, 2, 2, time.Second)
	// Stub prefix behind router 3 inherits the via-2 next hop.
	waitRoute(t, r1, spf.Dest{Kind: spf.DestPrefix, ID: 0x01010300}, 3, 2, time.Second)
}

func TestReconvergenceOnLinkFailure(t *testing.T) {
	cfg := node.DefaultConfig()
	e := emu.Build(cfg, triangle())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.Start(ctx)

	if !e.WaitStable(3, 2*time.Second) {
		t.Fatal("initial convergence failed")
	}
	r1 := e.Routers()[1]
	waitRoute(t, r1, spf.Dest{Kind: spf.DestRouter, ID: 3}, 2, 2, time.Second)

	// Cut the 2<->3 link. Routers 2 and 3 should time out the adjacency
	// (DeadInterval), re-originate without that link, and the network should
	// reroute 1->3 over the direct cost-5 link.
	e.SetLinkUp(2, 3, false)

	waitRoute(t, r1, spf.Dest{Kind: spf.DestRouter, ID: 3}, 5, 3, 3*time.Second)
	waitRoute(t, r1, spf.Dest{Kind: spf.DestPrefix, ID: 0x01010300}, 6, 3, 3*time.Second)
}
