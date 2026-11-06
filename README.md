# S-OSPF — Semantic OSPF (experimental)

A reference implementation of a **link-state routing protocol for semantic routing
between AI agents, agent gateways and agent swarms**. It keeps everything that makes
OSPF robust — the Hello/adjacency FSM, the link-state database, reliable flooding,
sequence numbers and aging, areas, and Dijkstra SPF — and adds a **semantic layer** on
top. Agents advertise *capabilities, intents, trust and telemetry* rather than only IP
prefixes, and routers compute paths by *meaning* ("who can do model inference on medical
imaging, under policy X, with trust ≥ 0.8, latency < 50 ms"), not only by destination
address.

> **"Layer 8 (Semantic/Intent)" and "Layer 9 (Agent Collaboration/Policy)" are project
> vocabulary only.** They are **not** standardized OSI layers and have no standing in any
> IETF or ISO document. We use the labels purely as a shorthand for "the semantic and
> policy concerns that sit above the classic network stack."

> **Experimental.** Every new wire element (LSA type, TLV, header, code point) is marked
> EXPERIMENTAL, drawn from experimental / private-use ranges, and labeled "S-OSPF
> experimental" — it is **not** defined by any IETF draft. The source drafts this work
> builds on are **individual drafts with no IETF standing**; several are expired or purely
> architectural. See `docs/SPEC_NOTES.md` for exactly what each one does and does not
> specify, and `docs/TRACEABILITY.md` for how our invented wire elements map back to draft
> concepts.

## Status

| Phase | Deliverable | State |
|---|---|---|
| 1 | Research & spec: `SPEC_NOTES`, `TRACEABILITY`, `WIRE_FORMAT`, ADRs | ✅ done |
| 2 | Classic OSPF core: packets, FSMs, LSDB, flooding, SPF (plain prefixes) | ✅ done |
| 3 | Opaque LSA framework + Agent Capability LSA + Semantic Prefixes + ABR summary | ✅ done |
| 4 | Semantic resolution + Semantic Forwarding Table + `sospfctl resolve-intent` | ✅ done |
| 5 | Trust LSA, Telemetry LSA, Trust Manager, dampening | ✅ done |
| 6 | Policy Engine + Gateway Mediation Layer | ✅ done |
| 7 | Security hardening: packet HMAC, LSA signatures, replay protection, rate limits | ✅ done |
| 8 | Agent SDK + MCP/A2A adapters + multi-domain demo | ⏳ not started |
| 9 | Observability + docs + demo script | ⏳ not started |

**Current checkpoint:** Phase 1 ends with a review of the wire format and design *before
any protocol code is written*. See `docs/WIRE_FORMAT.md`.

## Components (planned)

| Binary | Role |
|---|---|
| `sospfd` | The router daemon: interfaces, neighbors, LSDB, SPF, semantic planes |
| `sospfctl` | CLI: `show neighbors\|lsdb\|sft\|spf\|agents\|trust`, `resolve-intent` |
| `sospf-emu` | Multi-router emulator driven by a topology YAML |

## Repository layout

See `docs/` for the specification set. Code layout (filled in from Phase 2 on) follows
Section 6 of the project brief: `/cmd`, `/internal`, `/pkg/ash`, `/proto`, `/ontology`,
`/sdk/python`, `/topologies`, `/test/e2e`.

## Documentation

| Doc | Contents |
|---|---|
| `docs/SPEC_NOTES.md` | What each source IETF draft actually specifies (with citations) |
| `docs/TRACEABILITY.md` | Draft concept → our component → wire element |
| `docs/WIRE_FORMAT.md` | Byte-level LSA / TLV / header diagrams (RFC ASCII-art style) |
| `docs/TRUST_MODEL.md` | The trust score formula and defaults (our choice; no draft defines one) |
| `docs/FLOWS.md` | Registration → Intent → Policy → Routing → Telemetry sequence diagrams |
| `docs/SECURITY.md` | Threat model and mitigations |
| `docs/ADR/` | Architecture Decision Records |
| `docs/draft-sospf-semantic-link-state-00.md` | Internet-Draft-style write-up |

## Building and running

> Nothing to build yet — Phase 1 is specification only. Build instructions land with the
> Phase 2 classic core.

## License

To be decided before first public release.
