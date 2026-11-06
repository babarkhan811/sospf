// Package node is the S-OSPF router: it drives the neighbor FSM, originates its
// own Router LSA, floods LSAs reliably, and recomputes SPF when the database
// changes. One Router runs in its own goroutine and communicates only through
// messages, so there is no shared mutable state between routers (ADR-0001).
package node

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/babarkhan811/sospf/internal/fsm"
	"github.com/babarkhan811/sospf/internal/lsdb"
	"github.com/babarkhan811/sospf/internal/packet"
	"github.com/babarkhan811/sospf/internal/spf"
)

// Config holds the timers that drive a router. Tests use short intervals.
type Config struct {
	HelloInterval time.Duration
	DeadInterval  time.Duration
	InboxCap      int
	// SemanticBudget caps how many semantic updates are processed per scheduling
	// cycle, so a flood of agent/trust updates can never monopolise the router
	// and delay core routing (ADR-0005).
	SemanticBudget int
	SemInboxCap    int
}

// DefaultConfig returns emulator-friendly timers.
func DefaultConfig() Config {
	// Dead interval is a generous multiple of the hello interval so that
	// transient processing lag under a flooding storm does not trip a false
	// dead-neighbor detection (which would churn re-origination and prevent
	// convergence at scale).
	return Config{
		HelloInterval:  15 * time.Millisecond,
		DeadInterval:   1000 * time.Millisecond,
		InboxCap:       65536,
		SemanticBudget: 256,
		SemInboxCap:    1 << 20,
	}
}

type neighbor struct {
	id         uint32
	cost       uint16
	state      fsm.State
	lastHeard  time.Time
	heardMe    bool
	master     bool
	ddSummSent bool // have we sent our DB summary to this neighbor this round?
	ddRecvd    bool // have we received the neighbor's DB summary this round?
	reqList    map[packet.Key]bool
}

type envelope struct {
	from uint32
	msg  any
}

// Router is a single S-OSPF node.
type Router struct {
	ID   uint32
	Area uint32
	cfg  Config

	send     func(to uint32, msg any) // installed by the transport
	inbox    chan envelope            // core routing messages (high priority)
	semInbox chan packet.SemanticUpdate // semantic updates (low priority, budgeted)

	semDB        map[semKey]int32 // latest seq per semantic LSA (dampens re-floods)
	semProcessed uint64           // atomic: semantic updates applied

	neighbors map[uint32]*neighbor
	stubs     []packet.RouterLink
	db        *lsdb.DB
	selfSeq   int32

	mu       sync.RWMutex
	table    map[spf.Dest]spf.Route
	dirty    bool
	syncTick int

	started bool
}

// NewRouter creates a router with the given ID and stub prefixes.
func NewRouter(id, area uint32, cfg Config, stubs []packet.RouterLink) *Router {
	if cfg.InboxCap <= 0 {
		cfg.InboxCap = 8192
	}
	if cfg.SemInboxCap <= 0 {
		cfg.SemInboxCap = 1 << 20
	}
	if cfg.SemanticBudget <= 0 {
		cfg.SemanticBudget = 256
	}
	return &Router{
		ID:        id,
		Area:      area,
		cfg:       cfg,
		inbox:     make(chan envelope, cfg.InboxCap),
		semInbox:  make(chan packet.SemanticUpdate, cfg.SemInboxCap),
		semDB:     make(map[semKey]int32),
		neighbors: make(map[uint32]*neighbor),
		stubs:     stubs,
		db:        lsdb.New(),
		selfSeq:   packet.InitialSeqNum - 1,
		table:     map[spf.Dest]spf.Route{},
	}
}

// semKey identifies a semantic Opaque LSA for dampening.
type semKey struct {
	opaqueType uint8
	advRouter  uint32
	opaqueID   uint32
}

// AddLink configures a point-to-point link to neighbor id with the given metric.
func (r *Router) AddLink(id uint32, metric uint16) {
	r.neighbors[id] = &neighbor{id: id, cost: metric, state: fsm.Down, reqList: map[packet.Key]bool{}}
}

// SetSend installs the transport's send function.
func (r *Router) SetSend(f func(to uint32, msg any)) { r.send = f }

