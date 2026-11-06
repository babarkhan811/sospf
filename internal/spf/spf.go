// Package spf runs Dijkstra's shortest-path-first over the link-state database
// to build a routing table (RFC 2328 §16.1). This is Stage 1 (topology) of
// S-OSPF's semantic path computation; the semantic stages (brief §5.4) build on
// the table produced here.
//
// It also computes loop-free alternate (LFA) backup next hops (RFC 5286-style)
// so a router can fail over locally and instantly when a neighbor degrades,
// instead of waiting for reconvergence (EXPERIMENTAL idea #2: predictive
// failover driven by the trust/telemetry layer).
package spf

import (
	"container/heap"
	"sort"

	"github.com/babarkhan811/sospf/internal/packet"
)

// DestKind distinguishes a route to a router from a route to a stub prefix.
type DestKind uint8

const (
	DestRouter DestKind = iota
	DestPrefix
)

// Dest is a routing-table destination.
type Dest struct {
	Kind DestKind
	ID   uint32 // Router ID, or stub network number
}

// Route is a shortest-path result.
type Route struct {
	Cost    int
	NextHop uint32 // first-hop neighbor Router ID (0 = directly connected / self)
}

// Protected is a primary route plus an optional loop-free backup next hop.
type Protected struct {
	Primary   Route
	Backup    Route
	HasBackup bool
}

// Graph is the set of Router LSAs to run SPF over, keyed by Advertising Router.
type Graph map[uint32]*packet.RouterLSA

// BuildGraph indexes Router LSAs by their Advertising Router.
func BuildGraph(lsas []*packet.RouterLSA) Graph {
	g := make(Graph, len(lsas))
	for _, l := range lsas {
		g[l.Header.AdvRouter] = l
	}
	return g
}

// dijkstra computes shortest-path distances and first-hop neighbors from root
// over point-to-point links, applying the two-way-connectivity check.
func dijkstra(root uint32, g Graph) (dist map[uint32]int, nextHop map[uint32]uint32) {
	dist = map[uint32]int{root: 0}
	nextHop = map[uint32]uint32{root: 0}
	done := map[uint32]bool{}

	pq := &pq{}
	heap.Init(pq)
	heap.Push(pq, item{id: root, cost: 0})

	for pq.Len() > 0 {
		it := heap.Pop(pq).(item)
		u := it.id
		if done[u] {
			continue
		}
		done[u] = true
		lu := g[u]
		if lu == nil {
			continue
		}
		for _, link := range lu.Links {
			if link.Type != packet.LinkPointToPoint {
				continue
			}
			v := link.ID
			if !linkBack(g, v, u) {
				continue
			}
			nd := dist[u] + int(link.Metric)
			if cur, ok := dist[v]; !ok || nd < cur {
				dist[v] = nd
				nextHop[v] = firstHop(root, u, v, nextHop)
				heap.Push(pq, item{id: v, cost: nd})
			}
		}
	}
	return dist, nextHop
}

// Compute runs SPF rooted at root and returns the routing table (routers and
// stub prefixes).
func Compute(root uint32, g Graph) map[Dest]Route {
	dist, nextHop := dijkstra(root, g)
	table := map[Dest]Route{}
	for id, d := range dist {
		if id == root {
			continue
		}
		table[Dest{DestRouter, id}] = Route{Cost: d, NextHop: nextHop[id]}
	}
	addStubs(root, g, dist, nextHop, table)
	return table
}

