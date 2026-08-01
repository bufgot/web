// Package sign (client side) provides HTTP client tools for x-sign request signing
// and response verification.
package sign

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	mwsign "github.com/bufgot/web/middleware/sign"
)

// SignTransport is an http.RoundTripper that injects request headers into outgoing requests.
type SignTransport struct {
	Base        http.RoundTripper
	Config      mwsign.ClientSignConfig
	HeaderNames mwsign.HeaderNames
	NowFunc     func() time.Time
}

// NewSignTransport creates a SignTransport. If base is nil, http.DefaultTransport is used.
func NewSignTransport(cfg mwsign.ClientSignConfig, base http.RoundTripper) *SignTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &SignTransport{
		Base:    base,
		Config:  cfg,
		NowFunc: time.Now,
	}
}

// RoundTrip implements http.RoundTripper.
func (t *SignTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !t.Config.Enabled() {
		return t.Base.RoundTrip(req)
	}

	signer, err := mwsign.NewSigner(mwsign.SignMethod(t.Config.Method), mwsign.SignerOpts{
		Salt:       t.Config.Salt,
		PrivateKey: t.Config.PrivateKey,
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
		AppID:       t.Config.AppID,
		Method:      mwsign.SignMethod(t.Config.Method),
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

func (t *SignTransport) now() time.Time {
	if t.NowFunc != nil {
		return t.NowFunc()
	}
	return time.Now()
}

// VerifyTransport is an http.RoundTripper that validates response headers on HTTP responses.
type VerifyTransport struct {
	Base        http.RoundTripper
	Config      mwsign.ResponseVerifyConfig
	HeaderNames mwsign.HeaderNames
}

// NewVerifyTransport creates a VerifyTransport.
func NewVerifyTransport(cfg mwsign.ResponseVerifyConfig, base http.RoundTripper) *VerifyTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &VerifyTransport{Base: base, Config: cfg}
}

// RoundTrip implements http.RoundTripper.
func (t *VerifyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.Base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if !t.Config.Enable || len(t.Config.Verifications) == 0 {
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

	item := t.Config.LookupApp(app)
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

// readBodyBytes reads the request body and restores it.
func readBodyBytes(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}
