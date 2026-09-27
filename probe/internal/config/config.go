// Package config loads probe node identity and instrument configuration.
package config

import (
	"fmt"

	"github.com/vizanix/open-exchange-reliability-monitor/probe/internal/model"
)

// NodeIdentity identifies this probe for every measurement it emits and
// for the signed-batch identity described in SECURITY.md.
type NodeIdentity struct {
	NodeID    string
	Region    string
	Provider  string
	Latitude  float64
	Longitude float64
	Version   string
}

// Instrument is one canonical instrument's per-venue symbol mapping. See
// docs/EXCHANGE_ADAPTERS.md "Canonical instrument mapping".
type Instrument struct {
	Canonical model.CanonicalInstrument
	Venues    map[model.ExchangeID]string
}

// SymbolFor returns the venue-specific symbol for this instrument, or an
// error if the venue isn't configured for it — deliberately explicit
// rather than falling back to the canonical name, since assuming naming
// parity across exchanges is exactly the bug this type exists to prevent.
func (i Instrument) SymbolFor(exchange model.ExchangeID) (string, error) {
	sym, ok := i.Venues[exchange]
	if !ok {
		return "", fmt.Errorf("instrument %q has no configured symbol for venue %q", i.Canonical, exchange)
	}
	return sym, nil
}

// BTCPerpMVP is the default MVP instrument mapping. In production this is
// loaded from YAML (deploy/config/instruments.example.yaml carries the
// same values); it's defined here as a literal default so `go run` works
// with zero config for local evaluation.
var BTCPerpMVP = Instrument{
	Canonical: "BTC-USDT-PERP",
	Venues: map[model.ExchangeID]string{
		"bybit":   "BTCUSDT",
		"okx":     "BTC-USDT-SWAP",
		"binance": "BTCUSDT",
	},
}
