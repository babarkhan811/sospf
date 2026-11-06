# S-OSPF — Executive Summary

## The problem

AI agents, gateways and swarms increasingly need to find *each other* by
capability, not address: "who can do medical-imaging inference, under policy X,
with trust ≥ 0.8 and latency < 50 ms?" Today that selection lives in bespoke
app-layer registries and gateways. Several 2025–2026 IETF individual drafts
(SRA, Gateway Mediation Layer, semantic routing for agents) sketch an
architecture but **define no protocol** — no message formats, no trust model, no
advertisements.

## What we built

**S-OSPF** is a working, tested reference implementation that answers the
"route by meaning" question with the machinery the networking industry already
trusts: **link-state routing**. It keeps OSPF's adjacency FSM, link-state
database, reliable flooding, aging, areas and SPF, and adds an experimental
**semantic layer** carried in OSPF Opaque LSAs:

- **Agent Capability, Trust, Telemetry, Policy and Prefix-Summary LSAs** — agents
  advertise capabilities, trust and telemetry, not just prefixes.
- **Semantic Prefixes** (`domain/category/sub/agent`) that aggregate at area
  borders exactly like IP prefixes.
- **Three-stage path computation**: Dijkstra, then capability matching
  (exact/subclass/embedding, three-valued), then constrained ranking by a tunable
  composite cost.
- **A closed trust loop**: telemetry → EWMA trust → re-advertised Trust LSAs →
  automatic re-routing, with dampening so noisy metrics never flap routes.
- **A Gateway Mediation Layer** that prepares MCP/A2A/IAIP/HTTP handoffs without
  reimplementing them, and never leaks raw prompts or PII.
- **Security**: HMAC-authenticated packets with replay protection, Ed25519-signed
  advertisements (a compromised router can't forge another's agents), rate limits
  and a Sybil cap.

## What "Layer 8/9" means here

Project shorthand only: "Layer 8" = semantic/intent, "Layer 9" =
agent-collaboration/policy. They are **not** OSI layers. The value is putting
semantic selection on a robust, well-understood control-plane foundation.

## Demonstrated results

The `sospf-demo` scenario routes a planner to the best-trusted medical inference
agent; when that agent starts failing, its trust decays and traffic
**automatically re-routes** to the next-best agent — end to end, through the real
resolver, Trust Manager, Policy Engine and Gateway.

Measured in the emulator (single process, commodity laptop):

| Metric | Result |
|---|---|
| Convergence, 3 routers | ~43 ms |
| Convergence, 20 routers | ~67 ms |
| Convergence, 50 routers | ~340 ms |
| LSDB size | linear in router count (digests, not documents) |
| Intent resolution, 100 candidates | ~69 µs |
| Intent resolution, 1000 candidates | ~1.0 ms |
| Tests | 11 packages green; parsers fuzzed; 3-router convergence + link-failure reconvergence |

## The ask

A pilot to harden S-OSPF toward production: either harden the Go daemon or port
the classic core onto FRRouting and keep the semantic planes as a sidecar
(see `ROADMAP.md`). We also propose taking the wire design to the IETF as an
experimental draft (`STANDARDS_STRATEGY.md`).
