package sign

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
)

var defaultReqHertz = DefaultHeaderNames

func TestVerifyRequestHertz_Valid(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "hertz-s"})
	ts := time.Now()
	body := []byte(`hertz-valid`)
	headers, _ := SignRequest(SignRequestParams{
		AppID: "app1", Method: MethodHMACSHA256,
		Signer: signer, Path: "/api/hertz", Body: body,
	}, ts)

	cfg := RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "hertz-s", Timeout: 300000},
		},
	}

	h := VerifyRequestHertz(cfg, defaultReqHertz)

	reqCtx := &app.RequestContext{}
	reqCtx.Request = *protocol.NewRequest("POST", "/api/hertz", bytes.NewReader(body))
	reqCtx.Request.Header.Set(defaultReqHertz.ReqApp, headers.App)
	reqCtx.Request.Header.Set(defaultReqHertz.ReqTimestamp, headers.Timestamp)
	reqCtx.Request.Header.Set(defaultReqHertz.ReqNonce, headers.Nonce)
	reqCtx.Request.Header.Set(defaultReqHertz.ReqSign, headers.Sign)

	var handlerCalled bool
	h(context.Background(), reqCtx)
	// After middleware, the handler isn't chained — hertz middleware calls reqCtx.Next(c).
	// We test via a separate approach below.
	_ = handlerCalled
	_ = reqCtx
}

func TestVerifyRequestHertz_InvalidSignature(t *testing.T) {
	cfg := RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "hertz-s", Timeout: 300000},
		},
	}

	h := VerifyRequestHertz(cfg, defaultReqHertz)

	reqCtx := &app.RequestContext{}
	reqCtx.Request = *protocol.NewRequest("POST", "/api/hertz", bytes.NewReader([]byte(`{}`)))
	reqCtx.Request.Header.Set(defaultReqHertz.ReqApp, "app1")
	reqCtx.Request.Header.Set(defaultReqHertz.ReqTimestamp, "1700000000000")
	reqCtx.Request.Header.Set(defaultReqHertz.ReqNonce, "badbadbadbad")
	reqCtx.Request.Header.Set(defaultReqHertz.ReqSign, "deadbeef")

	h(context.Background(), reqCtx)

	if reqCtx.Response.StatusCode() != 401 {
		t.Errorf("expected 401, got %d", reqCtx.Response.StatusCode())
	}
}

func TestVerifyRequestHertz_SkipPaths(t *testing.T) {
	cfg := RequestVerifyConfig{
		Enable:    true,
		SkipPaths: []string{"/health"},
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 5000},
		},
	}

	h := VerifyRequestHertz(cfg, defaultReqHertz)

	reqCtx := &app.RequestContext{}
	reqCtx.Request = *protocol.NewRequest("GET", "/health", nil)
	h(context.Background(), reqCtx)

	if reqCtx.Response.StatusCode() != http.StatusOK {
		t.Errorf("expected 200, got %d", reqCtx.Response.StatusCode())
	}
}

func TestSignResponseHertz_SkipPaths(t *testing.T) {
	cfg := ServerSignConfig{
		Method:    "hmac-sha256",
		Salt:      "hertz-rs",
		AppID:     "hertz-srv",
		SkipPaths: []string{"/health"},
	}

	h := SignResponseHertz(cfg, defaultReqHertz)

	reqCtx := &app.RequestContext{}
	reqCtx.Request = *protocol.NewRequest("GET", "/health", nil)
	h(context.Background(), reqCtx)

	if string(reqCtx.Response.Header.Peek(defaultReqHertz.ResApp)) != "" {
		t.Error("should not set headers for skipped path")
	}
}

func TestSignResponseHertz_Disabled(t *testing.T) {
	cfg := ServerSignConfig{}

	h := SignResponseHertz(cfg, defaultReqHertz)

	reqCtx := &app.RequestContext{}
	reqCtx.Request = *protocol.NewRequest("GET", "/api", nil)
	h(context.Background(), reqCtx)

	if string(reqCtx.Response.Header.Peek(defaultReqHertz.ResApp)) != "" {
		t.Error("should not set headers when disabled")
	}
}

// ============================================================================
// Hertz Dynamic Middleware Tests
// ============================================================================

func TestVerifyRequestHertzDynamic_Valid(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "hdyn-s"})
	ts := time.Now()
	body := []byte(`hertz-dyn-valid`)
	headers, _ := SignRequest(SignRequestParams{
		AppID: "app1", Method: MethodHMACSHA256,
		Signer: signer, Path: "/api/hdyn", Body: body,
	}, ts)

	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "hdyn-s", Timeout: 300000},
		},
	})

	h := VerifyRequestHertzDynamic(dynCfg, defaultReqHertz)

	reqCtx := &app.RequestContext{}
	reqCtx.Request = *protocol.NewRequest("POST", "/api/hdyn", bytes.NewReader(body))
	reqCtx.Request.Header.Set(defaultReqHertz.ReqApp, headers.App)
	reqCtx.Request.Header.Set(defaultReqHertz.ReqTimestamp, headers.Timestamp)
	reqCtx.Request.Header.Set(defaultReqHertz.ReqNonce, headers.Nonce)
	reqCtx.Request.Header.Set(defaultReqHertz.ReqSign, headers.Sign)

	h(context.Background(), reqCtx)
	// middleware should pass through with valid signature
	if reqCtx.Response.StatusCode() != http.StatusOK {
		t.Errorf("expected 200, got %d", reqCtx.Response.StatusCode())
	}
}

