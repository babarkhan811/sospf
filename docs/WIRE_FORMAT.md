# WIRE_FORMAT — S-OSPF experimental on-the-wire encoding

> **EXPERIMENTAL / PROVISIONAL.** Every code point, LSA type, TLV type, header and field in
> this document is **S-OSPF experimental** and **not defined by any IETF draft** (see
> SPEC_NOTES.md: the source drafts define *no* wire formats). All values are drawn from
> experimental / private-use ranges and are **provisional pending the Phase 1 checkpoint
> review**. Do not treat any number here as an IANA allocation.

This is the document the Phase 1 checkpoint approves before protocol code is written
(brief §7.1). It specifies: encoding conventions, the Opaque-LSA carrier, the TLV registry,
Semantic Prefix encoding, the five semantic LSA families, the Intent Vector, the Agent
Semantic Header (ASH), authentication/signatures, and the provisional code-point registry.

---

## 1. Conventions

- **Bit/byte order:** network byte order (big-endian), consistent with OSPF (RFC 2328).
  Diagrams use the RFC 32-bit ruler; bit 0 is the most significant bit of octet 0.
- **Alignment:** every TLV starts on a 4-byte boundary. The **Length** field counts the
  value in octets **excluding** padding (OSPF-TE convention, RFC 3630 §2). Padding octets
  are zero and are not covered by Length.
- **TLV shape:** Type (16 bits) · Length (16 bits) · Value (Length octets) · Padding (0–3
  octets to the next 4-byte boundary).
- **Forward compatibility (brief §5.2):** a receiver that does not recognize a TLV **MUST
  ignore it and MUST still flood the containing LSA unchanged.** Unknown *opaque types* are
  flooded but not acted on. This lets the registry grow without flag days.
- **Sub-TLVs** use the same shape and nesting rules, within a parent TLV's value.
- **Strings** are UTF-8, not NUL-terminated (Length-delimited). All agent-supplied strings
  are **data, never executed or interpolated** (SECURITY.md).
- **Fixed-point:** trust/score fields are unsigned fixed-point in [0,1]; an 8-bit field maps
  value `v` to `v/255`, a 16-bit field maps `v/65535`. The encoding is stated per field.

---

## 2. Carrier: OSPF Opaque LSAs (RFC 5250)

Semantic data rides in **Opaque LSAs**, reusing OSPF's existing flooding, aging and LSDB
machinery. We use:

- **Opaque type 10 (area-scoped)** — intra-area semantics (most LSAs below).
- **Opaque type 11 (AS-scoped)** — domain-wide semantics (policy, cross-area summaries).
- (Type 9 link-local is unused in S-OSPF v0.)

