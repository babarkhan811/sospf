# ADR-0004: Policy engine — small embedded DSL in v0, OPA/Rego as a pluggable backend

- **Status:** Accepted
- **Date:** 2026-11-06
- **Phase:** 1 (required justification, brief §4)

## Context

The Policy Engine (SRA li §4/§6; yang Policy Constraints §8.4) turns intents + trust into
enforceable routing rules and filters/constrains candidate agents during semantic
resolution (brief §5.4 stage 3). Policy must express things like: *allowed intent categories
per domain; minimum trust floor; disclosure limits; tenant/domain isolation; regulatory
compliance tags.* We need it to be auditable, testable, and safe (policy text is operator-
authored, but intents and capabilities are untrusted — cui §12).

Two broad options:

- **OPA/Rego** — a mature, well-known policy language and evaluator (`github.com/open-policy-
  agent/opa`), decoupled policy, rich tooling, familiar to platform teams.
- **A small embedded decision DSL** — a minimal, total (non-Turing-complete) rule language
  evaluated in-process, purpose-built for S-OSPF's decision inputs.

## Decision

Define a **`PolicyEngine` interface** in `internal/policy` and ship **two implementations**:

1. **`dsl` (default in v0).** A small, declarative, **total** rule language over a fixed,
   typed input struct (`{intent_category, domain, tenant, trust, disclosure_level,
   compliance_tags, …}`) producing `{allow|deny|constrain, reason_code, constraints}`. No
   loops, no unbounded recursion, no I/O — evaluation always terminates and cannot block the
   event loop or be used for policy-probing amplification.
2. **`rego` (pluggable, build tag `opa`).** Delegates to OPA with the same typed input and
   output contract, for operators who already run Rego and want to reuse existing policy.

Policy results feed CSPF-style pruning (hard constraints) and the composite ranking
(soft preferences). A denied candidate yields a `FailureRecord` with a reason code from the
WIRE_FORMAT §7.3 enum (e.g. `POLICY_CONSTRAINT_FAILURE`, `TRUST_INSUFFICIENCY`).

## Rationale

- **Safety and determinism first.** The DSL is total and sandboxed by construction: policy
  evaluation sits on the resolve-intent hot path and must never hang, allocate unboundedly,
  or leak which policies exist (cui §12.2 policy probing). A constrained DSL makes these
  properties provable and fuzzable.
- **Small dependency surface for the reference impl.** The default build needs no large
  third-party policy runtime; OPA is opt-in.
- **Auditability.** A fixed input schema + total rules means every decision is explainable
  (which rule fired → which reason code), satisfying yang's "explain decisions/failures"
  operation (§9) and the minimal-disclosure rule (§14).
- **Interoperability for adopters.** Teams standardized on OPA lose nothing: the `rego`
  backend honors the same contract, and the proposal/ROADMAP can pitch Rego for production.

## Consequences

- We specify and test the DSL grammar and evaluator (fuzzed; must never panic on malformed
  policy). Grammar lands with Phase 6.
- The input/output contract is frozen early so both backends — and the reason-code mapping —
  stay aligned; changing it is an ADR-worthy event.
- OPA stays an optional dependency (build tag), keeping default builds lean.
- Policy decisions are logged with the firing rule and reason code for audit, but **without**
  raw prompts/PII/internal policy detail in anything returned to agents (yang §14).
