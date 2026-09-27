// Package metrics defines and registers the probe's local Prometheus
// exporter. Label sets are deliberately bounded — see docs/METRICS.md
// "Statistical aggregation notes" and ARCHITECTURE.md "cost-conscious
// deployment" for why unbounded labels (raw symbols beyond the configured
// instrument set, raw error text, per-connection IDs) are never used here.
package metrics

import "github.com/prometheus/client_golang/prometheus"

// bucket layouts tuned for network/exchange RTTs (1ms .. ~8s).
var latencyBuckets = prometheus.ExponentialBuckets(0.001, 2, 14)

var (
	RESTDNSSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "rest_dns_resolution_seconds",
		Help:    "DNS resolution time for a REST probe request. See docs/METRICS.md.",
		Buckets: latencyBuckets,
	}, []string{"exchange", "region"})

	RESTTCPConnectSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "rest_tcp_connect_seconds",
		Help:    "TCP connect time for a REST probe request. See docs/METRICS.md.",
		Buckets: latencyBuckets,
	}, []string{"exchange", "region"})

	RESTTLSHandshakeSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "rest_tls_handshake_seconds",
		Help:    "TLS handshake time for a REST probe request. See docs/METRICS.md.",
		Buckets: latencyBuckets,
	}, []string{"exchange", "region"})

	RESTTTFBSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "rest_ttfb_seconds",
		Help:    "Time to first response byte for a REST probe request. See docs/METRICS.md.",
		Buckets: latencyBuckets,
	}, []string{"exchange", "region"})

	RESTTotalLatencySeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "rest_total_latency_seconds",
		Help:    "End-to-end REST probe request latency. See docs/METRICS.md.",
		Buckets: latencyBuckets,
	}, []string{"exchange", "region"})

	RESTRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "rest_requests_total",
		Help: "REST probe attempts by outcome. See docs/METRICS.md.",
	}, []string{"exchange", "region", "endpoint", "outcome"})

	RESTTimestampFreshnessSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "rest_timestamp_freshness_seconds",
		Help:    "local_receive_time - exchange_reported_timestamp. See docs/METHODOLOGY.md clock caveats.",
		Buckets: latencyBuckets,
	}, []string{"exchange", "region"})

	ExchangeClockDriftSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "exchange_clock_drift_seconds",
		Help: "probe_local_time - exchange_server_time, sampled. See docs/METHODOLOGY.md.",
	}, []string{"exchange", "region"})

	WSConnectLatencySeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ws_connect_latency_seconds",
		Help:    "WebSocket connection establishment latency. See docs/METRICS.md.",
		Buckets: latencyBuckets,
	}, []string{"exchange", "region"})

	WSSubscriptionAckLatencySeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ws_subscription_ack_latency_seconds",
		Help:    "Subscription acknowledgement latency, where the protocol exposes one. See docs/METRICS.md.",
		Buckets: latencyBuckets,
	}, []string{"exchange", "region", "channel"})

	WSTimeToFirstEventSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ws_time_to_first_event_seconds",
		Help:    "Time from subscription ack to first market-data event. See docs/METRICS.md.",
		Buckets: latencyBuckets,
	}, []string{"exchange", "region", "channel"})

	WSConnectionLifetimeSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ws_connection_lifetime_seconds",
		Help:    "Duration a WebSocket connection stayed open. See docs/METRICS.md.",
		Buckets: prometheus.ExponentialBuckets(1, 2, 16),
	}, []string{"exchange", "region"})

	WSDisconnectsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ws_disconnects_total",
		Help: "WebSocket disconnects. See docs/METRICS.md.",
	}, []string{"exchange", "region", "channel", "planned"})

	WSReconnectDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ws_reconnect_duration_seconds",
		Help:    "Time from disconnect to next successful connection. See docs/METRICS.md.",
		Buckets: latencyBuckets,
	}, []string{"exchange", "region"})

	WSPingPongLatencySeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ws_ping_pong_latency_seconds",
		Help:    "Ping/pong round-trip latency. See docs/METRICS.md.",
		Buckets: latencyBuckets,
	}, []string{"exchange", "region"})

	WSMessagesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ws_messages_total",
		Help: "WebSocket messages received, by kind. See docs/METRICS.md.",
	}, []string{"exchange", "region", "channel", "kind"})

	WSObservedEventDelaySeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ws_observed_event_delay_seconds",
		Help:    "local_receive_timestamp - exchange_event_timestamp. See docs/METHODOLOGY.md clock caveats.",
		Buckets: latencyBuckets,
	}, []string{"exchange", "region", "channel"})

	WSSequenceGapsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ws_sequence_gaps_total",
		Help: "Detected sequence discontinuities. See docs/EXCHANGE_ADAPTERS.md for per-venue semantics.",
	}, []string{"exchange", "region", "channel"})

	WSDuplicateMessagesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ws_duplicate_messages_total",
		Help: "Detected duplicate messages. See docs/METRICS.md.",
	}, []string{"exchange", "region", "channel"})

	WSOutOfOrderMessagesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ws_out_of_order_messages_total",
		Help: "Detected out-of-order messages. See docs/METRICS.md.",
	}, []string{"exchange", "region", "channel"})

	DataStalePeriodsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "data_stale_periods_total",
		Help: "Stream-silence periods flagged stale. See docs/METHODOLOGY.md.",
	}, []string{"exchange", "region", "channel"})

	OrderBookReconstructionSuccessRatio = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "orderbook_reconstruction_success_ratio",
		Help: "Windowed fraction of gap-free order book reconstruction. See docs/METRICS.md.",
	}, []string{"exchange", "region"})

	OrderBookSequenceGapsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orderbook_sequence_gaps_total",
		Help: "Order book sequence gaps detected during reconstruction. See docs/METRICS.md.",
	}, []string{"exchange", "region"})

	OrderBookResyncTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orderbook_resync_total",
		Help: "Order book resyncs (explicit reset signal or gap-triggered). See docs/METRICS.md.",
	}, []string{"exchange", "region"})

	OrderBookCrossedBookTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orderbook_crossed_book_total",
		Help: "Detected crossed order books (best bid >= best ask). See docs/METRICS.md.",
	}, []string{"exchange", "region"})

	OrderBookInvalidQuantityTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orderbook_invalid_quantity_total",
		Help: "Detected negative/invalid quantities in order book deltas. See docs/METRICS.md.",
	}, []string{"exchange", "region"})

	ProbeUp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_up",
		Help: "1 if this probe process is running and healthy.",
	})

	ProbeControlEndpointSuccessRatio = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_control_endpoint_success_ratio",
		Help: "Success ratio against independent non-exchange control endpoints. See ARCHITECTURE.md.",
	})

	ProbeBufferSizeBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_buffer_size_bytes",
		Help: "Current size of the bounded local upload buffer.",
	})

	ProbeBufferDroppedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "probe_buffer_dropped_total",
		Help: "Measurements dropped because the local buffer overflowed.",
	})
)

