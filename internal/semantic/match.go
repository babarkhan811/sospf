package semantic

import "math"

// Outcome is the three-valued result of a capability match (draft-zhang §7.1):
// a bare boolean is never used.
type Outcome int

const (
	Incompatible Outcome = iota
	Compatible
	Indeterminate
)

func (o Outcome) String() string {
	switch o {
	case Incompatible:
		return "incompatible"
	case Compatible:
		return "compatible"
	default:
		return "indeterminate"
	}
}

// MatchResult pairs an outcome with the semantic similarity that produced it
// (1.0 for an exact/subclass match; the cosine value for an embedding fallback).
type MatchResult struct {
	Outcome    Outcome
	Similarity float64
}

// EmbeddingThreshold is the cosine similarity at or above which an embedding
// fallback is considered a match.
const EmbeddingThreshold = 0.75

// Match decides whether capability LSA c can serve intent iv. subclass maps a
// capability IRI to the ancestor IRIs it is a subclass of (child -> ancestors),
// so a capability for a subclass satisfies a requirement for a superclass.
//
// Decision order (brief §5.4 stage 2):
//  1. exact IRI or subclass match for every required capability -> Compatible.
//  2. otherwise, if both sides carry embeddings, cosine similarity decides.
//  3. otherwise the result is Indeterminate (we cannot confirm or deny).
func Match(iv IntentVector, c *CapabilityLSA, subclass map[string][]string) MatchResult {
	req := iv.required()
	if len(req) == 0 {
		return MatchResult{Outcome: Indeterminate}
	}
	provided := make(map[string]bool, len(c.Capabilities))
	for _, cap := range c.Capabilities {
		provided[cap] = true
	}
	allExact := true
	for _, r := range req {
		if !satisfies(provided, r, subclass) {
			allExact = false
			break
		}
	}
	if allExact {
		return MatchResult{Outcome: Compatible, Similarity: 1.0}
	}
	if len(iv.Embedding) > 0 && len(c.Embedding) == len(iv.Embedding) {
		sim := cosine(iv.Embedding, c.Embedding)
		if sim >= EmbeddingThreshold {
			return MatchResult{Outcome: Compatible, Similarity: sim}
		}
		return MatchResult{Outcome: Incompatible, Similarity: sim}
	}
	return MatchResult{Outcome: Indeterminate}
}

// satisfies reports whether some provided capability equals req or is a subclass
// of req.
func satisfies(provided map[string]bool, req string, subclass map[string][]string) bool {
	if provided[req] {
		return true
	}
	for cap := range provided {
		for _, anc := range subclass[cap] {
			if anc == req {
				return true
			}
		}
	}
	return false
}

func cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// Embedding is added to CapabilityLSA here to keep the capability wire format in
// capability.go focused; the matcher reads it when present.
func (c *CapabilityLSA) WithEmbedding(e []float32) *CapabilityLSA { c.Embedding = e; return c }
