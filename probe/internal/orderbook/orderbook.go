// Package orderbook implements exchange-agnostic local order-book state
// (price -> size per side) and integrity checks. It is deliberately
// oblivious to any exchange's sequence semantics: adapters decide, from
// their own protocol rules, when to call Reset vs ApplyDelta, and this
// package only tracks book state and detects universally-meaningful
// anomalies (crossed book, invalid quantity). See
// docs/EXCHANGE_ADAPTERS.md for why per-venue sequence rules are not
// implemented here.
package orderbook

import (
	"errors"
	"sort"
)

var ErrInvalidQuantity = errors.New("orderbook: negative or non-numeric quantity")

// PriceLevel is one side's price/size pair, using integer-scaled ticks to
// avoid floating point comparison issues; adapters convert from the
// exchange's decimal strings.
type PriceLevel struct {
	PriceTicks int64
	Size       float64
}

// Book is a local reconstruction of one instrument's order book.
type Book struct {
	bids map[int64]float64
	asks map[int64]float64

	CrossedBookCount     int
	InvalidQuantityCount int
}

func NewBook() *Book {
	return &Book{bids: map[int64]float64{}, asks: map[int64]float64{}}
}

// Reset replaces the book wholesale from a snapshot.
func (b *Book) Reset(bids, asks []PriceLevel) error {
	b.bids = make(map[int64]float64, len(bids))
	b.asks = make(map[int64]float64, len(asks))
	for _, l := range bids {
		if l.Size < 0 {
			b.InvalidQuantityCount++
			return ErrInvalidQuantity
		}
		b.bids[l.PriceTicks] = l.Size
	}
	for _, l := range asks {
		if l.Size < 0 {
			b.InvalidQuantityCount++
			return ErrInvalidQuantity
		}
		b.asks[l.PriceTicks] = l.Size
	}
	b.checkCrossed()
	return nil
}

// ApplyDelta applies one side's incremental updates. Per common
// exchange convention (documented per-adapter), size == 0 deletes the
// level; any other size inserts or overwrites it.
func (b *Book) ApplyDelta(bids, asks []PriceLevel) error {
	var firstErr error
	apply := func(side map[int64]float64, levels []PriceLevel) {
		for _, l := range levels {
			if l.Size < 0 {
				b.InvalidQuantityCount++
				if firstErr == nil {
					firstErr = ErrInvalidQuantity
				}
				continue
			}
			if l.Size == 0 {
				delete(side, l.PriceTicks)
				continue
			}
			side[l.PriceTicks] = l.Size
		}
	}
	apply(b.bids, bids)
	apply(b.asks, asks)
	b.checkCrossed()
	return firstErr
}

func (b *Book) checkCrossed() {
	bestBid, hasBid := b.BestBid()
	bestAsk, hasAsk := b.BestAsk()
	if hasBid && hasAsk && bestBid >= bestAsk {
		b.CrossedBookCount++
	}
}

// BestBid returns the highest bid price tick and whether one exists.
func (b *Book) BestBid() (int64, bool) {
	if len(b.bids) == 0 {
		return 0, false
	}
	best := int64(-1 << 62)
	for p := range b.bids {
		if p > best {
			best = p
		}
	}
	return best, true
}

// BestAsk returns the lowest ask price tick and whether one exists.
func (b *Book) BestAsk() (int64, bool) {
	if len(b.asks) == 0 {
		return 0, false
	}
	best := int64(1 << 62)
	for p := range b.asks {
		if p < best {
			best = p
		}
	}
	return best, true
}

// Depth returns the current number of non-zero levels per side, mainly
// for tests and debug introspection.
func (b *Book) Depth() (bids, asks int) { return len(b.bids), len(b.asks) }

// Snapshot returns sorted bid/ask levels (best-first), used for
// reconciliation against a fresh REST snapshot (docs/METRICS.md
// orderbook_reconciliation_mismatch_total).
func (b *Book) Snapshot() (bids, asks []PriceLevel) {
	for p, s := range b.bids {
		bids = append(bids, PriceLevel{PriceTicks: p, Size: s})
	}
	for p, s := range b.asks {
		asks = append(asks, PriceLevel{PriceTicks: p, Size: s})
	}
	sort.Slice(bids, func(i, j int) bool { return bids[i].PriceTicks > bids[j].PriceTicks })
	sort.Slice(asks, func(i, j int) bool { return asks[i].PriceTicks < asks[j].PriceTicks })
	return bids, asks
}
