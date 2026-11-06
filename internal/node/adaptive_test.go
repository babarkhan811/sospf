package node_test

import (
	"context"
	"testing"
	"time"

	"github.com/babarkhan811/sospf/internal/emu"
	"github.com/babarkhan811/sospf/internal/node"
	"github.com/babarkhan811/sospf/internal/spf"
)

// Two parallel paths from router 1 to router 4:
//
//	1 --10-- 2 --10-- 4   (base total 20, initially preferred)
//	1 --12-- 3 --12-- 4   (base total 24, the alternate)
//
// When the upper path congests, its dynamic cost rises and traffic should
// re-route to the lower path — without flapping.
func parallelPaths() *emu.Topo {
	return &emu.Topo{
		Routers: []emu.TopoRouter{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}},
		Links: []emu.TopoLink{
			{A: 1, B: 2, Metric: 10},
			{A: 2, B: 4, Metric: 10},
			{A: 1, B: 3, Metric: 12},
			{A: 3, B: 4, Metric: 12},
		},
	}
}

func waitNextHop(t *testing.T, r *node.Router, dest spf.Dest, want uint32, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last spf.Route
	for time.Now().Before(deadline) {
		last = r.Table()[dest]
		if last.NextHop == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("route to %+v did not reach next-hop %d (last: %+v)", dest, want, last)
}

func TestTelemetryDrivenReroute(t *testing.T) {
	e := emu.Build(node.DefaultConfig(), parallelPaths())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.Start(ctx)

	if !e.WaitStable(4, 3*time.Second) {
		t.Fatal("initial convergence failed")
	}
	r1 := e.Routers()[1]
	dest4 := spf.Dest{Kind: spf.DestRouter, ID: 4}

	// Initially the cheaper upper path (via router 2) is chosen.
	waitNextHop(t, r1, dest4, 2, 2*time.Second)

	// Congest the upper path: high latency + load on 1-2 and 2-4. Their dynamic
	// costs climb past the alternate, so 1->4 should re-route via router 3.
	for i := 0; i < 20; i++ { // repeat so the EWMA converges upward
		e.SetLinkTelemetry(1, 2, 300, 0.95)
		e.SetLinkTelemetry(2, 4, 300, 0.95)
		time.Sleep(10 * time.Millisecond)
	}

	waitNextHop(t, r1, dest4, 3, 3*time.Second)
}

func TestNoFlapUnderNoisyTelemetry(t *testing.T) {
	e := emu.Build(node.DefaultConfig(), parallelPaths())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.Start(ctx)
	if !e.WaitStable(4, 3*time.Second) {
		t.Fatal("initial convergence failed")
	}
	r1 := e.Routers()[1]
	dest4 := spf.Dest{Kind: spf.DestRouter, ID: 4}
	waitNextHop(t, r1, dest4, 2, 2*time.Second)

	// Inject small latency noise around a healthy, lightly-loaded operating point
	// (well below the alternate path's cost) for a while. The jitter stays within
	// the hysteresis band, so the advertised cost — and the route — must not move.
	for i := 0; i < 40; i++ {
		lat := 5.0
		if i%2 == 0 {
			lat = 8.0
		}
		e.SetLinkTelemetry(1, 2, lat, 0.0)
		e.SetLinkTelemetry(2, 4, lat, 0.0)
		time.Sleep(5 * time.Millisecond)
	}
	// The preferred next hop must not have flapped away from 2.
	if nh := r1.Table()[dest4].NextHop; nh != 2 {
		t.Fatalf("route flapped under noise: next-hop is %d, expected stable 2", nh)
	}
}
