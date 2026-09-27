# Security Policy

## Scope and posture

Probes run against **public, unauthenticated market-data APIs only**. No
component in this repository requests, stores, or requires exchange API
keys or trading credentials. This significantly narrows the blast radius of
a compromised probe (it cannot place trades or leak account data because it
never has access to any), but probes do process a high volume of
attacker-reachable input: **every byte a probe reads from an exchange
WebSocket or REST response is untrusted input**, since a malicious or
compromised exchange endpoint (or a MITM on a network a community probe
runs on) could serve adversarial payloads.

## Threat model

| Threat | Mitigation |
|---|---|
| Malicious/malformed exchange payloads (oversized messages, deeply nested JSON, decompression bombs, invalid UTF-8) | Strict size limits on WebSocket frames and HTTP response bodies before parsing; streaming/bounded JSON decoding; malformed input is classified `data.invalid_json`/`data.schema_mismatch` and dropped, never causes a panic (enforced via fuzzing, see `CONTRIBUTING.md`) |
| Decompression bombs (gzip/deflate response bodies) | Bounded decompression readers with a hard output-size cap; abort and classify as `data.invalid_json` rather than exhausting memory |
| Excessive reconnect loops (an exchange or MITM forcing rapid disconnect/reconnect) | Exponential backoff with jitter and a maximum retry ceiling per window; sustained failure surfaces as a probe health signal, not an infinite tight loop |
| Resource exhaustion (memory/goroutine growth from many venues/channels) | Bounded buffers everywhere (local upload buffer, per-connection read buffers); no unbounded queues |
| SSRF via configuration (an adapter or operator config pointing the probe at an internal address) | Adapter endpoints are fixed in code, not attacker- or arbitrary-operator-controllable at runtime; any future user-supplied-URL feature (e.g. custom community endpoints) must validate against a private-address blocklist before this repository accepts it — tracked as a hard requirement in `ROADMAP.md` Phase 3, not shipped without it |
| Dependency supply-chain risk | Dependencies pinned via lockfile; CI runs vulnerability scanning; minimal dependency surface preferred over convenience libraries, especially in the probe binary that runs on contributor infrastructure |
| Compromised or malicious community probe submitting fake measurements | Signed batches (see "Data authenticity" below); `operator_type` labeling so community data is never silently blended with official data; plausibility/anomaly checks at ingestion; per-identity rate limiting; see `ARCHITECTURE.md` "Community probes" |
| Fabricated measurements more generally | Signature verification at ingestion; publication of raw, inspectable data (not just conclusions) so anomalous submissions are auditable by third parties, not just by us |

## Data authenticity

Each probe holds an **Ed25519 keypair** identifying it (`node_id` ↔ public
key). Measurement batches are signed before leaving the probe (and before
being written to the local buffer, so a buffered-then-later-flushed batch's
provenance is still verifiable). Ingestion verifies the signature against a
registered public key before accepting a batch. This is not a full PKI in
the MVP (no revocation infrastructure, no certificate chain) — that's an
explicit, documented gap, not a silent one — but it establishes: (a) a
batch actually came from the probe identity it claims, and (b) it wasn't
altered in transit or in the local buffer.

## Privacy

This system does not collect, and must never be extended to collect:
- exchange user accounts, balances, or trading history,
- private API credentials of any kind,
- any personal data about probe operators beyond what they choose to
  disclose in `node_id`/`region`/`provider` metadata for their own node.

Public probes are designed to run without any personal data whatsoever.

## Reporting a vulnerability

Please **do not open a public GitHub issue** for a security vulnerability.
Instead, email the maintainer at the address listed in the repository's
GitHub profile (Vizanix) with:
- a description of the issue and its potential impact,
- steps to reproduce, if applicable,
- whether you believe it's already being exploited.

We aim to acknowledge reports within 5 business days. Coordinated
disclosure is preferred; we'll credit reporters (unless you'd rather stay
anonymous) once a fix ships.

## What's explicitly not a vulnerability report here

Disagreement with a published measurement or incident classification is not
a security issue — please use `docs/METHODOLOGY.md` and a regular GitHub
issue for that.
