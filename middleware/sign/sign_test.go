package sign

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	web "github.com/bufgot/web"
	"github.com/cloudwego/hertz/pkg/common/ut"
		"github.com/gofiber/fiber/v2"
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

func TestVerifyRequest_TimeoutZero(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "shared"})
	ts := time.Now()
	body := []byte("timeout-zero")
	headers, _ := SignRequest(SignRequestParams{
		AppID: "tz", Method: MethodHMACSHA256,
		Signer: signer, Path: "/tz", Body: body,
	}, ts)
	verifier, _ := NewVerifier(MethodHMACSHA256, SignerOpts{Salt: "shared"})
	err := VerifyRequest(VerifyRequestParams{
		AppHeader: headers.App, TimestampHeader: headers.Timestamp,
		NonceHeader: headers.Nonce, SignHeader: headers.Sign,
		Verifier: verifier, Path: "/tz", Body: body,
		TimeoutMS: 0, // use default
	})
	if err != nil {
		t.Errorf("unexpected error with TimeoutMS=0: %v", err)
	}
}

func TestVerifyRequest_InvalidTimestamp(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "shared"})
	ts := time.Now()
	body := []byte("bad-ts")
	headers, _ := SignRequest(SignRequestParams{
		AppID: "bts", Method: MethodHMACSHA256,
		Signer: signer, Path: "/bts", Body: body,
	}, ts)
	verifier, _ := NewVerifier(MethodHMACSHA256, SignerOpts{Salt: "shared"})
	err := VerifyRequest(VerifyRequestParams{
		AppHeader: headers.App, TimestampHeader: "not-a-number",
		NonceHeader: headers.Nonce, SignHeader: headers.Sign,
		Verifier: verifier, Path: "/bts", Body: body,
	})
	if err == nil {
		t.Error("expected error for invalid timestamp")
	}
}

func TestVerifyRequest_ExpiredTimestamp_Distant(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "shared"})
	ts := time.Now().Add(-24 * time.Hour) // 1 day ago
	body := []byte("expired")
	headers, _ := SignRequest(SignRequestParams{
		AppID: "exp", Method: MethodHMACSHA256,
		Signer: signer, Path: "/exp", Body: body,
	}, ts)
	verifier, _ := NewVerifier(MethodHMACSHA256, SignerOpts{Salt: "shared"})
	err := VerifyRequest(VerifyRequestParams{
		AppHeader: headers.App, TimestampHeader: headers.Timestamp,
		NonceHeader: headers.Nonce, SignHeader: headers.Sign,
		Verifier: verifier, Path: "/exp", Body: body,
	})
	if err == nil {
		t.Error("expected timestamp expired error")
	}
}

func TestVerifyResponse_ExpiredTimestamp(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "resp-exp"})
	ts := time.Now().Add(-24 * time.Hour)
	headers, err := SignResponse(SignResponseParams{
		AppID: "resp-exp", Method: MethodHMACSHA256,
		Signer: signer, StatusCode: 200, Body: []byte("expired"),
	}, ts)
	if err != nil {
		t.Fatalf("SignResponse: %v", err)
	}
	verifier, _ := NewVerifier(MethodHMACSHA256, SignerOpts{Salt: "resp-exp"})
	err = VerifyResponse(VerifyResponseParams{
		AppHeader: headers.App, TimestampHeader: headers.Timestamp,
		NonceHeader: headers.Nonce, SignHeader: headers.Sign,
		Verifier: verifier, StatusCode: 200, Body: []byte("expired"),
		HeaderNames: defaultReq,
	})
	if err == nil {
		t.Error("expected timestamp expired error for response")
	}
}


func TestVerifyResponse_TimeoutZero(t *testing.T) {
	cfg := ServerSignConfig{Method: string(MethodHMACSHA256), Salt: "rsp-z", AppID: "rsp-z"}
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "rsp-z"})
	headers, err := SignResponse(SignResponseParams{
		AppID: cfg.AppID, Method: MethodHMACSHA256,
		Signer: signer, StatusCode: 200, Body: []byte("data"),
	}, time.Now())
	if err != nil {
		t.Fatalf("SignResponse: %v", err)
	}
	verifier, _ := NewVerifier(MethodHMACSHA256, SignerOpts{Salt: "rsp-z"})
	err = VerifyResponse(VerifyResponseParams{
		AppHeader: headers.App, TimestampHeader: headers.Timestamp,
		NonceHeader: headers.Nonce, SignHeader: headers.Sign,
		Verifier: verifier, StatusCode: 200, Body: []byte("data"),
		TimeoutMS: 0, HeaderNames: defaultReq,
	})
	if err != nil {
		t.Errorf("VerifyResponse TimeoutMS=0: %v", err)
	}
}

func TestVerifyResponse_InvalidTimestamp(t *testing.T) {
	cfg := ServerSignConfig{Method: string(MethodHMACSHA256), Salt: "rsp-bts", AppID: "rsp-bts"}
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "rsp-bts"})
	headers, err := SignResponse(SignResponseParams{
		AppID: cfg.AppID, Method: MethodHMACSHA256,
		Signer: signer, StatusCode: 200, Body: []byte("data"),
	}, time.Now())
	if err != nil {
		t.Fatalf("SignResponse: %v", err)
	}
	verifier, _ := NewVerifier(MethodHMACSHA256, SignerOpts{Salt: "rsp-bts"})
	err = VerifyResponse(VerifyResponseParams{
		AppHeader: headers.App, TimestampHeader: "bad-ts",
		NonceHeader: headers.Nonce, SignHeader: headers.Sign,
		Verifier: verifier, StatusCode: 200, Body: []byte("data"),
		HeaderNames: defaultReq,
	})
	if err == nil {
		t.Error("expected error for invalid timestamp")
	}
}


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

