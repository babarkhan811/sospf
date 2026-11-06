// Package gateway is the S-OSPF Gateway Mediation Layer (brief §5.6; draft-yang
// -02). It sits on area/domain borders and turns a Mediation Request into a
// Mediation Response, a Handoff Context, a Failure Record and an Evidence
// Reference. It *prepares* a handoff; it never reimplements MCP/A2A/IAIP/HTTP
// (draft-yang §6 non-goal) and never inlines raw evidence or leaks raw prompts
// (draft-yang §14).
package gateway

import (
	"fmt"
	"time"

	"github.com/babarkhan811/sospf/internal/policy"
	"github.com/babarkhan811/sospf/internal/semantic"
)

// MediationRequest is the gateway input (draft-yang -02 §8). The -01 alias is
// documented in doc comments; the -02 names are primary.
type MediationRequest struct {
	Intent     semantic.IntentVector      // Goal + Context
	Tenant     string                     // Policy Scope: tenant
	Candidates []*semantic.Candidate      // Registered Capability Profiles + live metrics
	Validated  map[semantic.AgentID]bool  // Trust Binding: which claims are signature-verified
}

// MediationResponse is the primary decision artifact (§10.1; -01: Resolution
// Result).
type MediationResponse struct {
	Outcome            string // "SELECTED" | "REJECTED"
	SelectedTargetRef  string // semantic prefix of the chosen agent
	DecisionReason     string
	ProfileVersionRefs []string
	ValidationStatus   string // "verified" | "self-asserted"
	ExpiresAt          time.Time
}

// HandoffContext describes how to hand the task off (§10.2; -01: Handoff
// Descriptor). ContextSummary is redacted per disclosure limits.
type HandoffContext struct {
	TaskMeaning      string
	ContextSummary   string
	DisclosureLimit  uint8
	Mechanism        string // MCP | A2A | IAIP | HTTP
	TargetRef        string
	ScopedTokenRef   string // a reference, never the token itself
	Expiry           time.Time
	FailureSemantics string
}

// FailureRecord explains a non-selection (§10.3; -01: Failure Explanation). It
// carries no raw prompts, PII, or internal policy detail (§14).
type FailureRecord struct {
	ReasonCode semantic.ReasonCode
	Hint       string // "RETRY" | "FALLBACK" | "ESCALATE"
	HumanText  string
}

// EvidenceReference links a decision to evidence by pointer only (§10.4).
type EvidenceReference struct {
	SourceDescriptionRefs []string
	ValidationStatus      string
	AuditRefs             []string
}

// MediationResult bundles the outputs.
type MediationResult struct {
	Response MediationResponse
	Handoff  *HandoffContext
	Failure  *FailureRecord
	Evidence EvidenceReference
}

// Gateway mediates requests using a Policy Engine and the semantic Resolver.
type Gateway struct {
	Policy   policy.Engine
	Resolver *semantic.Resolver
}

// New builds a gateway.
func New(p policy.Engine, r *semantic.Resolver) *Gateway {
	if r == nil {
		r = semantic.NewResolver()
	}
	return &Gateway{Policy: p, Resolver: r}
}

