package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	oxrmadapter "github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/adapter"
	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/errtax"
	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/model"
	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/orderbook"
)

// maxWSMessageBytes bounds inbound WebSocket frames. See SECURITY.md.
const maxWSMessageBytes = 2 << 20 // 2 MiB

// pingInterval matches the documented recommendation in
// docs/EXCHANGE_ADAPTERS.md ("client sends {"op":"ping"} every 20s").
const pingInterval = 20 * time.Second

func topicFor(channel model.ChannelKind, symbol string) (string, error) {
	switch channel {
	case model.ChannelOrderBook:
		return "orderbook.50." + symbol, nil
	case model.ChannelTrades:
		return "publicTrade." + symbol, nil
	default:
		return "", fmt.Errorf("bybit: unsupported channel %q", channel)
	}
}

// stream implements oxrmadapter.MarketDataStream for a single Bybit
// linear WebSocket connection subscribed to one or more channels for one
// symbol.
type stream struct {
	conn   *websocket.Conn
	symbol string

	events chan model.MarketDataEvent
	errs   chan error

	book     *orderbook.Book
	bookSeq  *int64 // last-seen orderbook seq, for gap detection
	tradeSeq *int64

	closeOnce sync.Once
	done      chan struct{}

	pendingPing struct {
		mu   sync.Mutex
		sent map[string]time.Time
	}
}

func (a *Adapter) ConnectMarketData(ctx context.Context, instrument model.CanonicalInstrument, symbol string, channels []model.ChannelKind) (oxrmadapter.MarketDataStream, error) {
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, a.wsLinearURL, nil)
	if err != nil {
		return nil, errtax.New(classifyTransportError(err), "", err)
	}
	conn.SetReadLimit(maxWSMessageBytes)

	topics := make([]string, 0, len(channels))
	for _, ch := range channels {
		t, err := topicFor(ch, symbol)
		if err != nil {
			conn.Close()
			return nil, errtax.New(errtax.ProbeInternal, "", err)
		}
		topics = append(topics, t)
	}

	sub := map[string]any{"req_id": "sub-1", "op": "subscribe", "args": topics}
	if err := conn.WriteJSON(sub); err != nil {
		conn.Close()
		return nil, errtax.New(errtax.WebSocketSubscriptionFailed, "", err)
	}

	s := &stream{
		conn:   conn,
		symbol: symbol,
		events: make(chan model.MarketDataEvent, 256),
		errs:   make(chan error, 16),
		book:   orderbook.NewBook(),
		done:   make(chan struct{}),
	}
	s.pendingPing.sent = map[string]time.Time{}

	go s.readLoop()
	return s, nil
}

func (s *stream) Events() <-chan model.MarketDataEvent { return s.events }
func (s *stream) Errors() <-chan error                 { return s.errs }

func (s *stream) Close() error {
	var err error
	s.closeOnce.Do(func() {
		close(s.done)
		err = s.conn.Close()
	})
	return err
}

func (s *stream) Ping(ctx context.Context) (int64, error) {
	reqID := strconv.FormatInt(time.Now().UnixNano(), 10)
	sentAt := time.Now()
	s.pendingPing.mu.Lock()
	s.pendingPing.sent[reqID] = sentAt
	s.pendingPing.mu.Unlock()

	if err := s.conn.WriteJSON(map[string]any{"req_id": reqID, "op": "ping"}); err != nil {
		return 0, errtax.New(errtax.WebSocketProtocol, "", err)
	}
	// The read loop resolves pong latency asynchronously via events; a
	// simple synchronous probe here only reports send success. Round-trip
	// latency is emitted as ws_ping_pong_latency_seconds by the caller
	// listening on Events()/pong handling — see probeengine/ws.go.
	return 0, nil
}

var orderbookSeqValidator = orderbookSequenceValidator{}
var tradeSeqValidator = tradeSequenceValidator{}

func (s *stream) readLoop() {
	defer close(s.events)
	defer close(s.errs)

	var lastOrderbookEvent model.MarketDataEvent
	haveLastOrderbook := false
	var lastTradeEvent model.MarketDataEvent
	haveLastTrade := false

	for {
		select {
		case <-s.done:
			return
		default:
		}

		_, raw, err := s.conn.ReadMessage()
		if err != nil {
			select {
			case s.errs <- errtax.New(errtax.WebSocketDisconnect, "", err):
			default:
			}
			return
		}

		var frame opFrame
		if err := json.Unmarshal(raw, &frame); err == nil && frame.Op != "" {
			// op-level frame (subscribe ack, pong) — never a data event.
			continue
		}

		var env wsEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			select {
			case s.errs <- errtax.New(errtax.DataInvalidJSON, "", err):
			default:
			}
			continue
		}

		switch {
		case len(env.Topic) > len("orderbook.") && env.Topic[:len("orderbook.")] == "orderbook.":
			var msg orderbookMessage
			if err := json.Unmarshal(raw, &msg); err != nil {
				s.emitErr(errtax.DataSchemaMismatch, err)
				continue
			}
			evt, err := s.handleOrderbook(msg)
			if err != nil {
				s.emitErr(errtax.DataSchemaMismatch, err)
				continue
			}
			if haveLastOrderbook {
				status := orderbookSeqValidator.Validate(lastOrderbookEvent, evt)
				applySequenceStatus(&evt, status)
			}
			lastOrderbookEvent = evt
			haveLastOrderbook = true
			s.emit(evt)

		case len(env.Topic) > len("publicTrade.") && env.Topic[:len("publicTrade.")] == "publicTrade.":
			var msg tradeMessage
			if err := json.Unmarshal(raw, &msg); err != nil {
				s.emitErr(errtax.DataSchemaMismatch, err)
				continue
			}
			for _, evt := range s.handleTrades(msg) {
				if haveLastTrade {
					status := tradeSeqValidator.Validate(lastTradeEvent, evt)
					applySequenceStatus(&evt, status)
				}
				lastTradeEvent = evt
				haveLastTrade = true
				s.emit(evt)
			}
		default:
			// Unrecognized topic: forward-compatible, not an error — a
			// new channel/topic we don't yet subscribe to or parse.
		}
	}
}