// Registry returns a fresh, isolated registry with every metric above
// registered. Using a dedicated registry (not the global default) keeps
// the exporter's surface exactly equal to what's documented in
// docs/METRICS.md and makes it trivial to unit test.
func Registry() *prometheus.Registry {
	r := prometheus.NewRegistry()
	r.MustRegister(
		RESTDNSSeconds, RESTTCPConnectSeconds, RESTTLSHandshakeSeconds,
		RESTTTFBSeconds, RESTTotalLatencySeconds, RESTRequestsTotal,
		RESTTimestampFreshnessSeconds, ExchangeClockDriftSeconds,
		WSConnectLatencySeconds, WSSubscriptionAckLatencySeconds,
		WSTimeToFirstEventSeconds, WSConnectionLifetimeSeconds,
		WSDisconnectsTotal, WSReconnectDurationSeconds, WSPingPongLatencySeconds,
		WSMessagesTotal, WSObservedEventDelaySeconds, WSSequenceGapsTotal,
		WSDuplicateMessagesTotal, WSOutOfOrderMessagesTotal, DataStalePeriodsTotal,
		OrderBookReconstructionSuccessRatio, OrderBookSequenceGapsTotal,
		OrderBookResyncTotal, OrderBookCrossedBookTotal, OrderBookInvalidQuantityTotal,
		ProbeUp, ProbeControlEndpointSuccessRatio, ProbeBufferSizeBytes, ProbeBufferDroppedTotal,
	)
	return r
}
