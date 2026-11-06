package spf

import (
	"testing"

	"github.com/babarkhan811/sospf/internal/packet"
)

func rtr(id uint32, links ...packet.RouterLink) *packet.RouterLSA {
	return &packet.RouterLSA{
		Header: packet.LSAHeader{Type: packet.LSARouter, ID: id, AdvRouter: id, SeqNum: packet.InitialSeqNum},
		Links:  links,
	}
}
func p2p(to uint32, m uint16) packet.RouterLink {
	return packet.RouterLink{Type: packet.LinkPointToPoint, ID: to, Metric: m}
}
func stub(net uint32, m uint16) packet.RouterLink {
	return packet.RouterLink{Type: packet.LinkStub, ID: net, Metric: m}
}

// Triangle: 1-2 cost1, 2-3 cost1, 1-3 cost5. From 1, the path to 3 goes via 2.
func TestTriangleShortestPath(t *testing.T) {
	g := BuildGraph([]*packet.RouterLSA{
		rtr(1, p2p(2, 1), p2p(3, 5), stub(100, 1)),
		rtr(2, p2p(1, 1), p2p(3, 1), stub(200, 1)),
		rtr(3, p2p(1, 5), p2p(2, 1), stub(300, 1)),
	})
	tab := Compute(1, g)

	if r := tab[Dest{DestRouter, 2}]; r.Cost != 1 || r.NextHop != 2 {
		t.Errorf("route to 2: got %+v want cost1 nh2", r)
	}
	if r := tab[Dest{DestRouter, 3}]; r.Cost != 2 || r.NextHop != 2 {
		t.Errorf("route to 3: got %+v want cost2 nh2 (via 2, not the direct cost-5 link)", r)
	}
	if r := tab[Dest{DestPrefix, 300}]; r.Cost != 3 || r.NextHop != 2 {
		t.Errorf("prefix 300: got %+v want cost3 nh2", r)
	}
	if r := tab[Dest{DestPrefix, 100}]; r.Cost != 1 || r.NextHop != 0 {
		t.Errorf("local prefix 100: got %+v want cost1 nh0", r)
	}
}

// A one-way link (only 1 advertises 1->2) must not be used: the two-way check.
func TestOneWayLinkIsIgnored(t *testing.T) {
	g := BuildGraph([]*packet.RouterLSA{
		rtr(1, p2p(2, 1)),
		rtr(2), // router 2 advertises no link back to 1
	})
	tab := Compute(1, g)
	if _, ok := tab[Dest{DestRouter, 2}]; ok {
		t.Error("router 2 should be unreachable without a reverse link")
	}
}

func TestReconvergeAfterMetricChange(t *testing.T) {
	// After 2-3 is gone, 1 reaches 3 only via the direct cost-5 link.
	g := BuildGraph([]*packet.RouterLSA{
		rtr(1, p2p(2, 1), p2p(3, 5), stub(100, 1)),
		rtr(2, p2p(1, 1), stub(200, 1)),
		rtr(3, p2p(1, 5), stub(300, 1)),
	})
	tab := Compute(1, g)
	if r := tab[Dest{DestRouter, 3}]; r.Cost != 5 || r.NextHop != 3 {
		t.Errorf("route to 3 after topology change: got %+v want cost5 nh3", r)
	}
}