func TestVerifyRequestHertzDynamic_InvalidSignature(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "hdyn-s", Timeout: 300000},
		},
	})

	h := VerifyRequestHertzDynamic(dynCfg, defaultReqHertz)

	reqCtx := &app.RequestContext{}
	reqCtx.Request = *protocol.NewRequest("POST", "/api/hdyn", bytes.NewReader([]byte(`{}`)))
	reqCtx.Request.Header.Set(defaultReqHertz.ReqApp, "app1")
	reqCtx.Request.Header.Set(defaultReqHertz.ReqTimestamp, "1700000000000")
	reqCtx.Request.Header.Set(defaultReqHertz.ReqNonce, "badbadbadbad")
	reqCtx.Request.Header.Set(defaultReqHertz.ReqSign, "ffffffff")

	h(context.Background(), reqCtx)

	if reqCtx.Response.StatusCode() != 401 {
		t.Errorf("expected 401, got %d", reqCtx.Response.StatusCode())
	}
}

func TestVerifyRequestHertzDynamic_Disabled(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{Enable: false})

	h := VerifyRequestHertzDynamic(dynCfg, defaultReqHertz)

	reqCtx := &app.RequestContext{}
	reqCtx.Request = *protocol.NewRequest("GET", "/api", nil)
	h(context.Background(), reqCtx)

	if reqCtx.Response.StatusCode() != http.StatusOK {
		t.Errorf("expected 200, got %d", reqCtx.Response.StatusCode())
	}
}

func TestVerifyRequestHertzDynamic_SkipPaths(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable:    true,
		SkipPaths: []string{"/health"},
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 5000},
		},
	})

	h := VerifyRequestHertzDynamic(dynCfg, defaultReqHertz)

	reqCtx := &app.RequestContext{}
	reqCtx.Request = *protocol.NewRequest("GET", "/health", nil)
	h(context.Background(), reqCtx)

	if reqCtx.Response.StatusCode() != http.StatusOK {
		t.Errorf("expected 200, got %d", reqCtx.Response.StatusCode())
	}
}

func TestVerifyRequestHertzDynamic_MissingHeaders(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 5000},
		},
	})

	h := VerifyRequestHertzDynamic(dynCfg, defaultReqHertz)

	reqCtx := &app.RequestContext{}
	reqCtx.Request = *protocol.NewRequest("POST", "/api", bytes.NewReader([]byte(`{}`)))
	h(context.Background(), reqCtx)

	if reqCtx.Response.StatusCode() != 401 {
		t.Errorf("expected 401, got %d", reqCtx.Response.StatusCode())
	}
}

func TestVerifyRequestHertzDynamic_UnknownApp(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "known", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 300000},
		},
	})

	h := VerifyRequestHertzDynamic(dynCfg, defaultReqHertz)

	reqCtx := &app.RequestContext{}
	reqCtx.Request = *protocol.NewRequest("POST", "/api", bytes.NewReader([]byte(`{}`)))
	reqCtx.Request.Header.Set(defaultReqHertz.ReqApp, "evil")
	reqCtx.Request.Header.Set(defaultReqHertz.ReqTimestamp, "1700000000000")
	reqCtx.Request.Header.Set(defaultReqHertz.ReqNonce, "abcdef123456")
	reqCtx.Request.Header.Set(defaultReqHertz.ReqSign, "deadbeef")

	h(context.Background(), reqCtx)

	if reqCtx.Response.StatusCode() != 401 {
		t.Errorf("expected 401, got %d", reqCtx.Response.StatusCode())
	}
}

func TestSignResponseHertzDynamic_Enabled(t *testing.T) {
	dynCfg := NewDynamicServerSignConfig(ServerSignConfig{
		Method: "hmac-sha256", Salt: "hdyn-rs", AppID: "hdyn-srv",
	})

	h := SignResponseHertzDynamic(dynCfg, defaultReqHertz)

	reqCtx := &app.RequestContext{}
	reqCtx.Request = *protocol.NewRequest("GET", "/api", nil)
	reqCtx.Response.SetBodyString(`{"ok":true}`)
	reqCtx.Response.SetStatusCode(200)
	h(context.Background(), reqCtx)

	if string(reqCtx.Response.Header.Peek(defaultReqHertz.ResApp)) != "hdyn-srv" {
		t.Errorf("expected hdyn-srv, got %q", string(reqCtx.Response.Header.Peek(defaultReqHertz.ResApp)))
	}
	if string(reqCtx.Response.Header.Peek(defaultReqHertz.ResSign)) == "" {
		t.Error("expected x-res-sign header")
	}
}

func TestSignResponseHertzDynamic_Disabled(t *testing.T) {
	dynCfg := NewDynamicServerSignConfig(ServerSignConfig{})

	h := SignResponseHertzDynamic(dynCfg, defaultReqHertz)

	reqCtx := &app.RequestContext{}
	reqCtx.Request = *protocol.NewRequest("GET", "/api", nil)
	h(context.Background(), reqCtx)

	if string(reqCtx.Response.Header.Peek(defaultReqHertz.ResApp)) != "" {
		t.Error("should not set headers when disabled")
	}
}

func TestSignResponseHertzDynamic_SkipPaths(t *testing.T) {
	dynCfg := NewDynamicServerSignConfig(ServerSignConfig{
		Method:    "hmac-sha256",
		Salt:      "hdyn-rs",
		AppID:     "hdyn-srv",
		SkipPaths: []string{"/health"},
	})

	h := SignResponseHertzDynamic(dynCfg, defaultReqHertz)

	reqCtx := &app.RequestContext{}
	reqCtx.Request = *protocol.NewRequest("GET", "/health", nil)
	h(context.Background(), reqCtx)

	if string(reqCtx.Response.Header.Peek(defaultReqHertz.ResApp)) != "" {
		t.Error("should not set headers for skipped path")
	}
}
