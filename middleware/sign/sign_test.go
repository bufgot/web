package sign

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	web "github.com/bufgot/web"
	"github.com/valyala/fasthttp"
)

// shorthand for tests
var defaultReq = DefaultHeaderNames

// ============================================================================
// Signer Tests
// ============================================================================

func TestSigner_MD5_EndToEnd(t *testing.T) {
	signer, _ := NewSigner(MethodMD5, SignerOpts{Salt: "salt123"})
	payload := []byte("hello world")

	sig, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if !signer.Verify(payload, sig) {
		t.Error("md5 verify failed")
	}
	if signer.Verify([]byte("tampered"), sig) {
		t.Error("md5 verify should fail for tampered payload")
	}
}

func TestSigner_HMACSHA256_EndToEnd(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "secret-key"})
	payload := []byte("data to sign")

	sig, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if len(sig) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(sig))
	}
	if !signer.Verify(payload, sig) {
		t.Error("hmac verify failed")
	}
	if signer.Verify([]byte("tampered"), sig) {
		t.Error("hmac verify should fail for tampered payload")
	}
}

func TestSigner_Ed25519_EndToEnd(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	privateKeyHex := hex.EncodeToString(priv.Seed())
	publicKeyHex := hex.EncodeToString(pub)

	signer, err := NewSigner(MethodEd25519, SignerOpts{PrivateKey: privateKeyHex})
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}

	verifier, err := NewVerifier(MethodEd25519, SignerOpts{PublicKey: publicKeyHex})
	if err != nil {
		t.Fatalf("create verifier: %v", err)
	}

	payload := []byte("ed25519 test payload")
	sig, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if !verifier.Verify(payload, sig) {
		t.Error("ed25519 verify failed")
	}
	if verifier.Verify([]byte("tampered"), sig) {
		t.Error("ed25519 verify should fail for tampered payload")
	}
}

func TestSigner_Ed25519_Base64Keys(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	privateKeyB64 := base64.StdEncoding.EncodeToString(priv.Seed())
	publicKeyB64 := base64.StdEncoding.EncodeToString(pub)

	signer, err := NewSigner(MethodEd25519, SignerOpts{PrivateKey: privateKeyB64})
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}
	verifier, err := NewVerifier(MethodEd25519, SignerOpts{PublicKey: publicKeyB64})
	if err != nil {
		t.Fatalf("create verifier: %v", err)
	}

	payload := []byte("base64 key test")
	sig, _ := signer.Sign(payload)
	if !verifier.Verify(payload, sig) {
		t.Error("ed25519 base64 key verify failed")
	}
}

func TestSigner_UnknownMethod(t *testing.T) {
	_, err := NewSigner("unknown", SignerOpts{})
	if err == nil {
		t.Error("expected error for unknown method")
	}
}

func TestSigner_Method(t *testing.T) {
	s, _ := NewSigner(MethodMD5, SignerOpts{})
	if s.Method() != MethodMD5 {
		t.Errorf("expected md5, got %s", s.Method())
	}
}

// ============================================================================
// SignRequest / VerifyRequest Tests (4-header format)
// ============================================================================

func TestSignRequest_HMAC(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "shared-secret"})
	ts := time.UnixMilli(1700000000000)

	headers, err := SignRequest(SignRequestParams{
		AppID:  "my-app",
		Method: MethodHMACSHA256,
		Signer: signer,
		Path:   "/api/v1/users",
		Body:   []byte(`{"name":"test"}`),
	}, ts)

	if err != nil {
		t.Fatalf("SignRequest: %v", err)
	}
	if headers.App != "my-app" {
		t.Errorf("app: got %q, want %q", headers.App, "my-app")
	}
	if headers.Timestamp != "1700000000000" {
		t.Errorf("timestamp: got %q", headers.Timestamp)
	}
	if headers.Nonce == "" {
		t.Error("nonce should not be empty")
	}
	if headers.Sign == "" {
		t.Error("sign should not be empty")
	}
}

