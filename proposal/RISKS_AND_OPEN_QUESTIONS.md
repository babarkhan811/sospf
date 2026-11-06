# S-OSPF — Risks and Open Questions

## The central open question: does semantic matching belong in the IGP?

Putting capability/trust/policy into a flooded link-state database is powerful
(one consistent, loop-free, fast-converging view) but debatable. We present
**both** options and let adopters choose:

- **Full IGP extension (what this repo implements):** routers flood capability
  digests, trust and telemetry, and compute semantic paths. Pro: a single
  authoritative distributed view, sub-second convergence, no central bottleneck.
  Con: couples a fast-moving application concern (agents, trust) to the IGP's
  change rate; flooding scales with agent churn.
- **Lighter "digests + gateway" variant:** routers flood only Prefix-Summary
  Bloom digests; the actual matching happens at gateways/app layer. Pro: keeps
  the IGP stable; matches the drafts' gateway framing. Con: gateways become the
  selection authority and a scaling/consistency concern.

Our design deliberately supports the lighter variant too (digests and Bloom
summaries are already first-class; the Gateway Mediation Layer already does
matching), so the choice is a deployment decision, not a rewrite.

## Scaling risks

- **Flooding with many agents.** Mitigated by flooding digests not documents
  (LSDB grows with agent *count*, not content). Still, thousands of high-churn
  agents stress any IGP; the digests-only variant and per-area ABR summarization
  are the pressure valves. The 50-router emulator converges in ~340 ms but the
  in-process emulator is **not** the production transport — real convergence
  depends on the UDP/raw transport and per-neighbor retransmission (roadmap).
- **Trust-LSA churn.** Bounded by `ReOriginDelta` + `MinLSInterval` and telemetry
  dampening, but aggressive SLAs could still churn; needs field tuning.
- **Bloom false positives** cause needless cross-area capability fetches (never
  false hiding) — acceptable, tunable via filter sizing.

## Trust-model gaming

- Sybil floods (bounded per router), self-reported telemetry inflation
  (rate-limited, signed evidence), and collusion remain partially open; the EWMA
  + decay resists single-sample gaming but a patient adversary is an open problem.
  Reputation systems are hard; this is an area for research, not a solved claim.

## Privacy

- Minimal disclosure is enforced (redacted summaries, evidence by reference), but
  capability prefixes themselves reveal structure; cross-tenant isolation relies
  on correct policy authoring. DNS-rebinding at capability-fetch time is a
  residual risk (guard checks DNS, not connect-time identity).

## Implementation honesty (known limitations today)

- **Simplified DD exchange** without per-neighbor retransmission; the emulator
  uses a periodic anti-entropy sync to guarantee completeness. Production needs
  the real OSPF retransmission lists.
- **No DR/BDR / Network LSAs yet** — point-to-point only.
- **LSA checksum** is a self-consistent Fletcher-16, not the exact RFC 2328
  byte placement (deferred to the raw-interop phase).
- **Embeddings** are matched locally; inline wire encoding is deferred (digest +
  optional capped vector).
- **Race detector** runs on Linux CI (needs CGO), not on the Windows dev box.

## Security residuals

See `docs/SECURITY.md`. Compromised-router forgery is limited (not eliminated) by
Ed25519 signatures; a router can still forge *its own* advertisements, which is
why trust is anchored to signed evidence and cross-checked, not taken on faith.
