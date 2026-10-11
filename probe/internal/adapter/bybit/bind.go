package bybit

import (
	"context"

	oxrmadapter "github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/adapter"
	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/config"
	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/errtax"
	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/model"
)

// bound resolves the canonical-instrument -> Bybit symbol mapping so the
// concrete Adapter (which naturally works in terms of Bybit's own symbol
// strings) can satisfy the generic adapter.Adapter interface, which works
// in terms of model.CanonicalInstrument only. This indirection is what
// keeps "BTCUSDT vs BTC-USDT-SWAP vs BTC-USD" naming differences out of
// the shared probe engine entirely — see docs/EXCHANGE_ADAPTERS.md.
type bound struct {
	*Adapter
	instruments map[model.CanonicalInstrument]config.Instrument
}

// Bind returns an oxrmadapter.Adapter for the given instrument set.
func Bind(a *Adapter, instruments ...config.Instrument) oxrmadapter.Adapter {
	m := make(map[model.CanonicalInstrument]config.Instrument, len(instruments))
	for _, i := range instruments {
		m[i.Canonical] = i
	}
	return &bound{Adapter: a, instruments: m}
}

func (b *bound) symbolFor(instrument model.CanonicalInstrument) (string, error) {
	i, ok := b.instruments[instrument]
	if !ok {
		return "", errtax.New(errtax.ProbeInternal, "", errUnknownInstrument(instrument))
	}
	return i.SymbolFor(ExchangeID)
}

func (b *bound) RESTProbe(ctx context.Context, instrument model.CanonicalInstrument) (model.RESTObservation, error) {
	symbol, err := b.symbolFor(instrument)
	if err != nil {
		return model.RESTObservation{Exchange: ExchangeID, Instrument: instrument, ErrorCategory: errtax.ProbeInternal}, err
	}
	return b.Adapter.RESTProbe(ctx, instrument, symbol)
}

func (b *bound) ConnectMarketData(ctx context.Context, instrument model.CanonicalInstrument, channels []model.ChannelKind) (oxrmadapter.MarketDataStream, error) {
	symbol, err := b.symbolFor(instrument)
	if err != nil {
		return nil, err
	}
	return b.Adapter.ConnectMarketData(ctx, instrument, symbol, channels)
}

type errUnknownInstrument model.CanonicalInstrument

func (e errUnknownInstrument) Error() string {
	return "bybit: no symbol mapping for canonical instrument " + string(e)
}
