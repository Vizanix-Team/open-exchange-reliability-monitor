package config

import (
	"testing"

	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/model"
)

func TestSymbolForConfiguredVenue(t *testing.T) {
	cases := map[model.ExchangeID]string{
		"bybit":   "BTCUSDT",
		"okx":     "BTC-USDT-SWAP",
		"binance": "BTCUSDT",
	}
	for venue, want := range cases {
		got, err := BTCPerpMVP.SymbolFor(venue)
		if err != nil {
			t.Fatalf("SymbolFor(%q): %v", venue, err)
		}
		if got != want {
			t.Errorf("SymbolFor(%q) = %q, want %q", venue, got, want)
		}
	}
}

// The mapping must never fall back to the canonical name: assuming naming
// parity across exchanges is the bug this type exists to prevent.
func TestSymbolForUnknownVenueIsAnError(t *testing.T) {
	sym, err := BTCPerpMVP.SymbolFor("kraken")
	if err == nil {
		t.Fatalf("expected an error, got symbol %q", sym)
	}
	if sym != "" {
		t.Errorf("symbol must be empty on error, got %q", sym)
	}
}
