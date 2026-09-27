package bybit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestRESTProbe_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v5/market/tickers" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("symbol"); got != "BTCUSDT" {
			t.Fatalf("symbol = %q, want BTCUSDT", got)
		}
		w.Header().Set("X-Bapi-Limit", "120")
		w.Header().Set("X-Bapi-Limit-Status", "119")
		w.Write(fixture(t, "tickers_success.json"))
	}))
	defer srv.Close()

	a := New(WithRESTBaseURL(srv.URL))
	obs, err := a.RESTProbe(context.Background(), "BTC-USDT-PERP", "BTCUSDT")
	if err != nil {
		t.Fatalf("RESTProbe: %v", err)
	}
	if !obs.Success {
		t.Fatal("expected Success=true")
	}
	if obs.RateLimitLimit == nil || *obs.RateLimitLimit != 120 {
		t.Fatalf("RateLimitLimit = %v, want 120", obs.RateLimitLimit)
	}
	if obs.ExchangeTimestamp == nil {
		t.Fatal("expected ExchangeTimestamp to be set")
	}
}

func TestRESTProbe_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(fixture(t, "tickers_rate_limited.json"))
	}))
	defer srv.Close()

	a := New(WithRESTBaseURL(srv.URL))
	obs, err := a.RESTProbe(context.Background(), "BTC-USDT-PERP", "BTCUSDT")
	if err == nil {
		t.Fatal("expected error for rate-limited retCode")
	}
	if obs.ErrorCategory != "exchange.rate_limit" {
		t.Fatalf("ErrorCategory = %q, want exchange.rate_limit", obs.ErrorCategory)
	}
}

func TestRESTProbe_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := New(WithRESTBaseURL(srv.URL))
	obs, err := a.RESTProbe(context.Background(), "BTC-USDT-PERP", "BTCUSDT")
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if obs.ErrorCategory != "http.5xx" {
		t.Fatalf("ErrorCategory = %q, want http.5xx", obs.ErrorCategory)
	}
}

func TestRESTProbe_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"retCode": 0, "result": {`)) // truncated/invalid JSON
	}))
	defer srv.Close()

	a := New(WithRESTBaseURL(srv.URL))
	obs, err := a.RESTProbe(context.Background(), "BTC-USDT-PERP", "BTCUSDT")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if obs.ErrorCategory != "data.invalid_json" {
		t.Fatalf("ErrorCategory = %q, want data.invalid_json", obs.ErrorCategory)
	}
}

func TestRESTProbe_OversizedResponseRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, maxResponseBytes+1024)
		w.Write(buf)
	}))
	defer srv.Close()

	a := New(WithRESTBaseURL(srv.URL))
	obs, err := a.RESTProbe(context.Background(), "BTC-USDT-PERP", "BTCUSDT")
	if err == nil {
		t.Fatal("expected error for oversized response")
	}
	if obs.ErrorCategory != "data.invalid_json" {
		t.Fatalf("ErrorCategory = %q, want data.invalid_json", obs.ErrorCategory)
	}
}

func TestServerTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"retCode":0,"retMsg":"OK","result":{"timeSecond":"1758960000","timeNano":"1758960000123456789"},"time":1758960000123}`))
	}))
	defer srv.Close()

	a := New(WithRESTBaseURL(srv.URL))
	st, err := a.ServerTime(context.Background())
	if err != nil {
		t.Fatalf("ServerTime: %v", err)
	}
	if st.ExchangeTime.Unix() != 1758960000 {
		t.Fatalf("ExchangeTime.Unix() = %d, want 1758960000", st.ExchangeTime.Unix())
	}
}