### 2.1 Opaque LSA header (20 octets, RFC 2328 §A.4.1 + RFC 5250 §3)

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|            LS age             |    Options    | LS type=10/11 |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  Opaque Type  |            Opaque ID (24 bits)                |   <- Link State ID
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                     Advertising Router                        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                     LS sequence number                        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|         LS checksum           |            Length             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                   TLV area (semantic payload)   ...           |
+---------------------------------------------------------------+
```

The **Link State ID** of an Opaque LSA is `Opaque Type (8 bits) || Opaque ID (24 bits)`.
S-OSPF **opaque type** allocations (experimental range 128–255):

| Opaque type | LSA family | Scope | Section |
|---|---|---|---|
| **128** | Agent Capability LSA | area (10) | §5.1 |
| **129** | Trust LSA | area (10) | §5.2 |
| **130** | Semantic Telemetry / TE LSA | area (10) | §5.3 |
| **131** | Policy LSA | AS (11) | §5.4 |
| **132** | Semantic Prefix Summary LSA | AS (11) | §5.5 |

**Opaque ID** usage: for per-agent LSAs (128/129/130) the low 24 bits carry an *instance
index* so one router can originate many (one per agent/metric group). Standard LSA
comparison (seq → checksum → age), MaxAge flushing and LSRefreshTime apply unchanged.

---

## 3. TLV registry (S-OSPF experimental)

Top-level and sub-TLV **Type** is 16 bits. S-OSPF uses the experimental block
**0xF000–0xFFFF**. Provisional allocations:

| Type | Name | Used in | Value summary |
|---|---|---|---|
| 0xF001 | Agent-ID | 128/129/130 | 16 octets (128-bit), cert-bound |
| 0xF002 | Semantic Prefix | 128/132, ASH | §4 encoding |
| 0xF003 | Capability Descriptor | 128 | sub-TLVs: ontology IRI ref, embedding digest, compact vector |
| 0xF004 | Capability Content Digest + Fetch Hint | 128 | hash alg + digest + fetch endpoint hint (ADR-0003) |
| 0xF005 | Protocol Endpoint | 128 | mechanism enum + URI (MCP/A2A/IAIP/HTTP) |
| 0xF006 | Domain | 128/131 | UTF-8 domain label |
| 0xF007 | Disclosure Level | 128/131 | 1 octet enum |
| 0xF008 | Agent Status | 128 | 1 octet enum (IAIP LCR states) |
| 0xF010 | Trust Score | 129 | 16-bit fixed-point [0,1] + scope |
| 0xF011 | Evidence Digest | 129 | hash alg + digest (no raw evidence) |
| 0xF012 | Issuer | 129 | issuer key-id (16 octets) |
| 0xF013 | Validity Window | 129 | not-before / not-after (uint32 epoch secs) |
| 0xF020 | Link Metrics | 130 | latency, jitter, success-rate, load |
| 0xF021 | Agent Perf Metrics | 130 | queue depth, cost, energy, success-rate |
| 0xF030 | Allowed Intent Categories | 131 | list of intent-category IDs |
| 0xF031 | Disclosure Limits | 131 | 1 octet enum + sub-TLVs |
| 0xF032 | Compliance Tags | 131 | list of UTF-8 tags |
| 0xF040 | Aggregated Semantic Prefix | 132 | §4 encoding (prefix-length significant) |
| 0xF041 | Capability Bloom Filter | 132 | m-bit filter + k + salt (ADR-0003) |
| 0xF042 | Trust Range | 132 | min/max 16-bit fixed-point |
| 0xF0FF | Ed25519 Signature | 128/129 (and any) | key-id (16) + 64-octet signature; **MUST be last** |

> **Rule:** the Signature TLV (0xF0FF), when present, is always the final TLV and signs
> everything before it (see §8.2).

---

## 4. Semantic Prefix encoding

A **Semantic Prefix** is a hierarchical, aggregatable name
`domain/category/subcategory/agent` (e.g. `medical/imaging/segmentation/agent-7f3a`). It
encodes as length-prefixed labels plus a *prefix length* (in **labels**, analogous to an IP
prefix length in bits) so that an ABR can summarize a set of agents by truncating to a
shorter label prefix — exactly as OSPF summarizes IP prefixes.

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| Prefix Len    |  Label Count  |           Reserved            |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| L0 Len        | L0 octets ...                                 |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| L1 Len        | L1 octets ...                                 |
+---------------------------------------------------------------+
| ... (Label Count labels total, whole area padded to 4 bytes)  |
+---------------------------------------------------------------+
```

- **Prefix Len** — number of leading labels that are *significant* for aggregation/match
  (0 ≤ Prefix Len ≤ Label Count). A Summary LSA sets Prefix Len < Label Count.
- **Label Count** — total labels present.
- **Lx Len / Lx octets** — each label: 1-octet length (1–63) + UTF-8 octets.
- Labels use the restricted charset `[a-z0-9-]` (IDNA-style); case-folded on compare.
- Longest-significant-prefix match wins at lookup (like LPM for IP).

---

## 5. Semantic LSA families

Each LSA's payload is the Opaque LSA TLV area (§2.1). Only representative TLVs are drawn;
full TLV list is §3. Capability *bodies* (full JSON-LD) are **not** flooded — see ADR-0003.

### 5.1 Agent Capability LSA (opaque type 128, area-scoped)

Advertises agents reachable via this router. Realizes: SRA capability registration (li §6),
IAIP `CAP_ADV`+LCR (§6), draft-cui Target Profile (§5.2), yang Capability Descriptions
(§8.3). **Self-asserted** — kept separate from the gateway's validated Capability View.

```
+--------------------- Opaque LSA header (type 10, opaque 128) --+
|  Agent-ID TLV (0xF001)            : 16 octets                  |
|  Semantic Prefix TLV (0xF002)     : §4                         |
|  Domain TLV (0xF006)              : UTF-8                      |
|  Agent Status TLV (0xF008)        : 1 octet enum (see below)   |
|  Disclosure Level TLV (0xF007)    : 1 octet enum               |
|  Capability Descriptor TLV (0xF003) [1..n]                     |
|     sub-TLV ontology-IRI-ref      : IRI or interned ID         |
|     sub-TLV embedding-digest      : hash of the vector         |
|     sub-TLV compact-vector        : int8-quantized, optional   |
|  Capability Digest+Fetch TLV (0xF004) : digest + fetch hint    |
|  Protocol Endpoint TLV (0xF005) [1..n]: mechanism + URI        |
|  Ed25519 Signature TLV (0xF0FF)   : key-id + 64 octets (last)  |
+---------------------------------------------------------------+
```

