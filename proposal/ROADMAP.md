# S-OSPF — Roadmap: MVP → Pilot → Production

## Where we are (MVP, this repo)

- Classic OSPF core (point-to-point) over an in-process emulator; 3-router
  convergence and link-failure reconvergence tested.
- All five semantic LSAs encode/decode (fuzzed); semantic prefixes + ABR Bloom
  summaries; three-valued capability matching; constrained resolver + SFT.
- Trust Manager (EWMA + decay + dampening); Policy DSL; Gateway Mediation Layer.
- Security: HMAC + replay, Ed25519 signatures, rate limits, Sybil guard.
- Python agent SDK + MCP/A2A handoff adapters; `sospf-demo` end-to-end re-route.
- Benchmarks: 43 ms (3) → 340 ms (50) convergence; ~69 µs resolve / 100 candidates.

## Pilot (next)

1. **Real transport.** Finish the UDP overlay and raw IP proto-89 transports;
   per-neighbor LSA retransmission lists and explicit acks (replacing the
   emulator's anti-entropy sync); Wireshark OSPF-dissector validation via a pcap
   writer.
2. **DR/BDR + Network LSAs** for broadcast segments; full area/ABR summarization
   for semantic prefixes across areas.
3. **gRPC/REST daemon API** so `sospfctl` show-commands and agent SDKs talk to a
   live `sospfd`; JSON-LD ontology loading with SSRF-guarded remote `@context`.
4. **Capability document fetch-by-digest** service + cache (the digest is already
   flooded; wire the fetch path and digest verification).
5. **Scale hardening** to hundreds of agents per area: validated flooding
   back-pressure, SPF/trust throttling under load (the 50-router emulator case is
   reliable today but the in-process model is not the production target).

## Production

1. **Choose the data-plane path:**
   - *(a)* Harden the Go daemon for operations (HA, config, hitless restart), or
   - *(b)* **FRRouting integration** — implement the classic core as an FRR module
     and carry semantic Opaque LSAs as an FRR plugin, keeping the semantic control
     plane as a Go sidecar. Inherits real OSPF interop and a path into existing
     networks.
2. **IS-IS variant** of the semantic TLVs (same concepts, IS-IS TLV encoding) for
   operators who run IS-IS as their IGP.
3. **BGP-LS export** so an SDN controller / RR can collect the semantic topology
   for centralized computation; map the ASH to SRv6 SRH TLVs / BGP large
   communities (mapping already documented, WIRE_FORMAT §5.3).
4. **OPA/Rego policy backend** enabled for operators standardized on Rego.
5. **Observability**: real Prometheus client, OpenTelemetry traces for intent
   resolution, Grafana dashboards.

## Vendor / standards integration path

- FRRouting plugin (OSPF Opaque + sidecar) — the lowest-friction way into
  production networks.
- IETF experimental draft for the wire design (see `STANDARDS_STRATEGY.md`).
