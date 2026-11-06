package semantic

import (
	"encoding/binary"
	"errors"

	"github.com/babarkhan811/sospf/internal/packet"
)

var errBadTelemetry = errors.New("semantic: malformed Telemetry LSA")

// TelemetryLSA carries per-link and per-agent performance, modeled on OSPF-TE
// (Opaque type 130, area-scoped; WIRE_FORMAT §5.3).
type TelemetryLSA struct {
	AdvRouter uint32
	OpaqueID  uint32
	Seq       int32
	Age       uint16

	Subject     AgentID // zero if this is link-only telemetry
	HasSubject  bool
	LatencyUs   uint32
	JitterUs    uint32
	SuccessRate uint16 // fixed-point [0,1]
	Load        uint16 // fixed-point [0,1]
	QueueDepth  uint32
	Cost        uint32
	Energy      uint32
}

// Encode serialises the Telemetry LSA.
func (t *TelemetryLSA) Encode() []byte {
	link := make([]byte, 12)
	binary.BigEndian.PutUint32(link[0:], t.LatencyUs)
	binary.BigEndian.PutUint32(link[4:], t.JitterUs)
	binary.BigEndian.PutUint16(link[8:], t.SuccessRate)
	binary.BigEndian.PutUint16(link[10:], t.Load)
	agent := make([]byte, 14)
	binary.BigEndian.PutUint32(agent[0:], t.QueueDepth)
	binary.BigEndian.PutUint32(agent[4:], t.Cost)
	binary.BigEndian.PutUint32(agent[8:], t.Energy)
	binary.BigEndian.PutUint16(agent[12:], t.SuccessRate)

	tlvs := make([]packet.TLV, 0, 3)
	if t.HasSubject {
		tlvs = append(tlvs, packet.TLV{Type: packet.TLVAgentID, Value: t.Subject[:]})
	}
	tlvs = append(tlvs,
		packet.TLV{Type: packet.TLVLinkMetrics, Value: link},
		packet.TLV{Type: packet.TLVAgentMetrics, Value: agent},
	)
	body := packet.EncodeTLVs(tlvs)
	return packet.EncodeOpaque(packet.OpaqueArea, packet.OpaqueTelemetry, t.OpaqueID,
		t.AdvRouter, t.Seq, t.Age, body)
}

// DecodeTelemetryLSA parses a Telemetry LSA.
func DecodeTelemetryLSA(b []byte) (*TelemetryLSA, error) {
	h, body, err := packet.DecodeOpaque(b)
	if err != nil {
		return nil, err
	}
	ot, oid := packet.SplitLinkStateID(h.ID)
	if ot != packet.OpaqueTelemetry {
		return nil, errBadTelemetry
	}
	tlvs, err := packet.DecodeTLVs(body)
	if err != nil {
		return nil, err
	}
	t := &TelemetryLSA{AdvRouter: h.AdvRouter, OpaqueID: oid, Seq: h.SeqNum, Age: h.Age}
	for _, tlv := range tlvs {
		switch tlv.Type {
		case packet.TLVAgentID:
			if len(tlv.Value) == 16 {
				copy(t.Subject[:], tlv.Value)
				t.HasSubject = true
			}
		case packet.TLVLinkMetrics:
			if len(tlv.Value) >= 12 {
				t.LatencyUs = binary.BigEndian.Uint32(tlv.Value[0:])
				t.JitterUs = binary.BigEndian.Uint32(tlv.Value[4:])
				t.SuccessRate = binary.BigEndian.Uint16(tlv.Value[8:])
				t.Load = binary.BigEndian.Uint16(tlv.Value[10:])
			}
		case packet.TLVAgentMetrics:
			if len(tlv.Value) >= 14 {
				t.QueueDepth = binary.BigEndian.Uint32(tlv.Value[0:])
				t.Cost = binary.BigEndian.Uint32(tlv.Value[4:])
				t.Energy = binary.BigEndian.Uint32(tlv.Value[8:])
				if t.SuccessRate == 0 {
					t.SuccessRate = binary.BigEndian.Uint16(tlv.Value[12:])
				}
			}
		}
	}
	return t, nil
}
