# Open Exchange Reliability Monitor

**Independent, open-source measurements of public cryptocurrency exchange APIs.**

Track REST availability, WebSocket stability, observed market-data latency,
order-book sequence integrity, and reconnect behavior — from geographically
distributed probes, with reproducible methodology.

> **Measure, don't rank.**

This project does not recommend exchanges, does not assign reliability
scores, and does not produce leaderboards. It publishes reproducible
technical observations so developers, SREs, and researchers can inspect the
data themselves and draw their own conclusions.

---

## What this is

A probe fleet issues the same public, unauthenticated market-data requests
(REST and WebSocket) against several exchanges from several regions, records
what actually happened — latency, errors, disconnects, sequence gaps,
observed event delay — and exposes both the raw event stream and rolled-up
statistics through an open API and dashboard.

Every published number is traceable to:

- a **time window**,
- a **probe region**,
- a **sample size**,
- a **formal metric definition** (see [`docs/METRICS.md`](docs/METRICS.md)),
- the **probe software version** that collected it.

See [`docs/METHODOLOGY.md`](docs/METHODOLOGY.md) for what is and is not being
measured, and for the limitations (clock skew, geographic bias, sampling
bias) that apply to every chart in the dashboard.

## What this is not

- Not a trading system. Probes never hold API keys or send orders.
- Not a rating agency. There is no composite "reliability score."
- Not a claim about an exchange's internal matching engine — only about
  what a probe, from a specific vantage point, observed on the public API
  surface.

Language matters here: we say *"probes in Frankfurt observed an elevated
WebSocket disconnect rate for venue X between 14:02–14:19 UTC (n=340
connections)"*, never *"Exchange X is unreliable."*

## Status

Early-stage, MVP in progress. Currently implemented in this repository: a
working REST + WebSocket probe with order-book sequence validation and a
Prometheus exporter for the Bybit V5 public API (see
[`probe/`](probe/)). Central ingestion, storage, backend API, and dashboard
are designed in [`ARCHITECTURE.md`](ARCHITECTURE.md) and tracked in
[`ROADMAP.md`](ROADMAP.md).

## Quick start

Run the probe standalone and scrape its local Prometheus endpoint:

```bash
docker run --rm -p 9464:9464 \
  ghcr.io/vizanix/open-exchange-reliability-monitor/probe:latest \
  --exchange bybit \
  --instrument BTC-USDT-PERP \
  --region home-lab \
  --provider self-hosted

curl http://localhost:9464/metrics
```

Or run the full local demo stack (probe + Prometheus, no ClickHouse/backend
yet — see [`ROADMAP.md`](ROADMAP.md) Phase 2 for the central pipeline):

```bash
docker compose -f deploy/docker-compose.yml up
```

## Repository layout

```
/
├── probe/            # Go probe binary: adapters, REST/WS probe engines, exporter
├── adapters/          # Per-exchange protocol notes and fixtures (docs, not code)
├── proto/             # Shared wire/schema definitions (metrics, incidents) - future backend/dashboard
├── deploy/            # Dockerfiles, docker-compose, example configs
├── docs/               # Methodology, metrics spec, adapter guide, incident format
├── tests/              # Cross-cutting integration/fuzz fixtures
└── .github/            # CI
```

This is a monorepo by design at this stage: the probe, backend, frontend,
and shared schemas evolve together while the data model is still settling,
and a single CI pipeline can validate that a metric defined in `docs/`, sent
by a probe, is actually consumable by the backend and dashboard. We expect
to reassess this once the backend and frontend are implemented (see
`ROADMAP.md`).

## Documentation

- [`ARCHITECTURE.md`](ARCHITECTURE.md) — system design, data flow, technology choices
- [`docs/METHODOLOGY.md`](docs/METHODOLOGY.md) — what is measured, what isn't, and why numbers have limits
- [`docs/METRICS.md`](docs/METRICS.md) — formal definition of every published metric
- [`docs/EXCHANGE_ADAPTERS.md`](docs/EXCHANGE_ADAPTERS.md) — adapter interface and per-exchange notes
- [`docs/INCIDENTS.md`](docs/INCIDENTS.md) — how "Potential Incidents" are detected and published
- [`SECURITY.md`](SECURITY.md) — threat model and reporting
- [`ROADMAP.md`](ROADMAP.md) — phased delivery plan
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — how to add an exchange, a region, or a metric

## Research ideas

Open questions the dataset is designed to let *anyone* investigate, not
conclusions this project draws:

- How does observed API latency differ by region, per venue?
- How does WebSocket stability vary across venues under similar conditions?
- Does message rate correlate with disconnect probability?
- How do sequence-gap frequency and market volatility relate?
- How do exchanges behave, on the public API surface, during high-volatility events?

We deliberately avoid drawing causal conclusions in this repository — that's
what the public dataset and API are for.

## Data & API

- Read-only JSON API: see [`docs/METRICS.md`](docs/METRICS.md) and the OpenAPI
  spec (planned, tracked in `ROADMAP.md` Phase 4).
- Bulk dataset export (Parquet/JSON/CSV): planned, tracked in `ROADMAP.md` Phase 4.

## License

Apache License 2.0 — see [`LICENSE`](LICENSE).

## Maintained by Vizanix

Vizanix builds algorithmic trading infrastructure, exchange integrations,
execution systems, and quantitative tooling. This project is maintained as
independent, neutral infrastructure for the broader ecosystem — its
measurement logic and publication rules are the same regardless of who is
reading them.
