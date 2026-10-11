package errtax

import (
	"errors"
	"testing"
)

func TestClassifiedErrorMessage(t *testing.T) {
	cause := errors.New("connection reset")

	if got, want := New(NetworkConnect, "", cause).Error(), "network.connect: connection reset"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if got, want := New(DataSequenceGap, "", nil).Error(), "data.sequence_gap"; got != want {
		t.Errorf("Error() without cause = %q, want %q", got, want)
	}
}

func TestClassifiedErrorUnwrap(t *testing.T) {
	cause := errors.New("boom")
	err := error(New(ProbeInternal, "E1", cause))

	if !errors.Is(err, cause) {
		t.Error("errors.Is must see the wrapped cause")
	}
	var classified *ClassifiedError
	if !errors.As(err, &classified) {
		t.Fatal("errors.As must recover the *ClassifiedError")
	}
	if classified.Category != ProbeInternal || classified.Code != "E1" {
		t.Errorf("got category %q code %q", classified.Category, classified.Code)
	}
}

// Metrics label values come from Category, so every category must be a
// non-empty, unique string (None is the deliberate empty value).
func TestCategoriesAreUniqueAndNonEmpty(t *testing.T) {
	all := []Category{
		NetworkDNS, NetworkConnect, NetworkTimeout, NetworkTLS,
		HTTP4xx, HTTP429, HTTP5xx,
		ExchangeRateLimit, ExchangeInternal, ExchangeMaintenance,
		WebSocketDisconnect, WebSocketProtocol, WebSocketSubscriptionFailed,
		DataInvalidJSON, DataSchemaMismatch, DataStale, DataSequenceGap, DataDuplicate, DataOutOfOrder,
		ProbeInternal,
	}
	seen := make(map[Category]bool, len(all))
	for _, c := range all {
		if c == None {
			t.Errorf("category must not equal None: %q", c)
		}
		if seen[c] {
			t.Errorf("duplicate category %q", c)
		}
		seen[c] = true
	}
}
