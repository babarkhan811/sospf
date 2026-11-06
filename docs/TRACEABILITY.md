# TRACEABILITY — draft concept → S-OSPF component → wire element

Every S-OSPF feature traces back to a source concept (with the `SPEC_NOTES.md` citation) or
is marked **[S-OSPF experimental]** where we invented it because no draft defines it. This
is the table reviewers use to check we neither over-claim standards backing nor silently
smuggle in undocumented behavior.

Columns: **Concept** (and source §, per SPEC_NOTES) · **S-OSPF component** (code package) ·
**Wire element** (see WIRE_FORMAT.md) · **Notes**.

## A. Architecture & planes (SRA / draft-li)

| Concept (source) | S-OSPF component | Wire element | Notes |
|---|---|---|---|
| Application Plane — agents issue Intent Vectors (li §4) | `internal/api`, `sdk/python` | Intent Vector (proto+JSON-LD), ASH | Agents register + submit intents over gRPC/REST |
| Semantic Control Plane (li §4) | `internal/{semantic,trust,policy,telemetry,gateway}` | Agent Capability / Trust / Telemetry / Policy LSAs | One control plane per Semantic Router |
| Data/Forwarding Plane — Semantic Router + SFT (li §4) | `cmd/sospfd`, `internal/sft` | Semantic Forwarding Table entries | Table is local state, not flooded |
| Feedback loop (li §4; §4↔Fig.1 inconsistency) | `internal/telemetry` (a control-plane function) | Telemetry LSA | **We resolve the inconsistency:** feedback is a control-plane function, not a 4th plane (SPEC_NOTES §1) |
| Functional layers: Intent→Trust→Policy→Routing→Feedback (li §5) | pipeline across the packages above | — | Mirrors the resolve-intent call path |

## B. The five procedures (SRA §6) → flows (brief §5.7, docs/FLOWS.md)

| Procedure (li §6) | S-OSPF component | Wire element | Notes |
|---|---|---|---|
| 1. Agent Registration | `internal/api` + registration handler | mTLS client cert → assigned **Semantic Prefix**; Agent-ID (128-bit) | Cert-bound identity (borrow: ANS PKI, IAIP AIC) |
| 2. Intent Submission | `internal/api` → `internal/semantic` | Intent Vector; ASH | — |
| 3. Policy Translation | `internal/policy` | Policy LSA (domain summaries) | Rules from trust + objectives |
| 4. Routing Execution | `internal/spf` (semantic stages) → `internal/sft` | SFT entry (next hop, target agent, cost, constraints, expiry) | CSPF prune + composite rank |
| 5. Telemetry Feedback | `internal/telemetry` → `internal/trust` | Telemetry LSA → Trust LSA | Periodic recompute, threshold-triggered re-originate |

## C. Classic OSPF machinery (RFC 2328 / 5340 / 5250 / 8362) — preserved faithfully

| Concept | S-OSPF component | Wire element | Notes |
|---|---|---|---|
| Hello / DD / LSR / LSU / LSAck | `internal/packet`, `internal/fsm` | Standard OSPF packets | Decodable by Wireshark OSPF dissector in raw/overlay-off mode (brief §8) |
| Interface + Neighbor FSM, DR/BDR | `internal/fsm` | — | Down→Init→2-Way→ExStart→Exchange→Loading→Full |
| LSDB, aging, reliable flooding | `internal/lsdb` | LSA header + body | MaxAge, MinLSInterval, MinLSArrival, LSRefreshTime |
| Router/Network/Summary LSAs, areas, ABR | `internal/lsdb`, `internal/spf` | Standard LSA types 1/2/3 | Backbone area 0, stub areas |
| Dijkstra SPF, throttling, ECMP | `internal/spf` | — | Stage 1 of semantic path computation |
| Opaque LSA (type 9/10/11) RFC 5250 | `internal/lsdb` opaque store | Opaque LSA envelope | **Carrier for all semantic data** |

## D. Semantic advertisements **[S-OSPF experimental]** — no draft defines these

