# ADR-0005: Isolate the semantic plane so it cannot delay IP routing

- **Status:** Accepted
- **Date:** 2026-11-06
- **Phase:** 9 (performance hardening)

## Context

The whole point of S-OSPF is to make agent routing *smarter*, not to make the
underlying network *slower*. A valid objection: with up to millions of agents,
the flood of Agent Capability, Trust and Telemetry updates could swamp a router's
control plane and delay ordinary IP routing (adjacency maintenance and SPF
convergence). If the AI layer slows the packet layer, the project fails its own
purpose.

## Decision

Treat the semantic plane as **strictly lower priority, bounded, and off the IP
critical path**, enforced in three ways in `internal/node`:

1. **Separate queues.** Core routing messages (Hello, DD, LSR, LSU of topology
   LSAs, Ack) use the high-priority `inbox`; semantic updates use a separate
   `semInbox` (`packet.SemanticUpdate`).
2. **Core-first scheduling.** Every scheduling cycle calls `drainCore()` — which
   handles *all* pending core messages — before any semantic work. Adjacency and
   topology can never wait behind semantic traffic.
3. **A per-cycle semantic budget.** At most `SemanticBudget` (default 256)
   semantic updates are processed per cycle via `processSemanticBatch`. A flood
   is absorbed a bounded chunk at a time, always leaving the router free for
   routing. Under sustained overload the `semInbox` simply drops excess —
   semantics are soft state re-synced by periodic anti-entropy, so a drop costs
   freshness, never correctness.

Additionally, semantic updates land in a store (`semDB`) that is **separate from
the topology LSDB and never read by the SPF computation**. Therefore semantic
churn **cannot trigger an IP shortest-path recomputation** — the most expensive
routing operation — no matter how fast agents change.

## Rationale

- **Correct priority.** The IGP's job (loop-free IP reachability, fast
  reconvergence) is sacred; the semantic layer is value-add that must yield to it.
- **Bounded work per cycle** gives a hard ceiling on control-plane time spent on
  semantics, so worst-case routing responsiveness is predictable regardless of
  agent count.
- **Soft-state drop-under-overload** is the standard safe behavior for bulk
  advertised state: lose freshness, never stability.
- **SPF isolation** means the costly computation is driven only by topology, as
  in classic OSPF.

## Consequences

- Under extreme semantic load, agent/trust information may lag (bounded by the
  budget and queue), but **IP routing stays responsive** — the intended trade-off.
- Scaling to very large agent populations still relies on area scoping + ABR
  Bloom/summary aggregation (ADR-0003) and, optionally, the gateway-only variant
  (RISKS_AND_OPEN_QUESTIONS) so individual agents never flood globally. This ADR
  bounds the *per-router* cost; those bound the *network-wide* cost.
- `TestCoreRoutingIsNotStarvedBySemanticFlood` locks in the guarantee: a 100k
  semantic backlog does not delay a core Hello→DD, and semantic processing is
  exactly the budget per batch.
