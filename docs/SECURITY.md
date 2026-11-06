# SECURITY — S-OSPF threat model and mitigations

Covers the threats in brief §9 and the ones named by the source drafts
(draft-cui §12, draft-yang §13–14). Mitigations map to `internal/security`,
`internal/gateway`, `internal/trust` and the wire design (WIRE_FORMAT §8).

> **Rule:** all agent-supplied text (capability descriptions, intents, free-text
> goals) is treated as **data, never executed or interpolated** — including as a
> defense against prompt-injection payloads smuggled in capability fields
> (brief §9). Outputs are redacted (draft-yang §14).

| # | Threat (source) | Mitigation | Where |
|---|---|---|---|
| 1 | **Forged / poisoned capability advertisements** (cui §12.3) | Ed25519 signatures on Capability & Trust LSAs; a compromised router cannot forge another's agents. Capability *claims* are never proof — the gateway keeps a separate validated Capability View. | `security.SignLSA/Verify`, `gateway` validation_status |
| 2 | **Trust-score manipulation** (cui §12.4) | Trust derives from signed evidence digests; self-reported telemetry is rate-limited; Sybil cap per router; EWMA + decay resists single-sample gaming. | `trust`, `security.TokenBucket`, `security.SybilGuard` |
| 3 | **Target enumeration & policy probing** (cui §12.2) | `resolve-intent` is rate-limited per requester; responses are minimized (reason codes, not target inventories); failure text is generic. | `security.KeyedLimiter`, `gateway` redaction |
| 4 | **Decision integrity & replay** (cui §12.7) | Per-packet HMAC-SHA-256 over packet+KeyID+SeqNum; a monotonic crypto sequence number per neighbor rejects replays and reorderings. | `security.AuthTrailer`, `security.ReplayWindow` |
| 5 | **Context leakage across domains** (yang §14) | HandoffContext carries only task meaning + a shape-only summary, bounded by disclosure limits; no raw prompt/PII. | `gateway.redactedSummary` |
| 6 | **LSDB flooding DoS** (cui §12.8) | `MinLSArrival`/`MinLSInterval` pacing (LSDB), SPF throttle, per-neighbor and per-requester rate limits, Sybil cap bounding injected agents. | `lsdb`, `node`, `security` |
| 7 | **Compromised router** | Packet HMAC limits off-path injection; per-LSA Ed25519 limits a compromised router to forging only *its own* advertisements, not others'. Trust Anchoring keeps a bad node from distorting others' trust. | `security`, `trust` |
| 8 | **Prompt-injection in capability text** (brief §9) | All such text is data; never evaluated, never placed in a shell/LLM/template without escaping; signatures bind text to an origin. | project-wide rule |
| 9 | **Profile / semantic manipulation** (cui §12.4; zhang §7) | Three-valued matching returns **Indeterminate** rather than guessing; embedding similarity is a *fallback*, never sole proof; alignment artifacts must be explicitly defined (not inferred). | `semantic.Match`, resolver |
| 10 | **Confused deputy / tenant isolation** (cui §12.5) | Policy input carries tenant; per-candidate policy gating; scoped-token *reference* in handoff, never a bearer token inline. | `policy`, `gateway` |

## Key management

- **HMAC keys** are per-interface shared secrets, identified by a 16-bit Key ID,
  rotated by configuration; never flooded.
- **Ed25519 keys** are bound to a router / Trust-Manager identity, identified by
  a 16-octet Key ID, distributed out of band (a keystore), never flooded.
- Keys live outside the repo; `.gitignore` excludes `*.pem`, `*.key`, `keys/`.

## Residual risks (also in RISKS_AND_OPEN_QUESTIONS)

- **DNS-rebinding / connection-time divergence** for any fetch-on-demand of
  capability documents: the private-network guard checks before each request,
  but a hostile resolver can still diverge at connect time.
- **Bloom false positives** in Prefix Summary LSAs cause needless cross-area
  fetches (never false hiding).
- **Verification availability:** if an originating router is unreachable, a
  capability document may be unfetchable; matching degrades to **Indeterminate**
  rather than trusting an unverified claim.
