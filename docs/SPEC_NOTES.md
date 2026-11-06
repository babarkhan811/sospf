# SPEC_NOTES — what the source documents actually specify

**Purpose.** Before writing any S-OSPF code, this file records *exactly* what each source
document defines, what it leaves undefined, and the section we rely on. It is the evidence
base for every "the draft says…" claim elsewhere in this repo. Rule (brief §10): **no
other doc may claim a feature is defined by an IETF draft unless it cites a section here.**

**Legend.** ✅ specified concretely · 🟨 named/described in prose only (no schema) · ❌ not
defined (we must invent) · 📌 our design hook.

> **Standing of these documents.** All are **individual Internet-Drafts** with no IETF
> working-group adoption or consensus. Several are expired or have named successors (noted
> below). Nothing here is a standard. Our implementation treats them as *input concepts*,
> not normative references, and marks everything we build on top as **S-OSPF experimental**.

All URLs below were fetched successfully on 2026-11-06; no summary relies on search
snippets except where explicitly labeled **(abstract-only)**.

---

## 1. draft-li-semantic-routing-architecture-00 — the architecture we extend

- **Full title:** "Semantic Routing Architecture for AI Agents Communication."
- **Authors / origin:** Xueting Li, Aijun Wang (China Telecom).
- **Dates:** published 2025-11-04, Standards Track, expires 2026-05-08 (so **currently
  live**, not yet expired as of 2026-11-06 — the project brief's "now expired" is slightly
  ahead of the calendar). Uses "SRA" / "SAR" / "SR" interchangeably.

### What it is
A pure **architecture / framework** document. No protocol, data model, encoding, or
security mechanism. Everything is abstract with illustrative examples.

### Critical fact we design around ✅ (confirmed)
- **No message formats, no header layouts, no SFT field layouts, no Intent Vector schema,
  no trust formula, no link-state advertisements.** ❌ all of it.
- **IANA Considerations (§8) = "TBD"**; Acknowledgement (§9) also "TBD". ❌
- Caveat: the draft's own normative references look thin (only RFC 2119 clearly apt), which
  reinforces that it is directional, not implementable as written.

### Planes (§4, Figure 1) 🟨
- **Application Plane** — AI agents issue Intent Vectors describing goals.
- **Semantic Control Plane** — *Intent Controller* (collects intents, handles
  authentication), *Policy Engine* (intents → routing policy), *Trust Manager* (computes
  trust), *Telemetry Agents* (collect performance data).
- **Data / Forwarding Plane** — *Semantic Routers* forward using *Semantic Forwarding
  Tables* mapping intent types → paths/constraints.
- **Feedback loop** — telemetry on latency/trust/quality feeds the control plane.
- ⚠️ **Inconsistency to resolve:** §4 prose calls this a "Feedback Plane," but Figure 1
  places Telemetry Agents *inside* the Control Plane. 📌 We treat telemetry/feedback as a
  **function of the control plane**, not a 4th plane (see TRACEABILITY).

### Functional layers (§5) 🟨
Intent → Identity & Trust → Policy → Semantic Routing → Feedback. Each agent has an
identity certificate; trust scores derive from telemetry; the Policy Engine turns
intents + trust into enforceable rules; routers update SFT by trust/performance.

### Key terms (§3) 🟨 / ❌
- **AI Agent** — autonomous software entity making context-based decisions and
  communicating with other agents. 🟨
- **Intent Vector** — "structured representation of a communication goal"; **only example
  fields** listed (task type, priority, resource needs). **No schema.** ❌ → 📌 we define
  one (WIRE_FORMAT §5).
- **Semantic Router (SR)** — interprets intent metadata, enforces semantic forwarding. 🟨
- **Semantic Forwarding Table (SFT)** — maps intent categories → next hops + constraints.
  **No record layout.** ❌ → 📌 WIRE_FORMAT §6 / `internal/sft`.
- **Semantic prefix** — **not in the terminology section**; §6 only says the Intent
  Controller issues "a unique semantic prefix" at registration. **No format.** ❌ → 📌 we
  define a hierarchical, aggregatable encoding (WIRE_FORMAT §4).

### The five procedures (§6, Figure 2) 🟨 — implemented end-to-end (brief §5.7)
1. **Agent Registration** — authenticate to Intent Controller, register capabilities,
   receive credentials + a semantic prefix.
2. **Intent Submission** — submit an Intent Vector; Policy Engine parses it.
3. **Policy Translation** — compile router rules from trust + objectives (target domains,
   quality preferences, security constraints).
