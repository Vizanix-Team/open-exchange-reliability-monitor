// Package model holds the wire-agnostic types shared across adapters, the
// probe engines, and the exporter: canonical instruments, observation
// records, and market-data events. Field names and semantics mirror the
// probe_measurement / ws_event schema documented in docs/METRICS.md.
package model

import (
	"time"

	"github.com/vizanix/open-exchange-reliability-monitor/probe/internal/errtax"
)

// CanonicalInstrument is the project-wide instrument identifier, decoupled
// from any single exchange's symbol naming. See docs/EXCHANGE_ADAPTERS.md
// "Canonical instrument mapping".
type CanonicalInstrument string

// ExchangeID identifies a venue. Kept as a distinct type (not a bare
// string) so adapter registration is compile-time checked.
type ExchangeID string

// ChannelKind is a protocol-agnostic market-data channel classification.
type ChannelKind string

const (
	ChannelOrderBook ChannelKind = "orderbook"
	ChannelTrades    ChannelKind = "trades"
	ChannelTicker    ChannelKind = "ticker"
)

// ServerTime is the result of sampling an exchange's public server-time
// endpoint, used to compute exchange.clock_drift_seconds.
type ServerTime struct {
	ExchangeTime time.Time
	ProbeSentAt  time.Time
	ProbeRecvAt  time.Time
}

// RESTObservation is one classified REST probe attempt. Mirrors the
// probe_measurement record in docs/METRICS.md.
type RESTObservation struct {
	Exchange     ExchangeID
	Endpoint     string
	Instrument   CanonicalInstrument
	RequestStart time.Time

	DNSDone   time.Time
	ConnDone  time.Time
	TLSDone   time.Time
	FirstByte time.Time
	Done      time.Time

	Success            bool
	HTTPStatus         int
	ExchangeTimestamp  *time.Time
	RateLimitLimit     *int
	RateLimitRemaining *int

	ErrorCategory errtax.Category
	ErrorCode     string
}

// TotalLatency returns t_response_complete - t_request_start, or zero if
// the observation never completed.
func (o RESTObservation) TotalLatency() time.Duration {
	if o.Done.IsZero() || o.RequestStart.IsZero() {
		return 0
	}
	return o.Done.Sub(o.RequestStart)
}

// MarketDataEvent is one WebSocket market-data message, normalized across
// adapters. Sequence fields are optional because not every channel/venue
// exposes them; adapters populate what their documented protocol provides.
type MarketDataEvent struct {
	Exchange   ExchangeID
	Instrument CanonicalInstrument
	Channel    ChannelKind
	Kind       EventKind

	EventTimestamp   time.Time // exchange-reported, where available
	ReceiveTimestamp time.Time // probe-local

	Sequence    *int64
	IsReset     bool // adapter-detected explicit reset signal (e.g. Bybit orderbook u==1)
	PayloadSize int

	// Raw carries the adapter-parsed payload for downstream consumers
	// (e.g. order book reconstruction). Left as `any` deliberately: the
	// shared engine never inspects it, only adapter-specific code does.
	Raw any
}

type EventKind string

const (
	EventData      EventKind = "data"
	EventError     EventKind = "error"
	EventHeartbeat EventKind = "heartbeat"
	EventAck       EventKind = "ack"
)

// SequenceStatus is the result of validating one event against adapter- or
// channel-specific sequence semantics. See docs/EXCHANGE_ADAPTERS.md: this
// is deliberately per-adapter/per-channel, never a single generic rule.
type SequenceStatus struct {
	GapDetected bool
	Duplicate   bool
	OutOfOrder  bool
	Reset       bool
}
