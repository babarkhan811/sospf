// Package spf runs Dijkstra's shortest-path-first over the link-state database
// to build a routing table (RFC 2328 §16.1). This is Stage 1 (topology) of
// S-OSPF's semantic path computation; the semantic stages (brief §5.4) build on
// the table produced here.
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

// Compute runs SPF rooted at root and returns the routing table. A link to a
// neighbor is only usable if the neighbor's own Router LSA advertises a link
// back (the two-way-connectivity check, RFC 2328 §16.1 step 2).
func Compute(root uint32, g Graph) map[Dest]Route {
	const inf = int(^uint(0) >> 1)
	dist := map[uint32]int{root: 0}
	nextHop := map[uint32]uint32{root: 0}
	done := map[uint32]bool{}
	table := map[Dest]Route{}

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
		if u != root {
			table[Dest{DestRouter, u}] = Route{Cost: dist[u], NextHop: nextHop[u]}
		}
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
				continue // no reverse link advertised: not usable
			}
			nd := dist[u] + int(link.Metric)
			if cur, ok := dist[v]; !ok || nd < cur {
				dist[v] = nd
				nextHop[v] = firstHop(root, u, v, nextHop)
				heap.Push(pq, item{id: v, cost: nd})
			}
		}
	}

	// Second pass: stub prefixes hang off each reachable router.
	for id, l := range g {
		base, ok := dist[id]
		if !ok || !done[id] {
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
	return table
}

// firstHop returns the first-hop neighbor for v: if we are relaxing v from the
// root, it is v itself; otherwise v inherits u's first hop.
func firstHop(root, u, v uint32, nextHop map[uint32]uint32) uint32 {
	if u == root {
		return v
	}
	return nextHop[u]
}

// linkBack reports whether router a advertises a point-to-point link to b.
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

// SortedDests returns the table's destinations in a deterministic order, for
// stable display and testing.
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

func (q pq) Len() int            { return len(q) }
func (q pq) Less(i, j int) bool  { return q[i].cost < q[j].cost }
func (q pq) Swap(i, j int)       { q[i], q[j] = q[j], q[i] }
func (q *pq) Push(x any)         { *q = append(*q, x.(item)) }
func (q *pq) Pop() any {
	old := *q
	n := len(old)
	it := old[n-1]
	*q = old[:n-1]
	return it
}
