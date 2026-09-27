package bybit

// Wire types for Bybit V5 public REST/WS responses, verified against
// https://bybit-exchange.github.io/docs/v5/ (market/time, market/tickers,
// ws/connect, websocket/public/orderbook, websocket/public/trade) on
// 2026-09-27. See docs/EXCHANGE_ADAPTERS.md "Bybit" for the sequence
// semantics these types support.

type envelope[T any] struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  T      `json:"result"`
	Time    int64  `json:"time"`
}

type serverTimeResult struct {
	TimeSecond string `json:"timeSecond"`
	TimeNano   string `json:"timeNano"`
}

type tickersResult struct {
	Category string        `json:"category"`
	List     []tickerEntry `json:"list"`
}

type tickerEntry struct {
	Symbol    string `json:"symbol"`
	LastPrice string `json:"lastPrice"`
	Bid1Price string `json:"bid1Price"`
	Ask1Price string `json:"ask1Price"`
}

// wsEnvelope wraps both orderbook and trade push messages; fields not
// present for a given topic are simply left zero. Op-level frames (pong,
// subscribe ack) are parsed separately in ws.go via opFrame.
type wsEnvelope struct {
	Topic string `json:"topic"`
	Type  string `json:"type"` // "snapshot" | "delta"
	TS    int64  `json:"ts"`
	CTS   int64  `json:"cts"`
}

type opFrame struct {
	Op      string `json:"op"`
	ReqID   string `json:"req_id"`
	Success *bool  `json:"success"`
	RetMsg  string `json:"ret_msg"`
}

// orderbookData mirrors docs/v5/websocket/public/orderbook: b/a are
// [price, size] string pairs, u is the update id (u==1 signals a service
// restart requiring a snapshot-based reset), seq is the cross-sequence
// used for gap detection between consecutive deltas.
type orderbookData struct {
	Symbol string      `json:"s"`
	Bids   [][2]string `json:"b"`
	Asks   [][2]string `json:"a"`
	U      int64       `json:"u"`
	Seq    int64       `json:"seq"`
}

type orderbookMessage struct {
	wsEnvelope
	Data orderbookData `json:"data"`
}

// tradeEntry mirrors docs/v5/websocket/public/trade. Per the docs, a
// single message may carry up to 1024 trades and multiple messages may
// share the same seq (batching) — see docs/EXCHANGE_ADAPTERS.md.
type tradeEntry struct {
	T    int64  `json:"T"`
	S    string `json:"s"`
	Side string `json:"S"`
	V    string `json:"v"`
	P    string `json:"p"`
	I    string `json:"i"`
	Seq  int64  `json:"seq"`
}

type tradeMessage struct {
	wsEnvelope
	Data []tradeEntry `json:"data"`
}
