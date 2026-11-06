package packet

// OSPF control-message types. In Phase 2 the in-process emulator passes these as
// Go values over channels (ADR-0002), while the LSA payloads they carry use the
// byte codec in lsa.go. The UDP-overlay and raw-IP transports serialise these
// same structures; that serialisation lands with the overlay/raw transports.

// Packet type codes (RFC 2328 §A.2).
const (
	TypeHello uint8 = 1
	TypeDD    uint8 = 2
	TypeLSR   uint8 = 3
	TypeLSU   uint8 = 4
	TypeLSAck uint8 = 5
)

// Hello discovers neighbors and maintains adjacencies (RFC 2328 §A.3.2).
type Hello struct {
	HelloInterval uint16
	DeadInterval  uint32
	Neighbors     []uint32 // Router IDs this router has heard Hellos from
}

// DD flags (RFC 2328 §A.3.3).
const (
	DDFlagMoreBit   uint8 = 0x01 // M
	DDFlagInitBit   uint8 = 0x02 // I
	DDFlagMasterBit uint8 = 0x04 // MS
)

// DatabaseDescription summarises the sender's LSDB during the Exchange state.
type DatabaseDescription struct {
	Flags   uint8
	DDSeq   uint32
	Headers []LSAHeader
}

// LinkStateRequest asks a neighbor for full copies of specific LSAs.
type LinkStateRequest struct {
	Reqs []Key
}

// LinkStateUpdate carries full Router LSAs (the only LSA kind in Phase 2).
type LinkStateUpdate struct {
	Routers []*RouterLSA
}

// LinkStateAck acknowledges received LSAs by header.
type LinkStateAck struct {
	Headers []LSAHeader
}

// SemanticUpdate carries experimental semantic Opaque LSAs (capability, trust,
// telemetry, policy, summary) as raw encoded blobs. It is deliberately a
// *separate* message type from the core LSU so a router can process it at lower
// priority and under a budget, keeping semantic churn off the critical path of
// IP routing (ADR-0005).
type SemanticUpdate struct {
	Blobs [][]byte
}