All carried as Opaque LSAs; opaque types from the experimental range 128–255; TLVs from the
experimental TLV range. Justified by SRA's "MAY extend … to distribute semantic and trust
information" (li §5) but **the encoding is entirely ours**.

| Our LSA family | Draft concept it realizes | S-OSPF component | Wire element (WIRE_FORMAT §) |
|---|---|---|---|
| **Agent Capability LSA** | SRA capability registration (li §6.1); IAIP `CAP_ADV` + LCR entry (§6); draft-cui Target Profile (§5.2); yang Capability Descriptions (§8.3) | `internal/semantic` | Opaque type 128, §5.1 |
| **Trust LSA** | SRA Trust Manager / Trust Anchoring (li §4/§5); yang Trust Evidence (§8.5) | `internal/trust` | Opaque type 129, §5.2 |
| **Telemetry / Semantic-TE LSA** | SRA Telemetry + Feedback (li §4); OSPF-TE (RFC 3630) shape | `internal/telemetry` | Opaque type 130, §5.3 |
| **Policy LSA** | SRA Policy Engine (li §4/§6); yang Policy Constraints (§8.4) | `internal/policy` | Opaque type 131, §5.4 |
| **Semantic Prefix Summary LSA** | SRA "unique semantic prefix" (li §6) + OSPF ABR summarization | `internal/lsdb` (ABR) | Opaque type 132, §5.5 |

## E. Identifiers & naming **[S-OSPF experimental, concept-borrowed]**

| Concept (source) | S-OSPF element | Wire element | Notes |
|---|---|---|---|
| "unique semantic prefix" (li §6) + ANS DNS-style capability-aware naming | **Semantic Prefix** `domain/category/subcategory/agent` | Length-prefixed labels + prefix-length (§4) | Aggregatable like an IP prefix → ABR summary |
| IAIP Agent Identity Code (§6); ANS PKI identity | **Agent-ID** 128-bit, cert-bound | Agent-ID TLV (§4) | mTLS client-cert binding |
| IAIP LCR states (ACTIVE/PENDING/SUSPENDED/UNREACHABLE §6) | **Agent reachability state** | status field in Agent Capability LSA (§5.1) | Only ACTIVE agents are selectable |

## F. Semantic payload & header **[S-OSPF experimental]**

| Concept (source) | S-OSPF element | Wire element | Notes |
|---|---|---|---|
| Intent Vector (li §3, fields only) + draft-cui Routing Input (§5.1) + draft-cui-ai-agent-task structure | **Intent Vector** | protobuf + JSON-LD object (§5.6) | task_type IRI, priority, urgency, context, resource_needs, constraints, optional embedding |
| SRA metadata on "BGP communities / SRv6 tags" (li §4) | **ASH→SRH/BGP mapping** | mapping table only, **not implemented** (§5.3) | Documented per brief §5.3 |
| Agent traffic marking (overlay mode) | **Agent Semantic Header (ASH)** | `pkg/ash` header (§5.7) | version, flags, intent-category ID, semantic dest prefix, trust floor, flow ID, hop limit, TLV area |

## G. Semantic path computation **[S-OSPF experimental]** (brief §5.4; draft-cui SRF §2)

