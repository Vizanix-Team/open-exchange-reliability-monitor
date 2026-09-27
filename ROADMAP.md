# Roadmap

Each phase lists concrete deliverables, not aspirations. A phase is "done"
when its deliverables are merged and, where applicable, deployed.

## Phase 1 — MVP (current)

- [x] Governance docs: README, ARCHITECTURE, METHODOLOGY, METRICS,
      EXCHANGE_ADAPTERS, INCIDENTS, SECURITY, CONTRIBUTING, CODE_OF_CONDUCT, LICENSE.
- [x] `ExchangeAdapter` interface and shared probe engine skeleton (Go).
- [x] Bybit V5 adapter: REST server-time + ticker probe, WebSocket
      orderbook + trade probe, sequence/reset detection (`u`/`seq`
      semantics), local order-book reconstruction.
- [x] Prometheus exporter with a bounded, documented label set.
- [x] Fake-exchange test server + fixtures (including a gap and a malformed
      message); unit tests for adapter and sequence validation.
- [x] Dockerfile + `docker compose up` local demo (probe + Prometheus).
- [ ] Binance and OKX adapters (REST + WS + sequence semantics), following
      the same verification discipline as Bybit — **pending explicit
      verification against current official docs before implementation.**
- [ ] Canonical instrument mapping config wired through all three MVP
      adapters for BTC perpetual / closest liquid equivalent.
- [ ] Central ingestion API (batch intake, Ed25519 signature verification,
      bounded probe-side buffering already implemented client-side).
- [ ] ClickHouse schema + materialized-view rollups per `docs/METRICS.md`.
- [ ] Deploy 3 official probe nodes: Frankfurt, Singapore, New York.

## Phase 2 — Multi-region production

- [ ] Backend read API (`/v1/exchanges`, `/v1/metrics`, `/v1/incidents`,
      `/v1/probes`) with OpenAPI spec.
- [ ] Dashboard (Next.js): per-exchange metric pages, region breakdowns,
      1h/24h/7d windows, "Probe Health" page, "Methodology" page.
- [ ] Historical exploration UI (exchange/protocol/instrument/region/metric/
      time-range picker) with visible sample size on every chart.
- [ ] Additional regions beyond the initial 3, prioritized by where MVP
      data shows the most interesting regional variance.
- [ ] Order-book reconciliation-against-snapshot probe for venues whose
      docs support it, for all MVP exchanges.
- [ ] Incident engine v1 per `docs/INCIDENTS.md` (multi-probe corroboration,
      control-endpoint cross-check, minimum-duration threshold).

## Phase 3 — Community probes

- [ ] Community probe registration flow and `operator_type` classification
      (`official` / `verified-community` / `community`).
- [ ] Anti-abuse: per-identity rate limiting, plausibility/anomaly checks
      at ingestion, dashboard filtering by `operator_type`.
- [ ] SSRF-safe configuration path for any community-configurable endpoint
      (hard requirement before this phase ships — see `SECURITY.md`).
- [ ] "Verified community" review process documented in `CONTRIBUTING.md`.

## Phase 4 — Incident dataset / API

- [ ] Permanent incident pages (`/incidents/<slug>`) with timeline, affected
      probes/channels, raw metric excerpts, resolution, annotations.
- [ ] Public bulk dataset export (Parquet/JSON/CSV) at a `data.` subdomain.
- [ ] Stable, versioned public read API with OpenAPI spec published.
- [ ] Additional exchange adapters (Coinbase, Kraken to complete the
      originally-scoped 5; then Bitget, Gate.io, Deribit, Hyperliquid as
      community/maintainer bandwidth allows).

## Phase 5 — Research platform

- [ ] First public report: "State of Crypto Exchange APIs — 30 Day
      Observation" (see `README.md` "Research ideas" — venue/region
      comparisons without ranking, full methodology and sample sizes
      disclosed per chart).
- [ ] Structured support for external researchers: citable dataset
      snapshots, a research-ideas backlog, and a process for methodology
      review contributions from academia/industry.
- [ ] Expanded probe geography and exchange coverage driven by
      demonstrated research/community demand rather than pre-emptive
      breadth.
