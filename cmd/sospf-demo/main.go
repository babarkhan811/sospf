// Command sospf-demo runs the end-to-end scenario from the project brief (§7.8):
// three domains; a planner agent submits an intent and is routed to the
// best-trusted inference agent; then the chosen agent starts failing, its trust
// degrades, and traffic re-routes to the next-best agent — all through the real
// semantic resolver, Trust Manager, Policy Engine and Gateway Mediation Layer.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"sort"

	"github.com/babarkhan811/sospf/internal/gateway"
	"github.com/babarkhan811/sospf/internal/metrics"
	"github.com/babarkhan811/sospf/internal/policy"
	"github.com/babarkhan811/sospf/internal/semantic"
	"github.com/babarkhan811/sospf/internal/trust"
)

// agentDef is a demo inference agent.
type agentDef struct {
	name    string
	domain  string
	cap     string
	latency int
	igp     int
	nextHop uint32
}

func mkCap(a agentDef) *semantic.CapabilityLSA {
	var id semantic.AgentID
	copy(id[:], a.name)
	p, _ := semantic.ParsePrefix(a.domain + "/inference/" + a.name)
	return &semantic.CapabilityLSA{
		AgentID: id, Prefix: p, Domain: a.domain, Status: semantic.StatusActive,
		Disclosure: semantic.DisclosureDomainInternal, Capabilities: []string{a.cap},
		Endpoints: []semantic.Endpoint{{Mechanism: semantic.MechanismMCP, URI: "mcp://" + a.name}},
	}
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	tm := trust.New(trust.DefaultParams())
	reg := metrics.NewRegistry()
	resolved := reg.Counter("sospf_resolve_total", "resolve-intent calls")
	reroutes := reg.Counter("sospf_reroute_total", "times the selected target changed")

	// Three domains; we route within "medical". Two competing medical inference
	// agents plus distractors in other domains and with other capabilities.
	agents := []agentDef{
		{"med-imaging-a", "medical", "ont:medical-inference", 20, 2, 2},
		{"med-imaging-b", "medical", "ont:medical-inference", 25, 2, 3},
		{"fin-risk-1", "finance", "ont:finance-inference", 10, 1, 4},
		{"log-eta-1", "logistics", "ont:logistics-inference", 15, 1, 5},
	}
	caps := make(map[string]*semantic.CapabilityLSA)
	for _, a := range agents {
		caps[a.name] = mkCap(a)
	}

	// Seed trust: agent A is initially the most trusted.
	seed(tm, caps["med-imaging-a"].AgentID, 30, trust.Outcome{Success: true, SLAAdhered: true})
	seed(tm, caps["med-imaging-b"].AgentID, 20, trust.Outcome{Success: true, SLAAdhered: true})
	seed(tm, caps["med-imaging-b"].AgentID, 5, trust.Outcome{Success: true}) // slightly lower (missed SLA)

	gw := gateway.New(policy.NewRuleEngine(policy.Allow), semantic.NewResolver())

	intent := semantic.IntentVector{
		RoutingID: "plan-001", TaskTypeIRI: "ont:medical-inference", Domain: "medical",
		RequiredCapabilities: []string{"ont:medical-inference"},
		Constraints:          semantic.Constraints{MinTrust: 0.6, MaxLatencyMs: 100, DisclosureLimit: 2, AllowedDomains: []string{"medical"}},
	}

	fmt.Println("=== S-OSPF multi-domain demo: planner -> best-trusted medical inference agent ===")
	fmt.Printf("Planner intent: %s in %q (min_trust %.2f)\n\n", intent.TaskTypeIRI, intent.Domain, intent.Constraints.MinTrust)

	prevTarget := ""
	round := func(label string) string {
		resolved.Inc()
		req := buildRequest(intent, agents, caps, tm)
		res := gw.Mediate(req)
		printTrust(tm, caps)
		if res.Response.Outcome != "SELECTED" {
			fmt.Printf("[%s] NO TARGET — %s\n\n", label, res.Response.DecisionReason)
			return ""
		}
		target := res.Response.SelectedTargetRef
		fmt.Printf("[%s] SELECTED %s via %s  (handoff mechanism: %s)\n\n", label, target, res.Handoff.TargetRef, res.Handoff.Mechanism)
		if prevTarget != "" && target != prevTarget {
			reroutes.Inc()
			log.Info("re-route", "from", prevTarget, "to", target)
		}
		prevTarget = target
		return target
	}

	round("round 1")

	// Now med-imaging-a starts failing (anomalies). Its trust should fall below
	// med-imaging-b, forcing a re-route.
	fmt.Println("--- med-imaging-a begins failing (timeouts + anomalies) ---")
	seed(tm, caps["med-imaging-a"].AgentID, 25, trust.Outcome{Success: false, Anomaly: true})

	round("round 2")

	fmt.Println("=== metrics ===")
	fmt.Print(reg.Expose())
}

func seed(tm *trust.Manager, id semantic.AgentID, n int, o trust.Outcome) {
	for i := 0; i < n; i++ {
		tm.Update(id, o)
	}
}

func buildRequest(iv semantic.IntentVector, defs []agentDef, caps map[string]*semantic.CapabilityLSA, tm *trust.Manager) gateway.MediationRequest {
	cands := make([]*semantic.Candidate, 0, len(defs))
	for _, a := range defs {
		c := caps[a.name]
		cands = append(cands, &semantic.Candidate{
			Cap: c, Trust: tm.Score(c.AgentID), IGPCost: a.igp, LatencyMs: a.latency, Load: 0.2, NextHop: a.nextHop,
		})
	}
	return gateway.MediationRequest{Intent: iv, Tenant: "demo", Candidates: cands}
}

func printTrust(tm *trust.Manager, caps map[string]*semantic.CapabilityLSA) {
	type row struct {
		name  string
		trust float64
	}
	var rows []row
	for name, c := range caps {
		if c.Domain == "medical" {
			rows = append(rows, row{name, tm.Score(c.AgentID)})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
	fmt.Println("  medical agent trust:")
	for _, r := range rows {
		fmt.Printf("    %-16s %.3f\n", r.name, r.trust)
	}
}
