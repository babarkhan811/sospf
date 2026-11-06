package semantic

import (
	"errors"

	"github.com/babarkhan811/sospf/internal/packet"
)

// Agent status values (WIRE_FORMAT §5.1, from the IAIP LCR states). Only Active
// agents are eligible for selection.
const (
	StatusActive              uint8 = 0
	StatusPendingVerification uint8 = 1
	StatusSuspended           uint8 = 2
	StatusUnreachable         uint8 = 3
	StatusDeprecated          uint8 = 4
)

// Disclosure levels (WIRE_FORMAT §5.1).
const (
	DisclosurePublic         uint8 = 0
	DisclosureDomainInternal uint8 = 1
	DisclosureTenantInternal uint8 = 2
	DisclosureRestricted     uint8 = 3
)

// Protocol-endpoint mechanisms (WIRE_FORMAT §5.1).
const (
	MechanismMCP  uint8 = 0
	MechanismA2A  uint8 = 1
	MechanismIAIP uint8 = 2
	MechanismHTTP uint8 = 3
)

// AgentID is a 128-bit, certificate-bound agent identifier (WIRE_FORMAT §4).
type AgentID [16]byte

// Endpoint is one protocol endpoint an agent can be reached on.
type Endpoint struct {
	Mechanism uint8
	URI       string
}

// CapabilityLSA is the Agent Capability LSA (Opaque type 128, area-scoped;
// WIRE_FORMAT §5.1). Capability descriptions themselves are not flooded — only a
// content digest and a fetch hint are (ADR-0003).
type CapabilityLSA struct {
	AdvRouter uint32
	OpaqueID  uint32
	Seq       int32
	Age       uint16

	AgentID      AgentID
	Prefix       SemanticPrefix
	Domain       string
	Status       uint8
	Disclosure   uint8
	Capabilities []string // ontology IRI references
	DigestAlg    uint8
	Digest       []byte
	FetchHint    string
	Endpoints    []Endpoint
}

var errBadCapability = errors.New("semantic: malformed Agent Capability LSA")

// Encode serialises the LSA into its Opaque-LSA wire form.
func (c *CapabilityLSA) Encode() []byte {
	tlvs := []packet.TLV{
		{Type: packet.TLVAgentID, Value: c.AgentID[:]},
		{Type: packet.TLVSemanticPrefix, Value: c.Prefix.Encode()},
		{Type: packet.TLVDomain, Value: []byte(c.Domain)},
		{Type: packet.TLVAgentStatus, Value: []byte{c.Status}},
		{Type: packet.TLVDisclosure, Value: []byte{c.Disclosure}},
	}
	for _, cap := range c.Capabilities {
		tlvs = append(tlvs, packet.TLV{Type: packet.TLVCapability, Value: []byte(cap)})
	}
	if len(c.Digest) > 0 || c.FetchHint != "" {
		v := make([]byte, 0, 2+len(c.Digest)+len(c.FetchHint))
		v = append(v, c.DigestAlg, byte(len(c.Digest)))
		v = append(v, c.Digest...)
		v = append(v, []byte(c.FetchHint)...)
		tlvs = append(tlvs, packet.TLV{Type: packet.TLVCapDigest, Value: v})
	}
	for _, e := range c.Endpoints {
		v := append([]byte{e.Mechanism}, []byte(e.URI)...)
		tlvs = append(tlvs, packet.TLV{Type: packet.TLVEndpoint, Value: v})
	}
	body := packet.EncodeTLVs(tlvs)
	return packet.EncodeOpaque(packet.OpaqueArea, packet.OpaqueAgentCapability, c.OpaqueID,
		c.AdvRouter, c.Seq, c.Age, body)
}

// DecodeCapabilityLSA parses an Agent Capability LSA.
func DecodeCapabilityLSA(b []byte) (*CapabilityLSA, error) {
	h, body, err := packet.DecodeOpaque(b)
	if err != nil {
		return nil, err
	}
	ot, oid := packet.SplitLinkStateID(h.ID)
	if ot != packet.OpaqueAgentCapability {
		return nil, errBadCapability
	}
	tlvs, err := packet.DecodeTLVs(body)
	if err != nil {
		return nil, err
	}
	c := &CapabilityLSA{AdvRouter: h.AdvRouter, OpaqueID: oid, Seq: h.SeqNum, Age: h.Age}
	for _, t := range tlvs {
		switch t.Type {
		case packet.TLVAgentID:
			if len(t.Value) == 16 {
				copy(c.AgentID[:], t.Value)
			}
		case packet.TLVSemanticPrefix:
			if p, err := DecodePrefix(t.Value); err == nil {
				c.Prefix = p
			}
		case packet.TLVDomain:
			c.Domain = string(t.Value)
		case packet.TLVAgentStatus:
			if len(t.Value) == 1 {
				c.Status = t.Value[0]
			}
		case packet.TLVDisclosure:
			if len(t.Value) == 1 {
				c.Disclosure = t.Value[0]
			}
		case packet.TLVCapability:
			c.Capabilities = append(c.Capabilities, string(t.Value))
		case packet.TLVCapDigest:
			if len(t.Value) >= 2 {
				c.DigestAlg = t.Value[0]
				dl := int(t.Value[1])
				if 2+dl <= len(t.Value) {
					c.Digest = append([]byte(nil), t.Value[2:2+dl]...)
					c.FetchHint = string(t.Value[2+dl:])
				}
			}
		case packet.TLVEndpoint:
			if len(t.Value) >= 1 {
				c.Endpoints = append(c.Endpoints, Endpoint{Mechanism: t.Value[0], URI: string(t.Value[1:])})
			}
			// Unknown TLVs are ignored (and would still be re-flooded).
		}
	}
	return c, nil
}

// Eligible reports whether the agent may be selected (it is Active).
func (c *CapabilityLSA) Eligible() bool { return c.Status == StatusActive }
