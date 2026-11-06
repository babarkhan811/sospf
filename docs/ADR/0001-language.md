# ADR-0001: Implementation language — Go

- **Status:** Accepted
- **Date:** 2026-11-06
- **Deciders:** S-OSPF core team
- **Phase:** 1 (Research & spec)

## Context

S-OSPF is an experimental link-state routing daemon (`sospfd`), a CLI (`sospfctl`) and an
in-process multi-router emulator (`sospf-emu`). The workload is:

- Many long-lived concurrent activities: one reader/writer pair per interface and per
  neighbor, flooding, aging timers, SPF recomputation, telemetry collection, and an
  agent-facing gRPC/REST API — all needing clean cancellation and timeouts.
- Byte-exact packet and TLV encoding/decoding that must round-trip and never panic on
  malformed input (we require fuzzing).
- A gRPC + protobuf agent API and JSON-LD payloads.
- A need for the project to be read, reviewed and extended by networking engineers who
  may later integrate it with production routing stacks.

## Decision

Use **Go 1.22+** for `sospfd`, `sospfctl` and `sospf-emu`.

Concretely:

- **Concurrency:** a goroutine per interface and per neighbor; `context.Context` threaded
  through every blocking call for cancellation and deadlines. **No global mutable state** —
  each subsystem (LSDB, SFT, trust) owns its data and is reached through a typed handle.
- **Transport:** default to an **overlay over UDP** and an **in-process virtual-link
  emulator** (Go channels) for tests. Raw IP proto-89 sockets are optional, behind a build
  tag, and require root / `CAP_NET_RAW`.
- **Tooling:** `go vet`, `staticcheck`, `golangci-lint`, `go test` with race detector,
  and Go native fuzzing (`go test -fuzz`) on every parser.
- **Observability:** `log/slog` for structured logs, Prometheus client for metrics,
  OpenTelemetry for intent-resolution traces.

## Rationale

1. **Concurrency model fits the problem.** OSPF is a set of communicating state machines
   with timers. Goroutines + channels + `context` map onto "one neighbor FSM per
   goroutine" far more directly than thread pools or async/await state machines, and the
   race detector catches the data races this design is prone to.
2. **Memory safety without a GC-latency problem at our scale.** A routing control plane is
   not a per-packet hot path (that would be the forwarding plane, which here is a table
   lookup). Go's GC pauses are irrelevant for LSDB/SPF work on topologies of tens to low
   hundreds of nodes.
3. **First-class gRPC/protobuf and a strong std-library net stack**, so the agent API and
   the UDP overlay need few third-party dependencies.
4. **Fuzzing is built in.** Our testing requirements (Section 8 of the brief) demand fuzz
   tests on all parsers; Go ships this in the toolchain.
5. **Reviewability.** Go is widely read in the networking/infra community (Kubernetes,
   Cilium, many SDN controllers), which matters for the eventual company proposal and for
   external review.
6. **Precedent.** Production control planes and SDN tooling (e.g. BGP implementations like
   GoBGP) show Go is viable for routing control software.

## Alternatives considered

### Rust
- **Pros:** no GC, stronger compile-time guarantees (ownership, exhaustive matching),
  excellent for byte-exact parsing (e.g. `nom`), strong story for a future high-performance
  forwarding plane.
- **Cons:** slower to write the many-state-machine control plane; async ecosystem
  (`tokio`) adds conceptual overhead versus goroutines; smaller overlap with the target
  reviewer community.
- **What we'd change for Rust:** model each FSM as an explicit state enum driven by an
  `async` task; use `nom` or `deku` for packet codecs; use `tonic` for gRPC. **We would
  reach for Rust if a line-rate data plane became an in-scope deliverable.**

### C with FRRouting integration
- **Pros:** FRRouting already implements conformant OSPFv2/v3, Opaque LSA, areas, SPF and
  wire interop — we would not reimplement the classic core, and we would inherit real
  interop and a path into production networks.
- **Cons:** steep contribution model; harder to iterate on experimental wire formats; C
  memory-safety burden on exactly the parsers we most need to fuzz; the semantic layer
  (ontology, JSON-LD, embeddings, trust) is awkward in C.
- **What we'd change for FRR:** implement the classic machinery as an FRR module and add
  our semantic Opaque LSAs as an FRR plugin; keep the semantic control plane as a separate
  Go/Rust sidecar speaking to FRR over its API. **This is captured as a roadmap item
  (`/proposal/ROADMAP.md`) for the production path, not the reference implementation.**

## Consequences

- We reimplement the classic OSPF core ourselves rather than inheriting FRR's. This costs
  Phase 2 effort but buys full freedom to carry experimental Opaque LSAs and to keep the
  emulator in-process for fast, deterministic tests.
- A future production deployment likely means either (a) hardening this Go daemon, or
  (b) porting the classic core onto FRR and keeping the semantic planes as a Go sidecar.
  Both paths are documented in the proposal pack.
- Contributors need a Go 1.22+ toolchain and the lint/fuzz tools listed above.