func TestVerifyRequestFasthttp_UnknownApp(t *testing.T) {
	cfg := RequestVerifyConfig{Enable: true, Verifications: []VerificationItem{
		{App: "known-app", Enable: true, Method: string(MethodHMACSHA256), Salt: "test-salt"},
	}}
	mw := VerifyRequestFasthttp(cfg, defaultReq)
	handler := mw(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(200)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/unknown-fh")
	ctx.Request.Header.Set(defaultReq.ReqApp, "unknown-app")
	ctx.Request.Header.Set(defaultReq.ReqTimestamp, strconv.FormatInt(time.Now().UnixMilli(), 10))
	ctx.Request.Header.Set(defaultReq.ReqNonce, "nonce")
	ctx.Request.Header.Set(defaultReq.ReqSign, "sig")
	handler(ctx)

	if ctx.Response.StatusCode() != http.StatusUnauthorized {
		t.Errorf("expected 401 for unknown app, got %d", ctx.Response.StatusCode())
	}
}

func TestVerifyRequestFasthttp_VerifierFailure(t *testing.T) {
	cfg := RequestVerifyConfig{Enable: true, Verifications: []VerificationItem{
		{App: "bad-app", Enable: true, Method: "unknown-method", Salt: "bad"},
	}}
	mw := VerifyRequestFasthttp(cfg, defaultReq)
	handler := mw(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(200)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/vf-fh")
	ctx.Request.Header.Set(defaultReq.ReqApp, "bad-app")
	ctx.Request.Header.Set(defaultReq.ReqTimestamp, strconv.FormatInt(time.Now().UnixMilli(), 10))
	ctx.Request.Header.Set(defaultReq.ReqNonce, "nonce")
	ctx.Request.Header.Set(defaultReq.ReqSign, "sig")
	handler(ctx)

	if ctx.Response.StatusCode() != http.StatusInternalServerError {
		t.Errorf("expected 500 for verifier build failure, got %d", ctx.Response.StatusCode())
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

func TestVerifyRequestFiber_UnknownApp(t *testing.T) {
	app := fiber.New()
	cfg := RequestVerifyConfig{Enable: true, Verifications: []VerificationItem{
		{App: "known-app", Enable: true, Method: string(MethodHMACSHA256), Salt: "test-salt"},
	}}
	app.Get("/unknown", VerifyRequestFiber(cfg, defaultReq), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	req, _ := http.NewRequest("GET", "/unknown", nil)
	req.Host = "example.com"
	req.Header.Set(defaultReq.ReqApp, "unknown-app")
	req.Header.Set(defaultReq.ReqTimestamp, strconv.FormatInt(time.Now().UnixMilli(), 10))
	req.Header.Set(defaultReq.ReqNonce, "nonce")
	req.Header.Set(defaultReq.ReqSign, "sig")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for unknown app, got %d", resp.StatusCode)
	}
}

func TestVerifyRequestFiber_VerifierFailure(t *testing.T) {
	app := fiber.New()
	cfg := RequestVerifyConfig{Enable: true, Verifications: []VerificationItem{
		{App: "bad-app", Enable: true, Method: "unknown-method", Salt: "bad"},
	}}
	app.Get("/vf", VerifyRequestFiber(cfg, defaultReq), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	req, _ := http.NewRequest("GET", "/vf", nil)
	req.Host = "example.com"
	req.Header.Set(defaultReq.ReqApp, "bad-app")
	req.Header.Set(defaultReq.ReqTimestamp, strconv.FormatInt(time.Now().UnixMilli(), 10))
	req.Header.Set(defaultReq.ReqNonce, "nonce")
	req.Header.Set(defaultReq.ReqSign, "sig")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 for verifier build failure, got %d", resp.StatusCode)
	}
}

func TestSignResponseFiber_Enabled(t *testing.T) {
	app := fiber.New()
	app.Get("/sign-fiber", SignResponseFiber(ServerSignConfig{
		Method: string(MethodHMACSHA256), Salt: "fiber-s", AppID: "fiber-app",
	}, defaultReq), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	req, _ := http.NewRequest("GET", "/sign-fiber", nil)
	req.Host = "example.com"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get(defaultReq.ResApp) == "" {
		t.Error("expected ResApp header")
	}
	if resp.Header.Get(defaultReq.ResSign) == "" {
		t.Error("expected ResSign header")
	}
}

func TestSignResponseFiber_NewSignerError(t *testing.T) {
	app := fiber.New()
	// unknown method triggers NewSigner error
	app.Get("/bad", SignResponseFiber(ServerSignConfig{
		Method: "unknown", AppID: "bad-app",
	}, defaultReq), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	req, _ := http.NewRequest("GET", "/bad", nil)
	req.Host = "example.com"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("should pass through on signer error, got %d", resp.StatusCode)
	}
}

func TestSignResponseFiber_SignError(t *testing.T) {
	app := fiber.New()
	// empty PrivateKey for ed25519 causes Sign error in handler
	app.Get("/err", SignResponseFiber(ServerSignConfig{
		Method: "ed25519", AppID: "bad-key",
	}, defaultReq), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	req, _ := http.NewRequest("GET", "/err", nil)
	req.Host = "example.com"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("should pass through on sign error, got %d", resp.StatusCode)
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

func TestVerifyRequestHertz_UnknownApp(t *testing.T) {
	rctx := ut.CreateUtRequestContext("GET", "/unknown-hz", nil)
	rctx.Request.Header.Set(defaultReq.ReqApp, "unknown-app")
	rctx.Request.Header.Set(defaultReq.ReqTimestamp, strconv.FormatInt(time.Now().UnixMilli(), 10))
	rctx.Request.Header.Set(defaultReq.ReqNonce, "nonce")
	rctx.Request.Header.Set(defaultReq.ReqSign, "sig")

	cfg := RequestVerifyConfig{Enable: true, Verifications: []VerificationItem{
		{App: "known-app", Enable: true, Method: string(MethodHMACSHA256), Salt: "test-salt"},
	}}
	handler := VerifyRequestHertz(cfg, defaultReq)
	handler(context.Background(), rctx)

	if rctx.Response.StatusCode() != http.StatusUnauthorized {
		t.Errorf("expected 401 for unknown app, got %d", rctx.Response.StatusCode())
	}
}

func TestVerifyRequestHertz_VerifierFailure(t *testing.T) {
	rctx := ut.CreateUtRequestContext("GET", "/vf-hz", nil)
	rctx.Request.Header.Set(defaultReq.ReqApp, "bad-app")
	rctx.Request.Header.Set(defaultReq.ReqTimestamp, strconv.FormatInt(time.Now().UnixMilli(), 10))
	rctx.Request.Header.Set(defaultReq.ReqNonce, "nonce")
	rctx.Request.Header.Set(defaultReq.ReqSign, "sig")

	cfg := RequestVerifyConfig{Enable: true, Verifications: []VerificationItem{
		{App: "bad-app", Enable: true, Method: "unknown-method", Salt: "bad"},
	}}
	handler := VerifyRequestHertz(cfg, defaultReq)
	handler(context.Background(), rctx)

	if rctx.Response.StatusCode() != http.StatusInternalServerError {
		t.Errorf("expected 500 for verifier build failure, got %d", rctx.Response.StatusCode())
	}
}

func TestSignResponseHertz_Enabled(t *testing.T) {
	rctx := ut.CreateUtRequestContext("GET", "/sign-hz", nil)
	_ = ut.CreateUtRequestContext // suppress unused import

	handler := SignResponseHertz(ServerSignConfig{
		Method: string(MethodHMACSHA256), Salt: "hertz-s", AppID: "hertz-app",
	}, defaultReq)
	handler(context.Background(), rctx)

	if rctx.Response.StatusCode() == 0 {
		t.Error("expected response status code")
	}
	if string(rctx.Response.Header.Peek(defaultReq.ResApp)) == "" {
		t.Error("expected ResApp header")
	}
	if string(rctx.Response.Header.Peek(defaultReq.ResSign)) == "" {
		t.Error("expected ResSign header")
	}
}

func TestSignResponseHertz_NewSignerError(t *testing.T) {
	rctx := ut.CreateUtRequestContext("GET", "/bad-hz", nil)

	handler := SignResponseHertz(ServerSignConfig{
		Method: "unknown", AppID: "bad-app",
	}, defaultReq)
	// Should not panic — unknown method returns pass-through handler
	handler(context.Background(), rctx)
}

func TestSignResponseHertz_SignError(t *testing.T) {
	rctx := ut.CreateUtRequestContext("GET", "/err-hz", nil)

	handler := SignResponseHertz(ServerSignConfig{
		Method: "ed25519", AppID: "bad-key",
	}, defaultReq)
	// Empty ed25519 key causes Sign error — should pass through
	handler(context.Background(), rctx)
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

// ============================================================================
// Dynamic Config Tests
// ============================================================================

func TestDynamicRequestVerifyConfig_LoadStore(t *testing.T) {
	cfg1 := RequestVerifyConfig{Enable: true}
	d := NewDynamicRequestVerifyConfig(cfg1)
	if !d.Load().Enable {
		t.Error("should load initial config")
	}

	cfg2 := RequestVerifyConfig{Enable: false}
	d.Store(cfg2)
	if d.Load().Enable {
		t.Error("should load updated config")
	}
}

func TestDynamicServerSignConfig_LoadStore(t *testing.T) {
	cfg1 := ServerSignConfig{Method: "md5"}
	d := NewDynamicServerSignConfig(cfg1)
	if d.Load().Method != "md5" {
		t.Error("should load initial config")
	}

	cfg2 := ServerSignConfig{Method: "hmac-sha256"}
	d.Store(cfg2)
	if d.Load().Method != "hmac-sha256" {
		t.Error("should load updated config")
	}
}

func TestDynamicClientSignConfig_LoadStore(t *testing.T) {
	cfg1 := ClientSignConfig{Method: "hmac-sha256", Salt: "s1"}
	d := NewDynamicClientSignConfig(cfg1)
	if d.Load().Salt != "s1" {
		t.Error("should load initial config")
	}

	cfg2 := ClientSignConfig{Method: "ed25519", Salt: "s2"}
	d.Store(cfg2)
	if d.Load().Salt != "s2" {
		t.Error("should load updated config")
	}
}

func TestDynamicResponseVerifyConfig_LoadStore(t *testing.T) {
	cfg1 := ResponseVerifyConfig{Enable: false}
	d := NewDynamicResponseVerifyConfig(cfg1)
	if d.Load().Enable {
		t.Error("should load initial config")
	}

	cfg2 := ResponseVerifyConfig{Enable: true}
	d.Store(cfg2)
	if !d.Load().Enable {
		t.Error("should load updated config")
	}
}

// ============================================================================
// ConfigBridge Tests
// ============================================================================

func TestSignConfigBridge_OnChange(t *testing.T) {
	rv1 := RequestVerifyConfig{Enable: true}
	rs1 := ServerSignConfig{Method: "hmac-sha256", AppID: "old"}
	bridge := &SignConfigBridge{
		ReqVerify: NewDynamicRequestVerifyConfig(rv1),
		RespSign:  NewDynamicServerSignConfig(rs1),
		ExtractReqVerify: func(fullCfg any) RequestVerifyConfig {
			return fullCfg.(*testAppCfg).Verify
		},
		ExtractRespSign: func(fullCfg any) ServerSignConfig {
			return fullCfg.(*testAppCfg).Sign
		},
	}

	newCfg := &testAppCfg{
		Verify: RequestVerifyConfig{Enable: false},
		Sign:   ServerSignConfig{Method: "md5", AppID: "new"},
	}
	bridge.OnChange(newCfg)

	if bridge.ReqVerify.Load().Enable {
		t.Error("req verify should be updated to disabled")
	}
	if bridge.RespSign.Load().AppID != "new" {
		t.Errorf("resp sign should be updated, got %q", bridge.RespSign.Load().AppID)
	}
}

type testAppCfg struct {
	Verify RequestVerifyConfig
	Sign   ServerSignConfig
}

func TestSignConfigBridge_OnChange_NilExtractors(t *testing.T) {
	// Nil extractors should not panic.
	bridge := &SignConfigBridge{}
	var cfg any
	bridge.OnChange(cfg) // must not panic
}

// ============================================================================
// Net/HTTP Dynamic Middleware Tests
// ============================================================================

func TestVerifyRequestMiddlewareDynamic_Valid(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "dyn-s"})
	ts := time.Now()
	body := []byte(`{"hello":"dynamic"}`)
	headers, _ := SignRequest(SignRequestParams{
		AppID: "app1", Method: MethodHMACSHA256,
		Signer: signer, Path: "/api/dyn", Body: body,
	}, ts)

	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "dyn-s", Timeout: 300000},
		},
	})

	req := httptest.NewRequest("POST", "/api/dyn", bytes.NewReader(body))
	req.Header.Set(defaultReq.ReqApp, headers.App)
	req.Header.Set(defaultReq.ReqTimestamp, headers.Timestamp)
	req.Header.Set(defaultReq.ReqNonce, headers.Nonce)
	req.Header.Set(defaultReq.ReqSign, headers.Sign)
	rec := httptest.NewRecorder()

	var called bool
	mw := VerifyRequestMiddlewareDynamic(dynCfg, defaultReq)
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

func TestVerifyRequestMiddlewareDynamic_InvalidSignature(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "dyn-s", Timeout: 300000},
		},
	})

	body := []byte(`{}`)
	req := httptest.NewRequest("POST", "/api/fake", bytes.NewReader(body))
	req.Header.Set(defaultReq.ReqApp, "app1")
	req.Header.Set(defaultReq.ReqTimestamp, "1700000000000")
	req.Header.Set(defaultReq.ReqNonce, "abcdef123456")
	req.Header.Set(defaultReq.ReqSign, "deadbeef")
	rec := httptest.NewRecorder()

	mw := VerifyRequestMiddlewareDynamic(dynCfg, defaultReq)
	wrapped := func(w http.ResponseWriter, r *http.Request) {
		ctx := &simpleContext{req: r, w: w}
		mw(func(ctx web.Context) error {
			t.Error("handler should not be called with invalid signature")
			return nil
		})(ctx)
	}

	wrapped(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestVerifyRequestMiddlewareDynamic_Disabled(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{Enable: false})

	req := httptest.NewRequest("GET", "/api", nil)
	rec := httptest.NewRecorder()

	var called bool
	mw := VerifyRequestMiddlewareDynamic(dynCfg, defaultReq)
	h := mw(func(ctx web.Context) error { called = true; return nil })
	ctx := &simpleContext{req: req, w: rec}
	h(ctx)

	if !called {
		t.Error("handler should be called when disabled")
	}
}