// Deliver hands an inbound message to this router (called by the transport). It
// never blocks the caller: if the inbox is momentarily full it completes the
// send on a short-lived goroutine, so two routers flooding to each other can
// never deadlock. Reordering is acceptable — OSPF flooding tolerates it.
func (r *Router) Deliver(from uint32, msg any) {
	// Semantic updates go to the low-priority queue; everything else is core
	// routing and goes to the high-priority queue.
	if su, ok := msg.(packet.SemanticUpdate); ok {
		select {
		case r.semInbox <- su:
		default: // queue full: drop. Semantics are soft-state and re-synced by
			// periodic anti-entropy, so dropping under overload protects core routing.
		}
		return
	}
	e := envelope{from: from, msg: msg}
	select {
	case r.inbox <- e:
	default:
		go func() { r.inbox <- e }()
	}
}

// Start runs the router until ctx is cancelled.
func (r *Router) Start(ctx context.Context) {
	r.started = true
	r.originate() // initial self LSA (stubs only, no adjacencies yet)
	r.recompute()

	go func() {
		hello := time.NewTicker(r.cfg.HelloInterval)
		maint := time.NewTicker(r.cfg.HelloInterval)
		defer hello.Stop()
		defer maint.Stop()
		for {
			// Core routing is strictly higher priority than semantic work: every
			// cycle drains all pending core messages first, then processes at most
			// SemanticBudget semantic updates (ADR-0005). So a flood of agent/trust
			// updates can delay neither adjacency maintenance nor IP convergence.
			select {
			case <-ctx.Done():
				return
			case e := <-r.inbox:
				r.handle(e)
				r.drainCore()
			case <-hello.C:
				r.sendHellos()
			case <-maint.C:
				r.checkDead()
				r.syncTick++
				if r.syncTick%20 == 0 {
					r.antiEntropy() // periodic full-LSDB sync closes any flooding gap
				}
				if r.dirty {
					r.recompute() // at most once per tick, so message floods never starve timers
					r.dirty = false
				}
			case su := <-r.semInbox:
				r.drainCore() // core always first
				r.handleSemantic(su)
				r.processSemanticBatch(r.cfg.SemanticBudget - 1)
			}
		}
	}()
}

// drainCore handles every core routing message currently queued, without
// blocking. It is called before any semantic work so core routing is never
// starved.
func (r *Router) drainCore() {
	for {
		select {
		case e := <-r.inbox:
			r.handle(e)
		default:
			return
		}
	}
}

// processSemanticBatch applies up to n queued semantic updates, without blocking.
func (r *Router) processSemanticBatch(n int) {
	for i := 0; i < n; i++ {
		select {
		case su := <-r.semInbox:
			r.handleSemantic(su)
		default:
			return
		}
	}
}

// handleSemantic applies one SemanticUpdate: it dampens each Opaque LSA (keeps
// only newer instances) into a store that is SEPARATE from the topology LSDB and
// never feeds the IP shortest-path computation — so semantic churn cannot
// trigger an IP SPF run or delay IP convergence (ADR-0005).
func (r *Router) handleSemantic(su packet.SemanticUpdate) {
	for _, blob := range su.Blobs {
		h, err := packet.DecodeHeader(blob)
		if err != nil {
			continue
		}
		ot, oid := packet.SplitLinkStateID(h.ID)
		k := semKey{opaqueType: ot, advRouter: h.AdvRouter, opaqueID: oid}
		if cur, ok := r.semDB[k]; ok && h.SeqNum <= cur {
			continue // not newer: dampen (no re-flood, no work)
		}
		r.semDB[k] = h.SeqNum
		atomic.AddUint64(&r.semProcessed, 1)
	}
}

// SemProcessed returns how many semantic updates have been applied (for metrics
// and tests).
func (r *Router) SemProcessed() uint64 { return atomic.LoadUint64(&r.semProcessed) }

// SemLen returns the number of distinct semantic LSAs held.
func (r *Router) SemLen() int { return len(r.semDB) }

// ---- origination ----

