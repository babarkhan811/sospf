// Package policy is the S-OSPF Policy Engine (ADR-0004). The default engine is a
// small, total, declarative rule language over a fixed typed input: no loops, no
// recursion, no I/O, so evaluation always terminates and cannot be abused for
// policy-probing amplification (draft-cui §12.2). An OPA/Rego backend is a
// pluggable alternative behind the build tag `opa`, honoring the same Engine
// interface; it is not compiled by default.
package policy

import (
	"encoding/json"
	"os"

	"github.com/babarkhan811/sospf/internal/semantic"
)

// Input is the fixed, typed decision input.
type Input struct {
	IntentCategory  string
	Domain          string
	Tenant          string
	Trust           float64
	DisclosureLevel uint8
	ComplianceTags  []string
}

// Effect is a policy outcome.
type Effect int

const (
	Allow Effect = iota
	Deny
	Constrain
)

// Decision is the engine's output. When Effect is Constrain, MinTrust and
// MaxDisclosure tighten the intent's constraints before resolution.
type Decision struct {
	Effect        Effect
	Reason        semantic.ReasonCode
	MinTrust      float64
	MaxDisclosure uint8
}

// Engine evaluates policy for a candidate.
type Engine interface {
	Evaluate(Input) Decision
}

// Rule is one declarative rule. Empty match fields mean "any". The first rule
// that matches decides (total, order-dependent).
type Rule struct {
	// match
	IntentCategory string `json:"intent_category,omitempty"`
	Domain         string `json:"domain,omitempty"`
	Tenant         string `json:"tenant,omitempty"`
	RequireTag     string `json:"require_tag,omitempty"` // input must carry this compliance tag
	// effect
	Effect        string  `json:"effect"` // "allow" | "deny" | "constrain"
	MinTrust      float64 `json:"min_trust,omitempty"`
	MaxDisclosure uint8   `json:"max_disclosure,omitempty"`
	Reason        string  `json:"reason,omitempty"`
}

// RuleEngine is the default DSL engine.
type RuleEngine struct {
	Rules   []Rule
	Default Effect // used when no rule matches
}

// NewRuleEngine builds an engine with a default effect.
func NewRuleEngine(def Effect, rules ...Rule) *RuleEngine {
	return &RuleEngine{Rules: rules, Default: def}
}

// LoadRules reads rules from a JSON file: {"default":"allow|deny","rules":[...]}.
func LoadRules(path string) (*RuleEngine, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Default string `json:"default"`
		Rules   []Rule `json:"rules"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	def := Allow
	if doc.Default == "deny" {
		def = Deny
	}
	return &RuleEngine{Rules: doc.Rules, Default: def}, nil
}

// Evaluate returns the decision for in. First matching rule wins.
func (e *RuleEngine) Evaluate(in Input) Decision {
	for _, r := range e.Rules {
		if !ruleMatches(r, in) {
			continue
		}
		switch r.Effect {
		case "deny":
			return Decision{Effect: Deny, Reason: reasonOf(r.Reason, semantic.PolicyConstraintFailure)}
		case "constrain":
			return Decision{Effect: Constrain, MinTrust: r.MinTrust, MaxDisclosure: r.MaxDisclosure,
				Reason: reasonOf(r.Reason, semantic.PolicyConstraintFailure)}
		default: // "allow"
			return Decision{Effect: Allow}
		}
	}
	if e.Default == Deny {
		return Decision{Effect: Deny, Reason: semantic.PolicyConstraintFailure}
	}
	return Decision{Effect: Allow}
}

func ruleMatches(r Rule, in Input) bool {
	if r.IntentCategory != "" && r.IntentCategory != in.IntentCategory {
		return false
	}
	if r.Domain != "" && r.Domain != in.Domain {
		return false
	}
	if r.Tenant != "" && r.Tenant != in.Tenant {
		return false
	}
	if r.RequireTag != "" && !hasTag(in.ComplianceTags, r.RequireTag) {
		return false
	}
	return true
}

func hasTag(tags []string, want string) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}

func reasonOf(s string, def semantic.ReasonCode) semantic.ReasonCode {
	switch s {
	case "trust-insufficiency":
		return semantic.TrustInsufficiency
	case "capability-mismatch":
		return semantic.CapabilityMismatch
	case "policy-constraint-failure":
		return semantic.PolicyConstraintFailure
	default:
		return def
	}
}