func TestVerifyRequestMiddlewareDynamic_SkipPaths(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable:    true,
		SkipPaths: []string{"/health"},
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 5000},
		},
	})

	var called bool
	mw := VerifyRequestMiddlewareDynamic(dynCfg, defaultReq)
	h := mw(func(ctx web.Context) error { called = true; return nil })

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	ctx := &simpleContext{req: req, w: rec}
	h(ctx)

	if !called {
		t.Error("handler should be called for skipped path")
	}
}

func TestVerifyRequestMiddlewareDynamic_MissingHeaders(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 5000},
		},
	})

	req := httptest.NewRequest("POST", "/api", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()

	mw := VerifyRequestMiddlewareDynamic(dynCfg, defaultReq)
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
}

func TestVerifyRequestMiddlewareDynamic_UnknownApp(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "known", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 300000},
		},
	})

	body := []byte(`{}`)
	req := httptest.NewRequest("POST", "/api", bytes.NewReader(body))
	req.Header.Set(defaultReq.ReqApp, "evil-app")
	req.Header.Set(defaultReq.ReqTimestamp, "1700000000000")
	req.Header.Set(defaultReq.ReqNonce, "abcdef123456")
	req.Header.Set(defaultReq.ReqSign, "deadbeef")
	rec := httptest.NewRecorder()

	mw := VerifyRequestMiddlewareDynamic(dynCfg, defaultReq)
	wrapped := func(w http.ResponseWriter, r *http.Request) {
		ctx := &simpleContext{req: r, w: w}
		mw(func(ctx web.Context) error {
			t.Error("handler should not be called for unknown app")
			return nil
		})(ctx)
	}
	wrapped(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestSignResponseMiddlewareDynamic_Enabled(t *testing.T) {
	dynCfg := NewDynamicServerSignConfig(ServerSignConfig{
		Method: "hmac-sha256", Salt: "rs-dyn", AppID: "dyn-srv",
	})
	verifier, _ := NewVerifier(MethodHMACSHA256, SignerOpts{Salt: "rs-dyn"})

	mw := SignResponseMiddlewareDynamic(dynCfg, defaultReq)
	handlerBody := []byte(`{"dynamic":"ok"}`)
	h := mw(func(ctx web.Context) error {
		ctx.Status(200)
		w := ctx.ResponseWriter()
		w.WriteHeader(200)
		w.Write(handlerBody)
		return nil
	})

	req := httptest.NewRequest("GET", "/api", nil)
	rec := httptest.NewRecorder()
	ctx := &capturingContext{req: req, w: rec}
	h(ctx)

	app := rec.Header().Get(defaultReq.ResApp)
	ts := rec.Header().Get(defaultReq.ResTimestamp)
	nonce := rec.Header().Get(defaultReq.ResNonce)
	sig := rec.Header().Get(defaultReq.ResSign)

	if app != "dyn-srv" {
		t.Errorf("expected app dyn-srv, got %q", app)
	}
	if sig == "" {
		t.Fatal("expected x-res-sign header")
	}

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
		t.Errorf("dynamic signature should match body: %v", err)
	}
}