4. **Routing Execution** — router uses the SFT to choose next hops, weighing
   trust/latency/semantic relevance over plain IP reachability.
5. **Telemetry Feedback** — Telemetry Agent → Trust Manager; trust recomputed periodically;
   policy adjusts when thresholds are crossed.

### Principles (§5; interop in §1/§4/§7) 🟨
- **Semantic Composability** — intents decomposed/recombined for multi-step workflows.
- **Trust Anchoring** — dynamic trust so compromised agents can't distort routing.
- **Closed-Loop Adaptation** — "Every policy or path update is verified through telemetry."
- **Interoperability / backward-compat** — §5: *"SAR MAY extend BGP, IS-IS, or gRPC metadata
  to distribute semantic and trust information."* §4: metadata may ride on **BGP communities
  or SRv6 tags** (no encoding). §1: coexist with IP/BGP/service mesh. §7: BGP/IS-IS/SRv6
  integration is future work. **No mechanism anywhere.** ❌ → 📌 we pick **OSPF Opaque LSAs**
  as the carrier (ADR rationale: link-state already floods typed opaque data), and only
  *document* a BGP-community / SRv6 mapping (brief §5.3, WIRE_FORMAT §5.3).

### What an implementer must invent ❌ (all of it)
Intent Vector schema; SFT record layout and match semantics; semantic prefix format and
allocation; trust formula/thresholds; all message/header formats and control-plane
transport; concrete BGP-community/SRv6/IS-IS encodings; identity-cert format and
authn/authz; every IANA code point.

---

## 2. draft-yang-dmsc-gateway-semantic-layer — the Gateway Mediation Layer

- **-01:** "Semantic Layer for AI Agent Gateways in Gateway-mediated Collaboration" (short:
  "Gateway Semantic Layer"), June 2026, **Informational**, DMSC.
- **-02:** "Gateway Mediation Layer for AI Agent Collaboration," July 2026, Informational.
- Both fetched from the IETF archive HTML. **Section numbering for Inputs/Operations/
  Outputs/Non-goals/Privacy is stable across -01 and -02.**

> **Nothing here is a normative wire schema.** All artifacts are prose; field lists are
> informative; reason codes are explicitly deferred to *possible future IANA registries*
> (§12/§15). We implement the **-02 names** as primary and keep a -01 alias table.

### -01 Inputs (§8, prose) 🟨
- **§8.1 Business Intent** — the interaction goal; must be read with context/policy/
  capability/trust, not as a bare string.
- **§8.2 Task Context** — domain, service scope, time window, data sensitivity, operational
  state, user/tenant constraints.
- **§8.3 Capability Descriptions** — sources: A2A Agent Cards, MCP descriptors, ANML/other
  metadata, local registration payloads, gateway-managed directories.
- **§8.4 Policy Constraints** — authorization scope, data-exposure limits, privacy, tenant
  policy, domain policy, regulatory constraints, operational limits.
- **§8.5 Trust Evidence** — identity status, authorization/delegation, provenance,
  attestation, receipts, registry validation.

### -01 Operations (§9) 🟨
Normalize meanings into a **Gateway-managed Capability View**; match intent to capability;
apply policy and trust to filter/constrain candidates; prepare handoff semantics; explain
decisions/failures (retry, escalation, audit, accounting, troubleshooting).

### -01 Outputs (§10) 🟨
- **§10.1 Resolution Result** — selected/rejected candidate; decision reason; profile/
  version refs; applicable scope; policy/trust constraints.
- **§10.2 Handoff Descriptor** — task meaning, context summary, disclosure limits,
  interaction mechanism, target reference, failure semantics (used *with* A2A/MCP/IAIP/HTTP,
  not replacing them).
- **§10.3 Failure Explanation** — distinguishes failure categories.

### Failure / reason codes — ⚠️ no registered codes; prose categories, split across §10.3
and §12 (which differ slightly) 🟨
Union of both sections (and both revisions): **capability mismatch · intent
ambiguity/misunderstanding · parameter semantic mismatch · semantic profile mismatch ·
policy constraint failure · trust insufficiency · stale capability information · unavailable
context · context-version mismatch · handoff preparation failure.** 📌 We adopt this union as
our `FailureRecord.reason_code` enum (WIRE_FORMAT §7.3) and label it experimental.