func TestVerifyRequest_HMAC_Valid(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "shared"})
	ts := time.Now()
	body := []byte(`{"action":"test"}`)

	headers, _ := SignRequest(SignRequestParams{
		AppID: "app1", Method: MethodHMACSHA256,
		Signer: signer, Path: "/api/test", Body: body,
	}, ts)

	verifier, _ := NewVerifier(MethodHMACSHA256, SignerOpts{Salt: "shared"})
	err := VerifyRequest(VerifyRequestParams{
		AppHeader: headers.App, TimestampHeader: headers.Timestamp,
		NonceHeader: headers.Nonce, SignHeader: headers.Sign,
		Verifier: verifier, Path: "/api/test", Body: body,
		TimeoutMS: 300000,
	})
	if err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestVerifyRequest_TamperedBody(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "s"})
	ts := time.Now()
	body := []byte(`original`)

	headers, _ := SignRequest(SignRequestParams{
		AppID: "a", Method: MethodHMACSHA256,
		Signer: signer, Path: "/p", Body: body,
	}, ts)

	verifier, _ := NewVerifier(MethodHMACSHA256, SignerOpts{Salt: "s"})
	err := VerifyRequest(VerifyRequestParams{
		AppHeader: headers.App, TimestampHeader: headers.Timestamp,
		NonceHeader: headers.Nonce, SignHeader: headers.Sign,
		Verifier: verifier, Path: "/p", Body: []byte("tampered"),
		TimeoutMS: 300000,
	})
	if err == nil {
		t.Error("expected error for tampered body")
	}
}

func TestVerifyRequest_ExpiredTimestamp(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "s"})
	ts := time.Now().Add(-10 * time.Minute)
	body := []byte(`{}`)

	headers, _ := SignRequest(SignRequestParams{
		AppID: "a", Method: MethodHMACSHA256,
		Signer: signer, Path: "/p", Body: body,
	}, ts)

	verifier, _ := NewVerifier(MethodHMACSHA256, SignerOpts{Salt: "s"})
	err := VerifyRequest(VerifyRequestParams{
		AppHeader: headers.App, TimestampHeader: headers.Timestamp,
		NonceHeader: headers.Nonce, SignHeader: headers.Sign,
		Verifier: verifier, Path: "/p", Body: body,
		TimeoutMS: 5000,
	})
	if err == nil {
		t.Error("expected error for expired timestamp")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("error should mention expired: %v", err)
	}
}

func TestVerifyRequest_WrongPath(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "s"})
	ts := time.Now()
	body := []byte(`data`)

	headers, _ := SignRequest(SignRequestParams{
		AppID: "a", Method: MethodHMACSHA256,
		Signer: signer, Path: "/correct", Body: body,
	}, ts)

	verifier, _ := NewVerifier(MethodHMACSHA256, SignerOpts{Salt: "s"})
	err := VerifyRequest(VerifyRequestParams{
		AppHeader: headers.App, TimestampHeader: headers.Timestamp,
		NonceHeader: headers.Nonce, SignHeader: headers.Sign,
		Verifier: verifier, Path: "/wrong", Body: body,
		TimeoutMS: 300000,
	})
	if err == nil {
		t.Error("expected error for wrong path")
	}
}

func TestVerifyRequest_MissingHeaders(t *testing.T) {
	verifier, _ := NewVerifier(MethodHMACSHA256, SignerOpts{Salt: "s"})
	err := VerifyRequest(VerifyRequestParams{
		Verifier: verifier, Path: "/api", Body: nil, TimeoutMS: 5000,
	})
	if err == nil {
		t.Error("expected error for missing headers")
	}
}

// ============================================================================
// SignResponse / VerifyResponse Tests
// ============================================================================

func TestSignResponse_HMAC(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "resp-secret"})
	ts := time.UnixMilli(1700000000000)

	headers, err := SignResponse(SignResponseParams{
		AppID: "server-app", Method: MethodHMACSHA256,
		Signer: signer, StatusCode: 200,
		Body: []byte(`{"ok":true}`),
	}, ts)

	if err != nil {
		t.Fatalf("SignResponse: %v", err)
	}
	if headers.App != "server-app" {
		t.Errorf("app: got %q", headers.App)
	}
	if headers.Timestamp != "1700000000000" {
		t.Errorf("timestamp: got %q", headers.Timestamp)
	}
}

