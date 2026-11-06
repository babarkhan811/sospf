// Package semantic implements S-OSPF's experimental semantic layer: semantic
// prefixes, capability matching, Bloom summaries, and (later phases) trust and
// policy. Phase 3 covers Semantic Prefixes and ABR summarization (WIRE_FORMAT
// §4, §5.5).
package semantic

import (
	"errors"
	"fmt"
	"strings"
)

// SemanticPrefix is a hierarchical, aggregatable name
// (domain/category/subcategory/agent). Significant is the number of leading
// labels that matter for aggregation and longest-prefix match — the label-wise
// analogue of an IP prefix length (WIRE_FORMAT §4).
type SemanticPrefix struct {
	Labels      []string
	Significant uint8
}

const maxLabelLen = 63

var errBadPrefix = errors.New("semantic: malformed semantic prefix")

// ParsePrefix parses "a/b/c" into a fully-significant SemanticPrefix. Labels are
// restricted to [a-z0-9-] (IDNA-style), 1..63 octets, and are case-folded.
func ParsePrefix(s string) (SemanticPrefix, error) {
	s = strings.Trim(s, "/")
	if s == "" {
		return SemanticPrefix{}, errBadPrefix
	}
	parts := strings.Split(s, "/")
	if len(parts) > 255 {
		return SemanticPrefix{}, errBadPrefix
	}
	labels := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(p)
		if err := validLabel(p); err != nil {
			return SemanticPrefix{}, err
		}
		labels = append(labels, p)
	}
	return SemanticPrefix{Labels: labels, Significant: uint8(len(labels))}, nil
}

func validLabel(p string) error {
	if len(p) == 0 || len(p) > maxLabelLen {
		return fmt.Errorf("%w: label length %d", errBadPrefix, len(p))
	}
	for i := 0; i < len(p); i++ {
		c := p[i]
		ok := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-'
		if !ok {
			return fmt.Errorf("%w: bad char %q", errBadPrefix, c)
		}
	}
	return nil
}

// String renders the significant portion of the prefix as "a/b/c".
func (p SemanticPrefix) String() string {
	n := int(p.Significant)
	if n > len(p.Labels) {
		n = len(p.Labels)
	}
	return strings.Join(p.Labels[:n], "/")
}

// Covers reports whether p is a prefix of q: q's first Significant labels equal
// p's significant labels (longest-prefix-match containment).
func (p SemanticPrefix) Covers(q SemanticPrefix) bool {
	n := int(p.Significant)
	if n > len(p.Labels) || n > len(q.Labels) {
		return false
	}
	for i := 0; i < n; i++ {
		if p.Labels[i] != q.Labels[i] {
			return false
		}
	}
	return true
}

// Aggregate returns the longest common leading-label prefix of a set, with
// Significant set to that common length (what an ABR advertises in a Summary
// LSA). The returned Labels are the common labels only.
func Aggregate(ps []SemanticPrefix) SemanticPrefix {
	if len(ps) == 0 {
		return SemanticPrefix{}
	}
	common := append([]string(nil), ps[0].Labels...)
	for _, p := range ps[1:] {
		m := len(common)
		if len(p.Labels) < m {
			m = len(p.Labels)
		}
		k := 0
		for k < m && common[k] == p.Labels[k] {
			k++
		}
		common = common[:k]
		if k == 0 {
			break
		}
	}
	return SemanticPrefix{Labels: common, Significant: uint8(len(common))}
}

// Encode serialises the prefix per WIRE_FORMAT §4 (unpadded; the TLV layer pads).
func (p SemanticPrefix) Encode() []byte {
	n := len(p.Labels)
	size := 4
	for _, l := range p.Labels {
		size += 1 + len(l)
	}
	b := make([]byte, size)
	b[0] = p.Significant
	b[1] = byte(n)
	off := 4
	for _, l := range p.Labels {
		b[off] = byte(len(l))
		copy(b[off+1:], l)
		off += 1 + len(l)
	}
	return b
}

// DecodePrefix parses a prefix encoded by Encode.
func DecodePrefix(b []byte) (SemanticPrefix, error) {
	if len(b) < 4 {
		return SemanticPrefix{}, errBadPrefix
	}
	sig := b[0]
	count := int(b[1])
	off := 4
	labels := make([]string, 0, count)
	for i := 0; i < count; i++ {
		if off >= len(b) {
			return SemanticPrefix{}, errBadPrefix
		}
		ll := int(b[off])
		off++
		if ll == 0 || ll > maxLabelLen || off+ll > len(b) {
			return SemanticPrefix{}, errBadPrefix
		}
		labels = append(labels, string(b[off:off+ll]))
		off += ll
	}
	if int(sig) > count {
		sig = byte(count)
	}
	return SemanticPrefix{Labels: labels, Significant: sig}, nil
}
