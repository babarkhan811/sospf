// Package packet implements the S-OSPF wire encoding for link-state
// advertisements and the classic OSPF control packets.
//
// Phase 2 scope (brief §7.2): the classic core over point-to-point links. Only
// Router LSAs (LS type 1) are encoded here; Network LSAs (type 2, needed for
// broadcast segments with a Designated Router) and the experimental Opaque
// semantic LSAs (WIRE_FORMAT §5) arrive in later phases. The encoding of the
// 20-octet LSA header follows RFC 2328 §A.4.1 so a semantics-disabled capture is
// decodable by the standard OSPF dissector (brief §8).
package packet

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// LS type values (RFC 2328 §A.4.1). Phase 2 uses LSARouter only.
const (
	LSARouter  uint8 = 1
	LSANetwork uint8 = 2
)

// Router-LSA link types (RFC 2328 §A.4.2).
const (
	LinkPointToPoint uint8 = 1 // to another router; ID = neighbor Router ID
	LinkTransit      uint8 = 2 // to a transit network (DR); not used in Phase 2
	LinkStub         uint8 = 3 // to a stub network/prefix; ID = network number
)

// Sequence-number and age constants (RFC 2328 §12.1.6, §13.1).
const (
	InitialSeqNum int32  = int32(-0x7FFFFFFF) // 0x80000001
	MaxSeqNum     int32  = 0x7FFFFFFF
	MaxAge        uint16 = 3600 // seconds
	LSRefreshTime uint16 = 1800 // seconds
	MaxAgeDiff    uint16 = 900  // seconds (RFC 2328 §13.1)
)

// LSAHeaderLen is the fixed size of the LSA header in octets.
const LSAHeaderLen = 20

// LSAHeader is the 20-octet common header carried by every LSA.
type LSAHeader struct {
	Age       uint16
	Options   uint8
	Type      uint8
	ID        uint32 // Link State ID
	AdvRouter uint32 // Advertising Router
	SeqNum    int32
	Checksum  uint16
	Length    uint16 // total LSA length including this header
}

// Key uniquely identifies an LSA in the database (RFC 2328 §12.1).
type Key struct {
	Type      uint8
	ID        uint32
	AdvRouter uint32
}

// Key returns the database key for this header.
func (h LSAHeader) Key() Key { return Key{Type: h.Type, ID: h.ID, AdvRouter: h.AdvRouter} }

// RouterLink is one link entry inside a Router LSA (12 octets on the wire).
type RouterLink struct {
	ID     uint32 // meaning depends on Type (neighbor Router ID or network number)
	Data   uint32 // interface address / sub-mask, unused in the emulator
	Type   uint8
	Metric uint16
}

// RouterLSA is an LS-type-1 advertisement: the links out of one router.
type RouterLSA struct {
	Header LSAHeader
	Flags  uint8 // V/E/B bits (RFC 2328 §A.4.2); 0 in Phase 2
	Links  []RouterLink
}

var errShort = errors.New("packet: buffer too short")

// ---- LSA header ----

// EncodeHeaderInto writes the 20-octet header at the start of b (len(b) >= 20).
func (h LSAHeader) EncodeHeaderInto(b []byte) {
	binary.BigEndian.PutUint16(b[0:], h.Age)
	b[2] = h.Options
	b[3] = h.Type
	binary.BigEndian.PutUint32(b[4:], h.ID)
	binary.BigEndian.PutUint32(b[8:], h.AdvRouter)
	binary.BigEndian.PutUint32(b[12:], uint32(h.SeqNum))
	binary.BigEndian.PutUint16(b[16:], h.Checksum)
	binary.BigEndian.PutUint16(b[18:], h.Length)
}

