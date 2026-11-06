package security

import (
	"crypto/ed25519"
	"crypto/rand"
)

// Per-LSA Ed25519 signatures implement Trust Anchoring (WIRE_FORMAT §8.2): a
// compromised router that can produce valid packet HMACs still cannot forge
// another router's Capability or Trust LSAs.
//
// On the wire the signature is the final TLV (0xF0FF) and covers the LSA from
// the LS type octet to the end of the last preceding TLV, excluding the mutable
// LS age and the LS checksum. In the emulator we keep the signature detached
// (SignedLSA) over the same coverage bytes — functionally equivalent and simpler
// to test; the TLV-appended form lands with the raw transport.

// Coverage returns the bytes an LSA signature covers: from the LS type octet
// (offset 3) to the end, with the LS checksum (offsets 16..17) excluded. The LSA
// passed in must not already contain a signature TLV.
func Coverage(lsa []byte) []byte {
	if len(lsa) < 20 {
		return nil
	}
	out := make([]byte, 0, len(lsa)-3-2)
	out = append(out, lsa[3:16]...) // LS type .. just before checksum
	out = append(out, lsa[18:]...)  // after checksum .. end
	return out
}

// KeyPair is an Ed25519 signing key pair bound to a router/Trust-Manager
// identity.
type KeyPair struct {
	Pub  ed25519.PublicKey
	Priv ed25519.PrivateKey
}

// GenerateKey creates a fresh Ed25519 key pair.
func GenerateKey() (KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return KeyPair{}, err
	}
	return KeyPair{Pub: pub, Priv: priv}, nil
}

// SignedLSA is an LSA plus a detached signature over its coverage bytes.
type SignedLSA struct {
	LSA   []byte
	KeyID [16]byte
	Sig   []byte
}

// SignLSA signs the coverage of lsa with priv.
func SignLSA(priv ed25519.PrivateKey, keyID [16]byte, lsa []byte) SignedLSA {
	return SignedLSA{
		LSA:   lsa,
		KeyID: keyID,
		Sig:   ed25519.Sign(priv, Coverage(lsa)),
	}
}

// Verify reports whether s's signature is valid under pub.
func (s SignedLSA) Verify(pub ed25519.PublicKey) bool {
	if len(s.Sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(pub, Coverage(s.LSA), s.Sig)
}
