package semantic

import (
	"encoding/binary"
	"errors"
	"hash/fnv"
)

// Bloom is a classic Bloom filter used in Semantic Prefix Summary LSAs so a
// remote area can test "might an agent with capability X exist under this
// aggregated prefix?" without flooding every agent (WIRE_FORMAT §5.5,
// ADR-0003). False positives are tolerated; false negatives are impossible.
type Bloom struct {
	M    uint32 // number of bits
	K    uint8  // number of hash functions
	Salt uint32
	bits []byte
}

// NewBloom creates a filter with m bits (rounded up to a byte) and k hashes.
func NewBloom(m uint32, k uint8, salt uint32) *Bloom {
	if m < 8 {
		m = 8
	}
	if k == 0 {
		k = 1
	}
	return &Bloom{M: m, K: k, Salt: salt, bits: make([]byte, (m+7)/8)}
}

// SizedBloom picks parameters for n expected elements at roughly a 1% false
// positive rate: m ≈ n·10 bits (rounded up to a power of two, min 64), k = 4.
func SizedBloom(n int, salt uint32) *Bloom {
	bits := uint32(64)
	want := uint32(n) * 10
	for bits < want {
		bits <<= 1
	}
	return NewBloom(bits, 4, salt)
}

func (b *Bloom) index(s string, i uint8) uint32 {
	h := fnv.New32a()
	var pre [8]byte
	binary.BigEndian.PutUint32(pre[0:], b.Salt)
	binary.BigEndian.PutUint32(pre[4:], uint32(i))
	_, _ = h.Write(pre[:])
	_, _ = h.Write([]byte(s))
	return h.Sum32() % b.M
}

// Add inserts s.
func (b *Bloom) Add(s string) {
	for i := uint8(0); i < b.K; i++ {
		idx := b.index(s, i)
		b.bits[idx/8] |= 1 << (idx % 8)
	}
}

// Test reports whether s may be present (true) or is definitely absent (false).
func (b *Bloom) Test(s string) bool {
	for i := uint8(0); i < b.K; i++ {
		idx := b.index(s, i)
		if b.bits[idx/8]&(1<<(idx%8)) == 0 {
			return false
		}
	}
	return true
}

// Encode serialises the filter: M(4) K(1) Salt(4) then ceil(M/8) bit octets.
func (b *Bloom) Encode() []byte {
	out := make([]byte, 9+len(b.bits))
	binary.BigEndian.PutUint32(out[0:], b.M)
	out[4] = b.K
	binary.BigEndian.PutUint32(out[5:], b.Salt)
	copy(out[9:], b.bits)
	return out
}

var errBadBloom = errors.New("semantic: malformed bloom filter")

// DecodeBloom parses a filter encoded by Encode.
func DecodeBloom(data []byte) (*Bloom, error) {
	if len(data) < 9 {
		return nil, errBadBloom
	}
	m := binary.BigEndian.Uint32(data[0:])
	k := data[4]
	salt := binary.BigEndian.Uint32(data[5:])
	nbytes := int((m + 7) / 8)
	if len(data) < 9+nbytes {
		return nil, errBadBloom
	}
	b := &Bloom{M: m, K: k, Salt: salt, bits: make([]byte, nbytes)}
	copy(b.bits, data[9:9+nbytes])
	return b, nil
}