func TestSignResponseMiddlewareDynamic_Disabled(t *testing.T) {
	dynCfg := NewDynamicServerSignConfig(ServerSignConfig{})
	mw := SignResponseMiddlewareDynamic(dynCfg, defaultReq)
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

func TestSignResponseMiddlewareDynamic_SkipPaths(t *testing.T) {
	dynCfg := NewDynamicServerSignConfig(ServerSignConfig{
		Method: "hmac-sha256", Salt: "rs", AppID: "srv",
		SkipPaths: []string{"/health"},
	})
	mw := SignResponseMiddlewareDynamic(dynCfg, defaultReq)
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

// ============================================================================
// Body Stream Restore Tests
// ============================================================================

func TestVerifyRequestMiddleware_BodyRestored(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "body-s"})
	ts := time.Now()
	originalBody := []byte(`{"data":"must-be-readable"}`)
	headers, _ := SignRequest(SignRequestParams{
		AppID: "app1", Method: MethodHMACSHA256,
		Signer: signer, Path: "/api/body", Body: originalBody,
	}, ts)

	cfg := RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "body-s", Timeout: 300000},
		},
	}

	req := httptest.NewRequest("POST", "/api/body", bytes.NewReader(originalBody))
	req.Header.Set(defaultReq.ReqApp, headers.App)
	req.Header.Set(defaultReq.ReqTimestamp, headers.Timestamp)
	req.Header.Set(defaultReq.ReqNonce, headers.Nonce)
	req.Header.Set(defaultReq.ReqSign, headers.Sign)
	rec := httptest.NewRecorder()

	var downstreamBody []byte
	mw := VerifyRequestMiddleware(cfg, defaultReq)
	wrapped := func(w http.ResponseWriter, r *http.Request) {
		ctx := &simpleContext{req: r, w: w}
		mw(func(ctx web.Context) error {
			// Downstream handler must be able to read the body.
			var err error
			downstreamBody, err = ioReadAll(ctx.Request().Body)
			if err != nil {
				t.Errorf("downstream body read failed: %v", err)
			}
			w.WriteHeader(200)
			return nil
		})(ctx)
	}

	wrapped(rec, req)

	if string(downstreamBody) != string(originalBody) {
		t.Errorf("body mismatch after restore: got %q, want %q", downstreamBody, originalBody)
	}
}

func TestVerifyRequestMiddlewareDynamic_BodyRestored(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "dyn-body"})
	ts := time.Now()
	originalBody := []byte(`{"dynamic":"restored"}`)
	headers, _ := SignRequest(SignRequestParams{
		AppID: "app1", Method: MethodHMACSHA256,
		Signer: signer, Path: "/api/dyn-body", Body: originalBody,
	}, ts)

	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "dyn-body", Timeout: 300000},
		},
	})

	req := httptest.NewRequest("POST", "/api/dyn-body", bytes.NewReader(originalBody))
	req.Header.Set(defaultReq.ReqApp, headers.App)
	req.Header.Set(defaultReq.ReqTimestamp, headers.Timestamp)
	req.Header.Set(defaultReq.ReqNonce, headers.Nonce)
	req.Header.Set(defaultReq.ReqSign, headers.Sign)
	rec := httptest.NewRecorder()

	var downstreamBody []byte
	mw := VerifyRequestMiddlewareDynamic(dynCfg, defaultReq)
	wrapped := func(w http.ResponseWriter, r *http.Request) {
		ctx := &simpleContext{req: r, w: w}
		mw(func(ctx web.Context) error {
			var err error
			downstreamBody, err = ioReadAll(ctx.Request().Body)
			if err != nil {
				t.Errorf("downstream body read failed: %v", err)
			}
			w.WriteHeader(200)
			return nil
		})(ctx)
	}

	wrapped(rec, req)

	if string(downstreamBody) != string(originalBody) {
		t.Errorf("body mismatch after dynamic restore: got %q, want %q", downstreamBody, originalBody)
	}
}

func TestReadAndRestoreBody_NilBody(t *testing.T) {
	// httptest.NewRequest with nil body gives http.NoBody, which readAndRestoreBody handles.
	req := httptest.NewRequest("GET", "/api", nil)
	body, err := readAndRestoreBody(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// http.NoBody reads as empty, not nil — that's expected behavior.
	if len(body) != 0 {
		t.Errorf("expected empty body for nil Body, got %q", body)
	}
	// Body should be restored so downstream can read it again.
	secondRead, _ := ioReadAll(req.Body)
	if len(secondRead) != 0 {
		t.Errorf("restored body should be readable: %q", secondRead)
	}
}

// ============================================================================
// Fasthttp Dynamic Middleware Tests
// ============================================================================

func TestVerifyRequestFasthttpDynamic_Valid(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "ft-dyn"})
	ts := time.Now()
	body := []byte(`fasthttp-dynamic`)
	headers, _ := SignRequest(SignRequestParams{
		AppID: "app1", Method: MethodHMACSHA256,
		Signer: signer, Path: "/api/ft-dyn", Body: body,
	}, ts)

	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "ft-dyn", Timeout: 300000},
		},
	})

	var called bool
	mw := VerifyRequestFasthttpDynamic(dynCfg, defaultReq)
	h := mw(func(ctx *fasthttp.RequestCtx) { called = true })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/api/ft-dyn")
	ctx.Request.Header.Set(defaultReq.ReqApp, headers.App)
	ctx.Request.Header.Set(defaultReq.ReqTimestamp, headers.Timestamp)
	ctx.Request.Header.Set(defaultReq.ReqNonce, headers.Nonce)
	ctx.Request.Header.Set(defaultReq.ReqSign, headers.Sign)
	ctx.Request.SetBody(body)
	h(ctx)

	if !called {
		t.Error("dynamic fasthttp handler should be called with valid signature")
	}
}

func TestVerifyRequestFasthttpDynamic_Disabled(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{Enable: false})

	var called bool
	mw := VerifyRequestFasthttpDynamic(dynCfg, defaultReq)
	h := mw(func(ctx *fasthttp.RequestCtx) { called = true })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/api")
	h(ctx)

	if !called {
		t.Error("handler should be called when disabled")
	}
}

func TestVerifyRequestFasthttpDynamic_SkipPaths(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable:    true,
		SkipPaths: []string{"/health"},
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 5000},
		},
	})

	var called bool
	mw := VerifyRequestFasthttpDynamic(dynCfg, defaultReq)
	h := mw(func(ctx *fasthttp.RequestCtx) { called = true })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/health")
	h(ctx)

	if !called {
		t.Error("handler should be called for skipped path")
	}
}

func TestVerifyRequestFasthttpDynamic_MissingHeaders(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 5000},
		},
	})

	var called bool
	mw := VerifyRequestFasthttpDynamic(dynCfg, defaultReq)
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

func TestVerifyRequestFasthttpDynamic_InvalidSignature(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "ft-dyn", Timeout: 300000},
		},
	})

	var called bool
	mw := VerifyRequestFasthttpDynamic(dynCfg, defaultReq)
	h := mw(func(ctx *fasthttp.RequestCtx) { called = true })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/api/bad")
	ctx.Request.Header.Set(defaultReq.ReqApp, "app1")
	ctx.Request.Header.Set(defaultReq.ReqTimestamp, "1700000000000")
	ctx.Request.Header.Set(defaultReq.ReqNonce, "badbadbadbad")
	ctx.Request.Header.Set(defaultReq.ReqSign, "ffffffff")
	ctx.Request.SetBody([]byte(`{}`))
	h(ctx)

	if called {
		t.Error("handler should not be called with invalid signature")
	}
}

