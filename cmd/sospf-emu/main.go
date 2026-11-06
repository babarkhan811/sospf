// Command sospf-emu runs a multi-router S-OSPF topology in a single process and
// prints each router's routing table once the network converges.
//
//	sospf-emu -topo topologies/triangle.json
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/babarkhan811/sospf/internal/emu"
	"github.com/babarkhan811/sospf/internal/node"
	"github.com/babarkhan811/sospf/internal/spf"
)

func main() {
	topoPath := flag.String("topo", "", "path to a JSON topology file")
	wait := flag.Duration("wait", 3*time.Second, "max time to wait for convergence")
	flag.Parse()

	if *topoPath == "" {
		fmt.Fprintln(os.Stderr, "usage: sospf-emu -topo <file.json>")
		os.Exit(2)
	}
	t, err := emu.LoadTopo(*topoPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load topology:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	e := emu.Build(node.DefaultConfig(), t)
	e.Start(ctx)

	if e.WaitStable(len(t.Routers), *wait) {
		fmt.Printf("converged: %d routers, each holding %d LSAs\n\n", len(t.Routers), len(t.Routers))
	} else {
		fmt.Printf("WARNING: did not fully converge within %s\n\n", *wait)
	}
	printTables(e)
}

func printTables(e *emu.Emulator) {
	ids := make([]uint32, 0)
	for id := range e.Routers() {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		r := e.Routers()[id]
		fmt.Printf("router %d routing table:\n", id)
		table := r.Table()
		for _, d := range spf.SortedDests(table) {
			rt := table[d]
			kind := "router"
			if d.Kind == spf.DestPrefix {
				kind = "prefix"
			}
			fmt.Printf("  %-6s %-10d cost=%-4d next-hop=%d\n", kind, d.ID, rt.Cost, rt.NextHop)
		}
		fmt.Println()
	}
}
