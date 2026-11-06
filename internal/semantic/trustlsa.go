package semantic

import (
	"encoding/binary"
	"errors"

	"github.com/babarkhan811/sospf/internal/packet"
)

var errBadTrust = errors.New("semantic: malformed Trust LSA")

// Trust scope (WIRE_FORMAT §5.2).
const (
	TrustScopeOverall    uint8 = 0
	TrustScopePerDomain  uint8 = 1
	TrustScopePerCapabil uint8 = 2
)

// ScoreFromFloat / FloatFromScore convert between a [0,1] trust value and the
// 16-bit fixed-point wire form.
func ScoreFromFloat(f float64) uint16 {
	if f <= 0 {
		return 0
	}
	if f >= 1 {
		return 0xFFFF
	}
	return uint16(f * 0xFFFF)
}
func FloatFromScore(u uint16) float64 { return float64(u) / 0xFFFF }

// TrustLSA is the Trust Manager's signed attestation of an agent's trust score
// (Opaque type 129, area-scoped; WIRE_FORMAT §5.2). The Ed25519 signature TLV is
// added in Phase 7.
type TrustLSA struct {
	AdvRouter uint32
	OpaqueID  uint32
	Seq       int32
	Age       uint16

	Subject        AgentID
	Score          uint16
	Scope          uint8
	EvidenceAlg    uint8
	EvidenceDigest []byte
	Issuer         [16]byte
	NotBefore      uint32
	NotAfter       uint32
}

// Encode serialises the Trust LSA.
func (t *TrustLSA) Encode() []byte {
	score := make([]byte, 3)
	binary.BigEndian.PutUint16(score[0:], t.Score)
	score[2] = t.Scope
	ev := append([]byte{t.EvidenceAlg}, t.EvidenceDigest...)
	valid := make([]byte, 8)
	binary.BigEndian.PutUint32(valid[0:], t.NotBefore)
	binary.BigEndian.PutUint32(valid[4:], t.NotAfter)
	tlvs := []packet.TLV{
		{Type: packet.TLVAgentID, Value: t.Subject[:]},
		{Type: packet.TLVTrustScore, Value: score},
		{Type: packet.TLVEvidenceDigest, Value: ev},
		{Type: packet.TLVIssuer, Value: t.Issuer[:]},
		{Type: packet.TLVValidity, Value: valid},
	}
	body := packet.EncodeTLVs(tlvs)
	return packet.EncodeOpaque(packet.OpaqueArea, packet.OpaqueTrust, t.OpaqueID,
		t.AdvRouter, t.Seq, t.Age, body)
}

// DecodeTrustLSA parses a Trust LSA.
func DecodeTrustLSA(b []byte) (*TrustLSA, error) {
	h, body, err := packet.DecodeOpaque(b)
	if err != nil {
		return nil, err
	}
	ot, oid := packet.SplitLinkStateID(h.ID)
	if ot != packet.OpaqueTrust {
		return nil, errBadTrust
	}
	tlvs, err := packet.DecodeTLVs(body)
	if err != nil {
		return nil, err
	}
	t := &TrustLSA{AdvRouter: h.AdvRouter, OpaqueID: oid, Seq: h.SeqNum, Age: h.Age}
	for _, tlv := range tlvs {
		switch tlv.Type {
		case packet.TLVAgentID:
			if len(tlv.Value) == 16 {
				copy(t.Subject[:], tlv.Value)
			}
		case packet.TLVTrustScore:
			if len(tlv.Value) >= 3 {
				t.Score = binary.BigEndian.Uint16(tlv.Value[0:])
				t.Scope = tlv.Value[2]
			}
		case packet.TLVEvidenceDigest:
			if len(tlv.Value) >= 1 {
				t.EvidenceAlg = tlv.Value[0]
				t.EvidenceDigest = append([]byte(nil), tlv.Value[1:]...)
			}
		case packet.TLVIssuer:
			if len(tlv.Value) == 16 {
				copy(t.Issuer[:], tlv.Value)
			}
		case packet.TLVValidity:
			if len(tlv.Value) == 8 {
				t.NotBefore = binary.BigEndian.Uint32(tlv.Value[0:])
				t.NotAfter = binary.BigEndian.Uint32(tlv.Value[4:])
			}
		}
	}
	return t, nil
}
