// Package adapter defines the interface every exchange integration must
// implement. See docs/EXCHANGE_ADAPTERS.md for the full contract and the
// checklist for adding a new venue. Core probe code must never import a
// concrete adapter package directly except at binary-wiring time in
// cmd/probe — everything else depends on this interface only.
package adapter

import (
	"context"

	"github.com/vizanix/open-exchange-reliability-monitor/probe/internal/model"
)

// Adapter is the exchange-specific integration surface. Implementations
// own all protocol detail (URLs, message shapes, sequence semantics);
// the probe engines own timing, retries, metrics, and buffering.
type Adapter interface {
	Name() model.ExchangeID

	// ServerTime samples the exchange's public server-time endpoint, if
	// one exists. Adapters without one return errtax.ProbeInternal-free
	// ErrNotSupported so the engine can skip clock-drift sampling for
	// that venue without treating it as a failure.
	ServerTime(ctx context.Context) (model.ServerTime, error)

	// RESTProbe issues one public market-data request for instrument and
	// returns a fully-classified observation. It must not return a raw,
	// unclassified error for anything reachable from network/exchange
	// input — see docs/EXCHANGE_ADAPTERS.md.
	RESTProbe(ctx context.Context, instrument model.CanonicalInstrument) (model.RESTObservation, error)

	// ConnectMarketData opens a WebSocket connection, subscribes to the
	// requested channels for instrument, and returns a live stream.
	ConnectMarketData(ctx context.Context, instrument model.CanonicalInstrument, channels []model.ChannelKind) (MarketDataStream, error)
}

// MarketDataStream is the adapter-agnostic handle the WS probe engine
// drives. Close must be safe to call multiple times.
type MarketDataStream interface {
	Events() <-chan model.MarketDataEvent
	Errors() <-chan error
	// Ping sends a protocol-level keepalive and reports round-trip time.
	// Adapters without an application-level ping may implement this as a
	// WebSocket control-frame ping.
	Ping(ctx context.Context) (roundTrip int64, err error)
	Close() error
}

// SequenceValidator is implemented per adapter/channel because sequence
// semantics are not portable across venues (see docs/EXCHANGE_ADAPTERS.md,
// Bybit's u vs seq distinction as a concrete example of why not).
type SequenceValidator interface {
	Validate(prev, current model.MarketDataEvent) model.SequenceStatus
}
