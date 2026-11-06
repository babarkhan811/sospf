package packet

import "testing"

func sampleRouterLSA() *RouterLSA {
	return &RouterLSA{
		Header: LSAHeader{Type: LSARouter, AdvRouter: 0x01010101, SeqNum: InitialSeqNum, Age: 0},
		Links: []RouterLink{
			{Type: LinkPointToPoint, ID: 0x02020202, Metric: 10},
			{Type: LinkPointToPoint, ID: 0x03030303, Metric: 5},
			{Type: LinkStub, ID: 0x0a000000, Metric: 1},
		},
	}
}

func TestRouterLSARoundTrip(t *testing.T) {
	in := sampleRouterLSA()
	b := in.Encode()

	if !VerifyChecksum(b) {
		t.Fatal("checksum did not verify on freshly encoded LSA")
	}
	out, err := DecodeRouterLSA(b)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Header.AdvRouter != in.Header.AdvRouter || out.Header.SeqNum != in.Header.SeqNum {
		t.Fatalf("header mismatch: got %+v", out.Header)
	}
	if out.Header.ID != in.Header.AdvRouter {
		t.Fatalf("Router LSA Link State ID should equal Advertising Router, got %d", out.Header.ID)
	}
	if len(out.Links) != len(in.Links) {
		t.Fatalf("link count: got %d want %d", len(out.Links), len(in.Links))
	}
	for i := range in.Links {
		if out.Links[i] != in.Links[i] {
			t.Errorf("link %d: got %+v want %+v", i, out.Links[i], in.Links[i])
		}
	}
	if int(out.Header.Length) != len(b) {
		t.Errorf("length field %d != encoded size %d", out.Header.Length, len(b))
	}
}

func TestChecksumDetectsCorruption(t *testing.T) {
	b := sampleRouterLSA().Encode()
	b[25] ^= 0xFF // flip a byte inside the link area
	if VerifyChecksum(b) {
		t.Fatal("corruption not detected by checksum")
	}
}

func TestChecksumIgnoresAge(t *testing.T) {
	b := sampleRouterLSA().Encode()
	b[0], b[1] = 0x0E, 0x10 // change Age; must not affect the checksum
	if !VerifyChecksum(b) {
		t.Fatal("changing Age must not invalidate the checksum")
	}
}

func TestDecodeShortBuffers(t *testing.T) {
	if _, err := DecodeHeader(make([]byte, 5)); err == nil {
		t.Error("expected error on short header")
	}
	if _, err := DecodeRouterLSA(make([]byte, 21)); err == nil {
		t.Error("expected error on truncated router LSA")
	}
}

// FuzzDecodeRouterLSA asserts the parser never panics on arbitrary input
// (brief §8: parsers must never panic on malformed input).
func FuzzDecodeRouterLSA(f *testing.F) {
	f.Add(sampleRouterLSA().Encode())
	f.Add([]byte{})
	f.Add(make([]byte, 20))
	f.Fuzz(func(t *testing.T, data []byte) {
		if lsa, err := DecodeRouterLSA(data); err == nil && lsa != nil {
			_ = lsa.Encode() // re-encoding a successfully decoded LSA must also not panic
		}
	})
}
