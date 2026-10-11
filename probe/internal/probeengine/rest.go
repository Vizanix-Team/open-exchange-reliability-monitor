// Package probeengine drives adapters on a schedule and translates their
// observations into Prometheus metrics. It contains no exchange-specific
// logic — see docs/EXCHANGE_ADAPTERS.md.
package probeengine

import (
	"context"
	"log/slog"
	"time"

	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/adapter"
	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/metrics"
	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/model"
)

// RESTLoop polls Adapter.RESTProbe on interval until ctx is canceled.
type RESTLoop struct {
	Adapter    adapter.Adapter
	Instrument model.CanonicalInstrument
	Region     string
	Interval   time.Duration
	Logger     *slog.Logger
}

func (l *RESTLoop) Run(ctx context.Context) {
	if l.Interval <= 0 {
		l.Interval = 5 * time.Second
	}
	ticker := time.NewTicker(l.Interval)
	defer ticker.Stop()

	l.probeOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.probeOnce(ctx)
		}
	}
}

func (l *RESTLoop) probeOnce(ctx context.Context) {
	exchange := string(l.Adapter.Name())
	obs, err := l.Adapter.RESTProbe(ctx, l.Instrument)

	labels := prometheusLabels(exchange, l.Region)
	if !obs.RequestStart.IsZero() && !obs.DNSDone.IsZero() {
		metrics.RESTDNSSeconds.With(labels).Observe(obs.DNSDone.Sub(obs.RequestStart).Seconds())
	}
	if !obs.DNSDone.IsZero() && !obs.ConnDone.IsZero() {
		metrics.RESTTCPConnectSeconds.With(labels).Observe(obs.ConnDone.Sub(obs.DNSDone).Seconds())
	}
	if !obs.ConnDone.IsZero() && !obs.TLSDone.IsZero() {
		metrics.RESTTLSHandshakeSeconds.With(labels).Observe(obs.TLSDone.Sub(obs.ConnDone).Seconds())
	}
	if !obs.RequestStart.IsZero() && !obs.FirstByte.IsZero() {
		metrics.RESTTTFBSeconds.With(labels).Observe(obs.FirstByte.Sub(obs.RequestStart).Seconds())
	}
	if total := obs.TotalLatency(); total > 0 {
		metrics.RESTTotalLatencySeconds.With(labels).Observe(total.Seconds())
	}
	if obs.ExchangeTimestamp != nil {
		metrics.RESTTimestampFreshnessSeconds.With(labels).Observe(obs.Done.Sub(*obs.ExchangeTimestamp).Seconds())
	}

	outcome := "success"
	if err != nil {
		outcome = string(obs.ErrorCategory)
		if outcome == "" {
			outcome = "probe.internal"
		}
		l.Logger.Warn("rest probe failed", "exchange", exchange, "region", l.Region, "category", outcome, "err", err)
	}
	metrics.RESTRequestsTotal.WithLabelValues(exchange, l.Region, obs.Endpoint, outcome).Inc()
}

func prometheusLabels(exchange, region string) map[string]string {
	return map[string]string{"exchange": exchange, "region": region}
}
