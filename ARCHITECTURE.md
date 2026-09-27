# Architecture

## Design goals

1. Probes are simple, stateless-restartable, and never hold trading credentials.
2. A probe must keep working (and buffer results) when the central backend is unreachable.
3. No single metric may be published without its region, sample size, and method being derivable.
4. Exchange-specific logic (sequence semantics, symbol naming, WS quirks) is isolated per adapter — never leaked into shared/core code.
5. Storage cost is bounded by design (retention tiers, aggregation, sampling), not by after-the-fact cleanup.
6. Probe-side failure must never be misreported as exchange-side failure (see "Anti-false-positive design" below).

## High-level data flow

```mermaid
flowchart LR
    subgraph Region: Frankfurt
        P1[Probe Node]
    end
    subgraph Region: Singapore
        P2[Probe Node]
    end
    subgraph Region: New York
        P3[Probe Node]
    end

    P1 -- signed batches --> ING
    P2 -- signed batches --> ING
    P3 -- signed batches --> ING

    P1 -. local buffer .-> P1
    P2 -. local buffer .-> P2
    P3 -. local buffer .-> P3

    ING[Central Ingestion API] --> Q[Ingest Queue / Buffer]
    Q --> AGG[Aggregation Workers]
    AGG --> CH[(ClickHouse:\nprobe_measurement, ws_event,\nrollups)]
    AGG --> INC[Incident Engine]
    INC --> CH
    CH --> API[Public Read API]
    API --> FE[Dashboard / Frontend]
    API --> DS[Dataset Export\n Parquet / JSON / CSV]

    P1 -. Prometheus /metrics .-> PROM[Prometheus\n(node-local scrape,\noperator-owned)]
```

Each probe also exposes a local Prometheus `/metrics` endpoint independent of
the central pipeline (see "Prometheus compatibility" in `docs/METRICS.md`),
so an operator can self-host without ever depending on Vizanix's central
service.

## Why this shape (and alternatives considered)

**Push vs. pull for ingestion.** Probes push signed batches to central
ingestion rather than being scraped, because probes run in arbitrary
networks (including behind NAT, on contributor hardware) and must survive
being offline. Prometheus federation/remote-write was considered and is
supported as a *local* interface (operators can scrape their own probe), but
is not the mechanism for community data submission — it doesn't give us
batch signing, backpressure, or bounded local buffering semantics we need
for untrusted submitters.

**ClickHouse vs. TimescaleDB vs. Prometheus/VictoriaMetrics for the store.**
- *Prometheus/VictoriaMetrics*: excellent for low-cardinality operational
  metrics (this is what probes expose locally), poor fit for our central
  store because per-event fields (exchange sequence numbers, trade ids,
  error codes, region×exchange×instrument×probe combinations) would blow up
  label cardinality, and neither is designed to hold structured per-event
  records (a `ws_event` row) efficiently.
- *TimescaleDB*: a reasonable choice, and Postgres compatibility is
  attractive for a small team. But at our target volume (multiple regions ×
  multiple exchanges × sub-second WS events, potentially millions of rows/day
  once multi-region is live) ClickHouse's columnar compression and
  materialized-view rollups give materially better cost/query performance,
  and its `ReplacingMergeTree`/`AggregatingMergeTree` engines map directly
  onto our retention-tiered rollup strategy (see below).
- *ClickHouse (chosen)*: columnar storage compresses repetitive fields
  (exchange, region, instrument, error_category) extremely well; native
  downsampling via materialized views implements the retention policy
  without a separate cron/ETL system; proven at this exact workload shape
  (structured, high-volume, mostly-append event data) elsewhere in the
  industry.
- Raw WebSocket payloads (kept short-term only, for debugging/reproducing
  a specific incident) go to S3-compatible object storage, not ClickHouse —
  they're large, rarely queried, and don't benefit from columnar layout.

**Backend language.** Go, to share the error taxonomy, canonical instrument
model, and metric definitions (as a Go module) with the probe, and because
the read API's job is mostly "translate ClickHouse queries into typed JSON,"
which doesn't need Python's ecosystem or Rust's control. Rust was considered
for the probe (better guarantees, less footgun potential with unsafe code),
but Go's goroutine model is a very natural fit for "hold many concurrent
long-lived WebSocket connections and a REST poller per exchange," its
tooling produces trivially reproducible static binaries for `docker run
vizanix/exchange-reliability-probe`, and its lower barrier to entry matters
for a project that explicitly wants outside contributors to add exchange
adapters.