### -02 renames (§8/§10/§12) ✅ (confirmed, with the Evidence Reference addition)
- Inputs restructured as a single **Mediation Request** (§8): **Goal** (§8.1, outcome only —
  no agent/tool/endpoint/protocol fixed), **Context** (§8.2), **Registered Capability
  Profiles** (§8.3), **Policy Scope** (§8.4), **Trust Binding** (§8.5).
- **Resolution Result → Mediation Response** (§10.1/§12): adds selected target reference,
  source-description refs, validation status.
- **Handoff Descriptor → Handoff Context** (§10.2/§12): adds policy/trust constraints and
  profile references.
- **Failure Explanation → Failure Record** (§10.3/§12): category-defined, no field list.
- **Evidence Reference (NEW, §10.4)** — links a decision/handoff/failure to evidence
  (source descriptions, validation status, delegation, attestation, receipts, provenance,
  audit refs) **without embedding the evidence**. 📌 drives our `EvidenceReference` type and
  the "never inline raw evidence" rule.

### Non-goals (§6) ✅
No universal ontology; no full domain-ontology library; **no new agent communication
protocol**; does **not** replace A2A/MCP/Agent Cards/ANML/ATN/DAWN/OAuth/WIMSE/TLS/DNS/HTTP;
no discovery query protocol or global registry; no orchestration/session protocol; no
authn/authz/attestation/trust framework; no audit/payment/settlement system; no internal
gateway product architecture. **Matching/ranking is left to the implementation**; the
gateway only *prepares* handoff for A2A/MCP/IAIP/HTTP. 📌 Our gateway module obeys this: it
produces a HandoffContext; **adapters** perform the actual protocol.

### Privacy rule (§14) ✅
Minimal disclosure. Handoff carries only task meaning, context summary, and needed
disclosure limits. Failure records "should support retry and troubleshooting without
exposing [unnecessary] raw prompts, personal data, confidential business information, or
internal policy details." 📌 Enforced by redaction at the gateway and a SECURITY.md test.

---

## 3. draft-cui-semantic-routing-ai-agent-communication-00 — selection model + threats

- Reached via datatracker + the -00 HTML archive. Authors Cui, Chao, Du. Individual draft.
- An **application-layer** framework to select exactly **one Processing Target** (agent/
  model/tool/workflow/service) per task. Defines an abstract information model + selection
  behavior for protocol bindings to carry. **Out of scope:** task decomposition,
  orchestration, discovery, invocation, and any new protocol.

### Information model 🟨
- **Routing Input (§5.1)** — `routing_id`, `intent` (description + optional required
  capabilities), optional `context`, mandatory `constraints`, ordered `preferences`
  (preferences only reorder; they never make an ineligible target eligible). 📌 ≈ our Intent
  Vector + constraints.
- **Target Profile (§5.2)** — `target_id`, `profile_version`, `target_type`, `capabilities`,
  `status`, optional `endpoint`/`expires_at`. "Claims are not proof of trust or
  authorization." 📌 ≈ our Agent Capability LSA content (self-asserted, kept separate from
  the validated view).
- **Routing Decision (§5.3)** — exactly one `selected_target`, plus `decision_id`, optional
  `reason_codes`, `ignored_preferences`, **required `expires_at`.** 📌 ≈ our SFT entry +
  expiry, and `resolve-intent` output.
- **Semantic Routing Function / SRF (§2)** — logical function: authorize → validate →
  enforce constraints/policy → evaluate semantic fit → return Decision or Error. Not a
  mandated algorithm. 📌 ≈ our `internal/spf` semantic stages + `internal/policy`.
- **Secure fallback (§3.2, §6.7)** — on target unavailability, start a *new* operation with
  the original intent/constraints; replacement must satisfy all original mandatory
  requirements; never weaken security/privacy; don't retry side-effecting tasks unless
  idempotent; a decision is **not** a capacity reservation; expired decisions must not be
  used. 📌 drives SFT entry expiry + the fallback path.

### Threats (§12) ✅ — fold into SECURITY.md
- **§12.2 Target enumeration & policy probing** — responses/errors leak which targets/
  capabilities/policies exist → limit views and response detail by authorization.
- **§12.3 Profile-integrity** — capability claims are untrusted unless source + integrity
  verified; attacker may craft a profile matching many intents.
- Also: **§12.4** semantic manipulation · **§12.5** confused deputy & tenant isolation ·
  **§12.6** endpoint safety · **§12.7** decision integrity & replay · **§12.8** DoS.

---

## 4. draft-zhang-dmsc-ioa-semantic-interaction-03 — the ontology + JSON-LD