func TestVerifyResponse_Valid(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "rs"})
	ts := time.Now()
	body := []byte(`{"data":"ok"}`)

	headers, _ := SignResponse(SignResponseParams{
		AppID: "srv", Method: MethodHMACSHA256,
		Signer: signer, StatusCode: 200, Body: body,
	}, ts)

	verifier, _ := NewVerifier(MethodHMACSHA256, SignerOpts{Salt: "rs"})
	err := VerifyResponse(VerifyResponseParams{
		AppHeader: headers.App, TimestampHeader: headers.Timestamp,
		NonceHeader: headers.Nonce, SignHeader: headers.Sign,
		Verifier: verifier, StatusCode: 200, Body: body,
		TimeoutMS: 300000,
	})
	if err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestVerifyResponse_StatusCodeMismatch(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "rs"})
	ts := time.Now()
	body := []byte(`data`)

	headers, _ := SignResponse(SignResponseParams{
		AppID: "srv", Method: MethodHMACSHA256,
		Signer: signer, StatusCode: 200, Body: body,
	}, ts)

	verifier, _ := NewVerifier(MethodHMACSHA256, SignerOpts{Salt: "rs"})
	err := VerifyResponse(VerifyResponseParams{
		AppHeader: headers.App, TimestampHeader: headers.Timestamp,
		NonceHeader: headers.Nonce, SignHeader: headers.Sign,
		Verifier: verifier, StatusCode: 500, Body: body,
		TimeoutMS: 300000,
	})
	if err == nil {
		t.Error("expected error for status code mismatch")
	}
}

// ============================================================================
// Ed25519 request/response integration
// ============================================================================

func TestSignRequest_Ed25519(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	privateKeyHex := hex.EncodeToString(priv.Seed())
	publicKeyHex := hex.EncodeToString(pub)

	signer, _ := NewSigner(MethodEd25519, SignerOpts{PrivateKey: privateKeyHex})
	ts := time.Now()
	body := []byte(`ed25519-body`)

	headers, err := SignRequest(SignRequestParams{
		AppID: "ed-app", Method: MethodEd25519,
		Signer: signer, Path: "/ed/api", Body: body,
	}, ts)
	if err != nil {
		t.Fatalf("SignRequest ed25519: %v", err)
	}

	verifier, _ := NewVerifier(MethodEd25519, SignerOpts{PublicKey: publicKeyHex})
	err = VerifyRequest(VerifyRequestParams{
		AppHeader: headers.App, TimestampHeader: headers.Timestamp,
		NonceHeader: headers.Nonce, SignHeader: headers.Sign,
		Verifier: verifier, Path: "/ed/api", Body: body,
		TimeoutMS: 300000,
	})
	if err != nil {
		t.Errorf("ed25519 verify: %v", err)
	}
}

func TestSignRequest_MD5(t *testing.T) {
	signer, _ := NewSigner(MethodMD5, SignerOpts{Salt: "md5-salt"})
	ts := time.Now()
	body := []byte(`md5-body`)

	headers, err := SignRequest(SignRequestParams{
		AppID: "md5-app", Method: MethodMD5,
		Signer: signer, Path: "/md5/api", Body: body,
	}, ts)
	if err != nil {
		t.Fatalf("SignRequest md5: %v", err)
	}

	verifier, _ := NewVerifier(MethodMD5, SignerOpts{Salt: "md5-salt"})
	err = VerifyRequest(VerifyRequestParams{
		AppHeader: headers.App, TimestampHeader: headers.Timestamp,
		NonceHeader: headers.Nonce, SignHeader: headers.Sign,
		Verifier: verifier, Path: "/md5/api", Body: body,
		TimeoutMS: 300000,
	})
	if err != nil {
		t.Errorf("md5 verify: %v", err)
	}
}

// ============================================================================
// determinism / idempotent tests
// ============================================================================

func TestSignRequest_IdempotentPayload(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "det"})
	payload := []byte("app1234567890000abc123/api" + base64.StdEncoding.EncodeToString([]byte("consistent")))
	sig1, _ := signer.Sign(payload)
	sig2, _ := signer.Sign(payload)

	if string(sig1) != string(sig2) {
		t.Error("Same payload should produce identical signature")
	}
}

// ============================================================================
// Config Tests
// ============================================================================

func TestClientSignConfig_Enabled(t *testing.T) {
	if (ClientSignConfig{}).Enabled() {
		t.Error("empty config should be disabled")
	}
	if !(ClientSignConfig{Method: "hmac-sha256"}).Enabled() {
		t.Error("config with method should be enabled")
	}
}

func TestServerSignConfig_Enabled(t *testing.T) {
	if (ServerSignConfig{}).Enabled() {
		t.Error("empty config should be disabled")
	}
	if !(ServerSignConfig{Method: "md5"}).Enabled() {
		t.Error("config with method should be enabled")
	}
}

func TestRequestVerifyConfig_LookupApp(t *testing.T) {
	cfg := RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s1", Timeout: 5000},
			{App: "app2", Enable: false, Method: "hmac-sha256", Salt: "s2"},
			{App: "app3", Enable: true, Method: "md5", Salt: "s3"},
		},
	}

	if v := cfg.LookupApp("app1"); v == nil || v.App != "app1" {
		t.Error("should find app1")
	}
	if v := cfg.LookupApp("app2"); v != nil {
		t.Error("should not find disabled app2")
	}
	if v := cfg.LookupApp("nonexistent"); v != nil {
		t.Error("should not find nonexistent app")
	}
}

