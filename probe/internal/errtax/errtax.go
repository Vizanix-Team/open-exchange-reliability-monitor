// Package errtax defines the shared error taxonomy every probe outcome is
// classified into. See docs/METRICS.md ("Error taxonomy") for the
// authoritative list and docs/METHODOLOGY.md for why exchange-side and
// probe-side failures must never be conflated.
package errtax

// Category is a closed set of error classifications. New values must be
// added here and documented in docs/METRICS.md in the same change.
type Category string

const (
	NetworkDNS     Category = "network.dns"
	NetworkConnect Category = "network.connect"
	NetworkTimeout Category = "network.timeout"
	NetworkTLS     Category = "network.tls"

	HTTP4xx Category = "http.4xx"
	HTTP429 Category = "http.429"
	HTTP5xx Category = "http.5xx"

	ExchangeRateLimit   Category = "exchange.rate_limit"
	ExchangeInternal    Category = "exchange.internal_error"
	ExchangeMaintenance Category = "exchange.maintenance"

	WebSocketDisconnect         Category = "websocket.disconnect"
	WebSocketProtocol           Category = "websocket.protocol"
	WebSocketSubscriptionFailed Category = "websocket.subscription_failed"

	DataInvalidJSON    Category = "data.invalid_json"
	DataSchemaMismatch Category = "data.schema_mismatch"
	DataStale          Category = "data.stale"
	DataSequenceGap    Category = "data.sequence_gap"
	DataDuplicate      Category = "data.duplicate"
	DataOutOfOrder     Category = "data.out_of_order"

	// ProbeInternal marks a failure in the probe itself (bug, panic,
	// resource exhaustion). Never counted toward exchange-facing
	// statistics or incident-engine input — see ARCHITECTURE.md
	// "Anti-false-positive design".
	ProbeInternal Category = "probe.internal"

	None Category = ""
)

// ClassifiedError pairs a taxonomy category with the underlying error, so
// callers can log/debug the detail while metrics only ever see the bounded
// category label.
type ClassifiedError struct {
	Category Category
	Code     string // exchange- or protocol-specific code, if any; never used as a metric label
	Err      error
}

func (e *ClassifiedError) Error() string {
	if e.Err == nil {
		return string(e.Category)
	}
	return string(e.Category) + ": " + e.Err.Error()
}

func (e *ClassifiedError) Unwrap() error { return e.Err }

func New(cat Category, code string, err error) *ClassifiedError {
	return &ClassifiedError{Category: cat, Code: code, Err: err}
}