// Mediate applies policy, resolves, and prepares the handoff or failure.
func (g *Gateway) Mediate(req MediationRequest) MediationResult {
	iv := req.Intent
	category := intentCategory(iv)

	// Policy pass: deny drops a candidate; constrain tightens per-candidate gates.
	allowed := make([]*semantic.Candidate, 0, len(req.Candidates))
	for _, c := range req.Candidates {
		in := policy.Input{
			IntentCategory:  category,
			Domain:          c.Cap.Domain,
			Tenant:          req.Tenant,
			Trust:           c.Trust,
			DisclosureLevel: c.Cap.Disclosure,
		}
		d := g.Policy.Evaluate(in)
		switch d.Effect {
		case policy.Deny:
			continue
		case policy.Constrain:
			if c.Trust < d.MinTrust {
				continue
			}
			if d.MaxDisclosure != 0 && c.Cap.Disclosure > d.MaxDisclosure {
				continue
			}
		}
		allowed = append(allowed, c)
	}

	dec := g.Resolver.Resolve(iv, allowed)
	ev := EvidenceReference{
		ValidationStatus: "self-asserted",
		AuditRefs:        []string{fmt.Sprintf("audit:%s", iv.RoutingID)},
	}

	if dec.Selected == nil {
		reason := dec.Reason
		if len(allowed) == 0 && len(req.Candidates) > 0 {
			reason = semantic.PolicyConstraintFailure // everything was policy-filtered
		}
		return MediationResult{
			Response: MediationResponse{Outcome: "REJECTED", DecisionReason: reason.String(), ExpiresAt: dec.ExpiresAt},
			Failure: &FailureRecord{
				ReasonCode: reason,
				Hint:       hintFor(reason),
				HumanText:  redactedFailureText(reason),
			},
			Evidence: ev,
		}
	}

	sel := dec.Selected
	validation := "self-asserted"
	if req.Validated != nil && req.Validated[sel.Cap.AgentID] {
		validation = "verified"
	}
	ev.ValidationStatus = validation
	ev.SourceDescriptionRefs = []string{"cap:" + sel.Cap.Prefix.String()}

	return MediationResult{
		Response: MediationResponse{
			Outcome:           "SELECTED",
			SelectedTargetRef: sel.Cap.Prefix.String(),
			DecisionReason:    "best composite cost under policy and trust",
			ValidationStatus:  validation,
			ExpiresAt:         dec.ExpiresAt,
		},
		Handoff: &HandoffContext{
			TaskMeaning:      iv.TaskTypeIRI,
			ContextSummary:   redactedSummary(iv),
			DisclosureLimit:  iv.Constraints.DisclosureLimit,
			Mechanism:        mechanismName(sel.Cap),
			TargetRef:        sel.Cap.Prefix.String(),
			ScopedTokenRef:   "token-ref:" + iv.RoutingID,
			Expiry:           dec.ExpiresAt,
			FailureSemantics: "on failure, start a new mediation with the original intent (idempotent only)",
		},
		Evidence: ev,
	}
}

func intentCategory(iv semantic.IntentVector) string {
	if iv.TaskTypeIRI != "" {
		return iv.TaskTypeIRI
	}
	if len(iv.RequiredCapabilities) > 0 {
		return iv.RequiredCapabilities[0]
	}
	return ""
}

// redactedSummary produces a context summary with no raw prompt or PII (§14):
// only the domain and the shape of the request.
func redactedSummary(iv semantic.IntentVector) string {
	return fmt.Sprintf("domain=%q; %d required capability/ies; max_latency=%dms; min_trust=%.2f",
		iv.Domain, len(iv.Required()), iv.Constraints.MaxLatencyMs, iv.Constraints.MinTrust)
}

func redactedFailureText(r semantic.ReasonCode) string {
	// Minimal, operator-facing text; never includes request content.
	return "no eligible target: " + r.String()
}

func hintFor(r semantic.ReasonCode) string {
	switch r {
	case semantic.TrustInsufficiency, semantic.UnavailableContext, semantic.StaleCapabilityInfo:
		return "RETRY"
	case semantic.CapabilityMismatch, semantic.SemanticProfileMismatch:
		return "FALLBACK"
	case semantic.PolicyConstraintFailure:
		return "ESCALATE"
	default:
		return "RETRY"
	}
}

func mechanismName(c *semantic.CapabilityLSA) string {
	if len(c.Endpoints) == 0 {
		return "HTTP"
	}
	switch c.Endpoints[0].Mechanism {
	case semantic.MechanismMCP:
		return "MCP"
	case semantic.MechanismA2A:
		return "A2A"
	case semantic.MechanismIAIP:
		return "IAIP"
	default:
		return "HTTP"
	}
}