func TestResponseVerifyConfig_LookupApp(t *testing.T) {
	cfg := ResponseVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "srv1", Enable: true, Method: "ed25519", PublicKey: "abc"},
		},
	}
	if v := cfg.LookupApp("srv1"); v == nil {
		t.Error("should find srv1")
	}
}

// ============================================================================
// shouldSkip tests
// ============================================================================

func TestShouldSkip_ExactMatch(t *testing.T) {
	skips := []string{"/health", "/api/public/ping", "/metrics"}
	if !shouldSkip("/health", skips) {
		t.Error("exact match should be skipped")
	}
	if !shouldSkip("/api/public/ping", skips) {
		t.Error("exact match should be skipped")
	}
	if shouldSkip("/healthz", skips) {
		t.Error("partial match should not be skipped")
	}
}

func TestShouldSkip_PrefixMatch(t *testing.T) {
	skips := []string{"/api/public/", "/static/"}
	if !shouldSkip("/api/public/users", skips) {
		t.Error("prefix match should be skipped")
	}
	if !shouldSkip("/api/public/health", skips) {
		t.Error("prefix match should be skipped")
	}
	if !shouldSkip("/api/public/", skips) {
		t.Error("exact prefix path should be skipped")
	}
	if shouldSkip("/api/public", skips) {
		t.Error("should not match prefix without trailing slash on request path")
	}
	if shouldSkip("/api", skips) {
		t.Error("should not match unrelated path")
	}
}

func TestShouldSkip_EmptySkipPaths(t *testing.T) {
	if shouldSkip("/anything", nil) {
		t.Error("should not skip when skipPaths is nil")
	}
	if shouldSkip("/anything", []string{}) {
		t.Error("should not skip when skipPaths is empty")
	}
}

// ============================================================================
// HeaderNames custom tests
// ============================================================================

func TestSignRequest_CustomHeaderNames(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "custom"})
	ts := time.UnixMilli(1700000000000)
	customNames := HeaderNames{
		ReqApp:       "x-custom-app",
		ReqTimestamp: "x-custom-ts",
		ReqNonce:     "x-custom-nonce",
		ReqSign:      "x-custom-sign",
	}

	headers, err := SignRequest(SignRequestParams{
		AppID:       "my-app",
		Method:      MethodHMACSHA256,
		Signer:      signer,
		Path:        "/api",
		Body:        []byte(`data`),
		HeaderNames: customNames,
	}, ts)

	if err != nil {
		t.Fatalf("SignRequest: %v", err)
	}
	if headers.App != "my-app" {
		t.Errorf("app: got %q", headers.App)
	}
}

func TestVerifyRequest_CustomHeaderNames(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "custom"})
	ts := time.Now()
	body := []byte(`test-body`)
	customNames := HeaderNames{
		ReqApp:       "x-custom-app",
		ReqTimestamp: "x-custom-ts",
		ReqNonce:     "x-custom-nonce",
		ReqSign:      "x-custom-sign",
	}

	headers, _ := SignRequest(SignRequestParams{
		AppID: "app1", Method: MethodHMACSHA256,
		Signer: signer, Path: "/api", Body: body,
	}, ts)

	verifier, _ := NewVerifier(MethodHMACSHA256, SignerOpts{Salt: "custom"})
	// Verify defaults work (default header names)
	err := VerifyRequest(VerifyRequestParams{
		AppHeader: headers.App, TimestampHeader: headers.Timestamp,
		NonceHeader: headers.Nonce, SignHeader: headers.Sign,
		Verifier: verifier, Path: "/api", Body: body, TimeoutMS: 300000,
		HeaderNames: customNames,
	})
	if err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestHeaderNames_Effective(t *testing.T) {
	// Zero-value should fall back to defaults.
	var h HeaderNames
	e := h.Effective()
	if e.ReqApp != DefaultHeaderNames.ReqApp {
		t.Errorf("expected %q, got %q", DefaultHeaderNames.ReqApp, e.ReqApp)
	}

	// Partially filled.
	h2 := HeaderNames{ReqApp: "X-My-App"}
	e2 := h2.Effective()
	if e2.ReqApp != "X-My-App" {
		t.Errorf("expected X-My-App, got %q", e2.ReqApp)
	}
	if e2.ReqTimestamp != DefaultHeaderNames.ReqTimestamp {
		t.Errorf("expected default timestamp, got %q", e2.ReqTimestamp)
	}
}

