# Metrics Specification

Every metric below follows the same format: name, unit, type, formal
definition, and caveats. See `docs/METHODOLOGY.md` for the general
limitations (clock sync, geographic bias, sampling) that apply throughout.

Label discipline: no metric in this document uses an unbounded-cardinality
label (raw symbol strings beyond the small configured instrument set, raw
error message text, per-connection/per-message IDs, user identifiers).
Where a field like `error_code` is needed for debugging, it is stored in the
structured event record (ClickHouse), not exported as a Prometheus label.

---

## REST metrics

### `rest_dns_resolution_seconds`
- **Type:** histogram
- **Definition:** `t_dns_resolved - t_dns_start`
- **Caveats:** Reflects the probe's local resolver and network path;
  cached/OS-level DNS behavior varies by runtime.

### `rest_tcp_connect_seconds`
- **Type:** histogram
- **Definition:** `t_tcp_connected - t_tcp_start`
- **Caveats:** Not measurable separately from TLS handshake on some
  transports/proxies; reported only where measurable and marked `null`
  otherwise rather than estimated.

### `rest_tls_handshake_seconds`
- **Type:** histogram
- **Definition:** `t_tls_established - t_tcp_connected`

### `rest_ttfb_seconds`
- **Type:** histogram
- **Definition:** `t_first_response_byte - t_request_start`

### `rest_total_latency_seconds`
- **Type:** histogram
- **Definition:** `t_response_complete - t_request_start`
- **Caveats:** Includes DNS+TCP+TLS+TTFB+body transfer; use the phase
  metrics above to decompose a spike before attributing it to the exchange.

### `rest_requests_total`
- **Type:** counter
- **Labels:** `exchange`, `region`, `endpoint`, `outcome`
  (`outcome` ∈ `success`, `http_error`, `exchange_error`, `network_error`,
  `parse_error`)
- **Definition:** count of REST probe attempts by outcome.

### `rest_timestamp_freshness_seconds`
- **Type:** histogram
- **Definition:** `local_receive_time - exchange_reported_timestamp` for
  responses that include a data timestamp (e.g. ticker `time`).
- **Caveats:** Subject to clock-drift caveats in `docs/METHODOLOGY.md`.

### `exchange.clock_drift_seconds`
- **Type:** gauge (sampled)
- **Definition:** `probe_local_time - exchange_server_time`, sampled via
  the exchange's server-time endpoint where available.
- **Caveats:** Includes one-way network transit time to fetch server time;
  this is an upper-bound estimate of drift, not a precise NTP-grade
  measurement.

### `rest_rate_limit_remaining_ratio`
- **Type:** gauge
- **Definition:** exchange-reported remaining-quota header, normalized to
  `remaining / limit` where the exchange exposes both.

---

## WebSocket connection metrics

### `ws_connect_latency_seconds`
- **Type:** histogram
- **Definition:** `t_connection_established - t_connect_start` (through TCP+TLS+WS upgrade)

### `ws_subscription_ack_latency_seconds`
- **Type:** histogram
- **Definition:** `t_subscription_ack_received - t_subscription_sent`
- **Caveats:** Not all exchange protocols send an explicit subscribe ack;
  where absent, this metric is not emitted for that venue rather than
  estimated (see per-adapter notes in `docs/EXCHANGE_ADAPTERS.md`).

### `ws_time_to_first_event_seconds`
- **Type:** histogram
- **Definition:** `t_first_market_data_event - t_subscription_ack_received`
  (or `t_subscription_sent` where no ack exists)

### `ws_connection_lifetime_seconds`
- **Type:** histogram
- **Definition:** `t_disconnect - t_connection_established` for each completed connection.