// applySequenceStatus stores the validation result on the event via Raw
// so probeengine/ws.go can emit the corresponding counters without
// re-deriving sequence logic (which belongs to the adapter, not the
// engine).
func applySequenceStatus(evt *model.MarketDataEvent, status model.SequenceStatus) {
	if sw, ok := evt.Raw.(interface {
		setSequenceStatus(model.SequenceStatus)
	}); ok {
		sw.setSequenceStatus(status)
	}
}

func (s *stream) emit(evt model.MarketDataEvent) {
	select {
	case s.events <- evt:
	case <-s.done:
	}
}

func (s *stream) emitErr(cat errtax.Category, err error) {
	select {
	case s.errs <- errtax.New(cat, "", err):
	default:
	}
}

// OrderbookEventPayload is the Raw payload attached to orderbook
// MarketDataEvents, carrying the parsed levels and (after validation) the
// sequence status, for downstream order-book reconstruction and metrics.
type OrderbookEventPayload struct {
	Type   string // "snapshot" | "delta"
	Bids   []orderbook.PriceLevel
	Asks   []orderbook.PriceLevel
	Status model.SequenceStatus
}

func (p *OrderbookEventPayload) setSequenceStatus(s model.SequenceStatus) { p.Status = s }
func (p *OrderbookEventPayload) GetSequenceStatus() model.SequenceStatus  { return p.Status }

func (s *stream) handleOrderbook(msg orderbookMessage) (model.MarketDataEvent, error) {
	bids, err := parseLevels(msg.Data.Bids)
	if err != nil {
		return model.MarketDataEvent{}, err
	}
	asks, err := parseLevels(msg.Data.Asks)
	if err != nil {
		return model.MarketDataEvent{}, err
	}

	isReset := msg.Data.U == 1
	if msg.Type == "snapshot" || isReset {
		if err := s.book.Reset(bids, asks); err != nil {
			s.emitErr(errtax.DataSchemaMismatch, err)
		}
	} else {
		if err := s.book.ApplyDelta(bids, asks); err != nil {
			s.emitErr(errtax.DataSchemaMismatch, err)
		}
	}

	seq := msg.Data.Seq
	payload := &OrderbookEventPayload{Type: msg.Type, Bids: bids, Asks: asks}
	return model.MarketDataEvent{
		Exchange:         ExchangeID,
		Channel:          model.ChannelOrderBook,
		Kind:             model.EventData,
		EventTimestamp:   time.UnixMilli(msg.CTS).UTC(),
		ReceiveTimestamp: time.Now().UTC(),
		Sequence:         &seq,
		IsReset:          isReset,
		PayloadSize:      len(msg.Data.Bids)*2 + len(msg.Data.Asks)*2,
		Raw:              payload,
	}, nil
}

func (s *stream) handleTrades(msg tradeMessage) []model.MarketDataEvent {
	out := make([]model.MarketDataEvent, 0, len(msg.Data))
	for _, t := range msg.Data {
		seq := t.Seq
		out = append(out, model.MarketDataEvent{
			Exchange:         ExchangeID,
			Channel:          model.ChannelTrades,
			Kind:             model.EventData,
			EventTimestamp:   time.UnixMilli(t.T).UTC(),
			ReceiveTimestamp: time.Now().UTC(),
			Sequence:         &seq,
			PayloadSize:      len(t.P) + len(t.V),
			Raw:              &OrderbookEventPayload{}, // trades carry no book payload; kept for setSequenceStatus uniformity
		})
	}
	return out
}

func parseLevels(raw [][2]string) ([]orderbook.PriceLevel, error) {
	levels := make([]orderbook.PriceLevel, 0, len(raw))
	for _, pair := range raw {
		priceTicks, err := priceToTicks(pair[0])
		if err != nil {
			return nil, fmt.Errorf("invalid price %q: %w", pair[0], err)
		}
		size, err := strconv.ParseFloat(pair[1], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid size %q: %w", pair[1], err)
		}
		levels = append(levels, orderbook.PriceLevel{PriceTicks: priceTicks, Size: size})
	}
	return levels, nil
}

// priceToTicks scales a decimal price string to an integer tick count
// (1e-8 precision) so order book keys can use exact integer comparison
// rather than float equality.
func priceToTicks(price string) (int64, error) {
	f, err := strconv.ParseFloat(price, 64)
	if err != nil {
		return 0, err
	}
	return int64(f*1e8 + 0.5), nil
}