// ============================================================================
// Nonce generation
// ============================================================================

func TestGenerateNonce(t *testing.T) {
	n1, err := generateNonce(16)
	if err != nil {
		t.Fatalf("generateNonce: %v", err)
	}
	if len(n1) != 16 {
		t.Errorf("expected 16 chars, got %d", len(n1))
	}

	n2, _ := generateNonce(16)
	if n1 == n2 {
		t.Error("two nonces should be different")
	}
}

// ============================================================================
// Fasthttp Middleware Tests
// ============================================================================

func TestVerifyRequestFasthttp_Disabled(t *testing.T) {
	cfg := RequestVerifyConfig{Enable: false}
	mw := VerifyRequestFasthttp(cfg, HeaderNames{})
	h := mw(func(ctx *fasthttp.RequestCtx) { ctx.WriteString("ok") })
	if h == nil {
		t.Error("handler should not be nil")
	}
}

func TestVerifyRequestFasthttp_Enabled_MissingHeaders(t *testing.T) {
	cfg := RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 5000},
		},
	}

	var called bool
	mw := VerifyRequestFasthttp(cfg, defaultReq)
	h := mw(func(ctx *fasthttp.RequestCtx) { called = true })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/api")
	h(ctx)

	if called {
		t.Error("handler should not be called without headers")
	}
	if ctx.Response.StatusCode() != 401 {
		t.Errorf("expected 401, got %d", ctx.Response.StatusCode())
	}
}

func TestVerifyRequestFasthttp_Enabled_Valid(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "s"})
	ts := time.Now()
	body := []byte(`test`)
	headers, _ := SignRequest(SignRequestParams{
		AppID: "app1", Method: MethodHMACSHA256,
		Signer: signer, Path: "/api", Body: body,
	}, ts)

	cfg := RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 300000},
		},
	}

	var called bool
	mw := VerifyRequestFasthttp(cfg, defaultReq)
	h := mw(func(ctx *fasthttp.RequestCtx) { called = true })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/api")
	ctx.Request.Header.Set(defaultReq.ReqApp, headers.App)
	ctx.Request.Header.Set(defaultReq.ReqTimestamp, headers.Timestamp)
	ctx.Request.Header.Set(defaultReq.ReqNonce, headers.Nonce)
	ctx.Request.Header.Set(defaultReq.ReqSign, headers.Sign)
	ctx.Request.SetBody(body)
	h(ctx)

	if !called {
		t.Error("handler should be called with valid signature")
	}
}

func TestVerifyRequestFasthttp_SkipPaths(t *testing.T) {
	cfg := RequestVerifyConfig{
		Enable:    true,
		SkipPaths: []string{"/health", "/api/public/"},
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 5000},
		},
	}

	var called bool
	mw := VerifyRequestFasthttp(cfg, defaultReq)
	h := mw(func(ctx *fasthttp.RequestCtx) { called = true })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/health")
	h(ctx)

	if !called {
		t.Error("handler should be called for skipped path /health")
	}

	called = false
	ctx2 := &fasthttp.RequestCtx{}
	ctx2.Request.Header.SetMethod("GET")
	ctx2.Request.SetRequestURI("/api/public/data")
	mw(func(ctx *fasthttp.RequestCtx) { called = true })(ctx2)
	if !called {
		t.Error("handler should be called for skipped prefix path /api/public/")
	}
}

func TestSignResponseFasthttp_Enabled(t *testing.T) {
	cfg := ServerSignConfig{
		Method: "hmac-sha256", Salt: "rs", AppID: "srv",
	}
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "rs"})

	mw := SignResponseFasthttp(cfg, defaultReq)
	h := mw(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(200)
		ctx.WriteString(`{"ok":true}`)
	})
	_ = signer

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/api")
	h(ctx)

	if string(ctx.Response.Header.Peek(defaultReq.ResApp)) != "srv" {
		t.Error("expected x-res-app header")
	}
	if string(ctx.Response.Header.Peek(defaultReq.ResSign)) == "" {
		t.Error("expected x-res-sign header")
	}
}

func TestSignResponseFasthttp_Disabled(t *testing.T) {
	cfg := ServerSignConfig{}
	mw := SignResponseFasthttp(cfg, defaultReq)
	h := mw(func(ctx *fasthttp.RequestCtx) { ctx.WriteString("ok") })

	ctx := &fasthttp.RequestCtx{}
	h(ctx)

	if string(ctx.Response.Header.Peek(defaultReq.ResApp)) != "" {
		t.Error("should not set headers when disabled")
	}
}