### `ws_disconnects_total`
- **Type:** counter
- **Labels:** `exchange`, `region`, `channel`, `planned`
  (`planned` ∈ `true`/`false`, see Methodology's maintenance-window rule)

### `ws_reconnect_duration_seconds`
- **Type:** histogram
- **Definition:** `t_new_connection_established - t_previous_disconnect`

### `ws_ping_pong_latency_seconds`
- **Type:** histogram
- **Definition:** `t_pong_received - t_ping_sent`

### `ws_messages_total`
- **Type:** counter
- **Labels:** `exchange`, `region`, `channel`, `kind`
  (`kind` ∈ `data`, `error`, `heartbeat`, `ack`)

---

## Market-data event metrics

### `ws_observed_event_delay_seconds`
- **Type:** histogram
- **Definition:** `local_receive_timestamp - exchange_event_timestamp`
- **Caveats:** Correct only within the combined accuracy of exchange and
  probe clocks — see `docs/METHODOLOGY.md`. Always read alongside
  `exchange.clock_drift_seconds` for the same window.

### `ws_sequence_gaps_total`
- **Type:** counter
- **Labels:** `exchange`, `region`, `channel`
- **Definition:** count of detected discontinuities in the exchange's
  sequence/update-id field, per the adapter's documented sequence semantics
  (see `docs/EXCHANGE_ADAPTERS.md` — semantics differ per venue and must
  not be generalized).

### `ws_duplicate_messages_total` / `ws_out_of_order_messages_total`
- **Type:** counter
- **Definition:** messages whose sequence indicates a repeat, or an
  earlier point than already-processed state, respectively.

### `data_stale_periods_total`
- **Type:** counter
- **Definition:** count of periods flagged stale under the algorithm in
  `docs/METHODOLOGY.md` ("Market inactivity vs. API failure").
- **Caveats:** Heuristic; low-liquidity instruments and low-sample windows
  increase false-positive risk. Always cross-checked against connection
  health before being surfaced.

---

## Order-book integrity metrics

### `orderbook_reconstruction_success_ratio`
- **Type:** gauge (windowed)
- **Definition:** fraction of reconstruction attempts that maintained a
  valid, gap-free local book for the full window.

### `orderbook_sequence_gaps_total`, `orderbook_resync_total`,
`orderbook_crossed_book_total`, `orderbook_invalid_quantity_total`
- **Type:** counters
- **Definitions:** per-event detections during local reconstruction — see
  `probe/internal/orderbook` for the reference implementation and
  `docs/EXCHANGE_ADAPTERS.md` for per-venue sequence rules.

### `orderbook_snapshot_latency_seconds` / `orderbook_delta_latency_seconds`
- **Type:** histograms
- **Definition:** request/apply latency for snapshot fetch and per-delta
  apply time respectively.

### `orderbook_reconciliation_mismatch_total`
- **Type:** counter
- **Definition:** count of periodic reconciliations (local book vs. a fresh
  REST snapshot, where the exchange's spec permits this comparison) that
  disagreed beyond a configured tolerance.

---

## Probe health metrics

### `probe_up`
- **Type:** gauge (1/0)

### `probe_control_endpoint_success_ratio`
- **Type:** gauge
- **Definition:** success ratio against independent non-exchange control
  endpoints (see `ARCHITECTURE.md` "Anti-false-positive design").

### `probe_buffer_size_bytes` / `probe_buffer_dropped_total`
- **Type:** gauge / counter
- **Definition:** local bounded-buffer occupancy and drop count when
  central ingestion is unreachable.

### `probe_cpu_usage_ratio`, `probe_memory_usage_bytes`, `probe_ntp_sync_status`
- Standard node-health gauges; `probe_ntp_sync_status` is 1 when the OS
  reports NTP sync, 0 otherwise — this directly qualifies confidence in any
  timestamp-based metric emitted concurrently.

---

## Statistical aggregation notes

- Latency metrics are collected as histograms (HDR-style bucket layout),
  never as simple running averages, because **average latency hides tail
  behavior that is usually the operationally interesting part** — a venue
  with a fast median and a heavy p99 tail looks identical to a
  consistently-mediocre venue if you only look at the mean.
- p50/p90/p95/p99 are computed from histogram buckets at query time.
- **p99 with small sample size is explicitly flagged.** Any windowed
  aggregation with `sample_count < 200` (tunable) displays p99 with a
  "low sample size — high uncertainty" marker rather than presenting it
  with the same visual confidence as a well-sampled window; the API always
  returns `sample_count` alongside every percentile so consumers can apply
  their own threshold.
- Error rate = `outcome != success` count / total count over the window,
  always shown with the window's total count.

## `probe_measurement` record (ClickHouse / ingestion schema)

```
id                 UUID
timestamp          DateTime64(ms, UTC)
probe_node         String   (node_id)
region             String
exchange           LowCardinality(String)
protocol           Enum('rest','websocket')
endpoint           LowCardinality(String)
instrument         LowCardinality(String)   -- canonical instrument id
measurement_type   LowCardinality(String)
success            Bool
latency_us         UInt32
exchange_timestamp Nullable(DateTime64(ms, UTC))
local_timestamp    DateTime64(ms, UTC)
error_category     LowCardinality(String)   -- see error taxonomy below
error_code         Nullable(String)
metadata           Map(String, String)      -- bounded, small, debug-only
software_version   LowCardinality(String)
operator_type      Enum('official','verified-community','community')
```

## `ws_event` record

```
exchange           LowCardinality(String)
instrument         LowCardinality(String)
channel            LowCardinality(String)
event_timestamp    DateTime64(ms, UTC)
receive_timestamp  DateTime64(ms, UTC)
sequence           Nullable(Int64)
previous_sequence  Nullable(Int64)
gap_detected       Bool
duplicate          Bool
out_of_order       Bool
payload_size       UInt32
software_version   LowCardinality(String)
operator_type      Enum('official','verified-community','community')
```

`ws_event` rows are written at a configurable sampling rate in production
(see `ARCHITECTURE.md` retention policy) — not every single market-data
message is retained long-term; aggregated counters (gap/duplicate/OOO
totals) are always exact regardless of sampling, computed at collection
time before sampling is applied.

## Error taxonomy

```
network.dns
network.connect
network.timeout
network.tls

http.4xx
http.429
http.5xx

exchange.rate_limit
exchange.internal_error
exchange.maintenance

websocket.disconnect
websocket.protocol
websocket.subscription_failed

data.invalid_json
data.schema_mismatch
data.stale
data.sequence_gap
data.duplicate
data.out_of_order

probe.internal
```

`probe.internal` is reserved for failures in the probe itself (bug, panic,
resource exhaustion) and is never counted toward exchange-facing
statistics — see `ARCHITECTURE.md` "Anti-false-positive design."