**Agent Status enum (0xF008)** — from IAIP LCR states (SPEC_NOTES §6):
`0=ACTIVE · 1=PENDING_VERIFICATION · 2=SUSPENDED · 3=UNREACHABLE · 4=DEPRECATED`.
Only `ACTIVE` agents are eligible for selection.

**Disclosure Level enum (0xF007):** `0=public · 1=domain-internal · 2=tenant-internal ·
3=restricted`.

**Protocol Endpoint mechanism enum (0xF005):** `0=MCP · 1=A2A · 2=IAIP · 3=HTTP`.

### 5.2 Trust LSA (opaque type 129, area-scoped)

The Trust Manager's signed attestations. Realizes SRA Trust Anchoring; defends
profile-integrity (cui §12.3).

```
+--------------------- Opaque LSA header (type 10, opaque 129) --+
|  Agent-ID TLV (0xF001)        : subject, 16 octets            |
|  Trust Score TLV (0xF010)     : 16-bit fixed-point + scope    |
|  Evidence Digest TLV (0xF011) : hash alg + digest (no raw ev.) |
|  Issuer TLV (0xF012)          : issuer key-id, 16 octets       |
|  Validity Window TLV (0xF013) : not-before / not-after (u32)   |
|  Ed25519 Signature TLV (0xF0FF): key-id + 64 octets (last)     |
+---------------------------------------------------------------+
```

Trust Score scope byte: `0=overall · 1=per-domain · 2=per-capability` (scope target in a
sub-TLV). Signature is **mandatory** on Trust LSAs.

### 5.3 Semantic Telemetry / TE LSA (opaque type 130, area-scoped)

Per-link and per-agent performance, modeled on OSPF-TE (RFC 3630) but for agents.

```
+--------------------- Opaque LSA header (type 10, opaque 130) --+
|  Agent-ID TLV (0xF001)        : optional (per-agent metrics)   |
|  Link Metrics TLV (0xF020):                                    |
|     +-----------------------------------------------------+    |
|     | latency_us (u32) | jitter_us (u32)                  |    |
|     | success_rate (16fp) | load (16fp)                   |    |
|     +-----------------------------------------------------+    |
|  Agent Perf Metrics TLV (0xF021):                              |
|     | queue_depth (u32) | cost (u32) | energy (u32)        |    |
|     | success_rate (16fp)                                  |    |
+---------------------------------------------------------------+
```

Telemetry LSAs are **dampened** before re-origination (hysteresis; brief §5.4) so a noisy
metric cannot flap routes. Re-origination respects MinLSInterval.

#### 5.3.1 ASH → SRv6 SRH / BGP community mapping (**documented only, NOT implemented**)

Per brief §5.3 and SRA li §4 ("BGP communities or SRv6 tags"): the ASH's intent-category ID
and trust floor *could* map to an SRv6 SRH TLV (type TBD) carrying
`{intent_category_id, trust_floor}`, or to a BGP **large community** `(ASN : 0xF0 :
intent_category_id)`. **S-OSPF v0 implements neither** — this subsection exists so the
mapping is on record; see ROADMAP for the BGP-LS export path.

### 5.4 Policy LSA (opaque type 131, AS-scoped)

Domain policy summaries. Realizes SRA Policy Engine + yang Policy Constraints (§8.4).

```
+--------------------- Opaque LSA header (type 11, opaque 131) --+
|  Domain TLV (0xF006)                  : UTF-8                  |
|  Allowed Intent Categories TLV (0xF030): list of u16 IDs       |
|  Disclosure Limits TLV (0xF031)       : enum + sub-TLVs        |
|  Compliance Tags TLV (0xF032)         : list of UTF-8 tags     |
+---------------------------------------------------------------+
```

### 5.5 Semantic Prefix Summary LSA (opaque type 132, AS-scoped)

ABR aggregation of semantic prefixes across areas (mirrors OSPF Summary LSAs).

