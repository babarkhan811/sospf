# TRUST_MODEL — S-OSPF experimental trust scoring

> **S-OSPF's own choice.** No source draft defines a trust formula (SPEC_NOTES
> §1: draft-li names a Trust Manager but specifies no computation). Everything
> here is an S-OSPF experimental design, labeled as such, and tunable.

Implemented in `internal/trust`. Trust is a value in **[0, 1]** per agent,
carried in Trust LSAs as 16-bit fixed-point (WIRE_FORMAT §5.2) and consumed by
the resolver as a hard floor and a soft ranking term (brief §5.4).

## Inputs: telemetry outcomes

Each interaction yields an `Outcome{Success, SLAAdhered, Anomaly}`. It maps to a
sample in [0, 1]:

```
sample = clamp( 0.6·[Success] + 0.4·[SLAAdhered] − AnomalyPenalty·[Anomaly] )
```

Defaults: `AnomalyPenalty = 0.5`. So a clean success+SLA = 1.0; a success that
missed SLA = 0.6; a failure with an anomaly flag ≈ 0.

## Update: EWMA

```
trust ← clamp( α·sample + (1−α)·trust )
```

Default `α = 0.3` — recent behavior matters but one bad sample cannot crater a
long good history. New agents start at the **floor** `NewAgentFloor = 0.5`
(neither trusted nor distrusted).

## Decay toward the floor

Trust that is not refreshed decays toward the floor with a half-life
(`DecayHalfLife`, default 10 min):

```
trust ← floor + (trust − floor)·2^(−Δt / halfLife)
```

So an agent that stops reporting slowly loses accumulated trust rather than
keeping a stale high score — and a once-bad agent is rehabilitated over time.

## Re-origination (closed loop)

The Trust Manager re-originates a Trust LSA only when trust has moved at least
`ReOriginDelta` (default 0.05) from the last advertised value, and the caller
still honors `MinLSInterval`. This bounds Trust-LSA churn (SRA Closed-Loop
Adaptation, brief §5.5).

## Dampening (no route flap)

Telemetry scalars (latency, load, success rate) pass through a `Damper`: an EWMA
plus a hysteresis band. A new Telemetry LSA is originated only when the smoothed
value crosses the band, so noisy measurements cannot flap routes
(`TestDamperSuppressesNoiseButPassesSustainedChange`; brief §5.4).

## Trust anchoring (integrity)

Trust and Capability LSAs are signed (Ed25519, Phase 7 / WIRE_FORMAT §8.2) so a
compromised router cannot forge another router's agents, and capability *claims*
are never taken as proof (draft-cui §12.3). Anti-gaming measures (Sybil limits,
signed evidence, self-report rate limits) are in SECURITY.md.

## Defaults summary

| Parameter | Default | Meaning |
|---|---|---|
| `Alpha` | 0.3 | EWMA weight on newest sample |
| `NewAgentFloor` | 0.5 | starting / decay-target trust |
| `AnomalyPenalty` | 0.5 | sample reduction when an anomaly is flagged |
| `DecayHalfLife` | 10 min | unreferenced trust half-life |
| `ReOriginDelta` | 0.05 | trust move that triggers a new Trust LSA |
| Damper `band` | per-metric | hysteresis to suppress telemetry noise |