func (r *Router) originate() {
	r.selfSeq++
	links := make([]packet.RouterLink, 0, len(r.neighbors)+len(r.stubs))
	for _, n := range r.neighbors {
		if n.state == fsm.Full {
			links = append(links, packet.RouterLink{Type: packet.LinkPointToPoint, ID: n.id, Metric: n.cost})
		}
	}
	links = append(links, r.stubs...)
	lsa := &packet.RouterLSA{
		Header: packet.LSAHeader{Type: packet.LSARouter, AdvRouter: r.ID, SeqNum: r.selfSeq, Age: 0},
		Links:  links,
	}
	lsa.Encode() // fills Length + Checksum + ID
	if r.db.Install(lsa, time.Now()) {
		r.dirty = true
		r.flood(lsa, 0)
	}
}

// ---- message handling ----

func (r *Router) handle(e envelope) {
	switch m := e.msg.(type) {
	case packet.Hello:
		r.onHello(e.from, m)
	case packet.DatabaseDescription:
		r.onDD(e.from, m)
	case packet.LinkStateRequest:
		r.onLSR(e.from, m)
	case packet.LinkStateUpdate:
		r.onLSU(e.from, m)
	case packet.LinkStateAck:
		// Phase 2 emulator delivery is reliable, so no retransmission list is
		// kept; acks are accepted and ignored. Retransmission arrives with the
		// lossy UDP/raw transports.
	}
}

func (r *Router) onHello(from uint32, h packet.Hello) {
	n := r.neighbors[from]
	if n == nil {
		return // Hello from a router we have no configured link to
	}
	n.lastHeard = time.Now()
	n.state = fsm.Next(n.state, fsm.HelloReceived)
	if containsID(h.Neighbors, r.ID) {
		prev := n.state
		n.heardMe = true
		n.state = fsm.Next(n.state, fsm.TwoWayReceived)
		if n.state == fsm.ExStart && prev != fsm.ExStart {
			r.startExchange(n)
		}
	}
}

func (r *Router) startExchange(n *neighbor) {
	n.master = r.ID > n.id // higher Router ID is master (RFC 2328 §10.8)
	n.ddSummSent = false
	n.ddRecvd = false
	n.reqList = map[packet.Key]bool{}
	// Nudge the peer. If it has not yet reached ExStart it will ignore this; the
	// peer's own nudge (sent when it reaches ExStart) drives us forward, so the
	// handshake completes without needing DD retransmission on the reliable
	// emulator transport.
	flags := packet.DDFlagInitBit
	if n.master {
		flags |= packet.DDFlagMasterBit
	}
	r.send(n.id, packet.DatabaseDescription{Flags: flags, DDSeq: r.ID, Headers: r.dbHeaders()})
}

func (r *Router) onDD(from uint32, dd packet.DatabaseDescription) {
	n := r.neighbors[from]
	if n == nil || n.state < fsm.ExStart {
		return // DDs are only processed once we are negotiating (RFC 2328 §10.6)
	}
	if n.state == fsm.ExStart {
		n.state = fsm.Next(n.state, fsm.NegotiationDone) // -> Exchange
	}
	// Anything the neighbor holds that we lack (or that is newer) goes on the
	// request list.
	for _, h := range dd.Headers {
		if !r.db.Has(h) {
			n.reqList[h.Key()] = true
		}
	}
	n.ddRecvd = true
	// Send our summary exactly once per round, now that the peer is known to be
	// negotiating (so it will not ignore it).
	if !n.ddSummSent {
		n.ddSummSent = true
		flags := uint8(0)
		if n.master {
			flags |= packet.DDFlagMasterBit
		}
		r.send(n.id, packet.DatabaseDescription{Flags: flags, DDSeq: dd.DDSeq, Headers: r.dbHeaders()})
	}
	if n.state == fsm.Exchange && n.ddRecvd && n.ddSummSent {
		n.state = fsm.Next(n.state, fsm.ExchangeDone) // -> Loading
		r.enterLoading(n)
	}
}

func (r *Router) enterLoading(n *neighbor) {
	if len(n.reqList) == 0 {
		r.toFull(n)
		return
	}
	reqs := make([]packet.Key, 0, len(n.reqList))
	for k := range n.reqList {
		reqs = append(reqs, k)
	}
	r.send(n.id, packet.LinkStateRequest{Reqs: reqs})
}

