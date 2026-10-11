package bybit

import "github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/model"

// orderbookSequenceValidator implements the sequence rule documented in
// docs/EXCHANGE_ADAPTERS.md "Bybit — sequence semantics (orderbook
// channel)": data.u == 1 is an explicit service-restart/reset signal
// (never treated as a gap), and data.seq must be strictly increasing
// between consecutive messages for the same symbol/depth for there to be
// no gap. This type intentionally implements adapter.SequenceValidator
// and must not be reused for other venues — sequence semantics are not
// portable (see EXCHANGE_ADAPTERS.md's warning against one universal
// algorithm).
type orderbookSequenceValidator struct{}

func (orderbookSequenceValidator) Validate(prev, current model.MarketDataEvent) model.SequenceStatus {
	if current.IsReset {
		return model.SequenceStatus{Reset: true}
	}
	if prev.Sequence == nil || current.Sequence == nil {
		// First message in the stream, or a message type that doesn't
		// carry a sequence (shouldn't happen for orderbook, but fail
		// safe rather than panic).
		return model.SequenceStatus{}
	}
	switch {
	case *current.Sequence == *prev.Sequence:
		return model.SequenceStatus{Duplicate: true}
	case *current.Sequence < *prev.Sequence:
		return model.SequenceStatus{OutOfOrder: true}
	case *current.Sequence > *prev.Sequence+1:
		// Bybit's seq is a cross-sequence shared across the whole
		// orderbook stream for the symbol, not a strictly-per-message
		// increment-by-one counter in all depth configurations; treat any
		// forward jump as a potential gap. Adapters for other venues with
		// stricter per-message +1 semantics may tighten this.
		return model.SequenceStatus{GapDetected: true}
	default:
		return model.SequenceStatus{}
	}
}

// tradeSequenceValidator implements docs/EXCHANGE_ADAPTERS.md "Bybit —
// sequence semantics (publicTrade channel)": a single message may carry
// up to 1024 trades and multiple messages may share the same seq
// (batching), so equal consecutive seq values are expected and must not
// be flagged as duplicates. Only a backward jump is a real anomaly.
type tradeSequenceValidator struct{}

func (tradeSequenceValidator) Validate(prev, current model.MarketDataEvent) model.SequenceStatus {
	if prev.Sequence == nil || current.Sequence == nil {
		return model.SequenceStatus{}
	}
	if *current.Sequence < *prev.Sequence {
		return model.SequenceStatus{OutOfOrder: true}
	}
	return model.SequenceStatus{}
}
