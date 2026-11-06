# FLOWS — the five SRA procedures as sequence diagrams

These realize draft-li §6 (SPEC_NOTES §1) end to end (brief §5.7). Diagrams are
Mermaid; they render on GitHub.

## 1. Agent Registration

```mermaid
sequenceDiagram
    participant A as AI Agent
    participant R as Semantic Router (Intent Controller)
    participant S as SybilGuard / Trust Manager
    A->>R: register (mTLS client cert, capability profile)
    R->>R: authenticate cert, bind 128-bit Agent-ID
    R->>S: Admit(router) — per-router Sybil cap
    S-->>R: ok
    R->>R: assign Semantic Prefix (domain/category/.../agent)
    R-->>A: credentials + Semantic Prefix
    R->>R: originate Agent Capability LSA (Opaque 128), flood
```

## 2. Intent Submission

```mermaid
sequenceDiagram
    participant A as AI Agent
    participant R as Semantic Router
    participant L as Rate limiter
    A->>R: submit IntentVector (task IRI, constraints, optional embedding)
    R->>L: Allow(requester)?
    L-->>R: ok (else drop — anti-enumeration)
    R->>R: normalize to Gateway-managed Capability View
```

## 3. Policy Translation

```mermaid
sequenceDiagram
    participant R as Semantic Router
    participant P as Policy Engine (DSL)
    loop each candidate agent
        R->>P: Evaluate(intent category, domain, tenant, trust, disclosure)
        P-->>R: Allow | Deny | Constrain(min_trust, max_disclosure)
    end
    R->>R: drop denied; tighten constraints for constrained
```

## 4. Routing Execution

```mermaid
sequenceDiagram
    participant R as Semantic Router (SPF + Resolver)
    participant T as Trust Manager
    participant F as Semantic Forwarding Table
    R->>R: Stage 1 — Dijkstra SPF over LSDB (IGP cost, next hop)
    R->>R: Stage 2 — capability match (exact/subclass/embedding) -> Compatible/Incompatible/Indeterminate
    R->>T: trust per candidate
    R->>R: Stage 3 — CSPF prune hard constraints; rank by composite cost; top-k ECMP
    R->>F: install SFT entry (next hop, target, cost, expiry)
    R-->>R: RoutingDecision (one target + expiry) or FailureRecord(reason)
```

## 5. Telemetry Feedback (closed loop)

```mermaid
sequenceDiagram
    participant Te as Telemetry Agent
    participant D as Damper (hysteresis)
    participant T as Trust Manager (EWMA)
    participant R as Router
    Te->>D: latency / success / load samples
    D-->>Te: report only beyond the band (no route flap)
    Te->>T: Update(agent, outcome)
    T->>T: EWMA + decay; NeedsReOrigination?
    alt threshold crossed (and MinLSInterval ok)
        T->>R: originate Trust LSA (Opaque 129), flood
        R->>R: re-resolve affected intents -> traffic re-routes
    end
```

## Gateway mediation (handoff preparation, draft-yang -02)

```mermaid
sequenceDiagram
    participant Up as Upstream agent
    participant G as Gateway Mediation Layer
    participant Ad as MCP/A2A/IAIP/HTTP adapter
    Up->>G: MediationRequest (goal, context, capability profiles, policy scope, trust binding)
    G->>G: policy gate + resolve + redact
    alt selected
        G-->>Up: MediationResponse(SELECTED) + HandoffContext (redacted) + EvidenceReference (pointers)
        Up->>Ad: perform handoff over the named mechanism
    else none
        G-->>Up: MediationResponse(REJECTED) + FailureRecord(reason, hint)
    end
```
