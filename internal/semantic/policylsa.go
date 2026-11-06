package semantic

import (
	"errors"

	"github.com/babarkhan811/sospf/internal/packet"
)

var errBadPolicy = errors.New("semantic: malformed Policy LSA")

// PolicyLSA carries a domain's policy summary (Opaque type 131, AS-scoped;
// WIRE_FORMAT §5.4; realizes draft-yang Policy Constraints §8.4).
type PolicyLSA struct {
	AdvRouter uint32
	OpaqueID  uint32
	Seq       int32
	Age       uint16

	Domain          string
	AllowedIntents  []string
	DisclosureLimit uint8
	ComplianceTags  []string
}

// Encode serialises the Policy LSA.
func (p *PolicyLSA) Encode() []byte {
	tlvs := []packet.TLV{
		{Type: packet.TLVDomain, Value: []byte(p.Domain)},
		{Type: packet.TLVDisclosureLim, Value: []byte{p.DisclosureLimit}},
	}
	for _, in := range p.AllowedIntents {
		tlvs = append(tlvs, packet.TLV{Type: packet.TLVAllowedIntents, Value: []byte(in)})
	}
	for _, tag := range p.ComplianceTags {
		tlvs = append(tlvs, packet.TLV{Type: packet.TLVComplianceTags, Value: []byte(tag)})
	}
	body := packet.EncodeTLVs(tlvs)
	return packet.EncodeOpaque(packet.OpaqueAS, packet.OpaquePolicy, p.OpaqueID,
		p.AdvRouter, p.Seq, p.Age, body)
}

// DecodePolicyLSA parses a Policy LSA.
func DecodePolicyLSA(b []byte) (*PolicyLSA, error) {
	h, body, err := packet.DecodeOpaque(b)
	if err != nil {
		return nil, err
	}
	ot, oid := packet.SplitLinkStateID(h.ID)
	if ot != packet.OpaquePolicy {
		return nil, errBadPolicy
	}
	tlvs, err := packet.DecodeTLVs(body)
	if err != nil {
		return nil, err
	}
	p := &PolicyLSA{AdvRouter: h.AdvRouter, OpaqueID: oid, Seq: h.SeqNum, Age: h.Age}
	for _, t := range tlvs {
		switch t.Type {
		case packet.TLVDomain:
			p.Domain = string(t.Value)
		case packet.TLVDisclosureLim:
			if len(t.Value) == 1 {
				p.DisclosureLimit = t.Value[0]
			}
		case packet.TLVAllowedIntents:
			p.AllowedIntents = append(p.AllowedIntents, string(t.Value))
		case packet.TLVComplianceTags:
			p.ComplianceTags = append(p.ComplianceTags, string(t.Value))
		}
	}
	return p, nil
}
