package gateway

import (
	"strings"
	"testing"

	"github.com/babarkhan811/sospf/internal/policy"
	"github.com/babarkhan811/sospf/internal/semantic"
)

func cap(agent, domain string, disc uint8, caps ...string) *semantic.CapabilityLSA {
	var id semantic.AgentID
	copy(id[:], agent)
	p, _ := semantic.ParsePrefix(domain + "/svc/" + agent)
	return &semantic.CapabilityLSA{
		AgentID: id, Prefix: p, Domain: domain, Status: semantic.StatusActive,
		Disclosure: disc, Capabilities: caps,
		Endpoints: []semantic.Endpoint{{Mechanism: semantic.MechanismMCP, URI: "mcp://" + agent}},
	}
}

func baseIntent() semantic.IntentVector {
	return semantic.IntentVector{
		RoutingID: "r-1", TaskTypeIRI: "ont:segment", Domain: "medical",
		RequiredCapabilities: []string{"ont:segment"},
		Constraints:          semantic.Constraints{MinTrust: 0.6, DisclosureLimit: 2},
	}
}

func TestGatewaySelectsAndPreparesHandoff(t *testing.T) {
	g := New(policy.NewRuleEngine(policy.Allow), semantic.NewResolver())
	req := MediationRequest{
		Intent: baseIntent(),
		Tenant: "tenantA",
		Candidates: []*semantic.Candidate{
			{Cap: cap("seg1", "medical", 1, "ont:segment"), Trust: 0.9, IGPCost: 1, NextHop: 2},
		},
		Validated: map[semantic.AgentID]bool{},
	}
	res := g.Mediate(req)
	if res.Response.Outcome != "SELECTED" {
		t.Fatalf("expected SELECTED, got %q (reason %q)", res.Response.Outcome, res.Response.DecisionReason)
	}
	if res.Handoff == nil || res.Handoff.Mechanism != "MCP" {
		t.Fatalf("handoff mechanism: %+v", res.Handoff)
	}
	if res.Response.SelectedTargetRef != "medical/svc/seg1" {
		t.Errorf("target ref: %q", res.Response.SelectedTargetRef)
	}
	if !strings.HasPrefix(res.Handoff.ScopedTokenRef, "token-ref:") {
		t.Errorf("scoped token should be a reference, got %q", res.Handoff.ScopedTokenRef)
	}
	if len(res.Evidence.SourceDescriptionRefs) == 0 {
		t.Error("evidence should carry source-description references")
	}
}

func TestGatewayRedactionNoLeak(t *testing.T) {
	g := New(policy.NewRuleEngine(policy.Allow), semantic.NewResolver())
	iv := baseIntent()
	// Simulate a sensitive free-text goal that must never appear in outputs.
	secret := "PATIENT_SSN_123_45_6789"
	iv.RoutingID = "r-" + secret // even if routing id carried it, outputs must not echo raw PII beyond a ref
	req := MediationRequest{Intent: iv, Candidates: []*semantic.Candidate{
		{Cap: cap("seg1", "medical", 1, "ont:segment"), Trust: 0.9, NextHop: 2},
	}}
	res := g.Mediate(req)
	if strings.Contains(res.Handoff.ContextSummary, secret) {
		t.Error("context summary leaked sensitive text (must be redacted)")
	}
	// The handoff summary should describe shape only.
	if !strings.Contains(res.Handoff.ContextSummary, "domain=") {
		t.Errorf("summary should describe shape, got %q", res.Handoff.ContextSummary)
	}
}

func TestGatewayPolicyDenyProducesFailure(t *testing.T) {
	// Deny anything in the "finance" domain.
	eng := policy.NewRuleEngine(policy.Allow, policy.Rule{Domain: "finance", Effect: "deny"})
	g := New(eng, semantic.NewResolver())
	iv := baseIntent()
	iv.Domain = "finance"
	iv.Constraints.AllowedDomains = nil
	req := MediationRequest{Intent: iv, Candidates: []*semantic.Candidate{
		{Cap: cap("f1", "finance", 1, "ont:segment"), Trust: 0.99, NextHop: 2},
	}}
	res := g.Mediate(req)
	if res.Response.Outcome != "REJECTED" || res.Failure == nil {
		t.Fatalf("expected REJECTED with a failure record, got %+v", res.Response)
	}
	if res.Failure.ReasonCode != semantic.PolicyConstraintFailure {
		t.Errorf("reason: got %v want policy-constraint-failure", res.Failure.ReasonCode)
	}
	if res.Failure.Hint != "ESCALATE" {
		t.Errorf("policy failure hint should be ESCALATE, got %q", res.Failure.Hint)
	}
}

func TestGatewayConstrainRaisesTrustFloor(t *testing.T) {
	// Constrain medical domain to trust >= 0.9.
	eng := policy.NewRuleEngine(policy.Allow, policy.Rule{Domain: "medical", Effect: "constrain", MinTrust: 0.9})
	g := New(eng, semantic.NewResolver())
	req := MediationRequest{Intent: baseIntent(), Candidates: []*semantic.Candidate{
		{Cap: cap("mid", "medical", 1, "ont:segment"), Trust: 0.7, NextHop: 2}, // below constrained floor
	}}
	res := g.Mediate(req)
	if res.Response.Outcome != "REJECTED" {
		t.Fatalf("0.7-trust candidate should be filtered by the 0.9 constrain, got %q", res.Response.Outcome)
	}
}

func TestPolicyLSARoundTrip(t *testing.T) {
	in := &semantic.PolicyLSA{
		AdvRouter: 1, OpaqueID: 1, Seq: -0x7FFFFFFF, Domain: "medical",
		AllowedIntents: []string{"ont:segment", "ont:classify"}, DisclosureLimit: 2,
		ComplianceTags: []string{"hipaa", "gdpr"},
	}
	out, err := semantic.DecodePolicyLSA(in.Encode())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Domain != "medical" || out.DisclosureLimit != 2 ||
		len(out.AllowedIntents) != 2 || len(out.ComplianceTags) != 2 {
		t.Errorf("policy LSA round trip mismatch: %+v", out)
	}
}