- Latest is **-03** (HTML archive). Ontology-based semantic layer for the Internet of
  Agents. **Out of scope:** authorization, transport, session, directory sync.
- **Namespace** `https://example.org/ioa/semantic#`, prefix `ioa`. 📌 Our `/ontology`
  `@context` imports/aligns to this and adds a project namespace for experimental terms.

### Common ontology (§5) ✅
- **Classes (§5.1):** `ioa:Agent`, `ioa:Capability` (function/service with I/O
  constraints), `ioa:Intent` (goal + needed capabilities/conditions), `ioa:Task` (work unit
  tied to intent+context), `ioa:Context` (domain + interpretation conditions),
  `ioa:SemanticProfile`; also `ioa:Message`, `ioa:Domain`, `ioa:Concept`, `ioa:TermBinding`.
- **Properties (§5.2):** `hasCapability`, `providedBy`, `inputs`, `outputs` (media types);
  `appliesToDomain`, `requiresCapability`; `hasIntent`, `hasContext`; `domain`,
  `termBindings`, `term`, `sense`, `profileContext`; versioning `hasProfile`, `ontologyId`,
  `ontologyVersion`, `profileVersion`, `capabilityRefs`, `intentRef`, `alignmentRef`.
- **Constraints (§5.3–5.4):** a baseline task profile needs one `ontologyId`, one non-empty
  `ontologyVersion`, ≥1 `capabilityRefs`, one `intentRef`. **Versions are explicit
  identifiers, not ordered semver.** An unavailable required external constraint →
  **Indeterminate**.

### JSON-LD profile (§6) ✅
JSON-LD 1.1 recommended; inline `@context` is `@protected`; **remote contexts need trusted
sources, time/size budgets, SSRF protection**; decision-sensitive terms must not be silently
dropped on expansion; receivers must not guess identifiers or repair malformed input. 📌
This maps straight onto our ontology-loader security requirements.

### Alignment of heterogeneous ontologies (§7) ✅
- **Outcomes (§7.1):** **Compatible / Incompatible / Indeterminate.** Matching ontology IDs
  or major versions do **not** by themselves establish compatibility.
- **§7.2 alignment artifacts** must name source/target ontologies+versions, direction,
  scope; signature validity ≠ issuer authorization; reverse/chained mappings invalid unless
  explicitly defined; a confidence score can't substitute for a missing condition.
- **§7.3 conflict resolution** / **§7.4 candidate generation** — lexical/structural/
  model-assisted candidates are optional and never by themselves establish compatibility.
  📌 Our capability matcher returns a three-valued result (match / no-match / indeterminate),
  not a bare boolean, and treats embedding similarity strictly as a *fallback* candidate.

---

## 5. PL-IETF/AI-Agent-Communication-Networking — the index

Index/landing page for the **IETF 123 side meeting "AI-Agent Communication Networking Use
Cases and Protocols."** Points to the drafts below (also covered individually). Agenda
themes: 6G agent use cases, Agent ID, Agent Name Service (ANS), agent task management,
agentic web (W3C), multimodal agents. We use **ANS + Agent-ID** ideas for identifiers and
registration (brief §2).

---

## 6. draft-sz-dmsc-iaip-02 — IAIP (most implementation-ready input)

- "Intent-based Agent Interconnection Protocol at Agent Gateway," rev **-02** (May 2026),
  Standards Track, DMSC. Routes requests between agents by **semantic intent**, not fixed
  addresses. Agents ↔ an **Agent Gateway (AG)**.
- **AG components:** **AAC** (Agent Access Component: terminates sessions, authenticates
  sender, checks integrity/syntax) · **LCR** (Local Capability Registry: capability profile,
  endpoint, status per agent) · **IFR** (Intent Forwarding Routing: pulls LCR candidates,
  filters by constraints, ranks by semantic match).
- **Flow:** agent registers → gets an **Agent Identity Code (AIC)**; partners publish
  capabilities via `CAP_ADV`; a Leader Agent sends signed `INTENT_REQ`; AG returns a ranked
  candidate list; Leader verifies, picks a partner, then talks **directly end-to-end** — the
  **AG is a control-plane broker, NOT in the data path.**