func (r *Router) onLSR(from uint32, lsr packet.LinkStateRequest) {
	n := r.neighbors[from]
	if n == nil {
		return
	}
	out := make([]*packet.RouterLSA, 0, len(lsr.Reqs))
	for _, k := range lsr.Reqs {
		if lsa := r.db.Get(k); lsa != nil {
			out = append(out, lsa)
		}
	}
	if len(out) > 0 {
		r.send(from, packet.LinkStateUpdate{Routers: out})
	}
}

func (r *Router) onLSU(from uint32, lsu packet.LinkStateUpdate) {
	n := r.neighbors[from]
	acks := make([]packet.LSAHeader, 0, len(lsu.Routers))
	for _, lsa := range lsu.Routers {
		acks = append(acks, lsa.Header)
		if r.db.Install(lsa, time.Now()) {
			r.dirty = true
			r.flood(lsa, from) // re-flood newer LSAs to every other adjacency
		}
		if n != nil {
			delete(n.reqList, lsa.Header.Key())
		}
	}
	if n != nil && len(acks) > 0 {
		r.send(from, packet.LinkStateAck{Headers: acks})
	}
	if n != nil && n.state == fsm.Loading && len(n.reqList) == 0 {
		r.toFull(n)
	}
}

func (r *Router) toFull(n *neighbor) {
	n.state = fsm.Next(n.state, fsm.LoadingDone) // -> Full
	r.originate()                                // now advertise the p2p link to n
}

// ---- flooding / hellos / timers ----

func (r *Router) flood(lsa *packet.RouterLSA, except uint32) {
	for _, n := range r.neighbors {
		if n.id == except || !fsm.IsAdjacent(n.state) {
			continue
		}
		r.send(n.id, packet.LinkStateUpdate{Routers: []*packet.RouterLSA{lsa}})
	}
}

// antiEntropy re-floods the whole database to every adjacent neighbor. Receivers
// install only newer instances (and so do not re-flood), making this idempotent
// after convergence; it exists to repair any LSA a neighbor missed under
// message reordering, guaranteeing eventual completeness without a per-neighbor
// retransmission list (that optimization lands with the real transport).
func (r *Router) antiEntropy() {
	lsas := r.db.All(time.Now())
	if len(lsas) == 0 {
		return
	}
	for _, n := range r.neighbors {
		if fsm.IsAdjacent(n.state) {
			r.send(n.id, packet.LinkStateUpdate{Routers: lsas})
		}
	}
}

func (r *Router) sendHellos() {
	heard := make([]uint32, 0, len(r.neighbors))
	for _, n := range r.neighbors {
		if n.state >= fsm.Init {
			heard = append(heard, n.id)
		}
	}
	h := packet.Hello{
		HelloInterval: uint16(r.cfg.HelloInterval / time.Millisecond),
		DeadInterval:  uint32(r.cfg.DeadInterval / time.Millisecond),
		Neighbors:     heard,
	}
	for _, n := range r.neighbors {
		r.send(n.id, h)
	}
}

func (r *Router) checkDead() {
	now := time.Now()
	for _, n := range r.neighbors {
		if n.state == fsm.Down || n.lastHeard.IsZero() {
			continue
		}
		if now.Sub(n.lastHeard) > r.cfg.DeadInterval {
			n.state = fsm.Down
			n.heardMe, n.ddSummSent, n.ddRecvd = false, false, false
			n.reqList = map[packet.Key]bool{}
			r.originate() // re-originate without the dead link
		}
	}
}

// ---- helpers / accessors ----

func (r *Router) dbHeaders() []packet.LSAHeader {
	lsas := r.db.All(time.Now())
	hs := make([]packet.LSAHeader, 0, len(lsas))
	for _, l := range lsas {
		hs = append(hs, l.Header)
	}
	return hs
}

func (r *Router) recompute() {
	g := spf.BuildGraph(r.db.All(time.Now()))
	t := spf.Compute(r.ID, g)
	r.mu.Lock()
	r.table = t
	r.mu.Unlock()
}

// Table returns a snapshot of the routing table.
func (r *Router) Table() map[spf.Dest]spf.Route {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[spf.Dest]spf.Route, len(r.table))
	for k, v := range r.table {
		out[k] = v
	}
	return out
}

// DBLen returns the number of LSAs in this router's database.
func (r *Router) DBLen() int { return r.db.Len() }

func containsID(ids []uint32, id uint32) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}
