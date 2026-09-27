// Package bybit implements the reference Adapter for Bybit's V5 public
// API. Verified directly against https://bybit-exchange.github.io/docs/v5/
// on 2026-09-27 — see docs/EXCHANGE_ADAPTERS.md "Bybit" for the exact
// endpoints, sequence semantics, and known quirks this implementation
// relies on. Do not add behavior here that isn't traceable to that
// documentation; leave a `// TODO(verify)` instead of guessing.
package bybit

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"strings"
	"time"

	"github.com/vizanix/open-exchange-reliability-monitor/probe/internal/errtax"
	"github.com/vizanix/open-exchange-reliability-monitor/probe/internal/model"
)

const (
	ExchangeID model.ExchangeID = "bybit"

	defaultRESTBaseURL = "https://api.bybit.com"
	defaultWSLinearURL = "wss://stream.bybit.com/v5/public/linear"

	// maxResponseBytes bounds REST response bodies before parsing, per
	// SECURITY.md's decompression-bomb / oversized-payload mitigation.
	maxResponseBytes = 4 << 20 // 4 MiB
)

// Adapter implements adapter.Adapter for Bybit V5.
type Adapter struct {
	httpClient  *http.Client
	restBaseURL string
	wsLinearURL string
	category    string // "linear" — MVP scope is the USDT perpetual; see EXCHANGE_ADAPTERS.md
}

type Option func(*Adapter)

func WithRESTBaseURL(u string) Option      { return func(a *Adapter) { a.restBaseURL = u } }
func WithWSLinearURL(u string) Option      { return func(a *Adapter) { a.wsLinearURL = u } }
func WithHTTPClient(c *http.Client) Option { return func(a *Adapter) { a.httpClient = c } }