// ComputeWithBackup returns, for each destination, the primary route plus a
// loop-free alternate backup next hop when one exists. A neighbor N is a valid
// LFA for destination D if its own shortest path to D does not come back through
// us: dist(N,D) < dist(N,root) + dist(root,D) (RFC 5286 inequality).
func ComputeWithBackup(root uint32, g Graph) map[Dest]Protected {
	distS, nhS := dijkstra(root, g)
	out := map[Dest]Protected{}

	// Precompute each directly-connected neighbor's distances.
	neighbors := map[uint32]int{} // neighbor -> link cost
	if lr := g[root]; lr != nil {
		for _, link := range lr.Links {
			if link.Type == packet.LinkPointToPoint && linkBack(g, link.ID, root) {
				if c, ok := neighbors[link.ID]; !ok || int(link.Metric) < c {
					neighbors[link.ID] = int(link.Metric)
				}
			}
		}
	}
	neighborDist := map[uint32]map[uint32]int{}
	for n := range neighbors {
		d, _ := dijkstra(n, g)
		neighborDist[n] = d
	}

	backupRouter := map[uint32]Route{} // router dest -> backup

	for id, d := range distS {
		if id == root {
			continue
		}
		primary := Route{Cost: d, NextHop: nhS[id]}
		p := Protected{Primary: primary}

		bestCost := -1
		for n, linkCost := range neighbors {
			if n == primary.NextHop {
				continue // the backup must avoid the primary next hop
			}
			dn := neighborDist[n]
			dnD, ok := dn[id]
			if !ok {
				continue
			}
			dnRoot, ok := dn[root]
			if !ok {
				continue
			}
			if dnD < dnRoot+d { // loop-free alternate
				cand := linkCost + dnD
				if bestCost < 0 || cand < bestCost {
					bestCost = cand
					p.Backup = Route{Cost: cand, NextHop: n}
					p.HasBackup = true
				}
			}
		}
		out[Dest{DestRouter, id}] = p
		if p.HasBackup {
			backupRouter[id] = p.Backup
		}
	}

	// Stub prefixes inherit their router's primary and backup.
	for id, l := range g {
		pd, ok := distS[id]
		if !ok {
			continue
		}
		for _, link := range l.Links {
			if link.Type != packet.LinkStub {
				continue
			}
			dest := Dest{DestPrefix, link.ID}
			prim := Route{Cost: pd + int(link.Metric), NextHop: primaryNH(root, id, nhS)}
			p := Protected{Primary: prim}
			if id != root {
				if b, ok := backupRouter[id]; ok {
					p.Backup = Route{Cost: b.Cost + int(link.Metric), NextHop: b.NextHop}
					p.HasBackup = true
				}
			}
			if cur, exists := out[dest]; !exists || prim.Cost < cur.Primary.Cost {
				out[dest] = p
			}
		}
	}
	return out
}

func primaryNH(root, id uint32, nhS map[uint32]uint32) uint32 {
	if id == root {
		return 0
	}
	return nhS[id]
}

func addStubs(root uint32, g Graph, dist map[uint32]int, nextHop map[uint32]uint32, table map[Dest]Route) {
	for id, l := range g {
		base, ok := dist[id]
		if !ok {
			continue
		}
		for _, link := range l.Links {
			if link.Type != packet.LinkStub {
				continue
			}
			d := Dest{DestPrefix, link.ID}
			cost := base + int(link.Metric)
			nh := uint32(0)
			if id != root {
				nh = nextHop[id]
			}
			if cur, ok := table[d]; !ok || cost < cur.Cost {
				table[d] = Route{Cost: cost, NextHop: nh}
			}
		}
	}
}

func firstHop(root, u, v uint32, nextHop map[uint32]uint32) uint32 {
	if u == root {
		return v
	}
	return nextHop[u]
}

func linkBack(g Graph, a, b uint32) bool {
	la := g[a]
	if la == nil {
		return false
	}
	for _, link := range la.Links {
		if link.Type == packet.LinkPointToPoint && link.ID == b {
			return true
		}
	}
	return false
}

// SortedDests returns the table's destinations in a deterministic order.
func SortedDests(table map[Dest]Route) []Dest {
	out := make([]Dest, 0, len(table))
	for d := range table {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// ---- priority queue ----

type item struct {
	id   uint32
	cost int
}
type pq []item

func (q pq) Len() int           { return len(q) }
func (q pq) Less(i, j int) bool { return q[i].cost < q[j].cost }
func (q pq) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *pq) Push(x any)        { *q = append(*q, x.(item)) }
func (q *pq) Pop() any {
	old := *q
	n := len(old)
	it := old[n-1]
	*q = old[:n-1]
	return it
}
