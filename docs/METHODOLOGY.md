# Methodology

This document exists so that any metric published by this project can be
traced back to a precise definition and its limitations. If you are an
exchange engineer disputing an observation, this is the document to read
first, and any dispute should reference the specific formula and caveats
here.

## Core principle

We measure **observed behavior of a public API, from a specific network
vantage point, at a specific time**. We do not measure, and never claim to
measure, an exchange's internal matching engine performance, infrastructure
health, or intent.

Correct phrasing: *"observed latency from probe X in region Y."*
Incorrect phrasing: *"Exchange X's internal latency."*

## What is measured

- REST public market-data endpoint behavior: DNS/TCP/TLS/TTFB/total timing,
  HTTP status, exchange application error codes, malformed/invalid
  responses, timestamp freshness, rate-limit signaling.
- WebSocket public market-data channel behavior: connection/subscription
  latency, time-to-first-event, disconnect/reconnect frequency and
  duration, ping/pong latency, message arrival timing, sequence integrity.
- Local order-book reconstruction integrity, where an exchange's documented
  public API supports it: snapshot+delta reconstruction, sequence gaps,
  resyncs, crossed books, reconciliation against fresh snapshots.
- Probe node health (CPU, memory, outbound connectivity), so readers can
  assess whether a given observation window was affected by probe-side
  issues.

## What is NOT measured

- Private/authenticated API behavior (order placement, account data) — out
  of scope for this project entirely (see `README.md`).
- Exchange internal matching engine latency — we only ever see the public
  API's output, not internal processing time.
- Anything requiring API keys or trading credentials.
- A single "reliability" number — deliberately not computed. See "No
  composite scores" below.

## Network latency vs. exchange processing latency

A REST or WebSocket round-trip time includes DNS resolution, TCP/TLS
handshake, network transit (through the probe's ISP/cloud provider, transit
networks, and the exchange's edge/CDN), and the exchange's own request
handling. We report each phase separately where technically measurable
(DNS, TCP connect, TLS handshake, TTFB, total) precisely so that a spike in
"total latency" cannot be misread as "the exchange got slower" when, for
example, DNS resolution time explains it. We do not attempt to isolate pure
exchange-side processing time — that would require internal exchange
instrumentation we don't have.

## Timestamp-based measurements and clock synchronization

Several key metrics (`ws_observed_event_delay_seconds`, clock drift) are
computed as `local_receive_time - exchange_reported_time`. This is **only as
accurate as both clocks**:

- Probe nodes run NTP and we monitor NTP sync status as a probe health
  signal; a probe with unsynced clock is expected to have that reflected in
  its health metrics, and its event-delay measurements for that window
  should be treated with reduced confidence.
- We do not control, and cannot verify independently, the exchange's server
  clock accuracy. Where an exchange exposes a server-time endpoint, we
  sample it and publish the observed clock offset (`exchange.clock_drift`)
  as its own metric, precisely so a reader can factor it into their
  interpretation of event-delay numbers rather than have it silently baked
  in.
- A metric derived from cross-clock timestamps always carries this caveat
  in `docs/METRICS.md`; treat any single-digit-millisecond claim about
  event delay with proportionate skepticism unless clock drift for that
  window is also shown to be small.

## Geographical bias

Probes measure from wherever they are deployed. Results are **never**
aggregated across regions into a single number without the region breakdown
remaining visible — a "global average latency" figure does not exist in
this project's output. Regional differences reflect the combination of
probe-to-exchange network path, the probe's cloud provider/ISP peering, and
(for anycast/CDN-fronted APIs) which edge location actually served the
request — not necessarily anything about the exchange's primary
infrastructure location.

## ISP / cloud-provider effects

A probe's observed latency and error rate are influenced by its own
hosting provider's network quality and peering, independent of the
exchange. Each probe declares its `provider` field, which is published
alongside every measurement, so systematic provider-specific effects (e.g.
one cloud region having poor peering to an exchange's edge) can be
identified rather than misattributed to the exchange.

## Public API vs. internal matching engine

Everything this project measures is visible only through the exchange's
publicly documented, unauthenticated market-data surface. We have no
visibility into, and make no claims about, order matching internals, risk
engine behavior, or any system not exposed on that public surface.

## Market inactivity vs. API failure

A calm/low-volume market can produce long gaps between WebSocket messages
that look, superficially, like a stale or dead stream. We do not equate
"no messages for N seconds" with "stream failure." Instead:

1. We track a rolling baseline message rate per (exchange, instrument,
   channel, region) over a trailing window.
2. A stream is flagged `data.stale` only when the silence duration
   substantially exceeds what recent activity would predict **and**
   the underlying connection is still reported open/healthy (ruling out an
   undetected disconnect, which is tracked separately as
   `websocket.disconnect`).
3. Ping/pong continuing to succeed during a quiet period is evidence the
   connection itself is fine — quiet is not down.
4. We do not currently attempt to correlate silence against independent
   market-activity ground truth (e.g. another venue's trade tape); this is
   a known limitation and a candidate research question, not something we
   assert a conclusion about.

This heuristic is necessarily imperfect at low sample sizes; see "Sampling
bias" below.

## Maintenance windows: planned vs. unplanned disconnects

Where an exchange publishes a maintenance schedule via its API or status
page, disconnects that fall inside an announced maintenance window are
labeled `planned` rather than counted toward unplanned-disconnect
statistics or incident detection. Where no such signal exists, all
disconnects are treated as unplanned by default — we do not guess exchange
intent.

## Sampling bias

- Probe density and geographic coverage are not uniform across regions or
  exchanges at any point in the project's history; comparing two venues'
  numbers implicitly compares under different sample sizes unless you check
  the reported `sample_count`.
- Every chart and every API response for windowed metrics includes sample
  size. A p99 computed over 40 samples is not the same statistical object
  as one computed over 40,000 — see `docs/METRICS.md` for how we handle and
  disclose small-sample p99/p95.
- Incident detection requires corroboration across multiple independent
  probes specifically to reduce the false-positive rate from a single
  probe's sampling noise (see `docs/INCIDENTS.md`).

## No composite scores

We deliberately do not compute or publish any single "reliability score,"
letter grade, or ranking. Different consumers of this data care about
different properties (a market maker cares about WS event delay tail
latency; a retail-facing app might care about REST success rate); collapsing
these into one number would embed a value judgment about which failure
modes matter, which is exactly the kind of claim this project exists to
avoid making on anyone's behalf.

## Reproducibility

Every metric definition here has a corresponding formula in
`docs/METRICS.md`, and the probe that collected it is open source and
runnable by anyone (`docs/EXCHANGE_ADAPTERS.md`, top-level `README.md`
quick start). If our numbers and your own probe's numbers disagree for the
same window and region, that disagreement is itself useful information —
please open an issue.
