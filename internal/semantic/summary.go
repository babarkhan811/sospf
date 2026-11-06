package semantic

import (
	"encoding/binary"
	"errors"

	"github.com/babarkhan811/sospf/internal/packet"
)

// PrefixSummaryLSA is the Semantic Prefix Summary LSA (Opaque type 132,
// AS-scoped; WIRE_FORMAT §5.5). An ABR originates one to aggregate the semantic
// prefixes of an area's agents, plus a capability Bloom filter and a trust
// range, so other areas can test for capabilities without full flooding.
type PrefixSummaryLSA struct {
	AdvRouter uint32
	OpaqueID  uint32
	Seq       int32
	Age       uint16

	Agg      SemanticPrefix
	Bloom    *Bloom
	TrustMin uint16 // 16-bit fixed-point [0,1]
	TrustMax uint16
}

var errBadSummary = errors.New("semantic: malformed Prefix Summary LSA")

// Summarize aggregates a set of Agent Capability LSAs into a Prefix Summary.
// Trust range defaults to the full [0,1]; the Trust Manager narrows it in
// Phase 5 once Trust LSAs are integrated.
func Summarize(advRouter, opaqueID uint32, seq int32, caps []*CapabilityLSA, salt uint32) *PrefixSummaryLSA {
	prefixes := make([]SemanticPrefix, 0, len(caps))
	totalCaps := 0
	for _, c := range caps {
		prefixes = append(prefixes, c.Prefix)
		totalCaps += len(c.Capabilities)
	}
	bloom := SizedBloom(totalCaps, salt)
	for _, c := range caps {
		for _, cap := range c.Capabilities {
			bloom.Add(cap)
		}
	}
	return &PrefixSummaryLSA{
		AdvRouter: advRouter,
		OpaqueID:  opaqueID,
		Seq:       seq,
		Agg:       Aggregate(prefixes),
		Bloom:     bloom,
		TrustMin:  0,
		TrustMax:  0xFFFF,
	}
}

// MightHave reports whether an agent under this summary might advertise the
// given capability IRI (Bloom membership) and whether the prefix is covered.
func (s *PrefixSummaryLSA) MightHave(prefix SemanticPrefix, capabilityIRI string) bool {
	if s.Bloom == nil {
		return false
	}
	return s.Agg.Covers(prefix) && s.Bloom.Test(capabilityIRI)
}

// Encode serialises the summary into its Opaque-LSA wire form.
func (s *PrefixSummaryLSA) Encode() []byte {
	tr := make([]byte, 4)
	binary.BigEndian.PutUint16(tr[0:], s.TrustMin)
	binary.BigEndian.PutUint16(tr[2:], s.TrustMax)
	tlvs := []packet.TLV{
		{Type: packet.TLVAggPrefix, Value: s.Agg.Encode()},
		{Type: packet.TLVTrustRange, Value: tr},
	}
	if s.Bloom != nil {
		tlvs = append(tlvs, packet.TLV{Type: packet.TLVBloom, Value: s.Bloom.Encode()})
	}
	body := packet.EncodeTLVs(tlvs)
	return packet.EncodeOpaque(packet.OpaqueAS, packet.OpaquePrefixSummary, s.OpaqueID,
		s.AdvRouter, s.Seq, s.Age, body)
}

// DecodePrefixSummaryLSA parses a Prefix Summary LSA.
func DecodePrefixSummaryLSA(b []byte) (*PrefixSummaryLSA, error) {
	h, body, err := packet.DecodeOpaque(b)
	if err != nil {
		return nil, err
	}
	ot, oid := packet.SplitLinkStateID(h.ID)
	if ot != packet.OpaquePrefixSummary {
		return nil, errBadSummary
	}
	tlvs, err := packet.DecodeTLVs(body)
	if err != nil {
		return nil, err
	}
	s := &PrefixSummaryLSA{AdvRouter: h.AdvRouter, OpaqueID: oid, Seq: h.SeqNum, Age: h.Age}
	for _, t := range tlvs {
		switch t.Type {
		case packet.TLVAggPrefix:
			if p, err := DecodePrefix(t.Value); err == nil {
				s.Agg = p
			}
		case packet.TLVTrustRange:
			if len(t.Value) == 4 {
				s.TrustMin = binary.BigEndian.Uint16(t.Value[0:])
				s.TrustMax = binary.BigEndian.Uint16(t.Value[2:])
			}
		case packet.TLVBloom:
			if bl, err := DecodeBloom(t.Value); err == nil {
				s.Bloom = bl
			}
		}
	}
	return s, nil
}
