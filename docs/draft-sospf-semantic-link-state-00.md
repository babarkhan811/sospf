---
title: "S-OSPF: Semantic Link-State Routing for AI Agents (Experimental)"
abbrev: "S-OSPF"
category: exp
docname: draft-sospf-semantic-link-state-00
ipr: trust200902
area: Routing
workgroup: Individual Submission
keyword: [OSPF, semantic routing, AI agents, link-state]
---

# S-OSPF: Semantic Link-State Routing for AI Agents (Experimental)

> **Status of this memo.** This is an *experimental individual draft* describing
> a reference implementation. It is **not** an IETF work product and has no
> standing. Every code point it requests is from an experimental / private-use
> range. It builds on several individual drafts (draft-li, draft-yang, draft-cui,
> draft-zhang, draft-sz) that are themselves individual submissions with no IETF
> consensus; see `docs/SPEC_NOTES.md` for exactly what each specifies.

## Abstract

S-OSPF extends OSPFv2/v3 with a semantic layer for routing between AI agents,
agent gateways and agent swarms. It preserves OSPF's Hello/adjacency FSM,
link-state database, reliable flooding, aging, areas and SPF, and adds
experimental Opaque LSAs that advertise agent capabilities, trust, telemetry and
policy. Routers compute paths by *meaning* — matching an Intent Vector to agent
capabilities under trust and policy constraints — in addition to IP reachability.

## 1. Terminology

Agent-ID, Semantic Prefix, Intent Vector, Semantic Router, Semantic Forwarding
Table (SFT) are defined in `docs/WIRE_FORMAT.md`. "Layer 8/9" is project
vocabulary only and is **not** a standardized OSI layer.

## 2. Protocol overview

Classic OSPF carries topology. S-OSPF adds five Opaque-LSA families
(area-scoped type 10, AS-scoped type 11; experimental opaque types 128–132):
Agent Capability (128), Trust (129), Telemetry (130), Policy (131) and Semantic
Prefix Summary (132). Capability *documents* are not flooded; a content digest
plus a fetch hint is, and routers fetch on demand by digest (ADR-0003).

Path computation runs in three stages (WIRE_FORMAT §5.4): Dijkstra SPF, then
capability matching (exact IRI / subclass / embedding fallback, yielding
Compatible / Incompatible / **Indeterminate**), then constrained selection by a
documented composite cost with CSPF pruning and ECMP.

## 3. LSA formats

See `docs/WIRE_FORMAT.md` for byte-level diagrams: the Opaque LSA envelope (§2),
the TLV registry (§3), the Semantic Prefix encoding (§4), and each LSA family
(§5). All are marked EXPERIMENTAL.

## 4. Procedures

Agent Registration, Intent Submission, Policy Translation, Routing Execution and
Telemetry Feedback are specified in `docs/FLOWS.md`. Trust is an EWMA over
telemetry outcomes with decay and a new-agent floor (`docs/TRUST_MODEL.md`).

## 5. Security considerations

Per-packet HMAC-SHA-256 with a monotonic sequence number (replay protection) and
per-LSA Ed25519 signatures (Trust Anchoring) are specified in WIRE_FORMAT §8.
The threat model and mitigations — forged capabilities, trust gaming, target
enumeration, context leakage, flooding DoS, prompt-injection-as-data — are in
`docs/SECURITY.md`. Agent-supplied text is always treated as data, never
executed.

## 6. IANA considerations

This experimental document requests **no** permanent IANA allocations. It uses
self-assigned values from experimental / private-use ranges: Opaque types
128–132 and TLV types 0xF000–0xFFFF. Were this work adopted, a working group
would request real registries for LSA opaque types, semantic TLVs and failure
reason codes.

## 7. Reference implementation

A Go implementation (`sospfd`, `sospfctl`, `sospf-emu`, `sospf-demo`) with an
in-process emulator and ≥80%-targeted test coverage accompanies this document.
Benchmarks: convergence 43 ms (3 routers) to ~340 ms (50 routers); LSDB size
grows linearly with router count; intent resolution ~69 µs for 100 candidates.
