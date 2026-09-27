# Exchange Adapters

Exchange-specific behavior (endpoint URLs, message schemas, sequence
semantics, symbol naming) lives entirely inside a per-exchange adapter. Core
probe code (`probe/internal/probeengine`, `probe/internal/orderbook`,
`probe/internal/metrics`) must never contain an exchange-specific branch.
**Do not build one "universal" sequence-validation algorithm** — sequence
and update-id semantics genuinely differ per venue (see Bybit notes below
for a concrete example: `u` vs `seq` mean different things), and pretending
otherwise produces false gap/resync reports.

## Adding a new adapter — checklist

1. Read the exchange's **official, current** public API docs yourself.
   Never guess an endpoint, message field, or sequence rule. If you can't
   verify something, leave a `// TODO(verify): ...` comment and a stub
   rather than inventing behavior.
2. Implement the `ExchangeAdapter` interface (below).
3. Add canonical instrument mapping entries (see "Canonical instrument
   mapping").
4. Add fixtures under `probe/internal/adapter/<venue>/testdata/` captured
   from real (or clearly synthetic, clearly labeled) traffic, including at
   least one fixture with an intentional sequence gap and one with a
   malformed message.
5. Add a short "Sequence semantics" and "Known quirks" section to this file.
6. Adapters must degrade gracefully: an unparseable message is a
   `data.schema_mismatch`/`data.invalid_json` observation, never a panic.

## Canonical instrument mapping

Exchanges do not share symbol naming. Configuration must never assume they
do. Example:

```yaml
instrument:
  canonical: BTC-USDT-PERP

venues:
  bybit: BTCUSDT       # category: linear
  okx: BTC-USDT-SWAP
  binance: BTCUSDT
  coinbase: BTC-USD    # closest liquid equivalent (spot, no USDT perp)
  kraken: PF_XBTUSD    # closest liquid equivalent
```

See `probe/internal/config` for the loader and `deploy/config/instruments.example.yaml`.

## The `ExchangeAdapter` interface

```go
// probe/internal/adapter/adapter.go

type ExchangeID string

type Adapter interface {
    Name() ExchangeID

    // ServerTime returns the exchange's reported server time, where the
    // exchange's public API exposes one. Used for clock_drift sampling.
    ServerTime(ctx context.Context) (ServerTime, error)

    // RESTProbe issues one public market-data request for the given
    // canonical instrument and returns a fully-classified observation.
    // Implementations must classify failures into the shared error
    // taxonomy (probe/internal/errtax) rather than returning raw errors.
    RESTProbe(ctx context.Context, instrument CanonicalInstrument) (RESTObservation, error)

    // ConnectMarketData opens a WebSocket connection, subscribes to the
    // configured channels for instrument, and returns a MarketDataStream
    // that the generic WS probe engine drives. The adapter owns protocol
    // details (subscribe message shape, ping/pong op names, ack matching);
    // the engine owns timing/metrics/reconnect policy.
    ConnectMarketData(ctx context.Context, instrument CanonicalInstrument, channels []ChannelKind) (MarketDataStream, error)
}

// MarketDataStream is adapter-agnostic; the engine reads typed events from
// it without knowing the wire format.
type MarketDataStream interface {
    Events() <-chan MarketDataEvent
    Errors() <-chan error
    Close() error
}

// SequenceValidator is implemented per-adapter (or per-channel within an
// adapter) because sequence semantics are not portable across venues.
type SequenceValidator interface {
    Validate(prev, current MarketDataEvent) SequenceStatus
}

type SequenceStatus struct {
    GapDetected  bool
    Duplicate    bool
    OutOfOrder   bool
    ExpectedNext *int64
}
```

This is deliberately narrower than "make one interface that also does order
book reconstruction generically" — order-book reconstruction rules
(snapshot/delta application, what a "reset" means) are exposed through a
separate, optional `OrderBookReconstructor` that only venues with a
documented, reconstructable public book need to implement.

## Bybit (V5 public API) — reference adapter

Implemented in this repository (`probe/internal/adapter/bybit`). Verified
against the official docs at `bybit-exchange.github.io/docs/v5` on
2026-09-27.

- **REST base URL:** `https://api.bybit.com`
- **Server time:** `GET /v5/market/time` → `result.timeSecond`,
  `result.timeNano`.
- **Tickers:** `GET /v5/market/tickers?category=linear&symbol=BTCUSDT`.
  Top-level envelope: `retCode`, `retMsg`, `result`, `time`. `retCode != 0`
  is classified `exchange.internal_error` unless it matches a known
  rate-limit code (`10006`), which is classified `exchange.rate_limit`.
- **Rate limit headers:** `X-Bapi-Limit`, `X-Bapi-Limit-Status`,
  `X-Bapi-Limit-Reset-Timestamp`.
- **WebSocket URLs:** `wss://stream.bybit.com/v5/public/linear` (USDT
  perpetuals — used for our canonical instrument) and
  `.../v5/public/spot`. Testnet equivalents exist under
  `stream-testnet.bybit.com`.
- **Subscribe:** `{"req_id": "...", "op": "subscribe", "args": ["orderbook.50.BTCUSDT", "publicTrade.BTCUSDT"]}`.
  Linear has no documented args-count limit; combined arg length is capped
  at 21,000 characters across all public connections.
- **Heartbeat:** client sends `{"op": "ping"}` every 20s; server replies
  `{"op": "pong", ...}`. Used for `ws_ping_pong_latency_seconds`.

### Sequence semantics (orderbook channel) — do not generalize this to other venues

Each orderbook message carries **two** distinct fields that are easy to
conflate:

- `data.u` — update ID. **`u == 1` signals a service-side restart/reset**;
  on seeing it, the local book must be rebuilt from the next snapshot
  rather than treated as a gap.
- `data.seq` — cross-sequence, monotonic, comparable across messages for
  gap detection between consecutive deltas for the same symbol/depth.

Our adapter therefore treats `u == 1` as an explicit reset signal
(`orderbook_resync_total`, not `orderbook_sequence_gaps_total`), and uses
`seq` discontinuity for actual gap detection
(`orderbook_sequence_gaps_total`). Delta application rule: size `"0"` in a
level deletes that price level; otherwise insert-or-update.

### Sequence semantics (publicTrade channel)

`data[].seq` is a cross-sequence; the docs explicitly note **a single
message may carry up to 1024 trades, and multiple messages may share the
same `seq`** (batching), so trade-channel sequence checks must treat equal
consecutive `seq` values as expected, not as duplicates — this is called
out directly in the adapter code and covered by a fixture.

### Known quirks

- Top-of-book (level 1) message field ordering places `ts` before `type`;
  other depth levels place `ts` after `type` — irrelevant to us because we
  parse by field name, but noted here for anyone hand-inspecting payloads.
- `cts` (matching-engine timestamp) is present on orderbook messages and
  used, not `ts`, when correlating against the trade channel, per the
  documented intent of that field.

### Not yet implemented / explicit TODOs

- Spot channel adapter variant (args-limit-of-10 handling) — REST-only for
  spot is implemented; WS spot subscription batching is a `// TODO(verify)`
  pending confirmation of current spot-specific rate rules.
- Options category — out of scope for MVP instrument set.

## Other MVP venues (Binance, OKX) and future venues

Not yet implemented in this repository. Each will follow the same
checklist above; do not add a stub adapter that fabricates endpoint or
schema details — an unimplemented adapter should simply not exist yet
rather than exist with invented behavior.

## Community/verified-community adapters

Community-contributed adapters are welcome (see `CONTRIBUTING.md`) subject
to the same checklist, plus: the exact official doc URL(s) consulted must
be cited in the PR description, and maintainers verify at least the
REST/WS base URLs and one full message schema against those docs before
merge.