func TestSignResponseFasthttp_SkipPaths(t *testing.T) {
	cfg := ServerSignConfig{
		Method: "hmac-sha256", Salt: "rs", AppID: "srv",
		SkipPaths: []string{"/health"},
	}
	mw := SignResponseFasthttp(cfg, defaultReq)
	h := mw(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(200)
		ctx.WriteString(`{"ok":true}`)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/health")
	h(ctx)

	if string(ctx.Response.Header.Peek(defaultReq.ResApp)) != "" {
		t.Error("should not set response headers for skipped path")
	}
}

// ============================================================================
// Fiber Middleware Tests
// ============================================================================

func TestVerifyRequestFiber_Disabled(t *testing.T) {
	h := VerifyRequestFiber(RequestVerifyConfig{Enable: false}, defaultReq)
	if h == nil {
		t.Error("handler should not be nil")
	}
}

func TestSignResponseFiber_Enabled(t *testing.T) {
	h := SignResponseFiber(ServerSignConfig{
		Method: "hmac-sha256", Salt: "fiber-s", AppID: "fiber-app",
	}, defaultReq)
	if h == nil {
		t.Error("handler should not be nil")
	}
}

// ============================================================================
// Hertz Middleware Tests
// ============================================================================

func TestVerifyRequestHertz_Disabled(t *testing.T) {
	h := VerifyRequestHertz(RequestVerifyConfig{Enable: false}, defaultReq)
	if h == nil {
		t.Error("handler should not be nil")
	}
}

func TestSignResponseHertz_Enabled(t *testing.T) {
	h := SignResponseHertz(ServerSignConfig{
		Method: "hmac-sha256", Salt: "hertz-s", AppID: "hertz-app",
	}, defaultReq)
	if h == nil {
		t.Error("handler should not be nil")
	}
}

// ============================================================================
// Net/HTTP Middleware Tests (chi / echo / gin -- via httptest)
// ============================================================================

func TestVerifyRequestMiddleware_Enabled_MissingHeaders(t *testing.T) {
	cfg := RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 5000},
		},
	}

	req := httptest.NewRequest("POST", "/api/data", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()

	mw := VerifyRequestMiddleware(cfg, defaultReq)
	wrapped := func(w http.ResponseWriter, r *http.Request) {
		ctx := &simpleContext{req: r, w: w}
		mw(func(ctx web.Context) error {
			t.Error("handler should not be called")
			return nil
		})(ctx)
	}

	wrapped(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "missing") {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
}

func TestVerifyRequestMiddleware_Enabled_Valid(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "s"})
	ts := time.Now()
	body := []byte(`{"hello":"world"}`)
	headers, _ := SignRequest(SignRequestParams{
		AppID: "app1", Method: MethodHMACSHA256,
		Signer: signer, Path: "/api/test", Body: body,
	}, ts)

	cfg := RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 300000},
		},
	}

	req := httptest.NewRequest("POST", "/api/test", bytes.NewReader(body))
	req.Header.Set(defaultReq.ReqApp, headers.App)
	req.Header.Set(defaultReq.ReqTimestamp, headers.Timestamp)
	req.Header.Set(defaultReq.ReqNonce, headers.Nonce)
	req.Header.Set(defaultReq.ReqSign, headers.Sign)
	rec := httptest.NewRecorder()

	var called bool
	mw := VerifyRequestMiddleware(cfg, defaultReq)
	wrapped := func(w http.ResponseWriter, r *http.Request) {
		ctx := &simpleContext{req: r, w: w}
		mw(func(ctx web.Context) error {
			called = true
			w.WriteHeader(200)
			return nil
		})(ctx)
	}

	wrapped(rec, req)

	if !called {
		t.Error("handler should be called with valid signature")
	}
}

func TestVerifyRequestMiddleware_Disabled(t *testing.T) {
	cfg := RequestVerifyConfig{Enable: false}
	mw := VerifyRequestMiddleware(cfg, defaultReq)

	var called bool
	h := mw(func(ctx web.Context) error { called = true; return nil })

	req := httptest.NewRequest("GET", "/api", nil)
	rec := httptest.NewRecorder()
	ctx := &simpleContext{req: req, w: rec}
	h(ctx)

	if !called {
		t.Error("handler should be called when middleware is disabled")
	}
}

