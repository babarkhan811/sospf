// Command sospf-bench measures convergence time and LSDB size versus router
// count in the emulator, for the proposal benchmarks (brief §11.4).
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/babarkhan811/sospf/internal/emu"
	"github.com/babarkhan811/sospf/internal/node"
)

// ring builds a ring of n routers plus a few chords for path diversity.
func ring(n int) *emu.Topo {
	t := &emu.Topo{}
	for i := 1; i <= n; i++ {
		t.Routers = append(t.Routers, emu.TopoRouter{ID: uint32(i), Prefix: uint32(0x0A000000 + i), Metric: 1})
	}
	for i := 1; i <= n; i++ {
		j := i%n + 1
		t.Links = append(t.Links, emu.TopoLink{A: uint32(i), B: uint32(j), Metric: 1})
	}
	// Chords every ~n/4 hops for diversity.
	step := n / 4
	if step >= 2 {
		for i := 1; i+step <= n; i += step {
			t.Links = append(t.Links, emu.TopoLink{A: uint32(i), B: uint32(i + step), Metric: 2})
		}
	}
	return t
}

func main() {
	fmt.Printf("%-8s %-14s %-12s\n", "routers", "converge(ms)", "lsdb/router")
	for _, n := range []int{3, 10, 20, 30, 50} {
		t := ring(n)
		e := emu.Build(node.DefaultConfig(), t)
		ctx, cancel := context.WithCancel(context.Background())
		start := time.Now()
		e.Start(ctx)
		ok := e.WaitStable(n, 15*time.Second)
		elapsed := time.Since(start)
		cancel()

		lsdb := 0
		for _, r := range e.Routers() {
			lsdb = r.DBLen()
			break
		}
		status := ""
		if !ok {
			status = " (TIMEOUT)"
		}
		fmt.Printf("%-8d %-14.1f %-12d%s\n", n, float64(elapsed.Microseconds())/1000.0, lsdb, status)
	}
}
