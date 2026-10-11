package probeengine

import (
	"context"
	"log/slog"
	"time"

	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/adapter"
	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/metrics"
	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/model"
)

// staleThresholdMultiplier implements the Methodology's "silence
// substantially exceeds what recent activity would predict" rule: a
// stream is flagged stale only once the gap exceeds this multiple of the
// trailing average inter-message interval, and never below minStaleGap so
// a single slow message on a fast stream doesn't false-positive.
const staleThresholdMultiplier = 10
const minStaleGap = 30 * time.Second

// WSLoop maintains a WebSocket connection via Adapter.ConnectMarketData,
// reconnecting with backoff on disconnect, and turns stream events into
// metrics. Sequence semantics themselves are entirely adapter-owned; this
// loop only counts what the adapter already classified.
type WSLoop struct {
	Adapter    adapter.Adapter
	Instrument model.CanonicalInstrument
	Channels   []model.ChannelKind
	Region     string
	Logger     *slog.Logger

	MinBackoff time.Duration
	MaxBackoff time.Duration
}

func (l *WSLoop) Run(ctx context.Context) {
	backoff := l.MinBackoff
	if backoff <= 0 {
		backoff = time.Second
	}
	maxBackoff := l.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 60 * time.Second
	}

	exchange := string(l.Adapter.Name())
	var lastDisconnect time.Time

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		connectStart := time.Now()
		s, err := l.Adapter.ConnectMarketData(ctx, l.Instrument, l.Channels)
		if err != nil {
			l.Logger.Warn("ws connect failed", "exchange", exchange, "region", l.Region, "err", err)
			if !sleepCtx(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff, maxBackoff)
			continue
		}
		metrics.WSConnectLatencySeconds.WithLabelValues(exchange, l.Region).Observe(time.Since(connectStart).Seconds())
		if !lastDisconnect.IsZero() {
			metrics.WSReconnectDurationSeconds.WithLabelValues(exchange, l.Region).Observe(time.Since(lastDisconnect).Seconds())
		}
		backoff = l.MinBackoff
		if backoff <= 0 {
			backoff = time.Second
		}

		l.driveStream(ctx, s, exchange)
		lastDisconnect = time.Now()
		metrics.WSDisconnectsTotal.WithLabelValues(exchange, l.Region, "all", "false").Inc()

		if !sleepCtx(ctx, backoff) {
			return
		}
	}
}

func (l *WSLoop) driveStream(ctx context.Context, s adapter.MarketDataStream, exchange string) {
	defer s.Close()

	pingTicker := time.NewTicker(20 * time.Second)
	defer pingTicker.Stop()

	lastByChannel := map[model.ChannelKind]time.Time{}
	baselineIntervalByChannel := map[model.ChannelKind]time.Duration{}

	for {
		select {
		case <-ctx.Done():
			return
		case <-pingTicker.C:
			_, _ = s.Ping(ctx)
		case evt, ok := <-s.Events():
			if !ok {
				return
			}
			l.observeEvent(exchange, evt, lastByChannel, baselineIntervalByChannel)
		case err, ok := <-s.Errors():
			if !ok {
				return
			}
			l.Logger.Warn("ws stream error", "exchange", exchange, "region", l.Region, "err", err)
		}
	}
}

func (l *WSLoop) observeEvent(exchange string, evt model.MarketDataEvent, lastByChannel map[model.ChannelKind]time.Time, baseline map[model.ChannelKind]time.Duration) {
	labels3 := []string{exchange, l.Region, string(evt.Channel)}
	metrics.WSMessagesTotal.WithLabelValues(append(append([]string{}, labels3...), string(evt.Kind))...).Inc()

	if !evt.EventTimestamp.IsZero() {
		delay := evt.ReceiveTimestamp.Sub(evt.EventTimestamp).Seconds()
		metrics.WSObservedEventDelaySeconds.WithLabelValues(labels3...).Observe(delay)
	}

	// Stale-stream heuristic per docs/METHODOLOGY.md: compare against a
	// trailing baseline interval, not an absolute threshold.
	if last, ok := lastByChannel[evt.Channel]; ok {
		gap := evt.ReceiveTimestamp.Sub(last)
		base := baseline[evt.Channel]
		if base > 0 && gap > minStaleGap && gap > base*staleThresholdMultiplier {
			metrics.DataStalePeriodsTotal.WithLabelValues(labels3...).Inc()
		}
		if base == 0 {
			baseline[evt.Channel] = gap
		} else {
			// exponential moving average keeps the baseline adaptive to
			// genuine regime changes in market activity.
			baseline[evt.Channel] = (base*4 + gap) / 5
		}
	}
	lastByChannel[evt.Channel] = evt.ReceiveTimestamp

	if sw, ok := evt.Raw.(interface{ GetSequenceStatus() model.SequenceStatus }); ok {
		applySeqMetrics(labels3, sw.GetSequenceStatus())
	}
}

func applySeqMetrics(labels3 []string, status model.SequenceStatus) {
	if status.GapDetected {
		metrics.WSSequenceGapsTotal.WithLabelValues(labels3...).Inc()
	}
	if status.Duplicate {
		metrics.WSDuplicateMessagesTotal.WithLabelValues(labels3...).Inc()
	}
	if status.OutOfOrder {
		metrics.WSOutOfOrderMessagesTotal.WithLabelValues(labels3...).Inc()
	}
	if status.Reset {
		metrics.OrderBookResyncTotal.WithLabelValues(labels3[0], labels3[1]).Inc()
	}
	if status.GapDetected {
		metrics.OrderBookSequenceGapsTotal.WithLabelValues(labels3[0], labels3[1]).Inc()
	}
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func nextBackoff(cur, max time.Duration) time.Duration {
	next := cur * 2
	if next > max {
		next = max
	}
	return next
}
