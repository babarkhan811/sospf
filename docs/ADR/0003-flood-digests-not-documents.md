# ADR-0003: Flood capability digests, not full capability documents

- **Status:** Accepted
- **Date:** 2026-11-06
- **Phase:** 1 (required justification, brief §5.2)

## Context

Agents advertise rich capability descriptions: full JSON-LD documents aligned to the IoA
ontology (draft-zhang §5/§6), possibly with embeddings. Link-state flooding replicates every
LSA to every router in the scope (area or AS) and stores it in every LSDB. OSPF LSAs are
also bounded by IP MTU when fragmented across the network, and LSDB size drives SPF and
memory cost.

If we flooded whole capability documents:

- **Scale:** LSDB size would grow with (agents × document size), not just agent count. A
  swarm of thousands of agents with multi-KB JSON-LD profiles would bloat every router.
- **MTU:** large LSAs stress fragmentation and refresh churn.
- **Churn:** any capability text edit re-floods the whole document.

## Decision

Flood a **content digest plus a fetch hint**, not the document:

- The **Agent Capability LSA** (WIRE_FORMAT §5.1) carries a **Capability Content Digest +
  Fetch Hint TLV (0xF004)**: `{hash_alg, digest, fetch_endpoint_hint}`, plus small
  structured fields needed for *first-pass* matching (ontology IRI ref, optional size-capped
  int8 embedding, status, domain, disclosure level).
- The **full JSON-LD document** is retrieved **on demand** over the gRPC API by digest and
  **cached by digest** (`internal/semantic` cache). Two agents with identical capabilities
  share one cached document.
- The **Semantic Prefix Summary LSA** (§5.5) carries a **Capability Bloom Filter** so a
  remote area can test "could capability X exist under this aggregated prefix?" before
  fetching anything.

Hash: SHA-256 (truncatable to 128 bits for the digest field if size pressure demands;
default full 256-bit). Bloom parameters sized for a target false-positive rate of ~1% at the
expected agents-per-summary; `m`, `k`, and a per-LSA `salt` are carried in the TLV.

## Rationale

- **LSDB size tracks agent count, not document size.** This keeps flooding and SPF costs
  governed by topology, matching OSPF's design assumptions.
- **Deduplication.** Caching by digest collapses identical capabilities across agents.
- **Cheap refresh.** A capability edit changes the digest; routers notice the change and
  fetch lazily only if/when they need the document for a resolve-intent.
- **Bloom pre-filter.** Avoids fetching across areas for capabilities that certainly aren't
  present (no false negatives).
- Consistent with how modern link-state extensions avoid flooding bulk data and with the
  brief's explicit instruction (§5.2).

## Consequences

- A **fetch path** is required: `internal/api` serves capability documents by digest;
  `internal/semantic` caches them with an LRU keyed by digest, bounded in size.
- **Availability:** if the originating router is unreachable, a document may be
  un-fetchable; matching then proceeds on the flooded structured fields and returns
  **Indeterminate** rather than guessing (draft-zhang §7 three-valued outcome).
- **Integrity:** the fetched document must hash to the flooded digest, or it is rejected —
  this binds the lazily-fetched body to the signed LSA (WIRE_FORMAT §8.2).
- **Benchmarks** (proposal pack) must report LSDB size vs. agent count to validate the
  scaling claim.
- Bloom false positives cause occasional needless fetches — acceptable; false negatives are
  impossible by construction, so no agent is wrongly hidden.
