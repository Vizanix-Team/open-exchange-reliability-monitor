// Command probe runs the Open Exchange Reliability Monitor probe: REST and
// WebSocket loops against a configured exchange/instrument, exporting
// Prometheus metrics locally. See README.md "Quick start".
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/adapter/bybit"
	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/config"
	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/metrics"
	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/model"
	"github.com/Vizanix-Team/open-exchange-reliability-monitor/probe/internal/probeengine"
)

func main() {
	var (
		exchange     = flag.String("exchange", "bybit", "exchange to probe (currently supported: bybit)")
		instrument   = flag.String("instrument", "BTC-USDT-PERP", "canonical instrument id")
		region       = flag.String("region", "unspecified", "probe region label, e.g. frankfurt")
		provider     = flag.String("provider", "unspecified", "hosting provider label, never a specific cloud vendor requirement")
		nodeID       = flag.String("node-id", "", "probe node id; defaults to hostname-region")
		listenAddr   = flag.String("listen", ":9464", "address to serve /metrics on")
		restInterval = flag.Duration("rest-interval", 5*time.Second, "REST probe interval")
	)
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if *nodeID == "" {
		host, _ := os.Hostname()
		*nodeID = host + "-" + *region
	}
	identity := config.NodeIdentity{
		NodeID:   *nodeID,
		Region:   *region,
		Provider: *provider,
		Version:  version(),
	}
	logger.Info("starting probe", "node_id", identity.NodeID, "region", identity.Region, "exchange", *exchange, "instrument", *instrument, "version", identity.Version)

	reg := metrics.Registry()
	reg.MustRegister(prometheus.NewGoCollector())
	metrics.ProbeUp.Set(1)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch *exchange {
	case "bybit":
		runBybit(ctx, logger, model.CanonicalInstrument(*instrument), *region, *restInterval)
	default:
		logger.Error("unsupported exchange; see docs/EXCHANGE_ADAPTERS.md for how to add one", "exchange", *exchange)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	srv := &http.Server{Addr: *listenAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	logger.Info("serving metrics", "addr", *listenAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("metrics server failed", "err", err)
		os.Exit(1)
	}
}

func runBybit(ctx context.Context, logger *slog.Logger, instrument model.CanonicalInstrument, region string, restInterval time.Duration) {
	raw := bybit.New()
	bound := bybit.Bind(raw, config.BTCPerpMVP)

	rl := &probeengine.RESTLoop{
		Adapter:    bound,
		Instrument: instrument,
		Region:     region,
		Interval:   restInterval,
		Logger:     logger,
	}
	wl := &probeengine.WSLoop{
		Adapter:    bound,
		Instrument: instrument,
		Channels:   []model.ChannelKind{model.ChannelOrderBook, model.ChannelTrades},
		Region:     region,
		Logger:     logger,
	}

	go rl.Run(ctx)
	go wl.Run(ctx)
}

// version is overridden at build time via -ldflags "-X main.buildVersion=...".
var buildVersion = "dev"

func version() string { return buildVersion }