**Frontend.** Next.js/React: standard, wide contributor pool, works well
with a typed OpenAPI-generated client against the read API, supports static
generation for permanent incident pages (`/incidents/<slug>`) which should
be cheap to serve and cacheable.

## Probe autonomy and buffering

A probe must never lose data silently and must never grow unbounded on
disk:

- Local **bounded ring buffer** (default: last N hours or M megabytes,
  configurable, oldest-dropped-first) holds measurements pending upload.
- On central-backend unavailability, the probe keeps measuring and
  buffering; on reconnect it flushes the buffer in batches with original
  timestamps preserved.
- If the buffer overflows, the probe increments a `probe_buffer_dropped_total`
  counter (itself an exported Prometheus metric) rather than blocking or
  crashing — data loss is visible, not silent.
- Buffered batches are signed (see "Data authenticity" in `SECURITY.md`)
  before local storage, so a flushed batch's origin and integrity can be
  verified by ingestion regardless of when it arrives.

## Anti-false-positive design

A probe reporting "Binance WebSocket unreachable" is worthless — and
actively harmful to this project's credibility — if the probe's own network
egress is down. Every probe therefore runs parallel **control probes**
against independent, high-availability, non-exchange endpoints (e.g. a
well-known public DNS resolver, a major CDN/edge endpoint) on the same
cadence as exchange probes. Ingestion cross-references:

- If a probe loses connectivity to *multiple independent exchanges and its
  control endpoints simultaneously*, this is classified as a **probe/node
  health event**, not an exchange incident, and is excluded from
  incident-engine input for that window.
- If a probe loses connectivity to *one exchange* while control endpoints
  and other exchanges remain healthy, that's a real signal.
- Incident detection additionally requires **corroboration across
  independent probe nodes** (see `docs/INCIDENTS.md`) before elevating
  anything to "Potential Incident" — a single node's report is never
  sufficient on its own.
- Each probe also emits its own heartbeat/health metrics (CPU, memory,
  event-loop lag, outbound connectivity) to central ingestion, visible on a
  public "Probe Health" page, so third parties can audit whether a given
  probe was itself healthy during a reported incident window.

## Data model summary

See `docs/METRICS.md` for the authoritative field-by-field spec. Two core
record types:

- `probe_measurement` — one row per REST probe attempt or WS lifecycle event.
- `ws_event` — one row per (sampled) market-data event received over WebSocket.

## Retention policy

| Tier | Content | Retention |
|---|---|---|
| Aggregated rollups (p50/p95/p99, error rates, per hour/day) | derived, small | long-term (years) |
| Sampled raw measurements/events | `probe_measurement`, sampled `ws_event` | medium-term (weeks–months, exact window tunable per deployment cost) |
| Full raw payloads (complete WS message bodies) | object storage, opt-in, debug only | short-term (days), and only for windows flagged interesting (incidents, manual capture) |

Full raw payload capture is **off by default** in production probes; it is
enabled selectively (e.g. during an active Potential Incident, or via
explicit operator flag for debugging) to keep storage cost proportional to
what's actually useful, not to total traffic volume.

## Community probes

Community-submitted probe data is architecturally supported (signed batches
already carry a probe identity) but is **never merged into the same visual
series as official probes** without explicit labeling. Every record carries
`operator_type: official | verified-community | community`, and the
dashboard/API let a consumer filter or exclude non-official data. See
`docs/EXCHANGE_ADAPTERS.md` and `SECURITY.md` for anti-abuse mechanisms
(signature verification, plausibility checks, rate limiting of ingestion per
probe identity).

## Cost-conscious MVP deployment

A minimal production deployment for the 3-exchange/3-region MVP:

- 3 small probe VMs (one per region), each running the probe container —
  this is the only geographically-distributed cost.
- Single-node ClickHouse (a few tens of GB/month at MVP volume with rollups
  and sampling as described above) — vertical scaling is sufficient at this
  stage; sharding is a Phase 2+ concern.
- Single small backend/API instance; the read API is mostly serving
  pre-aggregated rollups, so it's cheap to run and cache aggressively
  (CDN-cacheable JSON for anything older than a few minutes).
- Static frontend hosting (CDN).
- Object storage bucket for the (small, opt-in) raw-payload debug tier.

The dominant lever for cost is **cardinality discipline**: never put
unbounded values (raw error strings, arbitrary symbols in high-cardinality
combination, per-connection IDs) into a Prometheus label or a ClickHouse
`ORDER BY`/`PARTITION BY` key. See `docs/METRICS.md` for the enforced label
set per metric.
