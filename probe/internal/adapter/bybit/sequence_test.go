package bybit

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/model"
)

func loadOrderbookMsg(t *testing.T, name string) orderbookMessage {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var msg orderbookMessage
	if err := json.Unmarshal(b, &msg); err != nil {
		t.Fatalf("unmarshal %s: %v", name, err)
	}
	return msg
}

func toEvent(msg orderbookMessage) model.MarketDataEvent {
	seq := msg.Data.Seq
	return model.MarketDataEvent{
		Sequence: &seq,
		IsReset:  msg.Data.U == 1 && msg.Type != "snapshot",
	}
}

func TestOrderbookSequence_NoGapOnConsecutive(t *testing.T) {
	snap := toEvent(loadOrderbookMsg(t, "orderbook_snapshot.json"))
	delta := toEvent(loadOrderbookMsg(t, "orderbook_delta_ok.json"))

	status := orderbookSeqValidator.Validate(snap, delta)
	if status.GapDetected || status.Duplicate || status.OutOfOrder {
		t.Fatalf("expected clean sequence transition, got %+v", status)
	}
}

func TestOrderbookSequence_GapDetected(t *testing.T) {
	delta1 := toEvent(loadOrderbookMsg(t, "orderbook_delta_ok.json"))  // seq 100000002
	delta2 := toEvent(loadOrderbookMsg(t, "orderbook_delta_gap.json")) // seq 100000009 - jumps ahead

	status := orderbookSeqValidator.Validate(delta1, delta2)
	if !status.GapDetected {
		t.Fatalf("expected GapDetected=true, got %+v", status)
	}
}

func TestOrderbookSequence_ResetOnUpdateIDOne(t *testing.T) {
	// A message where u==1 but type != snapshot signals a service
	// restart mid-stream per docs/EXCHANGE_ADAPTERS.md; the validator
	// must treat it as an explicit reset, never as a sequence gap.
	prior := toEvent(loadOrderbookMsg(t, "orderbook_delta_ok.json"))
	resetMsg := orderbookMessage{
		wsEnvelope: wsEnvelope{Type: "delta"},
		Data:       orderbookData{U: 1, Seq: 999999999},
	}
	resetEvt := toEvent(resetMsg)
	resetEvt.IsReset = resetMsg.Data.U == 1

	status := orderbookSeqValidator.Validate(prior, resetEvt)
	if !status.Reset {
		t.Fatalf("expected Reset=true for u==1, got %+v", status)
	}
	if status.GapDetected {
		t.Fatal("u==1 reset must not also be reported as a gap")
	}
}

func TestTradeSequence_SharedSeqAcrossBatchIsNotDuplicate(t *testing.T) {
	b, err := os.ReadFile("testdata/trade_batch_shared_seq.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var msg tradeMessage
	if err := json.Unmarshal(b, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(msg.Data) != 2 {
		t.Fatalf("expected 2 trades in batch, got %d", len(msg.Data))
	}
	if msg.Data[0].Seq != msg.Data[1].Seq {
		t.Fatal("fixture should exercise the documented same-seq-across-batch case")
	}

	seq0 := msg.Data[0].Seq
	seq1 := msg.Data[1].Seq
	evt0 := model.MarketDataEvent{Sequence: &seq0}
	evt1 := model.MarketDataEvent{Sequence: &seq1}

	status := tradeSeqValidator.Validate(evt0, evt1)
	if status.Duplicate {
		t.Fatal("equal consecutive seq within a batched trade push must not be flagged as duplicate")
	}
}

func TestOrderbookDelta_MalformedLevelsRejected(t *testing.T) {
	msg := loadOrderbookMsg(t, "orderbook_delta_malformed.json")
	_, err := parseLevels(msg.Data.Bids)
	if err == nil {
		t.Fatal("expected parse error for non-numeric price")
	}
}
