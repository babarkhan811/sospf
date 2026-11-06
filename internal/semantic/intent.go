package semantic

// IntentVector is a communication goal submitted by an agent (WIRE_FORMAT §5.6;
// draft-li §3 + draft-cui §5.1). The embedding is optional and used only as a
// matching fallback.
type IntentVector struct {
	RoutingID            string
	TaskTypeIRI          string
	Priority             uint32
	Urgency              uint32
	Domain               string
	RequiredCapabilities []string
	Constraints          Constraints
	Embedding            []float32
}

// Constraints are the hard limits a candidate must satisfy (draft-cui §5.1).
type Constraints struct {
	MaxLatencyMs    int      // 0 = unlimited
	MinTrust        float64  // [0,1]
	AllowedDomains  []string // empty = any
	DisclosureLimit uint8    // candidate Disclosure must be <= this
	CostCeiling     float64  // 0 = unlimited (applies to composite cost)
}

// Required returns the capability IRIs the intent needs: the explicit list, or
// the task-type IRI if the list is empty.
func (iv IntentVector) Required() []string { return iv.required() }

func (iv IntentVector) required() []string {
	if len(iv.RequiredCapabilities) > 0 {
		return iv.RequiredCapabilities
	}
	if iv.TaskTypeIRI != "" {
		return []string{iv.TaskTypeIRI}
	}
	return nil
}