| Stage | Concept | S-OSPF component | Notes |
|---|---|---|---|
| 1. Topology SPF | classic Dijkstra | `internal/spf` | over the link-state graph |
| 2. Semantic resolution | capability match: exact ontology IRI + subclass, embedding cosine fallback (zhang §7) | `internal/semantic` | returns Compatible/Incompatible/**Indeterminate** |
| 3. Constrained selection | CSPF prune + composite cost; top-k ECMP | `internal/spf` | `cost = w_path·igp + w_lat·lat + w_trust·(1−trust) + w_load·load + w_sem·(1−sim)` |
| Result | draft-cui Routing Decision (§5.3): one target + expiry | `internal/sft` | SFT entry with expiry; no capacity reservation |
| Dampening | SRA Closed-Loop + stability | `internal/telemetry` hysteresis | no route flap under noisy telemetry (tested) |

## H. Trust model **[S-OSPF experimental]** (brief §5.5; no draft defines a formula)

| Concept (source) | S-OSPF element | Wire element | Notes |
|---|---|---|---|
| Trust score (li — no formula) | EWMA over telemetry outcomes, decay, new-agent floor | Trust Score TLV, fixed-point 0–1 (§5.2) | Formula + defaults in TRUST_MODEL.md, labeled our choice |
| Trust Anchoring (li §5); profile-integrity (cui §12.3) | Ed25519-signed Trust + Capability LSAs | Signature TLV (§5.2) | compromised router can't forge another's agents |
| Reputation manipulation defense | Sybil limits per router, signed evidence, self-report rate limits | — | SECURITY.md |

## I. Gateway Mediation Layer (draft-yang -02; brief §5.6)

| Concept (yang §) | S-OSPF element | Wire element | Notes |
|---|---|---|---|
| Mediation Request: Goal, Context, Registered Capability Profiles, Policy Scope, Trust Binding (§8) | `MediationRequest` | gRPC/JSON-LD message (§7.1) | primary API = -02 names; -01 alias table kept |
| Mediation Response (§10.1/§12) | `MediationResponse` | message (§7.2) | selected/rejected target, reason, profile/version refs, scope, constraints, validation status |
| Handoff Context (§10.2/§12) | `HandoffContext` | message (§7.2) | redacted per disclosure limits; mechanism ∈ {MCP,A2A,IAIP,HTTP}; scoped-token ref, expiry, failure semantics |
| Failure Record (§10.3/§12) | `FailureRecord` | message (§7.3) | reason_code enum = draft-yang union; + retry/fallback/escalate hint + minimal human text |
| Evidence Reference (§10.4) | `EvidenceReference` | message (§7.2) | pointers only — **never inline raw evidence** |
| Non-goal: don't reimplement MCP/A2A (§6) | adapters in `sdk/python` + `internal/gateway` | — | gateway prepares; adapters perform |
| Self-asserted vs validated view (§9) | `internal/gateway` capability view | — | keep Gateway-managed Capability View separate from raw claims |

## J. Security & privacy (draft-cui §12, draft-yang §14; brief §9)

| Threat / rule (source) | Mitigation | S-OSPF component |
|---|---|---|
| Forged/poisoned capability ads (cui §12.3) | Ed25519 LSA signatures; validated view separate from claims | `internal/semantic`, `internal/gateway` |
| Trust-score manipulation (cui §12.4) | signed evidence, Sybil limits, self-report rate limits | `internal/trust` |
| Target enumeration / policy probing (cui §12.2) | rate limits + response minimization on `resolve-intent` | `internal/api`, `internal/spf` |
| Decision integrity & replay (cui §12.7) | packet HMAC-SHA-256 (RFC 5709-style), sequence/age, nonce | `internal/packet` |
| Context leakage across domains (yang §14) | disclosure limits + redaction in HandoffContext | `internal/gateway` |
| LSDB flooding DoS (cui §12.8) | MinLSArrival, flood pacing, per-neighbor rate limits | `internal/lsdb` |
| Prompt-injection in capability text (brief §9) | treat all agent-supplied text as **data, never executed** | everywhere |
| Minimal disclosure (yang §14) | FailureRecord carries no raw prompts/PII/internal policy | `internal/gateway` |

## K. Explicitly NOT built (respecting source non-goals)

| Not built | Why (source) |
|---|---|
| A new agent communication protocol | draft-yang §6 non-goal; we prepare handoff, adapters perform |
| A universal ontology | draft-yang §6; we align to draft-zhang's and add experimental terms only |
| A discovery/registry protocol or global registry | draft-yang §6; registration is local to a router |
| BGP-community / SRv6-tag encodings | brief §5.3 — **mapping documented, not implemented** |
| Replacing MCP/A2A/OAuth/TLS | draft-yang §6 |