func TestVerifyRequestFasthttpDynamic_UnknownApp(t *testing.T) {
	dynCfg := &DynamicRequestVerifyConfig{}
	dynCfg.Store(RequestVerifyConfig{Enable: true, Verifications: []VerificationItem{
		{App: "known-app", Enable: true, Method: string(MethodHMACSHA256), Salt: "test-salt"},
	}})
	mw := VerifyRequestFasthttpDynamic(dynCfg, defaultReq)
	handler := mw(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(200)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/unknown-fh-d")
	ctx.Request.Header.Set(defaultReq.ReqApp, "unknown-app")
	ctx.Request.Header.Set(defaultReq.ReqTimestamp, strconv.FormatInt(time.Now().UnixMilli(), 10))
	ctx.Request.Header.Set(defaultReq.ReqNonce, "nonce")
	ctx.Request.Header.Set(defaultReq.ReqSign, "sig")
	handler(ctx)

	if ctx.Response.StatusCode() != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", ctx.Response.StatusCode())
	}
}

func TestVerifyRequestFasthttpDynamic_VerifierFailure(t *testing.T) {
	dynCfg := &DynamicRequestVerifyConfig{}
	dynCfg.Store(RequestVerifyConfig{Enable: true, Verifications: []VerificationItem{
		{App: "bad-app", Enable: true, Method: "unknown-method", Salt: "bad"},
	}})
	mw := VerifyRequestFasthttpDynamic(dynCfg, defaultReq)
	handler := mw(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(200)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/vf-fh-d")
	ctx.Request.Header.Set(defaultReq.ReqApp, "bad-app")
	ctx.Request.Header.Set(defaultReq.ReqTimestamp, strconv.FormatInt(time.Now().UnixMilli(), 10))
	ctx.Request.Header.Set(defaultReq.ReqNonce, "nonce")
	ctx.Request.Header.Set(defaultReq.ReqSign, "sig")
	handler(ctx)

	if ctx.Response.StatusCode() != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", ctx.Response.StatusCode())
	}
}

func TestSignResponseFasthttpDynamic_Enabled(t *testing.T) {
	dynCfg := NewDynamicServerSignConfig(ServerSignConfig{
		Method: "hmac-sha256", Salt: "ft-rs", AppID: "ft-srv",
	})

	mw := SignResponseFasthttpDynamic(dynCfg, defaultReq)
	h := mw(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(200)
		ctx.WriteString(`{"ft":"dynamic"}`)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/api")
	h(ctx)

	if string(ctx.Response.Header.Peek(defaultReq.ResApp)) != "ft-srv" {
		t.Error("expected ft-srv in res app header")
	}
	if string(ctx.Response.Header.Peek(defaultReq.ResSign)) == "" {
		t.Error("expected x-res-sign header")
	}
}

func TestSignResponseFasthttpDynamic_Disabled(t *testing.T) {
	dynCfg := NewDynamicServerSignConfig(ServerSignConfig{})
	mw := SignResponseFasthttpDynamic(dynCfg, defaultReq)
	h := mw(func(ctx *fasthttp.RequestCtx) { ctx.WriteString("ok") })

	ctx := &fasthttp.RequestCtx{}
	h(ctx)

	if string(ctx.Response.Header.Peek(defaultReq.ResApp)) != "" {
		t.Error("should not set headers when disabled")
	}
}

func TestSignResponseFasthttpDynamic_SkipPaths(t *testing.T) {
	dynCfg := NewDynamicServerSignConfig(ServerSignConfig{
		Method: "hmac-sha256", Salt: "ft-rs", AppID: "ft-srv",
		SkipPaths: []string{"/health"},
	})
	mw := SignResponseFasthttpDynamic(dynCfg, defaultReq)
	h := mw(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(200)
		ctx.WriteString(`ok`)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/health")
	h(ctx)

	if string(ctx.Response.Header.Peek(defaultReq.ResApp)) != "" {
		t.Error("should not set headers for skipped path")
	}
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestUnwrap(t *testing.T) {
	inner := httptest.NewRecorder()
	rr := &responseRecorder{ResponseWriter: inner, buf: &bytes.Buffer{}}
	if rr.Unwrap() != inner {
		t.Error("Unwrap should return inner ResponseWriter")
	}
}

func TestNewVerifier_UnknownMethod(t *testing.T) {
	_, err := NewVerifier("bogus", SignerOpts{})
	if err == nil {
		t.Error("expected error for unknown verifier method")
	}
}

func TestVerifyRequestMiddleware_NilBody(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "nilbody"})
	ts := time.Now()
	headers, _ := SignRequest(SignRequestParams{
		AppID: "app1", Method: MethodHMACSHA256,
		Signer: signer, Path: "/api/no-body", Body: nil,
	}, ts)

	cfg := RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "nilbody", Timeout: 300000},
		},
	}

	req := httptest.NewRequest("GET", "/api/no-body", nil)
	req.Header.Set(defaultReq.ReqApp, headers.App)
	req.Header.Set(defaultReq.ReqTimestamp, headers.Timestamp)
	req.Header.Set(defaultReq.ReqNonce, headers.Nonce)
	req.Header.Set(defaultReq.ReqSign, headers.Sign)
	rec := httptest.NewRecorder()

	var called bool
	mw := VerifyRequestMiddleware(cfg, defaultReq)
	wrapped := func(w http.ResponseWriter, r *http.Request) {
		ctx := &simpleContext{req: r, w: w}
		mw(func(ctx web.Context) error { called = true; return nil })(ctx)
	}
	wrapped(rec, req)

	if !called {
		t.Error("handler should be called with nil body")
	}
}

func TestSignResponseMiddleware_NilResponseWriter(t *testing.T) {
	cfg := ServerSignConfig{
		Method: "hmac-sha256", Salt: "nilrw", AppID: "nilrw",
	}
	mw := SignResponseMiddleware(cfg, defaultReq)
	var called bool
	h := mw(func(ctx web.Context) error { called = true; return nil })

	// context with nil response writer
	ctx := &simpleContext{req: httptest.NewRequest("GET", "/api", nil), w: nil}
	h(ctx)

	if !called {
		t.Error("handler should still be called when response writer is nil")
	}
}

func TestSignResponseMiddlewareDynamic_NilResponseWriter(t *testing.T) {
	dynCfg := NewDynamicServerSignConfig(ServerSignConfig{
		Method: "hmac-sha256", Salt: "nilrw-dyn", AppID: "nilrw",
	})
	mw := SignResponseMiddlewareDynamic(dynCfg, defaultReq)
	var called bool
	h := mw(func(ctx web.Context) error { called = true; return nil })

	ctx := &simpleContext{req: httptest.NewRequest("GET", "/api", nil), w: nil}
	h(ctx)

	if !called {
		t.Error("handler should still be called when response writer is nil")
	}
}

func TestServerSignConfig_toSignerOpts(t *testing.T) {
	cfg := ServerSignConfig{
		Salt:       "ts",
		PrivateKey: "pk",
	}
	opts := cfg.toSignerOpts()
	if opts.Salt != "ts" {
		t.Errorf("expected Salt ts, got %q", opts.Salt)
	}
	if opts.PrivateKey != "pk" {
		t.Errorf("expected PrivateKey pk, got %q", opts.PrivateKey)
	}
}

// ioReadAll is a helper that reads all bytes from an io.Reader (used in test).
func ioReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

// ============================================================================
// ClientSignConfig & ResponseVerifyConfig helpers
// ============================================================================

