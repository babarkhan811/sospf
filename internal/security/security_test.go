package security

import (
	"testing"
	"time"
)

func TestHMACAuthAndTamperDetection(t *testing.T) {
	key := []byte("shared-interface-key")
	pkt := []byte("OSPF hello payload ...")
	tr := ComputeTrailer(pkt, key, 1, 100)

	if !tr.Verify(pkt, key) {
		t.Fatal("valid trailer should verify")
	}
	// Tamper with the packet.
	bad := append([]byte(nil), pkt...)
	bad[0] ^= 0xFF
	if tr.Verify(bad, key) {
		t.Error("tampered packet must not verify")
	}
	// Wrong key.
	if tr.Verify(pkt, []byte("other-key")) {
		t.Error("wrong key must not verify")
	}
	// Trailer encode has the expected size (2+2+8+32).
	if len(tr.Encode()) != 44 {
		t.Errorf("trailer size: got %d want 44", len(tr.Encode()))
	}
}

func TestReplayWindowRejectsOldAndEqual(t *testing.T) {
	w := NewReplayWindow()
	if !w.Accept(5, 1) || !w.Accept(5, 2) {
		t.Fatal("increasing sequence numbers should be accepted")
	}
	if w.Accept(5, 2) {
		t.Error("replay of an equal sequence number must be rejected")
	}
	if w.Accept(5, 1) {
		t.Error("an older sequence number must be rejected")
	}
	if !w.Accept(6, 1) {
		t.Error("a different neighbor has its own window")
	}
}

func TestEd25519SignVerifyAndForgery(t *testing.T) {
	kp, err := GenerateKey()
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	// A plausible 24-byte "LSA" (header + a little body).
	lsa := make([]byte, 24)
	for i := range lsa {
		lsa[i] = byte(i)
	}
	var keyID [16]byte
	signed := SignLSA(kp.Priv, keyID, lsa)
	if !signed.Verify(kp.Pub) {
		t.Fatal("genuine signature should verify")
	}

	// Forge: change a covered byte (offset >= 3, not in checksum).
	forged := signed
	forged.LSA = append([]byte(nil), signed.LSA...)
	forged.LSA[8] ^= 0xFF
	if forged.Verify(kp.Pub) {
		t.Error("signature over tampered LSA must fail")
	}

	// A different key must not verify.
	other, _ := GenerateKey()
	if signed.Verify(other.Pub) {
		t.Error("another router's key must not verify this signature")
	}

	// Mutating LS age (offsets 0..1) must NOT break the signature.
	ageChanged := signed
	ageChanged.LSA = append([]byte(nil), signed.LSA...)
	ageChanged.LSA[0] ^= 0xFF
	if !ageChanged.Verify(kp.Pub) {
		t.Error("changing LS age must not invalidate the signature (it is excluded from coverage)")
	}
}

func TestTokenBucketLimits(t *testing.T) {
	now := time.Unix(0, 0)
	b := NewTokenBucket(3, 1) // capacity 3, 1 token/sec
	b.now = func() time.Time { return now }
	if !b.Allow() || !b.Allow() || !b.Allow() {
		t.Fatal("first 3 requests should pass")
	}
	if b.Allow() {
		t.Error("4th request should be limited")
	}
	now = now.Add(2 * time.Second) // refill 2 tokens
	if !b.Allow() || !b.Allow() {
		t.Error("should allow 2 after refill")
	}
	if b.Allow() {
		t.Error("should be empty again")
	}
}

func TestSybilGuardCapsPerRouter(t *testing.T) {
	g := NewSybilGuard(2)
	if !g.Admit(1) || !g.Admit(1) {
		t.Fatal("first 2 agents from router 1 should be admitted")
	}
	if g.Admit(1) {
		t.Error("3rd agent from router 1 should be rejected")
	}
	if !g.Admit(2) {
		t.Error("router 2 has its own budget")
	}
	g.Release(1)
	if !g.Admit(1) {
		t.Error("after release, router 1 can admit again")
	}
}
