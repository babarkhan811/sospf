package semantic

import (
	"testing"
)

func TestPrefixParseRoundTripCoverAggregate(t *testing.T) {
	p, err := ParsePrefix("medical/imaging/segmentation/agent-7f3a")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.String() != "medical/imaging/segmentation/agent-7f3a" {
		t.Fatalf("string round trip: %q", p.String())
	}
	dec, err := DecodePrefix(p.Encode())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dec.String() != p.String() || dec.Significant != p.Significant {
		t.Fatalf("encode/decode mismatch: %+v vs %+v", dec, p)
	}

	short, _ := ParsePrefix("medical/imaging")
	if !short.Covers(p) {
		t.Error("medical/imaging should cover medical/imaging/segmentation/...")
	}
	if p.Covers(short) {
		t.Error("a longer prefix must not cover a shorter one")
	}

	a, _ := ParsePrefix("medical/imaging/seg")
	b, _ := ParsePrefix("medical/imaging/detect")
	agg := Aggregate([]SemanticPrefix{a, b})
	if agg.String() != "medical/imaging" {
		t.Errorf("aggregate: got %q want medical/imaging", agg.String())
	}
}

func TestPrefixRejectsBadInput(t *testing.T) {
	for _, s := range []string{"", "///", "Bad Label", "a/" + string(make([]byte, 100))} {
		if _, err := ParsePrefix(s); err == nil {
			t.Errorf("expected error for %q", s)
		}
	}
}

func TestBloomNoFalseNegatives(t *testing.T) {
	b := SizedBloom(4, 42)
	members := []string{"ont:classify", "ont:segment", "ont:detect", "ont:ocr"}
	for _, m := range members {
		b.Add(m)
	}
	for _, m := range members {
		if !b.Test(m) {
			t.Errorf("false negative for %q (must never happen)", m)
		}
	}
	dec, err := DecodeBloom(b.Encode())
	if err != nil {
		t.Fatalf("decode bloom: %v", err)
	}
	for _, m := range members {
		if !dec.Test(m) {
			t.Errorf("decoded bloom lost member %q", m)
		}
	}
}

func sampleCap() *CapabilityLSA {
	var id AgentID
	copy(id[:], []byte("agent-0000000001"))
	p, _ := ParsePrefix("medical/imaging/segmentation/agent-1")
	return &CapabilityLSA{
		AdvRouter:    0x01010101,
		OpaqueID:     1,
		Seq:          -0x7FFFFFFF,
		AgentID:      id,
		Prefix:       p,
		Domain:       "medical",
		Status:       StatusActive,
		Disclosure:   DisclosureDomainInternal,
		Capabilities: []string{"ont:image-segmentation", "ont:classify"},
		DigestAlg:    1,
		Digest:       []byte{0xDE, 0xAD, 0xBE, 0xEF},
		FetchHint:    "grpc://router/cap/deadbeef",
		Endpoints:    []Endpoint{{Mechanism: MechanismMCP, URI: "mcp://agent-1/tools"}},
	}
}

func TestCapabilityLSARoundTrip(t *testing.T) {
	in := sampleCap()
	out, err := DecodeCapabilityLSA(in.Encode())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.AgentID != in.AgentID || out.Domain != in.Domain || out.Status != in.Status {
		t.Errorf("scalar fields mismatch: %+v", out)
	}
	if out.Prefix.String() != in.Prefix.String() {
		t.Errorf("prefix mismatch: %q vs %q", out.Prefix.String(), in.Prefix.String())
	}
	if len(out.Capabilities) != 2 || out.Capabilities[0] != "ont:image-segmentation" {
		t.Errorf("capabilities mismatch: %v", out.Capabilities)
	}
	if out.FetchHint != in.FetchHint || string(out.Digest) != string(in.Digest) {
		t.Errorf("digest/fetch mismatch: %q %x", out.FetchHint, out.Digest)
	}
	if len(out.Endpoints) != 1 || out.Endpoints[0].URI != "mcp://agent-1/tools" {
		t.Errorf("endpoint mismatch: %v", out.Endpoints)
	}
	if !out.Eligible() {
		t.Error("active agent should be eligible")
	}
}

func TestSummarizeAndMightHave(t *testing.T) {
	c1 := sampleCap()
	c2 := sampleCap()
	p2, _ := ParsePrefix("medical/imaging/detection/agent-2")
	c2.Prefix = p2
	c2.Capabilities = []string{"ont:object-detection"}

	sum := Summarize(0x01010101, 1, -0x7FFFFFFF, []*CapabilityLSA{c1, c2}, 99)
	if sum.Agg.String() != "medical/imaging" {
		t.Errorf("aggregated prefix: got %q want medical/imaging", sum.Agg.String())
	}

	dec, err := DecodePrefixSummaryLSA(sum.Encode())
	if err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	q, _ := ParsePrefix("medical/imaging/segmentation/agent-1")
	if !dec.MightHave(q, "ont:image-segmentation") {
		t.Error("summary should report a present capability as possible")
	}
	if dec.MightHave(q, "ont:speech-to-text") {
		// Possible false positive, but with these inputs it should be absent.
		t.Error("summary reported an absent capability (unexpected false positive)")
	}
}

func FuzzDecodeCapabilityLSA(f *testing.F) {
	f.Add(sampleCap().Encode())
	f.Add(make([]byte, 20))
	f.Fuzz(func(t *testing.T, data []byte) {
		if c, err := DecodeCapabilityLSA(data); err == nil && c != nil {
			_ = c.Encode()
		}
	})
}