func TestVerifyRequestMiddleware_SkipPaths(t *testing.T) {
	cfg := RequestVerifyConfig{
		Enable:    true,
		SkipPaths: []string{"/health", "/api/public/"},
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 5000},
		},
	}

	var called bool
	mw := VerifyRequestMiddleware(cfg, defaultReq)
	h := mw(func(ctx web.Context) error { called = true; return nil })

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	ctx := &simpleContext{req: req, w: rec}
	h(ctx)

	if !called {
		t.Error("handler should be called for skipped exact path")
	}

	called = false
	req2 := httptest.NewRequest("GET", "/api/public/data", nil)
	rec2 := httptest.NewRecorder()
	ctx2 := &simpleContext{req: req2, w: rec2}
	h(ctx2)

	if !called {
		t.Error("handler should be called for skipped prefix path")
	}
}

func TestSignResponseMiddleware_Enabled(t *testing.T) {
	cfg := ServerSignConfig{
		Method: "hmac-sha256", Salt: "rs", AppID: "srv",
	}
	mw := SignResponseMiddleware(cfg, defaultReq)
	h := mw(func(ctx web.Context) error {
		ctx.Status(200)
		return ctx.JSON(200, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest("GET", "/api", nil)
	rec := httptest.NewRecorder()
	ctx := &simpleContext{req: req, w: rec}
	h(ctx)

	if rec.Header().Get(defaultReq.ResApp) == "" {
		t.Error("expected x-res-app header")
	}
	if rec.Header().Get(defaultReq.ResSign) == "" {
		t.Error("expected x-res-sign header")
	}
}

func TestSignResponseMiddleware_Disabled(t *testing.T) {
	cfg := ServerSignConfig{}
	mw := SignResponseMiddleware(cfg, defaultReq)
	h := mw(func(ctx web.Context) error {
		ctx.Status(200)
		return ctx.JSON(200, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest("GET", "/api", nil)
	rec := httptest.NewRecorder()
	ctx := &simpleContext{req: req, w: rec}
	h(ctx)

	if rec.Header().Get(defaultReq.ResApp) != "" {
		t.Error("should not set headers when disabled")
	}
}

func TestSignResponseMiddleware_SkipPaths(t *testing.T) {
	cfg := ServerSignConfig{
		Method:    "hmac-sha256",
		Salt:      "rs",
		AppID:     "srv",
		SkipPaths: []string{"/health"},
	}
	mw := SignResponseMiddleware(cfg, defaultReq)
	h := mw(func(ctx web.Context) error {
		ctx.Status(200)
		return ctx.JSON(200, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	ctx := &simpleContext{req: req, w: rec}
	h(ctx)

	if rec.Header().Get(defaultReq.ResApp) != "" {
		t.Error("should not set headers for skipped path")
	}
}

func TestSignResponseMiddleware_CapturesBody(t *testing.T) {
	cfg := ServerSignConfig{
		Method: "hmac-sha256", Salt: "rs", AppID: "srv",
	}
	// Create a verifier to check the signature matches the body
	verifier, _ := NewVerifier(MethodHMACSHA256, SignerOpts{Salt: "rs"})

	mw := SignResponseMiddleware(cfg, defaultReq)
	handlerBody := []byte(`{"response":"data"}`)
	h := mw(func(ctx web.Context) error {
		// Write a body the same way gin does -- via ResponseWriter
		ctx.Status(200)
		w := ctx.ResponseWriter()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(handlerBody)
		return nil
	})

	req := httptest.NewRequest("GET", "/api", nil)
	rec := httptest.NewRecorder()
	ctx := &capturingContext{req: req, w: rec}
	h(ctx)

	sig := rec.Header().Get(defaultReq.ResSign)
	if sig == "" {
		t.Fatal("expected x-res-sign header")
	}

	// Verify the signature matches the body
	app := rec.Header().Get(defaultReq.ResApp)
	ts := rec.Header().Get(defaultReq.ResTimestamp)
	nonce := rec.Header().Get(defaultReq.ResNonce)

	err := VerifyResponse(VerifyResponseParams{
		AppHeader:       app,
		TimestampHeader: ts,
		NonceHeader:     nonce,
		SignHeader:      sig,
		Verifier:        verifier,
		StatusCode:      200,
		Body:            handlerBody,
		TimeoutMS:       300000,
	})
	if err != nil {
		t.Errorf("signature should match body: %v", err)
	}
}

// ============================================================================
// Helpers
// ============================================================================

type simpleContext struct {
	req *http.Request
	w   http.ResponseWriter
}

func (c *simpleContext) Request() *http.Request                { return c.req }
func (c *simpleContext) Method() string                        { return c.req.Method }
func (c *simpleContext) Path() string                          { return c.req.URL.Path }
func (c *simpleContext) QueryParam(name string) string         { return c.req.URL.Query().Get(name) }
func (c *simpleContext) Param(name string) string              { return "" }
func (c *simpleContext) FormValue(key string) string           { return "" }
func (c *simpleContext) PostForm(key string) string            { return "" }
func (c *simpleContext) ParseForm() error                      { return nil }
func (c *simpleContext) ParseMultipartForm(max int64) error    { return nil }
func (c *simpleContext) Status(code int)                        { c.w.WriteHeader(code) }
func (c *simpleContext) JSON(code int, obj interface{}) error {
	c.w.Header().Set("Content-Type", "application/json")
	c.w.WriteHeader(code)
	data, _ := json.Marshal(obj)
	c.w.Write(data)
	return nil
}
func (c *simpleContext) XML(code int, obj interface{}) error   { return nil }
func (c *simpleContext) Text(code int, text string) error      { c.w.WriteHeader(code); c.w.Write([]byte(text)); return nil }
func (c *simpleContext) HTML(code int, html string) error      { return nil }
func (c *simpleContext) Redirect(code int, url string) error   { return nil }
func (c *simpleContext) Cookie(name string) (string, error)    { return "", nil }
func (c *simpleContext) SetCookie(cookie *http.Cookie)          {}
func (c *simpleContext) Logger() web.Logger                     { return nil }
func (c *simpleContext) BindJSON(obj interface{}) error        { return nil }
func (c *simpleContext) BindXML(obj interface{}) error         { return nil }
func (c *simpleContext) BindQuery(obj interface{}) error       { return nil }
func (c *simpleContext) Set(key string, value interface{})      {}
func (c *simpleContext) Get(key string) interface{}             { return nil }
func (c *simpleContext) Context() context.Context              { return c.req.Context() }
func (c *simpleContext) ResponseWriter() http.ResponseWriter    { return c.w }
func (c *simpleContext) SetResponseWriter(w http.ResponseWriter) { c.w = w }

// capturingContext supports SetResponseWriter so the middleware can intercept the body.
type capturingContext struct {
	req *http.Request
	w   http.ResponseWriter
}

func (c *capturingContext) Request() *http.Request                { return c.req }
func (c *capturingContext) Method() string                        { return c.req.Method }
func (c *capturingContext) Path() string                          { return c.req.URL.Path }
func (c *capturingContext) QueryParam(name string) string         { return c.req.URL.Query().Get(name) }
func (c *capturingContext) Param(name string) string              { return "" }
func (c *capturingContext) FormValue(key string) string           { return "" }
func (c *capturingContext) PostForm(key string) string            { return "" }
func (c *capturingContext) ParseForm() error                      { return nil }
func (c *capturingContext) ParseMultipartForm(max int64) error    { return nil }
func (c *capturingContext) Status(code int)                        { c.w.WriteHeader(code) }
func (c *capturingContext) JSON(code int, obj interface{}) error {
	c.w.Header().Set("Content-Type", "application/json")
	c.w.WriteHeader(code)
	data, _ := json.Marshal(obj)
	c.w.Write(data)
	return nil
}
func (c *capturingContext) XML(code int, obj interface{}) error   { return nil }
func (c *capturingContext) Text(code int, text string) error      { c.w.WriteHeader(code); c.w.Write([]byte(text)); return nil }
func (c *capturingContext) HTML(code int, html string) error      { return nil }
func (c *capturingContext) Redirect(code int, url string) error   { return nil }
func (c *capturingContext) Cookie(name string) (string, error)    { return "", nil }
func (c *capturingContext) SetCookie(cookie *http.Cookie)          {}
func (c *capturingContext) Logger() web.Logger                     { return nil }
func (c *capturingContext) BindJSON(obj interface{}) error        { return nil }
func (c *capturingContext) BindXML(obj interface{}) error         { return nil }
func (c *capturingContext) BindQuery(obj interface{}) error       { return nil }
func (c *capturingContext) Set(key string, value interface{})      {}
func (c *capturingContext) Get(key string) interface{}             { return nil }
func (c *capturingContext) Context() context.Context              { return c.req.Context() }
func (c *capturingContext) ResponseWriter() http.ResponseWriter    { return c.w }
func (c *capturingContext) SetResponseWriter(w http.ResponseWriter) { c.w = w }