func TestClientSignConfig_toSignerOpts(t *testing.T) {
	cfg := ClientSignConfig{
		Salt:       "cs",
		PrivateKey: "pk-cs",
	}
	opts := cfg.toSignerOpts()
	if opts.Salt != "cs" {
		t.Errorf("expected Salt cs, got %q", opts.Salt)
	}
	if opts.PrivateKey != "pk-cs" {
		t.Errorf("expected PrivateKey pk-cs, got %q", opts.PrivateKey)
	}
}

func TestResponseVerifyConfig_LookupApp_Disabled(t *testing.T) {
	cfg := ResponseVerifyConfig{
		Verifications: []VerificationItem{
			{App: "app1", Enable: false, Method: "hmac-sha256", Salt: "s", Timeout: 5000},
		},
	}
	item := cfg.LookupApp("app1")
	if item != nil {
		t.Error("should return nil for disabled app")
	}
}

func TestResponseVerifyConfig_LookupApp_NotFound(t *testing.T) {
	cfg := ResponseVerifyConfig{
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s"},
		},
	}
	if cfg.LookupApp("nobody") != nil {
		t.Error("should return nil for unknown app")
	}
}

func TestResponseVerifyConfig_LookupApp_Found(t *testing.T) {
	cfg := ResponseVerifyConfig{
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s"},
		},
	}
	item := cfg.LookupApp("app1")
	if item == nil {
		t.Fatal("should find enabled app")
	}
	if item.Method != "hmac-sha256" {
		t.Errorf("wrong method: %s", item.Method)
	}
}

// ============================================================================
// Sign edge cases
// ============================================================================

func TestSignRequest_NewSignerError(t *testing.T) {
	_, err := SignRequest(SignRequestParams{
		AppID:  "app1",
		Signer: nil, // nil triggers error
	}, time.Now())
	if err == nil || !strings.Contains(err.Error(), "Signer is nil") {
		t.Errorf("expected Signer is nil error, got %v", err)
	}
}

func TestSignResponse_NewSignerError(t *testing.T) {
	_, err := SignResponse(SignResponseParams{
		AppID:  "app1",
		Signer: nil,
	}, time.Now())
	if err == nil || !strings.Contains(err.Error(), "Signer is nil") {
		t.Errorf("expected Signer is nil error, got %v", err)
	}
}

func TestVerifyRequest_NewVerifierError(t *testing.T) {
	err := VerifyRequest(VerifyRequestParams{
		AppHeader:       "app1",
		TimestampHeader: "1700000000000",
		NonceHeader:     "abcdef123456",
		SignHeader:      "deadbeef",
		Verifier:        nil, // nil verifier triggers error path
		Path:            "/api",
		Body:            []byte(`{}`),
	})
	if err == nil {
		t.Error("expected error with nil verifier")
	}
}

func TestVerifyResponse_NewVerifierError(t *testing.T) {
	err := VerifyResponse(VerifyResponseParams{
		AppHeader:       "app1",
		TimestampHeader: "1700000000000",
		NonceHeader:     "abcdef123456",
		SignHeader:      "deadbeef",
		Verifier:        nil,
		StatusCode:      200,
		Body:            []byte(`{}`),
	})
	if err == nil {
		t.Error("expected error with nil verifier")
	}
}

func TestSignRequest_VerifyError(t *testing.T) {
	// ED25519 requires actual key pair — sign with generated key, then alter sig
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	signer := &ed25519Signer{privKey: priv}
	ts := time.Now()
	headers, err := SignRequest(SignRequestParams{
		AppID:  "app1",
		Method: "ed25519",
		Signer: signer,
		Path:   "/api/ed",
		Body:   []byte("test"),
	}, ts)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}

	// Now verify with a different body — should fail
	verifier := &ed25519Signer{pubKey: pub}
	err = VerifyRequest(VerifyRequestParams{
		AppHeader:       headers.App,
		TimestampHeader: headers.Timestamp,
		NonceHeader:     headers.Nonce,
		SignHeader:      headers.Sign,
		Verifier:        verifier,
		Path:            "/api/ed",
		Body:            []byte("wrong-body"),
	})
	if err == nil {
		t.Error("expected verification error with mismatched body")
	}
}

func TestSignResponse_VerifyError(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	signer := &ed25519Signer{privKey: priv}
	ts := time.Now()
	headers, err := SignResponse(SignResponseParams{
		AppID:  "app1",
		Method: "ed25519",
		Signer: signer,
		StatusCode: 200,
		Body:    []byte("resp"),
	}, ts)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}

	verifier := &ed25519Signer{pubKey: pub}
	err = VerifyResponse(VerifyResponseParams{
		AppHeader:       headers.App,
		TimestampHeader: headers.Timestamp,
		NonceHeader:     headers.Nonce,
		SignHeader:      headers.Sign,
		Verifier:        verifier,
		StatusCode:      200,
		Body:            []byte("tampered"),
	})
	if err == nil {
		t.Error("expected verification error with tampered body")
	}
}

func TestSignRequest_Base64Fallback(t *testing.T) {
	// Generate raw ed25519 keys, encode as base64 for SignerOpts.PrivateKey.
	// ed25519.GenerateKey returns a 64-byte PrivateKey (seed+public);
	// newEd25519Signer expects only the 32-byte seed.
	pubRaw, privRaw, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	seed := privRaw.Seed()
	pubB64 := base64.StdEncoding.EncodeToString(pubRaw)
	seedB64 := base64.StdEncoding.EncodeToString(seed)

	signer, err := NewSigner("ed25519", SignerOpts{
		PrivateKey: seedB64,
		PublicKey:  pubB64,
	})
	if err != nil {
		t.Fatalf("NewSigner with base64 keys: %v", err)
	}

	ts := time.Now()
	headers, err := SignRequest(SignRequestParams{
		AppID:  "base64test",
		Method: "ed25519",
		Signer: signer,
		Path:   "/api/b64",
		Body:   []byte("base64"),
	}, ts)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}

	verifier, err := NewVerifier("ed25519", SignerOpts{PublicKey: pubB64})
	if err != nil {
		t.Fatalf("NewVerifier with base64: %v", err)
	}
	err = VerifyRequest(VerifyRequestParams{
		AppHeader:       headers.App,
		TimestampHeader: headers.Timestamp,
		NonceHeader:     headers.Nonce,
		SignHeader:      headers.Sign,
		Verifier:        verifier,
		Path:            "/api/b64",
		Body:            []byte("base64"),
	})
	if err != nil {
		t.Errorf("verification should pass with base64 keys: %v", err)
	}
}

func TestValidateHeaders_EmptySalt(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: ""})
	// Should still work
	ts := time.Now()
	_, err := SignRequest(SignRequestParams{
		AppID:  "app1",
		Method: MethodHMACSHA256,
		Signer: signer,
		Path:   "/api",
		Body:   []byte("test"),
	}, ts)
	if err != nil {
		t.Errorf("signing with empty salt should work: %v", err)
	}
}

// ============================================================================
// Ed25519 signer/verifier error paths
// ============================================================================

func TestNewEd25519Signer_ShortKey(t *testing.T) {
	_, err := newEd25519Signer(SignerOpts{PrivateKey: "too-short"})
	if err == nil {
		t.Error("expected error for short private key")
	}
}

func TestNewEd25519Signer_InvalidBase64(t *testing.T) {
	_, err := newEd25519Signer(SignerOpts{PrivateKey: "!!!not-base64!!!"})
	if err == nil {
		t.Error("expected error for invalid base64 private key")
	}
}

func TestNewEd25519Verifier_ShortKey(t *testing.T) {
	_, err := newEd25519Verifier(SignerOpts{PublicKey: "short"})
	if err == nil {
		t.Error("expected error for short public key")
	}
}