func New(opts ...Option) *Adapter {
	a := &Adapter{
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		restBaseURL: defaultRESTBaseURL,
		wsLinearURL: defaultWSLinearURL,
		category:    "linear",
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *Adapter) Name() model.ExchangeID { return ExchangeID }

// ServerTime calls GET /v5/market/time. See docs/EXCHANGE_ADAPTERS.md.
func (a *Adapter) ServerTime(ctx context.Context) (model.ServerTime, error) {
	sentAt := time.Now().UTC()
	var out envelope[serverTimeResult]
	if err := a.getJSON(ctx, "/v5/market/time", nil, &out); err != nil {
		return model.ServerTime{}, err
	}
	recvAt := time.Now().UTC()
	if out.RetCode != 0 {
		cat := classifyRetCode(out.RetCode, out.RetMsg)
		return model.ServerTime{}, errtax.New(cat, strconv.Itoa(out.RetCode), fmt.Errorf("%s", out.RetMsg))
	}
	sec, err := strconv.ParseInt(out.Result.TimeSecond, 10, 64)
	if err != nil {
		return model.ServerTime{}, errtax.New(errtax.DataSchemaMismatch, "", fmt.Errorf("parse timeSecond: %w", err))
	}
	return model.ServerTime{
		ExchangeTime: time.Unix(sec, 0).UTC(),
		ProbeSentAt:  sentAt,
		ProbeRecvAt:  recvAt,
	}, nil
}

// RESTProbe calls GET /v5/market/tickers for the mapped symbol. See
// docs/EXCHANGE_ADAPTERS.md.
func (a *Adapter) RESTProbe(ctx context.Context, instrument model.CanonicalInstrument, symbol string) (model.RESTObservation, error) {
	obs := model.RESTObservation{
		Exchange:   ExchangeID,
		Endpoint:   "/v5/market/tickers",
		Instrument: instrument,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.restBaseURL+"/v5/market/tickers", nil)
	if err != nil {
		obs.ErrorCategory = errtax.ProbeInternal
		return obs, errtax.New(errtax.ProbeInternal, "", err)
	}
	q := req.URL.Query()
	q.Set("category", a.category)
	q.Set("symbol", symbol)
	req.URL.RawQuery = q.Encode()

	obs.RequestStart = time.Now().UTC()
	trace := &httptrace.ClientTrace{
		DNSDone:              func(httptrace.DNSDoneInfo) { obs.DNSDone = time.Now().UTC() },
		ConnectDone:          func(string, string, error) { obs.ConnDone = time.Now().UTC() },
		TLSHandshakeDone:     func(tls.ConnectionState, error) { obs.TLSDone = time.Now().UTC() },
		GotFirstResponseByte: func() { obs.FirstByte = time.Now().UTC() },
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	resp, err := a.httpClient.Do(req)
	if err != nil {
		obs.Done = time.Now().UTC()
		cat := classifyTransportError(err)
		obs.ErrorCategory = cat
		return obs, errtax.New(cat, "", err)
	}
	defer resp.Body.Close()

	if limit := resp.Header.Get("X-Bapi-Limit"); limit != "" {
		if v, err := strconv.Atoi(limit); err == nil {
			obs.RateLimitLimit = &v
		}
	}
	if remaining := resp.Header.Get("X-Bapi-Limit-Status"); remaining != "" {
		if v, err := strconv.Atoi(remaining); err == nil {
			obs.RateLimitRemaining = &v
		}
	}

	obs.HTTPStatus = resp.StatusCode
	if resp.StatusCode == http.StatusTooManyRequests {
		obs.Done = time.Now().UTC()
		obs.ErrorCategory = errtax.HTTP429
		return obs, errtax.New(errtax.HTTP429, strconv.Itoa(resp.StatusCode), fmt.Errorf("http %d", resp.StatusCode))
	}
	if resp.StatusCode >= 500 {
		obs.Done = time.Now().UTC()
		obs.ErrorCategory = errtax.HTTP5xx
		return obs, errtax.New(errtax.HTTP5xx, strconv.Itoa(resp.StatusCode), fmt.Errorf("http %d", resp.StatusCode))
	}
	if resp.StatusCode >= 400 {
		obs.Done = time.Now().UTC()
		obs.ErrorCategory = errtax.HTTP4xx
		return obs, errtax.New(errtax.HTTP4xx, strconv.Itoa(resp.StatusCode), fmt.Errorf("http %d", resp.StatusCode))
	}

	body, err := readBounded(resp.Body, maxResponseBytes)
	if err != nil {
		obs.Done = time.Now().UTC()
		obs.ErrorCategory = errtax.DataInvalidJSON
		return obs, errtax.New(errtax.DataInvalidJSON, "", err)
	}
	obs.Done = time.Now().UTC()

	var out envelope[tickersResult]
	if err := json.Unmarshal(body, &out); err != nil {
		obs.ErrorCategory = errtax.DataInvalidJSON
		return obs, errtax.New(errtax.DataInvalidJSON, "", err)
	}
	if out.RetCode != 0 {
		cat := classifyRetCode(out.RetCode, out.RetMsg)
		obs.ErrorCategory = cat
		obs.ErrorCode = strconv.Itoa(out.RetCode)
		return obs, errtax.New(cat, strconv.Itoa(out.RetCode), fmt.Errorf("%s", out.RetMsg))
	}
	if len(out.Result.List) == 0 {
		obs.ErrorCategory = errtax.DataSchemaMismatch
		return obs, errtax.New(errtax.DataSchemaMismatch, "", fmt.Errorf("empty ticker list for symbol %q", symbol))
	}

	exTime := time.UnixMilli(out.Time).UTC()
	obs.ExchangeTimestamp = &exTime
	obs.Success = true
	return obs, nil
}

func (a *Adapter) getJSON(ctx context.Context, path string, query map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.restBaseURL+path, nil)
	if err != nil {
		return errtax.New(errtax.ProbeInternal, "", err)
	}
	if len(query) > 0 {
		q := req.URL.Query()
		for k, v := range query {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return errtax.New(classifyTransportError(err), "", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return errtax.New(errtax.HTTP5xx, "", fmt.Errorf("http %d", resp.StatusCode))
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return errtax.New(errtax.HTTP429, "", fmt.Errorf("http %d", resp.StatusCode))
	}
	if resp.StatusCode >= 400 {
		return errtax.New(errtax.HTTP4xx, "", fmt.Errorf("http %d", resp.StatusCode))
	}
	body, err := readBounded(resp.Body, maxResponseBytes)
	if err != nil {
		return errtax.New(errtax.DataInvalidJSON, "", err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return errtax.New(errtax.DataInvalidJSON, "", err)
	}
	return nil
}

// classifyRetCode maps Bybit's application-level retCode to the shared
// taxonomy. 10006 is documented as the rate-limit code; anything else
// non-zero is treated as a generic exchange-side application error rather
// than guessed at.
func classifyRetCode(code int, msg string) errtax.Category {
	switch code {
	case 10006:
		return errtax.ExchangeRateLimit
	default:
		_ = msg
		return errtax.ExchangeInternal
	}
}

func classifyTransportError(err error) errtax.Category {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no such host") || strings.Contains(msg, "dns"):
		return errtax.NetworkDNS
	case strings.Contains(msg, "tls") || strings.Contains(msg, "certificate"):
		return errtax.NetworkTLS
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded"):
		return errtax.NetworkTimeout
	default:
		return errtax.NetworkConnect
	}
}
