// Command sospfctl is the S-OSPF control CLI.
//
// In Phase 4 the show-* subcommands still need the daemon RPC (Phase 8), but
// resolve-intent is fully functional offline: it runs the real semantic resolver
// against a scenario file describing an intent and a set of candidate agents.
//
//	sospfctl resolve-intent -scenario scenarios/medical.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/babarkhan811/sospf/internal/semantic"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "resolve-intent":
		os.Exit(resolveIntent(os.Args[2:]))
	case "neighbors", "lsdb", "sft", "spf", "agents", "trust":
		fmt.Printf("sospfctl %s: needs a running daemon RPC (phase 8).\n", os.Args[1])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(`sospfctl <command>
  resolve-intent -scenario <file.json>   resolve an intent to a target agent
  neighbors | lsdb | sft | spf | agents | trust   (need the daemon, phase 8)`)
}

// scenario is the JSON shape resolve-intent reads.
type scenario struct {
	Intent     intentJSON          `json:"intent"`
	Agents     []agentJSON         `json:"agents"`
	Subclasses map[string][]string `json:"subclasses"`
}
type intentJSON struct {
	TaskTypeIRI     string    `json:"task_type_iri"`
	Domain          string    `json:"domain"`
	Required        []string  `json:"required_capabilities"`
	MaxLatencyMs    int       `json:"max_latency_ms"`
	MinTrust        float64   `json:"min_trust"`
	AllowedDomains  []string  `json:"allowed_domains"`
	DisclosureLimit uint8     `json:"disclosure_limit"`
	Embedding       []float32 `json:"embedding"`
}
type agentJSON struct {
	AgentID      string    `json:"agent_id"`
	Prefix       string    `json:"prefix"`
	Domain       string    `json:"domain"`
	Status       uint8     `json:"status"`
	Disclosure   uint8     `json:"disclosure"`
	Capabilities []string  `json:"capabilities"`
	Embedding    []float32 `json:"embedding"`
	Trust        float64   `json:"trust"`
	IGPCost      int       `json:"igp_cost"`
	LatencyMs    int       `json:"latency_ms"`
	Load         float64   `json:"load"`
	NextHop      uint32    `json:"next_hop"`
}

func resolveIntent(args []string) int {
	fs := flag.NewFlagSet("resolve-intent", flag.ExitOnError)
	path := fs.String("scenario", "", "path to a scenario JSON file")
	_ = fs.Parse(args)
	if *path == "" {
		fmt.Fprintln(os.Stderr, "usage: sospfctl resolve-intent -scenario <file.json>")
		return 2
	}
	raw, err := os.ReadFile(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read scenario:", err)
		return 1
	}
	var sc scenario
	if err := json.Unmarshal(raw, &sc); err != nil {
		fmt.Fprintln(os.Stderr, "parse scenario:", err)
		return 1
	}

	iv := semantic.IntentVector{
		TaskTypeIRI:          sc.Intent.TaskTypeIRI,
		Domain:               sc.Intent.Domain,
		RequiredCapabilities: sc.Intent.Required,
		Embedding:            sc.Intent.Embedding,
		Constraints: semantic.Constraints{
			MaxLatencyMs:    sc.Intent.MaxLatencyMs,
			MinTrust:        sc.Intent.MinTrust,
			AllowedDomains:  sc.Intent.AllowedDomains,
			DisclosureLimit: sc.Intent.DisclosureLimit,
		},
	}

	cands := make([]*semantic.Candidate, 0, len(sc.Agents))
	for _, a := range sc.Agents {
		var id semantic.AgentID
		copy(id[:], a.AgentID)
		p, err := semantic.ParsePrefix(a.Prefix)
		if err != nil {
			fmt.Fprintf(os.Stderr, "agent %q: bad prefix: %v\n", a.AgentID, err)
			return 1
		}
		cands = append(cands, &semantic.Candidate{
			Cap: &semantic.CapabilityLSA{
				AgentID: id, Prefix: p, Domain: a.Domain, Status: a.Status,
				Disclosure: a.Disclosure, Capabilities: a.Capabilities, Embedding: a.Embedding,
			},
			Trust: a.Trust, IGPCost: a.IGPCost, LatencyMs: a.LatencyMs, Load: a.Load, NextHop: a.NextHop,
		})
	}

	r := semantic.NewResolver()
	r.Subclass = sc.Subclasses
	d := r.Resolve(iv, cands)
	printDecision(iv, d)
	return 0
}

func printDecision(iv semantic.IntentVector, d semantic.Decision) {
	req := iv.RequiredCapabilities
	fmt.Printf("intent: domain=%q required=%v min_trust=%.2f max_latency=%dms\n\n",
		iv.Domain, req, iv.Constraints.MinTrust, iv.Constraints.MaxLatencyMs)
	if d.Selected == nil {
		fmt.Printf("NO TARGET SELECTED — reason: %s\n", d.Reason)
		if len(d.Indeterminate) > 0 {
			fmt.Printf("  %d candidate(s) were indeterminate (could not confirm a match)\n", len(d.Indeterminate))
		}
		fmt.Printf("  decision expires: %s\n", d.ExpiresAt.Format("15:04:05"))
		return
	}
	fmt.Printf("SELECTED agent %q  (composite cost %.3f, next-hop %d)\n",
		trim(d.Selected.Cap.AgentID), d.Cost, d.Selected.NextHop)
	fmt.Printf("  trust=%.2f latency=%dms load=%.2f similarity=%.2f\n",
		d.Selected.Trust, d.Selected.LatencyMs, d.Selected.Load, d.Selected.Result.Similarity)
	if len(d.ECMP) > 0 {
		fmt.Printf("  %d equal-cost alternate(s) (ECMP)\n", len(d.ECMP))
	}
	fmt.Printf("\nranked eligible candidates:\n")
	for i, r := range d.Ranked {
		fmt.Printf("  %d. %-20q cost=%.3f trust=%.2f\n", i+1, trim(r.Cand.Cap.AgentID), r.Cost, r.Cand.Trust)
	}
	if len(d.Indeterminate) > 0 {
		fmt.Printf("\n%d indeterminate candidate(s) reported but not selected.\n", len(d.Indeterminate))
	}
	fmt.Printf("\ndecision expires: %s\n", d.ExpiresAt.Format("15:04:05"))
}

func trim(id semantic.AgentID) string {
	n := len(id)
	for n > 0 && id[n-1] == 0 {
		n--
	}
	return string(id[:n])
}
