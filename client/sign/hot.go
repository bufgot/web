package sign

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	mwsign "github.com/bufgot/web/middleware/sign"
)

// ============================================================================
// Dynamic Client Transports (hot-reload)
// ============================================================================

// DynamicSignTransport is like SignTransport but reads mwsign.ClientSignConfig
// from a mwsign.DynamicClientSignConfig on each RoundTrip, enabling hot-reload.
type DynamicSignTransport struct {
	Base        http.RoundTripper
	DynCfg      *mwsign.DynamicClientSignConfig
	HeaderNames mwsign.HeaderNames
	NowFunc     func() time.Time
}

// NewDynamicSignTransport creates a DynamicSignTransport. If base is nil, http.DefaultTransport is used.
func NewDynamicSignTransport(dynCfg *mwsign.DynamicClientSignConfig, base http.RoundTripper) *DynamicSignTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &DynamicSignTransport{
		Base:    base,
		DynCfg:  dynCfg,
		NowFunc: time.Now,
	}
}

// RoundTrip implements http.RoundTripper.
func (t *DynamicSignTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cfg := t.DynCfg.Load()
	if !cfg.Enabled() {
		return t.Base.RoundTrip(req)
	}

	signer, err := mwsign.NewSigner(mwsign.SignMethod(cfg.Method), mwsign.SignerOpts{
		Salt:       cfg.Salt,
		PrivateKey: cfg.PrivateKey,
	})
	if err != nil {
		return nil, fmt.Errorf("sign: create signer: %w", err)
	}

	body, err := readBodyBytes(req)
	if err != nil {
		return nil, fmt.Errorf("sign: read request body: %w", err)
	}

	ts := t.now()
	h := t.HeaderNames.Effective()
	headers, err := mwsign.SignRequest(mwsign.SignRequestParams{
		AppID:       cfg.AppID,
		Method:      mwsign.SignMethod(cfg.Method),
		Signer:      signer,
		Path:        req.URL.Path,
		Body:        body,
		HeaderNames: t.HeaderNames,
	}, ts)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	req.Header.Set(h.ReqApp, headers.App)
	req.Header.Set(h.ReqTimestamp, headers.Timestamp)
	req.Header.Set(h.ReqNonce, headers.Nonce)
	req.Header.Set(h.ReqSign, headers.Sign)

	return t.Base.RoundTrip(req)
}

func (t *DynamicSignTransport) now() time.Time {
	if t.NowFunc != nil {
		return t.NowFunc()
	}
	return time.Now()
}

// DynamicVerifyTransport is like VerifyTransport but reads mwsign.ResponseVerifyConfig
// from a mwsign.DynamicResponseVerifyConfig on each RoundTrip, enabling hot-reload.
type DynamicVerifyTransport struct {
	Base        http.RoundTripper
	DynCfg      *mwsign.DynamicResponseVerifyConfig
	HeaderNames mwsign.HeaderNames
}

// NewDynamicVerifyTransport creates a DynamicVerifyTransport.
func NewDynamicVerifyTransport(dynCfg *mwsign.DynamicResponseVerifyConfig, base http.RoundTripper) *DynamicVerifyTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &DynamicVerifyTransport{Base: base, DynCfg: dynCfg}
}

// RoundTrip implements http.RoundTripper.
func (t *DynamicVerifyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.Base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	cfg := t.DynCfg.Load()
	if !cfg.Enable || len(cfg.Verifications) == 0 {
		return resp, nil
	}

	h := t.HeaderNames.Effective()

	app := resp.Header.Get(h.ResApp)
	ts := resp.Header.Get(h.ResTimestamp)
	nonce := resp.Header.Get(h.ResNonce)
	sig := resp.Header.Get(h.ResSign)

	if app == "" || ts == "" || nonce == "" || sig == "" {
		resp.Body.Close()
		return nil, fmt.Errorf("sign: response missing signature headers")
	}

	item := cfg.LookupApp(app)
	if item == nil {
		resp.Body.Close()
		return nil, fmt.Errorf("sign: unknown or disabled app %q in response", app)
	}

	verifier, err := mwsign.NewVerifier(mwsign.SignMethod(item.Method), mwsign.SignerOpts{
		Salt:      item.Salt,
		PublicKey: item.PublicKey,
	})
	if err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("sign: create verifier: %w", err)
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("sign: read response body: %w", err)
	}

	if err := mwsign.VerifyResponse(mwsign.VerifyResponseParams{
		AppHeader:       app,
		TimestampHeader: ts,
		NonceHeader:     nonce,
		SignHeader:      sig,
		Verifier:        verifier,
		StatusCode:      resp.StatusCode,
		Body:            body,
		TimeoutMS:       item.Timeout,
		HeaderNames:     t.HeaderNames,
	}); err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	resp.Body = io.NopCloser(bytes.NewReader(body))
	return resp, nil
}
