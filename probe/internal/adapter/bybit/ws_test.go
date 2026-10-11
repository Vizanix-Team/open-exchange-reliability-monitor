package bybit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/model"
)

// fakeExchangeServer is the CI stand-in for Bybit's public WebSocket,
// referenced by CONTRIBUTING.md's testing expectations: it must not
// depend on real exchange availability. It accepts a subscribe frame and
// then replays a fixed, deterministic scripted sequence of messages,
// including one that exercises the gap-detection path.
func fakeExchangeServer(t *testing.T) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		// Read (and discard) the subscribe frame.
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		_ = conn.WriteJSON(map[string]any{"op": "subscribe", "success": true, "req_id": "sub-1"})

		script := []string{
			`{"topic":"orderbook.50.BTCUSDT","type":"snapshot","ts":1758960000100,"cts":1758960000090,"data":{"s":"BTCUSDT","b":[["65000.5","1.0"]],"a":[["65001.0","1.0"]],"u":1,"seq":1}}`,
			`{"topic":"orderbook.50.BTCUSDT","type":"delta","ts":1758960000200,"cts":1758960000190,"data":{"s":"BTCUSDT","b":[["65000.5","0.5"]],"a":[],"u":2,"seq":2}}`,
			// deliberate gap: seq jumps from 2 to 9.
			`{"topic":"orderbook.50.BTCUSDT","type":"delta","ts":1758960000300,"cts":1758960000290,"data":{"s":"BTCUSDT","b":[],"a":[["65001.0","0.5"]],"u":3,"seq":9}}`,
		}
		for _, msg := range script {
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		// keep the connection open briefly so the client can finish
		// reading before the test tears the server down.
		time.Sleep(50 * time.Millisecond)
	}))
}

func TestConnectMarketData_GapDetectionEndToEnd(t *testing.T) {
	srv := fakeExchangeServer(t)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	a := New(WithWSLinearURL(wsURL))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := a.ConnectMarketData(ctx, "BTC-USDT-PERP", "BTCUSDT", []model.ChannelKind{model.ChannelOrderBook})
	if err != nil {
		t.Fatalf("ConnectMarketData: %v", err)
	}
	defer stream.Close()

	var gapSeen bool
	var eventCount int
	timeout := time.After(3 * time.Second)
loop:
	for {
		select {
		case evt, ok := <-stream.Events():
			if !ok {
				break loop
			}
			eventCount++
			if payload, ok := evt.Raw.(*OrderbookEventPayload); ok && payload.Status.GapDetected {
				gapSeen = true
			}
			if eventCount >= 3 {
				break loop
			}
		case <-timeout:
			break loop
		}
	}

	if eventCount < 3 {
		t.Fatalf("received %d events, want 3", eventCount)
	}
	if !gapSeen {
		t.Fatal("expected the scripted seq jump (2 -> 9) to be detected as a gap")
	}
}