func TestNewEd25519Verifier_InvalidHex(t *testing.T) {
	_, err := newEd25519Verifier(SignerOpts{PublicKey: "gggg"})
	if err == nil {
		t.Error("expected error for invalid hex public key")
	}
}

func TestEd25519Signer_Method(t *testing.T) {
	s := &ed25519Signer{}
	if s.Method() != "ed25519" {
		t.Errorf("expected ed25519, got %s", s.Method())
	}
}

func TestEd25519Verifier_Method(t *testing.T) {
	v := &ed25519Signer{}
	if v.Method() != "ed25519" {
		t.Errorf("expected ed25519, got %s", v.Method())
	}
}

func TestNewEd25519Signer_ValidHex(t *testing.T) {
	_, privRaw, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	seed := privRaw.Seed()
	privHex := hex.EncodeToString(seed)
	s, err := newEd25519Signer(SignerOpts{PrivateKey: privHex})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Error("signer should not be nil")
	}
}

func TestNewEd25519Verifier_ValidHex(t *testing.T) {
	pubRaw, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	pubHex := hex.EncodeToString(pubRaw)
	v, err := newEd25519Verifier(SignerOpts{PublicKey: pubHex})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v == nil {
		t.Error("verifier should not be nil")
	}
}

func TestGenerateNonce_ShortFallback(t *testing.T) {
	// generateNonce with length 0 returns empty string without error
	nonce, err := generateNonce(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nonce) != 0 {
		t.Errorf("expected 0 bytes for zero length, got %d", len(nonce))
	}
}

// ============================================================================
// Ed25519 direct sign/verify on raw bytes (coverage for signer/verifier)
// ============================================================================

func TestEd25519Signer_Sign(t *testing.T) {
	_, privRaw, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	s := &ed25519Signer{privKey: privRaw}
	msg := []byte("hello world")
	sig, err := s.Sign(msg)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	if len(sig) != ed25519.SignatureSize {
		t.Errorf("expected signature size %d, got %d", ed25519.SignatureSize, len(sig))
	}
}

func TestEd25519Verifier_Verify(t *testing.T) {
	pubRaw, privRaw, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	s := &ed25519Signer{privKey: privRaw}
	msg := []byte("verify me")
	sig, _ := s.Sign(msg)

	v := &ed25519Signer{pubKey: pubRaw}
	if !v.Verify(msg, sig) {
		t.Error("verification should succeed with valid message")
	}

	// tampered message
	if v.Verify([]byte("tampered"), sig) {
		t.Error("verification should fail with tampered message")
	}
}

// ============================================================================
// Middleware: nil request body / unknown app / verifier build failure
// ============================================================================

func TestVerifyRequestMiddleware_NilRequest(t *testing.T) {
	cfg := RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 5000},
		},
	}

	req := httptest.NewRequest("GET", "/api", nil)
	rec := httptest.NewRecorder()

	mw := VerifyRequestMiddleware(cfg, defaultReq)
	ctx := &simpleContext{req: req, w: rec}
	h := mw(func(ctx web.Context) error {
		t.Error("should not reach handler")
		return nil
	})
	h(ctx)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestVerifyRequestMiddleware_UnknownApp(t *testing.T) {
	cfg := RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "known", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 300000},
		},
	}

	body := []byte(`{}`)
	req := httptest.NewRequest("POST", "/api", bytes.NewReader(body))
	req.Header.Set(defaultReq.ReqApp, "evil-app")
	req.Header.Set(defaultReq.ReqTimestamp, "1700000000000")
	req.Header.Set(defaultReq.ReqNonce, "abcdef123456")
	req.Header.Set(defaultReq.ReqSign, "deadbeef")
	rec := httptest.NewRecorder()

	mw := VerifyRequestMiddleware(cfg, defaultReq)
	wrapped := func(w http.ResponseWriter, r *http.Request) {
		ctx := &simpleContext{req: r, w: w}
		mw(func(ctx web.Context) error {
			t.Error("should not reach handler")
			return nil
		})(ctx)
	}
	wrapped(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "unknown") {
		t.Errorf("expected unknown app message, got %s", rec.Body.String())
	}
}

func TestVerifyRequestMiddleware_VerifierBuildFailure(t *testing.T) {
	// Use "ed25519" with invalid key to trigger verifier build failure
	cfg := RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "ed25519", PublicKey: "bad-key", Timeout: 300000},
		},
	}

	body := []byte(`{}`)
	req := httptest.NewRequest("POST", "/api", bytes.NewReader(body))
	req.Header.Set(defaultReq.ReqApp, "app1")
	req.Header.Set(defaultReq.ReqTimestamp, "1700000000000")
	req.Header.Set(defaultReq.ReqNonce, "abcdef123456")
	req.Header.Set(defaultReq.ReqSign, "deadbeef")
	rec := httptest.NewRecorder()

	mw := VerifyRequestMiddleware(cfg, defaultReq)
	wrapped := func(w http.ResponseWriter, r *http.Request) {
		ctx := &simpleContext{req: r, w: w}
		mw(func(ctx web.Context) error {
			t.Error("should not reach handler")
			return nil
		})(ctx)
	}
	wrapped(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ============================================================================
// Dynamic middleware: verifier build failure
// ============================================================================

func TestVerifyRequestMiddlewareDynamic_VerifierBuildFailure(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "ed25519", PublicKey: "bad-key", Timeout: 300000},
		},
	})

	body := []byte(`{}`)
	req := httptest.NewRequest("POST", "/api", bytes.NewReader(body))
	req.Header.Set(defaultReq.ReqApp, "app1")
	req.Header.Set(defaultReq.ReqTimestamp, "1700000000000")
	req.Header.Set(defaultReq.ReqNonce, "abcdef123456")
	req.Header.Set(defaultReq.ReqSign, "deadbeef")
	rec := httptest.NewRecorder()

	mw := VerifyRequestMiddlewareDynamic(dynCfg, defaultReq)
	wrapped := func(w http.ResponseWriter, r *http.Request) {
		ctx := &simpleContext{req: r, w: w}
		mw(func(ctx web.Context) error {
			t.Error("should not reach handler")
			return nil
		})(ctx)
	}
	wrapped(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

// ============================================================================
// SignResponseMiddleware: response body written after header (realistic)
// ============================================================================

func TestSignResponseMiddleware_HandlesConcurrentWrites(t *testing.T) {
	cfg := ServerSignConfig{
		Method: "hmac-sha256", Salt: "concurrent", AppID: "app",
	}
	verifier, _ := NewVerifier(MethodHMACSHA256, SignerOpts{Salt: "concurrent"})

	mw := SignResponseMiddleware(cfg, defaultReq)
	h := mw(func(ctx web.Context) error {
		ctx.JSON(200, map[string]string{"a": "1"})
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

	// Read the body that was actually written to the recorder
	body := rec.Body.Bytes()
	err := VerifyResponse(VerifyResponseParams{
		AppHeader:       rec.Header().Get(defaultReq.ResApp),
		TimestampHeader: rec.Header().Get(defaultReq.ResTimestamp),
		NonceHeader:     rec.Header().Get(defaultReq.ResNonce),
		SignHeader:      sig,
		Verifier:        verifier,
		StatusCode:      200,
		Body:            body,
		TimeoutMS:       300000,
	})
	if err != nil {
		t.Errorf("signature should match body: %v", err)
	}
}

// ============================================================================
// VerifyRequest with nil body (readAndRestoreBody nil path)
// ============================================================================

func TestReadAndRestoreBody_NilRequest(t *testing.T) {
	// A nil *http.Request should not pass through the middleware normally,
	// but readAndRestoreBody itself should handle nil Body/NoBody.
	req := httptest.NewRequest("GET", "/", nil)
	// httptest.NewRequest with nil body yields http.NoBody
	b, err := readAndRestoreBody(req)
	if err != nil {
		t.Fatalf("readAndRestoreBody error: %v", err)
	}
	if len(b) != 0 {
		t.Errorf("expected empty body for NoBody, got %d bytes", len(b))
	}
	// Restored body must still be readable
	after, _ := io.ReadAll(req.Body)
	if len(after) != 0 {
		t.Errorf("restored body should be empty, got %d bytes", len(after))
	}
}

func TestReadAndRestoreBody_ErrorReader(t *testing.T) {
	req := httptest.NewRequest("POST", "/", &errorReader{})
	_, err := readAndRestoreBody(req)
	if err == nil {
		t.Error("expected error from error reader")
	}
}

type errorReader struct{}

func (e *errorReader) Read(p []byte) (int, error) { return 0, io.ErrUnexpectedEOF }

// ============================================================================
// Nonce 0-byte sign edge case
// ============================================================================

func TestGenerateNonce_ZeroLength(t *testing.T) {
	nonce, err := generateNonce(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nonce) != 0 {
		t.Errorf("expected 0 bytes for zero length, got %d", len(nonce))
	}
}

func TestVerifyRequestMiddleware_TimeSkew(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "skew"})
	ts := time.Now().Add(-1 * time.Hour) // 1 hour ago
	body := []byte(`{}`)
	headers, _ := SignRequest(SignRequestParams{
		AppID: "app1", Method: MethodHMACSHA256,
		Signer: signer, Path: "/api/skew", Body: body,
	}, ts)

	cfg := RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "skew", Timeout: 1000}, // 1s timeout
		},
	}

	req := httptest.NewRequest("POST", "/api/skew", bytes.NewReader(body))
	req.Header.Set(defaultReq.ReqApp, headers.App)
	req.Header.Set(defaultReq.ReqTimestamp, headers.Timestamp)
	req.Header.Set(defaultReq.ReqNonce, headers.Nonce)
	req.Header.Set(defaultReq.ReqSign, headers.Sign)
	rec := httptest.NewRecorder()

	mw := VerifyRequestMiddleware(cfg, defaultReq)
	wrapped := func(w http.ResponseWriter, r *http.Request) {
		ctx := &simpleContext{req: r, w: w}
		mw(func(ctx web.Context) error {
			t.Error("should not be called with expired timestamp")
			return nil
		})(ctx)
	}
	wrapped(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired timestamp, got %d", rec.Code)
	}
}