// DecodeHeader parses a 20-octet LSA header.
func DecodeHeader(b []byte) (LSAHeader, error) {
	if len(b) < LSAHeaderLen {
		return LSAHeader{}, errShort
	}
	return LSAHeader{
		Age:       binary.BigEndian.Uint16(b[0:]),
		Options:   b[2],
		Type:      b[3],
		ID:        binary.BigEndian.Uint32(b[4:]),
		AdvRouter: binary.BigEndian.Uint32(b[8:]),
		SeqNum:    int32(binary.BigEndian.Uint32(b[12:])),
		Checksum:  binary.BigEndian.Uint16(b[16:]),
		Length:    binary.BigEndian.Uint16(b[18:]),
	}, nil
}

// ---- Router LSA ----

// Encode serialises a Router LSA, filling in Length and Checksum.
func (r *RouterLSA) Encode() []byte {
	n := len(r.Links)
	total := LSAHeaderLen + 4 + 12*n
	b := make([]byte, total)

	r.Header.Type = LSARouter
	r.Header.Length = uint16(total)
	r.Header.ID = r.Header.AdvRouter // for Router LSAs, Link State ID == Advertising Router
	r.Header.Checksum = 0
	r.Header.EncodeHeaderInto(b)

	b[20] = r.Flags
	b[21] = 0 // reserved
	binary.BigEndian.PutUint16(b[22:], uint16(n))
	off := 24
	for _, l := range r.Links {
		binary.BigEndian.PutUint32(b[off:], l.ID)
		binary.BigEndian.PutUint32(b[off+4:], l.Data)
		b[off+8] = l.Type
		b[off+9] = 0 // # of TOS metrics
		binary.BigEndian.PutUint16(b[off+10:], l.Metric)
		off += 12
	}

	cs := fletcher16(b)
	binary.BigEndian.PutUint16(b[16:], cs)
	r.Header.Checksum = cs
	return b
}

// DecodeRouterLSA parses a Router LSA (header + body).
func DecodeRouterLSA(b []byte) (*RouterLSA, error) {
	h, err := DecodeHeader(b)
	if err != nil {
		return nil, err
	}
	if h.Type != LSARouter {
		return nil, fmt.Errorf("packet: not a Router LSA (type %d)", h.Type)
	}
	if len(b) < 24 {
		return nil, errShort
	}
	n := int(binary.BigEndian.Uint16(b[22:]))
	if len(b) < 24+12*n {
		return nil, errShort
	}
	r := &RouterLSA{Header: h, Flags: b[20]}
	off := 24
	for i := 0; i < n; i++ {
		r.Links = append(r.Links, RouterLink{
			ID:     binary.BigEndian.Uint32(b[off:]),
			Data:   binary.BigEndian.Uint32(b[off+4:]),
			Type:   b[off+8],
			Metric: binary.BigEndian.Uint16(b[off+10:]),
		})
		off += 12
	}
	return r, nil
}

// VerifyChecksum recomputes the LSA checksum over b and compares it to the
// stored value.
func VerifyChecksum(b []byte) bool {
	if len(b) < LSAHeaderLen {
		return false
	}
	stored := binary.BigEndian.Uint16(b[16:])
	tmp := make([]byte, len(b))
	copy(tmp, b)
	binary.BigEndian.PutUint16(tmp[16:], 0)
	return fletcher16(tmp) == stored
}

// fletcher16 computes a Fletcher-16 checksum over the LSA from the Options field
// to the end, with the checksum field treated as zero.
//
// NOTE: this is a self-consistent Fletcher-16, not the exact RFC 2328 §12.1.7
// checksum-byte placement. It is sufficient for the emulator (round-trip and
// corruption detection) in Phases 2–7; the RFC-exact placement is implemented
// with the raw-socket interop path (brief §8) when Wireshark cross-checking
// begins.
func fletcher16(full []byte) uint16 {
	var c0, c1 uint32
	// Skip Age (bytes 0..1), which mutates in flight and is excluded from the
	// checksum just as in OSPF.
	for i := 2; i < len(full); i++ {
		if i == 16 || i == 17 {
			continue // checksum field itself is treated as zero
		}
		c0 = (c0 + uint32(full[i])) % 255
		c1 = (c1 + c0) % 255
	}
	return uint16(c1<<8 | c0)
}
