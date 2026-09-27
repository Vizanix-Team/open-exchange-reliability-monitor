package orderbook

import "testing"

func TestResetAndApplyDelta(t *testing.T) {
	b := NewBook()
	if err := b.Reset([]PriceLevel{{PriceTicks: 100, Size: 1}}, []PriceLevel{{PriceTicks: 200, Size: 1}}); err != nil {
		t.Fatalf("reset: %v", err)
	}
	bids, asks := b.Depth()
	if bids != 1 || asks != 1 {
		t.Fatalf("depth = %d/%d, want 1/1", bids, asks)
	}

	// size 0 deletes a level.
	if err := b.ApplyDelta([]PriceLevel{{PriceTicks: 100, Size: 0}}, nil); err != nil {
		t.Fatalf("apply delta: %v", err)
	}
	bids, _ = b.Depth()
	if bids != 0 {
		t.Fatalf("bids depth after delete = %d, want 0", bids)
	}
}

func TestCrossedBookDetected(t *testing.T) {
	b := NewBook()
	// bid >= ask is a crossed book.
	if err := b.Reset([]PriceLevel{{PriceTicks: 300, Size: 1}}, []PriceLevel{{PriceTicks: 200, Size: 1}}); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if b.CrossedBookCount != 1 {
		t.Fatalf("CrossedBookCount = %d, want 1", b.CrossedBookCount)
	}
}

func TestInvalidQuantityRejected(t *testing.T) {
	b := NewBook()
	err := b.Reset([]PriceLevel{{PriceTicks: 100, Size: -1}}, nil)
	if err != ErrInvalidQuantity {
		t.Fatalf("err = %v, want ErrInvalidQuantity", err)
	}
	if b.InvalidQuantityCount != 1 {
		t.Fatalf("InvalidQuantityCount = %d, want 1", b.InvalidQuantityCount)
	}
}

func TestSnapshotSortedBestFirst(t *testing.T) {
	b := NewBook()
	_ = b.Reset(
		[]PriceLevel{{PriceTicks: 100, Size: 1}, {PriceTicks: 300, Size: 1}, {PriceTicks: 200, Size: 1}},
		[]PriceLevel{{PriceTicks: 500, Size: 1}, {PriceTicks: 400, Size: 1}},
	)
	bids, asks := b.Snapshot()
	if bids[0].PriceTicks != 300 {
		t.Fatalf("best bid = %d, want 300 (highest first)", bids[0].PriceTicks)
	}
	if asks[0].PriceTicks != 400 {
		t.Fatalf("best ask = %d, want 400 (lowest first)", asks[0].PriceTicks)
	}
}
