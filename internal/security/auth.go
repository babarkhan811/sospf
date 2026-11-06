// Package security implements S-OSPF's cryptographic protections (brief §7,
// WIRE_FORMAT §8): per-packet HMAC-SHA-256 authentication with replay
// protection, per-LSA Ed25519 signatures (Trust Anchoring), rate limiting
// against enumeration/DoS, and a Sybil guard.
package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"sync"
)

// AuthTrailer is the per-packet cryptographic authentication trailer
// (RFC 5709-style; WIRE_FORMAT §8.1).
type AuthTrailer struct {
	KeyID  uint16
	SeqNum uint64 // monotonic, for replay protection
	Digest [32]byte
}

// ComputeTrailer returns the trailer for a packet under key. The HMAC covers the
// packet bytes plus the KeyID and SeqNum, so those fields cannot be altered
// without detection.
func ComputeTrailer(packet, key []byte, keyID uint16, seq uint64) AuthTrailer {
	mac := hmac.New(sha256.New, key)
	mac.Write(packet)
	var meta [10]byte
	binary.BigEndian.PutUint16(meta[0:], keyID)
	binary.BigEndian.PutUint64(meta[2:], seq)
	mac.Write(meta[:])
	var t AuthTrailer
	t.KeyID = keyID
	t.SeqNum = seq
	copy(t.Digest[:], mac.Sum(nil))
	return t
}

// Verify reports whether t is a valid trailer for packet under key (constant
// time).
func (t AuthTrailer) Verify(packet, key []byte) bool {
	want := ComputeTrailer(packet, key, t.KeyID, t.SeqNum)
	return hmac.Equal(want.Digest[:], t.Digest[:])
}

// Encode serialises the trailer (KeyID, reserved, SeqNum, Digest).
func (t AuthTrailer) Encode() []byte {
	b := make([]byte, 2+2+8+32)
	binary.BigEndian.PutUint16(b[0:], t.KeyID)
	binary.BigEndian.PutUint16(b[2:], 32) // Auth Data Length
	binary.BigEndian.PutUint64(b[4:], t.SeqNum)
	copy(b[12:], t.Digest[:])
	return b
}

// ReplayWindow enforces monotonically increasing sequence numbers per neighbor
// (WIRE_FORMAT §8.1; draft-cui §12.7 replay).
type ReplayWindow struct {
	mu   sync.Mutex
	last map[uint32]uint64
}

// NewReplayWindow creates an empty window.
func NewReplayWindow() *ReplayWindow { return &ReplayWindow{last: make(map[uint32]uint64)} }

// Accept records seq for neighbor and reports whether it is fresh (strictly
// greater than the last accepted). Replays and reorderings are rejected.
func (w *ReplayWindow) Accept(neighbor uint32, seq uint64) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if seq <= w.last[neighbor] {
		return false
	}
	w.last[neighbor] = seq
	return true
}
