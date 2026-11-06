package packet

import (
	"bytes"
	"testing"
)

func TestTLVRoundTripAndPadding(t *testing.T) {
	in := []TLV{
		{Type: TLVAgentID, Value: bytes.Repeat([]byte{0xAB}, 16)},
		{Type: TLVDomain, Value: []byte("medical")}, // 7 bytes -> padded to 8
		{Type: TLVAgentStatus, Value: []byte{0x00}}, // 1 byte -> padded to 4
	}
	b := EncodeTLVs(in)
	if len(b)%4 != 0 {
		t.Fatalf("encoded stream not 4-byte aligned: %d", len(b))
	}
	out, err := DecodeTLVs(b)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("got %d TLVs want %d", len(out), len(in))
	}
	for i := range in {
		if out[i].Type != in[i].Type || !bytes.Equal(out[i].Value, in[i].Value) {
			t.Errorf("tlv %d: got %+v want %+v", i, out[i], in[i])
		}
	}
}

func TestDecodeTLVsRejectsTruncation(t *testing.T) {
	if _, err := DecodeTLVs([]byte{0xF0, 0x01, 0x00, 0x10, 0x01}); err == nil {
		t.Error("expected truncation error (claims 16 bytes, has 1)")
	}
}

func TestOpaqueRoundTrip(t *testing.T) {
	body := EncodeTLVs([]TLV{{Type: TLVDomain, Value: []byte("imaging")}})
	b := EncodeOpaque(OpaqueArea, OpaqueAgentCapability, 7, 0x01010101, InitialSeqNum, 0, body)
	if !VerifyChecksum(b) {
		t.Fatal("opaque checksum did not verify")
	}
	h, gotBody, err := DecodeOpaque(b)
	if err != nil {
		t.Fatalf("decode opaque: %v", err)
	}
	ot, oid := SplitLinkStateID(h.ID)
	if ot != OpaqueAgentCapability || oid != 7 {
		t.Errorf("opaque type/id: got %d/%d want 128/7", ot, oid)
	}
	if !bytes.Equal(gotBody, body) {
		t.Error("opaque body round-trip mismatch")
	}
}

func FuzzDecodeTLVs(f *testing.F) {
	f.Add(EncodeTLVs([]TLV{{Type: TLVDomain, Value: []byte("x")}}))
	f.Add([]byte{})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeTLVs(data) // must not panic
	})
}

func FuzzDecodeOpaque(f *testing.F) {
	f.Add(EncodeOpaque(OpaqueArea, OpaqueTrust, 1, 2, InitialSeqNum, 0, []byte{1, 2, 3, 4}))
	f.Add(make([]byte, 20))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _, _ = DecodeOpaque(data) // must not panic
	})
}
