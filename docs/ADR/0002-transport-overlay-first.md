# ADR-0002: Transport — overlay (UDP) and in-process emulator first, raw IP proto-89 optional

- **Status:** Accepted
- **Date:** 2026-11-06
- **Phase:** 1

## Context

OSPF natively runs directly over IP as protocol 89, using multicast
(224.0.0.5 / 224.0.0.6) on broadcast segments. That requires raw sockets and elevated
privileges (`CAP_NET_RAW` / root / Administrator), is awkward on Windows (our primary dev
host), and is painful to test deterministically. We need fast, hermetic tests of flooding,
FSMs and SPF, plus a portable way to run multi-router demos without root.

## Decision

Support three transports behind one interface (`internal/packet` + a `Transport` port):

1. **In-process virtual-link emulator (default for tests).** Routers are goroutines; links
   are Go channels with configurable latency/loss/MTU. `sospf-emu` builds a topology from
   YAML. Fully deterministic, no sockets, no privileges.
2. **UDP overlay (default for `sospfd` runtime).** OSPF packets (with the §8.1 auth trailer)
   are carried in UDP datagrams on a configurable port; neighbors are configured or
   discovered via a small overlay hello. Runs unprivileged on Windows/Linux/macOS.
3. **Raw IP proto-89 (optional, build tag `rawip`).** Real OSPF framing for interop and
   Wireshark validation. Requires privileges; Linux-first.

The wire-level packet/LSA/TLV encoding (WIRE_FORMAT.md) is **identical** across all three;
only framing/addressing differs. The auth trailer (§8.1) is applied in all modes so there
is a single security code path.

## Rationale

- **Testability first.** Our test matrix (brief §8 — flooding, FSM flaps, 50-node
  convergence, no-flap-under-noise) is only practical with a deterministic in-process
  transport. Real sockets make these flaky and slow.
- **Portability.** The user develops on Windows 11; UDP overlay runs there with no special
  privileges, so `make demo` works everywhere.
- **Interop when it matters.** Raw mode exists so we can emit a pcap and confirm a
  semantics-disabled capture decodes in Wireshark's OSPF dissector (brief §8). It is not on
  the default path because it is not needed for the reference implementation's core value.

## Consequences

- One `Transport` interface with three implementations; packet codec is transport-agnostic.
- Overlay mode needs a tiny neighbor-bootstrap (static config or overlay hello) since there
  is no link-layer multicast; documented as an overlay-only addition, not part of OSPF.
- Raw mode is CI-tested on Linux only; Windows/macOS CI runs emulator + overlay.
- A future production path may replace overlay with raw or with an FRR integration
  (ADR-0001, ROADMAP) without touching the semantic layer.
