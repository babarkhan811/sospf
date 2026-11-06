package semantic

// CapabilityIndex groups agents by their Semantic Prefix in a trie so that
// locating candidates for an intent is O(prefix depth + matches) instead of a
// full scan of every agent (prototype of "group agents for fast access").
//
// It is deliberately a *coarse, fast front-door* (Stage A): it narrows millions
// of agents to a short list by location (prefix subtree). The exact Resolver
// (Stage B) then enforces capability matching, trust, latency and policy on that
// short list — so the index can never cause a wrong agent to be selected, only a
// smaller set to be examined. Each node also keeps a capability multiset so an
// exact-capability query can skip whole subtrees that certainly lack it.
type CapabilityIndex struct {
	root *indexNode
	n    int
}

type indexNode struct {
	children map[string]*indexNode
	agents   []*Candidate
	caps     map[string]int // capability IRI -> count within this subtree
}

func newIndexNode() *indexNode {
	return &indexNode{children: map[string]*indexNode{}, caps: map[string]int{}}
}

// NewIndex returns an empty index.
func NewIndex() *CapabilityIndex { return &CapabilityIndex{root: newIndexNode()} }

// Insert adds a candidate, filing it under its agent's Semantic Prefix and
// recording its capabilities along the whole path (so ancestors' capability sets
// cover their descendants).
func (idx *CapabilityIndex) Insert(c *Candidate) {
	n := idx.root
	addCaps(n, c.Cap.Capabilities)
	for _, label := range c.Cap.Prefix.Labels {
		child := n.children[label]
		if child == nil {
			child = newIndexNode()
			n.children[label] = child
		}
		addCaps(child, c.Cap.Capabilities)
		n = child
	}
	n.agents = append(n.agents, c)
	idx.n++
}

func addCaps(n *indexNode, caps []string) {
	for _, c := range caps {
		n.caps[c]++
	}
}

// Len returns the number of indexed agents.
func (idx *CapabilityIndex) Len() int { return idx.n }

// ShortlistByPrefix returns every agent filed under the given label path (the
// whole subtree). An empty path returns all agents. This is always correct for
// Stage B — it never drops an agent that is actually under the prefix — so
// subclass and embedding matching still work on the result.
func (idx *CapabilityIndex) ShortlistByPrefix(labels []string) []*Candidate {
	n := idx.root
	for _, l := range labels {
		n = n.children[l]
		if n == nil {
			return nil
		}
	}
	var out []*Candidate
	collect(n, &out)
	return out
}

// ShortlistExact narrows by prefix AND skips the subtree entirely if no agent in
// it advertises capabilityIRI. Use it only when an exact capability match is
// required (no subclass/embedding fallback), because it filters on the exact IRI.
func (idx *CapabilityIndex) ShortlistExact(labels []string, capabilityIRI string) []*Candidate {
	n := idx.root
	for _, l := range labels {
		n = n.children[l]
		if n == nil {
			return nil
		}
	}
	if capabilityIRI != "" && n.caps[capabilityIRI] == 0 {
		return nil // subtree certainly lacks the capability — skip it
	}
	var out []*Candidate
	collect(n, &out)
	return out
}

func collect(n *indexNode, out *[]*Candidate) {
	*out = append(*out, n.agents...)
	for _, c := range n.children {
		collect(c, out)
	}
}

// ResolveIndexed is the two-stage path: narrow by the intent's domain (Stage A),
// then run the exact resolver on the short list (Stage B). It returns the same
// decision a full scan would, as long as candidate agents are filed under their
// domain — but examines far fewer agents.
func (idx *CapabilityIndex) ResolveIndexed(r *Resolver, iv IntentVector) (Decision, int) {
	var labels []string
	if iv.Domain != "" {
		labels = []string{iv.Domain}
	}
	shortlist := idx.ShortlistByPrefix(labels)
	return r.Resolve(iv, shortlist), len(shortlist)
}
