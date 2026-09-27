# Incident Detection & Publication

## Terminology

We use **"Potential Incident"**, always, until corroborating evidence meets
the bar below. We never publish "Exchange X is down." We never assign
blame or fault. An incident page documents what was *observed*, by whom,
where, and for how long — not why it happened or whose fault it was.

## Why "potential"

A single probe's report of elevated errors could be:
- a real, venue-wide issue,
- an issue specific to that probe's network path,
- a probe-node health problem (see `ARCHITECTURE.md` "Anti-false-positive
  design"),
- normal noise at low sample size.

Only some of these are exchange-relevant. The incident engine's job is to
separate them, conservatively, before anything gets a permanent public URL.

## Detection criteria

A candidate is escalated from "elevated signal" to "Potential Incident"
only when **all** of the following hold:

1. **Multiple independent probe nodes** (different regions and, where
   possible, different underlying providers) show the anomalous signal
   concurrently for the same exchange/channel.
2. **Control endpoints remain healthy** on the affected probes — ruling out
   a probe/network-egress problem being misattributed to the exchange (see
   `probe_control_endpoint_success_ratio`).
3. The signal exceeds a **statistically meaningful threshold** relative to
   that venue/region/channel's own recent baseline (not an absolute
   cross-venue threshold — venues differ in normal baseline behavior), and
   the window's `sample_count` is large enough to make the threshold
   meaningful (see `docs/METRICS.md` small-sample p99 handling).
4. The anomaly persists for a **minimum sustained duration** (default: a
   few minutes; exact value configurable and documented on the incident
   page itself) — single-sample blips do not qualify.

Candidate symptoms that can contribute to escalation:
- elevated REST error rate or timeout rate,
- elevated WebSocket disconnect rate,
- elevated sequence-gap rate,
- abnormal stale-stream detections (per the Methodology algorithm) that
  survive the "still connected" check,
- REST/WS divergence (e.g. WS degraded while REST is healthy, or vice
  versa) — itself useful information, reported as-is rather than collapsed.

## What gets published, and how

Each Potential Incident gets a **permanent URL**:

```
/incidents/<date>-<exchange>-<short-descriptor>
```

e.g. `/incidents/2026-09-27-bybit-ws-eu`

Page contents:
- **Timeline** — first-detected, escalated, resolved (or "ongoing") timestamps, UTC.
- **Affected probes** — node IDs and regions that contributed corroborating signal.
- **Affected channels/instrument(s)**.
- **Raw metric excerpts** — the actual time series for the window, not a
  summary claim, linking back to the exact `docs/METRICS.md` definitions
  used.
- **Methodology** — a restatement of which detection criteria were met and
  the sample sizes involved, so the escalation itself is auditable.
- **Resolution** — when symptoms returned to baseline, per the same
  criteria used to detect.
- **Comments/annotations** — space for the exchange's own engineers or
  outside researchers to add corroborating or contradicting context. We do
  not moderate these into a narrative; annotations are attributed and
  timestamped, not merged into our own text.

What an incident page never contains: a severity score, a "vendor
statement" framed as our conclusion, or language assigning cause without
attributed, cited evidence.

## Non-incidents

Explicitly excluded from the incident pipeline (logged, but not escalated):
- Announced maintenance windows (see Methodology "planned vs. unplanned").
- Single-probe-only anomalies that fail the control-endpoint check.
- Low-liquidity quiet periods correctly classified as `data.stale` under
  the Methodology algorithm, without other corroborating symptoms.

## Reproducibility for exchange engineers

Every incident page links the exact metric definitions and probe software
version involved, so an exchange's own engineers can attempt to reproduce
the observation independently using the open-source probe (`README.md`
quick start) rather than having to trust our infrastructure.