```
+--------------------- Opaque LSA header (type 11, opaque 132) --+
|  Aggregated Semantic Prefix TLV (0xF040): §4, Prefix Len<Count |
|  Capability Bloom Filter TLV (0xF041)   : m,k,salt + bitfield  |
|  Trust Range TLV (0xF042)               : min/max 16fp         |
+---------------------------------------------------------------+
```

The **Bloom filter** lets a remote area test "might an agent with capability X exist under
this aggregated prefix?" without flooding every agent (false positives tolerated, false
negatives not). Parameters and sizing justified in ADR-0003.

---

## 6. Semantic Forwarding Table (SFT) — local state, not flooded

The SFT is computed, not transmitted, so it has no wire format. Record shape (for
`sospfctl show sft` and `internal/sft`):

```
intent_category | semantic_prefix -> { next_hop, out_iface, target_agent_id,
                                       composite_cost, constraints, expiry }
```

`expiry` implements draft-cui's "a decision is not a capacity reservation; expired decisions
must not be used" (§6.7). Entries past expiry are recomputed on demand.

---

## 7. Agent-facing messages (gRPC + JSON-LD) — Gateway Mediation Layer

These are **not** flooded; they are the agent↔router / gateway API (protobuf on the wire,
JSON-LD for ontology payloads). Primary names follow draft-yang **-02**; a -01 alias table
lives in `internal/gateway/aliases.go`.

### 7.1 Intent Vector (proto sketch; realizes li §3 + cui §5.1 + draft-cui-ai-agent-task)

```proto
message IntentVector {
  string  routing_id      = 1;              // cui §5.1
  string  task_type_iri   = 2;              // ontology IRI (zhang §5)
  uint32  priority        = 3;
  uint32  urgency         = 4;
  Context context         = 5;              // domain, sensitivity, tenant...
  ResourceNeeds resources = 6;
  Constraints constraints = 7;              // max_latency, min_trust, allowed_domains,
                                            //   disclosure_limit, cost_ceiling
  repeated float embedding = 8;             // optional; cosine fallback only
  repeated Preference preferences = 9;      // ordering only (cui §5.1)
}
```

JSON-LD form carries the same fields under the project `@context`, typed
`@type: "ioa:Intent"` and aligned to draft-zhang's ontology (`ioa:appliesToDomain`,
`ioa:requiresCapability`, …).

### 7.2 MediationRequest / MediationResponse / HandoffContext / EvidenceReference

```proto
message MediationRequest {                  // yang-02 §8
  IntentVector goal                 = 1;    // Goal (outcome only)
  Context      context              = 2;
  repeated CapabilityProfileRef registered_capabilities = 3;
  PolicyScope  policy_scope         = 4;
  TrustBinding trust_binding        = 5;
}
message MediationResponse {                 // yang-02 §10.1
  enum Outcome { SELECTED=0; REJECTED=1; CONSTRAINED=2; HANDED_OFF=3; }
  Outcome outcome                   = 1;
  string  selected_target_ref       = 2;    // agent-id / semantic prefix
  string  decision_reason           = 3;
  repeated string profile_version_refs = 4;
  Scope   applicable_scope          = 5;
  repeated string constraint_refs   = 6;
  string  validation_status         = 7;    // -02 addition
  google.protobuf.Timestamp expires_at = 8; // cui §5.3 (required)
}
message HandoffContext {                    // yang-02 §10.2
  string task_meaning               = 1;
  string context_summary            = 2;    // redacted per disclosure limits
  DisclosureLimits disclosure_limits= 3;
  Mechanism mechanism               = 4;    // MCP|A2A|IAIP|HTTP
  string target_ref                 = 5;
  string scoped_token_ref           = 6;    // reference, not the token
  google.protobuf.Timestamp expiry  = 7;
  FailureSemantics failure_semantics= 8;
}
message EvidenceReference {                 // yang-02 §10.4 — pointers only
  repeated string source_description_refs = 1;
  string validation_status          = 2;
  repeated string delegation_refs   = 3;
  repeated string attestation_refs  = 4;
  repeated string receipt_refs      = 5;
  repeated string provenance_refs   = 6;
  repeated string audit_refs        = 7;
  // NO inline raw evidence (yang §10.4, §14)
}
```

### 7.3 FailureRecord and the experimental reason-code enum