// ============================================================================
// OnChange: full bridge with all 4 directions
// ============================================================================

// ============================================================================
// hmacSHA256Signer Method — trivial but coverage gap
// ============================================================================
func TestHMACSHA256Signer_Method(t *testing.T) {
	s := &hmacSHA256Signer{}
	if m := s.Method(); m != MethodHMACSHA256 {
		t.Errorf("expected %s, got %s", MethodHMACSHA256, m)
	}
}

// ============================================================================
// ed25519 nil key paths
// ============================================================================
func TestEd25519Signer_Sign_NilKey(t *testing.T) {
	s := &ed25519Signer{privKey: nil}
	_, err := s.Sign([]byte("msg"))
	if err == nil {
		t.Error("expected error with nil private key")
	}
}

func TestEd25519Signer_Verify_NilKey(t *testing.T) {
	v := &ed25519Signer{pubKey: nil}
	if v.Verify([]byte("msg"), []byte("sig")) {
		t.Error("expected false with nil public key")
	}
}

// ============================================================================
// Signer/Verifier nil path tests for SignRequest/SignResponse/VerifyRequest/VerifyResponse
// ============================================================================
func TestSignRequest_NilSigner(t *testing.T) {
	_, err := SignRequest(SignRequestParams{
		AppID:  "app",
		Method: MethodHMACSHA256,
		Signer: nil,
		Path:   "/",
		Body:   []byte("data"),
	}, time.Now())
	if err == nil {
		t.Error("expected error for nil signer")
	}
}

func TestSignResponse_NilSigner(t *testing.T) {
	_, err := SignResponse(SignResponseParams{
		AppID:      "app",
		Method:     MethodHMACSHA256,
		Signer:     nil,
		StatusCode: 200,
		Body:       []byte("data"),
	}, time.Now())
	if err == nil {
		t.Error("expected error for nil signer")
	}
}

func TestVerifyRequest_NilVerifier(t *testing.T) {
	err := VerifyRequest(VerifyRequestParams{
		AppHeader:       "app",
		TimestampHeader: strconv.FormatInt(time.Now().UnixMilli(), 10),
		NonceHeader:     "nonce",
		SignHeader:      "sig",
		Verifier:        nil,
		Path:            "/",
		Body:            []byte("data"),
	})
	if err == nil {
		t.Error("expected error for nil verifier")
	}
}

func TestVerifyResponse_NilVerifier(t *testing.T) {
	err := VerifyResponse(VerifyResponseParams{
		AppHeader:       "app",
		TimestampHeader: strconv.FormatInt(time.Now().UnixMilli(), 10),
		NonceHeader:     "nonce",
		SignHeader:      "sig",
		Verifier:        nil,
		StatusCode:      200,
		Body:            []byte("data"),
	})
	if err == nil {
		t.Error("expected error for nil verifier")
	}
}

func TestSignConfigBridge_OnChange_AllDirections(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(priv)

	bridge := &SignConfigBridge{
		ReqVerify:  NewDynamicRequestVerifyConfig(RequestVerifyConfig{Enable: false}),
		RespSign:   NewDynamicServerSignConfig(ServerSignConfig{Method: ""}),
		ClientSign: NewDynamicClientSignConfig(ClientSignConfig{Method: ""}),
		RespVerify: NewDynamicResponseVerifyConfig(ResponseVerifyConfig{Enable: false}),
		ExtractReqVerify: func(fullCfg any) RequestVerifyConfig {
			return fullCfg.(*fullAppCfg).ReqV
		},
		ExtractRespSign: func(fullCfg any) ServerSignConfig {
			return fullCfg.(*fullAppCfg).RespS
		},
		ExtractClientSign: func(fullCfg any) ClientSignConfig {
			return fullCfg.(*fullAppCfg).CliS
		},
		ExtractRespVerify: func(fullCfg any) ResponseVerifyConfig {
			return fullCfg.(*fullAppCfg).RespV
		},
	}

	cfg := &fullAppCfg{
		ReqV:  RequestVerifyConfig{Enable: true, Verifications: []VerificationItem{{App: "a", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 3000}}},
		RespS: ServerSignConfig{Method: "ed25519", AppID: "ed", PrivateKey: privB64},
		CliS:  ClientSignConfig{Method: "md5", Salt: "md", AppID: "md5app"},
		RespV: ResponseVerifyConfig{Enable: true, Verifications: []VerificationItem{{App: "a", Enable: true, Method: "ed25519", PublicKey: pubB64, Timeout: 5000}}},
	}

	bridge.OnChange(cfg)

	if !bridge.ReqVerify.Load().Enable {
		t.Error("ReqVerify should be enabled")
	}
	if bridge.RespSign.Load().Method != "ed25519" {
		t.Errorf("RespSign method: %s", bridge.RespSign.Load().Method)
	}
	if bridge.ClientSign.Load().AppID != "md5app" {
		t.Errorf("ClientSign app: %s", bridge.ClientSign.Load().AppID)
	}
	if !bridge.RespVerify.Load().Enable {
		t.Error("RespVerify should be enabled")
	}
}

type fullAppCfg struct {
	ReqV  RequestVerifyConfig
	RespS ServerSignConfig
	CliS  ClientSignConfig
	RespV ResponseVerifyConfig
}
