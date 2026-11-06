package semantic

// ReasonCode is the experimental reason-code enum (WIRE_FORMAT §7.3), the union
// of draft-yang -01 §10.3 and §12. It is shared by the resolver (Phase 4) and
// the Gateway Mediation Layer's FailureRecord (Phase 6).
type ReasonCode int

const (
	ReasonNone ReasonCode = iota
	CapabilityMismatch
	IntentAmbiguity
	ParameterSemanticMismatch
	SemanticProfileMismatch
	PolicyConstraintFailure
	TrustInsufficiency
	StaleCapabilityInfo
	UnavailableContext
	ContextVersionMismatch
	HandoffPreparationFailure
)

func (r ReasonCode) String() string {
	switch r {
	case ReasonNone:
		return "none"
	case CapabilityMismatch:
		return "capability-mismatch"
	case IntentAmbiguity:
		return "intent-ambiguity"
	case ParameterSemanticMismatch:
		return "parameter-semantic-mismatch"
	case SemanticProfileMismatch:
		return "semantic-profile-mismatch"
	case PolicyConstraintFailure:
		return "policy-constraint-failure"
	case TrustInsufficiency:
		return "trust-insufficiency"
	case StaleCapabilityInfo:
		return "stale-capability-info"
	case UnavailableContext:
		return "unavailable-context"
	case ContextVersionMismatch:
		return "context-version-mismatch"
	case HandoffPreparationFailure:
		return "handoff-preparation-failure"
	default:
		return "unknown"
	}
}
