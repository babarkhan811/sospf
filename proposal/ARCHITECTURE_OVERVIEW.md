# S-OSPF — Architecture Overview

For engineers and technical leadership. Deep detail lives in `docs/` (SPEC_NOTES,
WIRE_FORMAT, TRACEABILITY, TRUST_MODEL, SECURITY, FLOWS).

## Layered view

```
+--------------------------------------------------------------+
| Application Plane: Agents / Swarms (MCP servers, A2A agents)  |
|   register, advertise Capability Profiles, submit Intents     |
+-----------------------------+--------------------------------+
                              | Agent<->Router API + scenario JSON (SDK)
+-----------------------------v--------------------------------+
| Semantic Control Plane (per Semantic Router)                 |
|  Resolver | Trust Manager | Policy Engine | Telemetry        |
|  Gateway Mediation Layer (resolution / handoff / failure)    |
+-----------------------------+--------------------------------+
                              |
+-----------------------------v--------------------------------+
| S-OSPF Core (link-state)                                     |
|  Hello/Adjacency FSM | LSDB | Flooding | Aging | SPF         |
|  + Semantic Opaque LSAs (128-132) + semantic/constrained SPF |
+-----------------------------+--------------------------------+
                              |
+-----------------------------v--------------------------------+
| Forwarding: Semantic Forwarding Table (SFT) + IP RIB export  |
+--------------------------------------------------------------+
```

## Code map (Go)

| Package | Responsibility |
|---|---|
| `internal/packet` | LSA + control-packet codecs; TLV framework; Opaque envelope; fuzzed |
| `internal/fsm` | Neighbor state machine (Down..Full) |
| `internal/lsdb` | Link-state database, RFC 2328 comparison, aging |
| `internal/spf` | Dijkstra with two-way check, stub prefixes, ECMP-ready |
| `internal/node` | Router engine: adjacency, reliable flooding, anti-entropy, SPF throttle |
| `internal/emu` | In-process multi-router emulator + link-failure injection |
| `internal/semantic` | Semantic prefixes, Bloom, capability/trust/telemetry/policy/summary LSAs, matching, resolver |
| `internal/sft` | Semantic Forwarding Table (expiry) |
| `internal/trust` | EWMA trust model + damper |
| `internal/policy` | Total declarative policy DSL (OPA backend optional) |
| `internal/gateway` | Gateway Mediation Layer (draft-yang -02 artifacts) |
| `internal/security` | HMAC auth + replay, Ed25519 signatures, rate limit, Sybil guard |
| `internal/metrics` | Dependency-free Prometheus-text metrics |
| `cmd/*` | `sospfd`, `sospfctl`, `sospf-emu`, `sospf-demo`, `sospf-bench` |
| `sdk/python` | Agent SDK + MCP/A2A handoff adapters |

## Key design choices (ADRs)

- **Go** for the control plane; Rust or FRR for a production data plane (ADR-0001).
- **Overlay/emulator transport first**, raw IP proto-89 optional (ADR-0002).
- **Flood digests, not capability documents** — LSDB scales with agent *count*,
  not document size (ADR-0003).
- **Total policy DSL** by default, OPA/Rego pluggable (ADR-0004).

## Data flow for one intent

1. Agent registers (mTLS, Agent-ID, Semantic Prefix) → Capability LSA floods.
2. Agent submits an Intent Vector (rate-limited).
3. Policy Engine gates candidates; constraints tighten.
4. SPF (topology) → capability match (three-valued) → CSPF rank by composite cost.
5. SFT entry installed with expiry; Gateway prepares the handoff; an adapter runs it.
6. Telemetry updates trust (EWMA + dampening); threshold crossings re-advertise and
   re-route.

## Interop posture

The core remains a conformant-in-spirit OSPF: with semantics disabled it routes
plain prefixes, and a raw-mode pcap is intended to decode in Wireshark's OSPF
dissector (roadmap item). Semantic data rides in Opaque LSAs, so a non-S-OSPF
OSPF speaker floods them transparently and simply ignores what it doesn't parse.