```proto
message FailureRecord {                     // yang-02 §10.3
  ReasonCode reason_code            = 1;
  Hint       hint                   = 2;    // RETRY | FALLBACK | ESCALATE
  string     human_text             = 3;    // minimal; no raw prompts/PII (yang §14)
  EvidenceReference evidence        = 4;
}
enum ReasonCode {                           // union of yang-01 §10.3 + §12, experimental
  CAPABILITY_MISMATCH          = 0;
  INTENT_AMBIGUITY             = 1;         // a.k.a. intent misunderstanding
  PARAMETER_SEMANTIC_MISMATCH  = 2;
  SEMANTIC_PROFILE_MISMATCH    = 3;
  POLICY_CONSTRAINT_FAILURE    = 4;
  TRUST_INSUFFICIENCY          = 5;
  STALE_CAPABILITY_INFO        = 6;
  UNAVAILABLE_CONTEXT          = 7;
  CONTEXT_VERSION_MISMATCH     = 8;
  HANDOFF_PREPARATION_FAILURE  = 9;
}
```

---

## 8. Authentication & integrity

### 8.1 Per-packet cryptographic authentication (RFC 5709-style, HMAC-SHA-256)

Every OSPF packet carries an **Authentication Trailer** appended after the OSPF payload
(OSPFv2 AuType = Cryptographic Authentication). Fields:

```
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|      Key ID (16)              |   Auth Data Len (16) = 32      |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|            Cryptographic Sequence Number (64 bits)            |
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|        HMAC-SHA-256 digest (32 octets) ...                    |
+---------------------------------------------------------------+
```

- HMAC covers the OSPF packet + a per-interface shared key, per RFC 5709.
- The 64-bit crypto sequence number (monotonic) provides **replay protection** (cui §12.7);
  receivers reject non-increasing sequence numbers per neighbor.
- Overlay (UDP) mode uses the same trailer so emulated and raw modes share one code path.

### 8.2 Per-LSA Ed25519 signatures (Trust Anchoring)

Capability and Trust LSAs additionally carry an **Ed25519 Signature TLV (0xF0FF)**, so a
compromised router cannot forge *another* router's agents even if it can produce valid
packet HMACs (brief §5.2; SRA Trust Anchoring; cui §12.3).

- **Coverage:** the signature covers the Opaque LSA from the **LS type** octet through the
  end of the last TLV **before** the Signature TLV (i.e. everything except `LS age`,
  `LS checksum`, and the Signature TLV itself — `LS age` mutates in flight, so it is
  excluded, matching how OSPF excludes age from the checksum).
- **Key-id** (16 octets) identifies the signing key; keys are bound to the Advertising
  Router / Trust Manager identity and distributed out of band (keystore, not flooded).
- Verification failure ⇒ the LSA is flooded (forward-compat) but **not installed** into the
  semantic view; an alarm is raised.

---

## 9. Provisional experimental code-point registry (summary)

| Space | Range used | Values |
|---|---|---|
| Opaque LSA type | 128–255 (experimental) | 128 Capability · 129 Trust · 130 Telemetry · 131 Policy · 132 Prefix-Summary |
| TLV type | 0xF000–0xFFFF (experimental) | see §3 |
| ASH version | — | 1 |
| Reason codes | experimental enum | see §7.3 |
| HMAC Key ID / Ed25519 Key ID | local admin | 16-bit / 16-octet |

**IANA posture:** `docs/draft-sospf-semantic-link-state-00.md` will request experimental
code points only, and will state plainly that these are self-assigned experimental values
with no IANA standing until/unless a working group adopts the work.

---

## 10. Open questions for the checkpoint review

1. **Opaque vs. a new LSA function code.** We chose Opaque LSAs (reuse flooding + no new LS
   type). Alternative: a brand-new experimental LS type. Opaque is less invasive and
   RFC-5250-blessed for exactly this; confirm.
2. **Embedding transport.** Compact int8-quantized vector *inline* vs. digest-only +
   fetch-on-demand for the vector too. Current design: digest always; inline vector
   optional and size-capped. Confirm the cap (proposed: ≤ 64 dims inline).
3. **Semantic Prefix charset.** IDNA-style `[a-z0-9-]` labels, case-folded. Confirm we don't
   need full UTF-8 labels in v0.
4. **Trust fixed-point width.** 16-bit [0,1] for Trust Score; 8-bit for ASH trust floor.
   Confirm precision is enough.
5. **Signature coverage of `LS age`.** Excluded (age mutates in flight). Confirm.
6. **Reason-code enum.** Adopt the draft-yang union (10 values) as-is? Add an
   `UNSPECIFIED`/`OTHER`?

Once these are settled, Phase 2 (classic core) begins; semantic LSAs are implemented in
Phase 3 against exactly this document.
