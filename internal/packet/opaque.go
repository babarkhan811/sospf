package packet

import (
	"encoding/binary"
	"fmt"
)

// Opaque LSA scope (LS type) and S-OSPF experimental opaque-type allocations
// (WIRE_FORMAT §2). Semantic payloads ride in Opaque LSAs so OSPF's flooding,
// aging and LSDB machinery is reused unchanged.
const (
	OpaqueArea uint8 = 10 // area-scoped
	OpaqueAS   uint8 = 11 // AS-scoped

	OpaqueAgentCapability uint8 = 128
	OpaqueTrust           uint8 = 129
	OpaqueTelemetry       uint8 = 130
	OpaquePolicy          uint8 = 131
	OpaquePrefixSummary   uint8 = 132
)

// MakeLinkStateID packs an opaque type (8 bits) and opaque ID (24 bits) into the
// Link State ID field (WIRE_FORMAT §2.1).
func MakeLinkStateID(opaqueType uint8, opaqueID uint32) uint32 {
	return uint32(opaqueType)<<24 | (opaqueID & 0x00FFFFFF)
}

// SplitLinkStateID reverses MakeLinkStateID.
func SplitLinkStateID(id uint32) (opaqueType uint8, opaqueID uint32) {
	return uint8(id >> 24), id & 0x00FFFFFF
}

// EncodeOpaque builds an Opaque LSA (20-octet header + body), filling Length and
// the checksum. scope is OpaqueArea or OpaqueAS.
func EncodeOpaque(scope, opaqueType uint8, opaqueID uint32, advRouter uint32, seq int32, age uint16, body []byte) []byte {
	total := LSAHeaderLen + len(body)
	b := make([]byte, total)
	h := LSAHeader{
		Age:       age,
		Options:   0,
		Type:      scope,
		ID:        MakeLinkStateID(opaqueType, opaqueID),
		AdvRouter: advRouter,
		SeqNum:    seq,
		Length:    uint16(total),
	}
	h.EncodeHeaderInto(b)
	copy(b[LSAHeaderLen:], body)
	cs := fletcher16(b)
	binary.BigEndian.PutUint16(b[16:], cs)
	return b
}

// DecodeOpaque parses an Opaque LSA, returning the header and the body (the TLV
// area).
func DecodeOpaque(b []byte) (LSAHeader, []byte, error) {
	h, err := DecodeHeader(b)
	if err != nil {
		return LSAHeader{}, nil, err
	}
	if h.Type != OpaqueArea && h.Type != OpaqueAS {
		return LSAHeader{}, nil, fmt.Errorf("packet: LS type %d is not an Opaque LSA", h.Type)
	}
	if int(h.Length) > len(b) || int(h.Length) < LSAHeaderLen {
		// Fall back to the whole buffer if Length is implausible, but never slice
		// past the end.
		return h, b[LSAHeaderLen:], nil
	}
	body := make([]byte, int(h.Length)-LSAHeaderLen)
	copy(body, b[LSAHeaderLen:h.Length])
	return h, body, nil
}