- **Messages:** TLV format; common header = VERSION, MSG_TYPE, MSG_ID, SENDER_ID,
  TTL/EXPIRY, CORRELATION_ID, INTEGRITY_TAG. Types: `CAP_ADV`, `CAP_REFRESH`,
  `CAP_DEREGISTER`, `INTENT_REQ`, route responses, `ROUTING_FEEDBACK`, `ERROR`,
  `SECURITY_EXT`. **LCR states:** ACTIVE / PENDING_VERIFICATION / deprecated / SUSPENDED /
  UNREACHABLE — only **ACTIVE** agents receive routed intents.
- **Identity/auth:** TLS to AG; sender ID matched to client cert or `AUTHN_INFO`; intent
  signatures use registration credentials; AG signs candidate lists. Trust scope limits
  discoverability. Loop prevention via TTL + MSG_ID dedup. Transport TCP/TLS mandatory (4-
  octet length prefix), QUIC optional. Security + IANA sections TBD.

📌 **This is the closest existing analogue to S-OSPF's control plane**, and the single most
useful borrow: *broker/flood the control plane, route data end-to-end.* The mapping is in
TRACEABILITY (`CAP_ADV` ≈ Agent Capability LSA; AIC ≈ our 128-bit Agent-ID; LCR ≈ the
semantic LSDB/SFT; IFR ≈ semantic SPF; AG states ≈ agent reachability states).

---

## 7. Supporting drafts (abstract-only unless noted) — identity, discovery, tasks

| Draft | Status | Core idea we borrow |
|---|---|---|
| **draft-rosenberg-ai-protocols** | EXPIRED → *draft-rosenberg-aiproto-framework* | Framework for agent comms; surveys **MCP, A2A, Agntcy**. Check successor for the full taxonomy. |
| **draft-narajala-ans** | EXPIRED → *draft-narajala-courtney-ansv2* | **Agent Name Service**: DNS-inspired, protocol-agnostic directory; **PKI-cert identity**; capability-aware resolution; Protocol Adapter Layer (A2A/MCP/ACP). → our **Semantic Prefix** naming + cert-bound Agent-ID. |
| **draft-yl-agent-id-requirements** | EXPIRED | Case for a universal, interoperable, secure agent **digital identity** (requirement list is in full text). |
| **draft-yao-agent-auth-considerations** | current (abstract-only) | **Extends OAuth**; new authn/authz workflows for Agent Communication Networks. → optional delegated-authz layer above mTLS. |
| **draft-stephan-ai-agent-6g** | → *draft-stephan-agentproto-6g-usecases* | Use cases from **3GPP TR 22.870** (agent↔agent, agent↔tool). → demo/use-case framing. |
| **draft-cui-ai-agent-task** | current, rev -01 | **Task structure**: Metadata (id/type/priority/times/initiator) · Objective (NL goal) · Constraints · Artifact specs. **Lifecycle:** submitted/running/suspended/completed/canceled/rejected/failed; least-privilege context. → shape of our semantic payload + task states. |
| **draft-xie-ai-agent-multimodal** | EXPIRED | Multimodal channel negotiation (text/file/audio/video/sensor), adaptive QoS. → future Telemetry metric dimensions. |

---

## 8. Net design consequences (feeds TRACEABILITY + WIRE_FORMAT)

1. **We own every wire format.** No source defines one. Everything goes in experimental /
   private-use ranges and is labeled "S-OSPF experimental."
2. **Carrier = OSPF Opaque LSAs** (type 10 area-scoped, type 11 AS-scoped), TLV-encoded. The
   SRA's "MAY extend BGP/IS-IS/gRPC" is satisfied by a *documented mapping only* (brief §5.3).
3. **Flood digests, not documents.** Capability descriptions (full JSON-LD) are fetched on
   demand by digest; only a content digest + fetch hint is flooded (brief §5.2; ADR-0003).
4. **Three-valued semantics.** Capability/alignment checks return Compatible/Incompatible/
   **Indeterminate** (draft-zhang §7), never a bare boolean.
5. **Gateway prepares, adapters perform.** The Gateway Mediation Layer emits
   MediationResponse/HandoffContext/FailureRecord/EvidenceReference (draft-yang -02) and
   never reimplements MCP/A2A/IAIP/HTTP (draft-yang §6).
6. **Broker the control plane, route data end-to-end** (IAIP). The router/LSDB is the
   broker; agents talk directly after resolution.
7. **Trust & capability authenticity need their own signatures** (Ed25519) beyond packet
   HMAC, because capability *claims* are untrusted (draft-cui §12.3) and a compromised router
   must not forge another's agents (SRA Trust Anchoring).
8. **Reason codes** = the draft-yang union set, as an experimental enum pending a future
   registry.
