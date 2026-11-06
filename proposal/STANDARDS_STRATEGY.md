# S-OSPF — Standards Strategy

## Honest framing

The drafts S-OSPF builds on are **individual Internet-Drafts with no IETF
standing** — several are expired or purely architectural, and none defines a wire
format (see `docs/SPEC_NOTES.md`). S-OSPF's own wire design is therefore **our
invention**, marked experimental, using private-use code points. Nothing here is
a standard, and we will not represent it as one.

## Where this fits in the IETF

| Concern | Likely venue |
|---|---|
| Agent communication use cases & architecture | **DMSC** BoF / side meetings (where SRA, Gateway Mediation, IAIP are discussed), and the PL-IETF index group |
| The **link-state extensions** (new Opaque LSA types, semantic TLVs) | **LSR** (Link-State Routing) WG — this is where OSPF/IS-IS extensions belong |
| Routing-wide architecture questions | **RTGWG** |
| BGP-LS export of semantic topology | **IDR** |
| Agent identity / auth we depend on | the Agent-ID / ANS / OAuth-extension drafts (track, don't duplicate) |

## Draft we would submit

`docs/draft-sospf-semantic-link-state-00.md` — an **experimental** individual
submission describing the Opaque-LSA families, semantic-prefix encoding, the
three-stage computation, the trust model, and security. It requests experimental
code points only and states plainly that it has no consensus.

## Sequencing

1. **Socialize** at a DMSC side meeting: present the reference implementation and
   benchmarks as an existence proof that "route by meaning on a link-state base"
   is buildable.
2. **Split the work**: take the *semantic selection / gateway* concepts to DMSC,
   and the *IGP encoding* to LSR as a separate draft — the IETF keeps IGP
   mechanics in LSR regardless of the application.
3. **Interop first**: demonstrate that semantics-disabled S-OSPF decodes in
   standard OSPF tooling and that Opaque LSAs are flooded transparently by
   unmodified OSPF speakers — the strongest argument for the Opaque-LSA choice.
4. **Argue the layering question openly** (`RISKS_AND_OPEN_QUESTIONS.md`): whether
   semantic matching belongs in the IGP at all, or only at the gateway, and offer
   both the full-IGP variant and the lighter digests-only variant.

## What we are *not* claiming

- Not claiming IETF adoption, consensus, or any registry allocation.
- Not claiming to standardize MCP/A2A/OAuth/TLS — we prepare handoffs for them.
- Not claiming the trust formula or reason codes are anything but S-OSPF's own
  experimental choice pending a working group's registries.
