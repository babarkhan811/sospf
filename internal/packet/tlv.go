package packet

import (
	"encoding/binary"
	"errors"
)

// S-OSPF experimental TLV type registry (WIRE_FORMAT §3). All values are in the
// experimental block 0xF000-0xFFFF and are provisional.
const (
	TLVAgentID        uint16 = 0xF001
	TLVSemanticPrefix uint16 = 0xF002
	TLVCapability     uint16 = 0xF003 // Capability Descriptor (ontology IRI ref)
	TLVCapDigest      uint16 = 0xF004 // content digest + fetch hint
	TLVEndpoint       uint16 = 0xF005 // mechanism + URI
	TLVDomain         uint16 = 0xF006
	TLVDisclosure     uint16 = 0xF007
	TLVAgentStatus    uint16 = 0xF008
	TLVTrustScore     uint16 = 0xF010
	TLVEvidenceDigest uint16 = 0xF011
	TLVIssuer         uint16 = 0xF012
	TLVValidity       uint16 = 0xF013
	TLVLinkMetrics    uint16 = 0xF020
	TLVAgentMetrics   uint16 = 0xF021
	TLVAllowedIntents uint16 = 0xF030
	TLVDisclosureLim  uint16 = 0xF031
	TLVComplianceTags uint16 = 0xF032
	TLVAggPrefix      uint16 = 0xF040
	TLVBloom          uint16 = 0xF041
	TLVTrustRange     uint16 = 0xF042
	TLVSignature      uint16 = 0xF0FF
)

// TLV is a type-length-value element. Value excludes 4-byte alignment padding.
type TLV struct {
	Type  uint16
	Value []byte
}

// EncodeTLVs serialises a sequence of TLVs, each padded to a 4-byte boundary.
// The on-wire Length counts the value only, excluding padding (RFC 3630 §2).
func EncodeTLVs(tlvs []TLV) []byte {
	total := 0
	for _, t := range tlvs {
		total += 4 + pad4(len(t.Value))
	}
	b := make([]byte, total)
	off := 0
	for _, t := range tlvs {
		binary.BigEndian.PutUint16(b[off:], t.Type)
		binary.BigEndian.PutUint16(b[off+2:], uint16(len(t.Value)))
		copy(b[off+4:], t.Value)
		off += 4 + pad4(len(t.Value))
	}
	return b
}

var errBadTLV = errors.New("packet: malformed TLV stream")

// DecodeTLVs parses a TLV stream. Unknown types are returned as-is (the caller
// ignores and re-floods them, WIRE_FORMAT §1); truncation is an error. It never
// panics on malformed input.
func DecodeTLVs(b []byte) ([]TLV, error) {
	var out []TLV
	off := 0
	for off < len(b) {
		if off+4 > len(b) {
			return nil, errBadTLV
		}
		typ := binary.BigEndian.Uint16(b[off:])
		ln := int(binary.BigEndian.Uint16(b[off+2:]))
		if off+4+ln > len(b) {
			return nil, errBadTLV
		}
		val := make([]byte, ln)
		copy(val, b[off+4:off+4+ln])
		out = append(out, TLV{Type: typ, Value: val})
		off += 4 + pad4(ln)
	}
	return out, nil
}

// FindTLV returns the first TLV of type t, or nil.
func FindTLV(tlvs []TLV, t uint16) []byte {
	for _, tlv := range tlvs {
		if tlv.Type == t {
			return tlv.Value
		}
	}
	return nil
}

func pad4(n int) int { return (n + 3) &^ 3 }
