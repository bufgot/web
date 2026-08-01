package sign

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

var defaultReqFiber = DefaultHeaderNames

func TestVerifyRequestFiber_Valid(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "fiber-s"})
	ts := time.Now()
	body := []byte(`fiber-valid`)
	headers, _ := SignRequest(SignRequestParams{
		AppID: "app1", Method: MethodHMACSHA256,
		Signer: signer, Path: "/api/fiber", Body: body,
	}, ts)

	cfg := RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "fiber-s", Timeout: 300000},
		},
	}

	app := fiber.New()
	app.Use(VerifyRequestFiber(cfg, defaultReqFiber))
	app.Post("/api/fiber", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req, _ := http.NewRequest("POST", "/api/fiber", bytes.NewReader(body))
	req.Host = "localhost"
	req.Header.Set(defaultReqFiber.ReqApp, headers.App)
	req.Header.Set(defaultReqFiber.ReqTimestamp, headers.Timestamp)
	req.Header.Set(defaultReqFiber.ReqNonce, headers.Nonce)
	req.Header.Set(defaultReqFiber.ReqSign, headers.Sign)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestVerifyRequestFiber_SkipPaths(t *testing.T) {
	cfg := RequestVerifyConfig{
		Enable:    true,
		SkipPaths: []string{"/health"},
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 5000},
		},
	}

	app := fiber.New()
	app.Use(VerifyRequestFiber(cfg, defaultReqFiber))
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("healthy")
	})

	req, _ := http.NewRequest("GET", "/health", nil)
	req.Host = "localhost"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestVerifyRequestFiber_InvalidSignature(t *testing.T) {
	cfg := RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "fiber-s", Timeout: 300000},
		},
	}

	app := fiber.New()
	app.Use(VerifyRequestFiber(cfg, defaultReqFiber))
	app.Post("/api/fiber", func(c *fiber.Ctx) error {
		t.Error("should not reach handler")
		return c.SendString("ok")
	})

	req, _ := http.NewRequest("POST", "/api/fiber", bytes.NewReader([]byte(`{}`)))
	req.Host = "localhost"
	req.Header.Set(defaultReqFiber.ReqApp, "app1")
	req.Header.Set(defaultReqFiber.ReqTimestamp, "1700000000000")
	req.Header.Set(defaultReqFiber.ReqNonce, "badbadbadbad")
	req.Header.Set(defaultReqFiber.ReqSign, "deadbeef")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestSignResponseFiber_SkipPaths(t *testing.T) {
	cfg := ServerSignConfig{
		Method:    "hmac-sha256",
		Salt:      "fiber-rs",
		AppID:     "fiber-srv",
		SkipPaths: []string{"/health"},
	}

	app := fiber.New()
	app.Use(SignResponseFiber(cfg, defaultReqFiber))
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("healthy")
	})

	req, _ := http.NewRequest("GET", "/health", nil)
	req.Host = "localhost"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	if resp.Header.Get(defaultReqFiber.ResApp) != "" {
		t.Error("should not set headers for skipped path")
	}
}

func TestSignResponseFiber_Disabled(t *testing.T) {
	cfg := ServerSignConfig{}

	app := fiber.New()
	app.Use(SignResponseFiber(cfg, defaultReqFiber))
	app.Get("/api", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req, _ := http.NewRequest("GET", "/api", nil)
	req.Host = "localhost"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	if resp.Header.Get(defaultReqFiber.ResApp) != "" {
		t.Error("should not set headers when disabled")
	}
}

// ============================================================================
// Fiber Dynamic Middleware Tests
// ============================================================================

func TestVerifyRequestFiberDynamic_Valid(t *testing.T) {
	signer, _ := NewSigner(MethodHMACSHA256, SignerOpts{Salt: "fdyn-s"})
	ts := time.Now()
	body := []byte(`fiber-dyn-valid`)
	headers, _ := SignRequest(SignRequestParams{
		AppID: "app1", Method: MethodHMACSHA256,
		Signer: signer, Path: "/api/fdyn", Body: body,
	}, ts)

	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "fdyn-s", Timeout: 300000},
		},
	})

	app := fiber.New()
	app.Use(VerifyRequestFiberDynamic(dynCfg, defaultReqFiber))
	app.Post("/api/fdyn", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req, _ := http.NewRequest("POST", "/api/fdyn", bytes.NewReader(body))
	req.Host = "localhost"
	req.Header.Set(defaultReqFiber.ReqApp, headers.App)
	req.Header.Set(defaultReqFiber.ReqTimestamp, headers.Timestamp)
	req.Header.Set(defaultReqFiber.ReqNonce, headers.Nonce)
	req.Header.Set(defaultReqFiber.ReqSign, headers.Sign)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestVerifyRequestFiberDynamic_InvalidSignature(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "fdyn-s", Timeout: 300000},
		},
	})

	app := fiber.New()
	app.Use(VerifyRequestFiberDynamic(dynCfg, defaultReqFiber))
	app.Post("/api/fdyn", func(c *fiber.Ctx) error {
		t.Error("should not reach handler")
		return c.SendString("ok")
	})

	req, _ := http.NewRequest("POST", "/api/fdyn", bytes.NewReader([]byte(`{}`)))
	req.Host = "localhost"
	req.Header.Set(defaultReqFiber.ReqApp, "app1")
	req.Header.Set(defaultReqFiber.ReqTimestamp, "1700000000000")
	req.Header.Set(defaultReqFiber.ReqNonce, "badbadbadbad")
	req.Header.Set(defaultReqFiber.ReqSign, "ffffffff")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestVerifyRequestFiberDynamic_Disabled(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{Enable: false})

	app := fiber.New()
	app.Use(VerifyRequestFiberDynamic(dynCfg, defaultReqFiber))
	app.Get("/api", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req, _ := http.NewRequest("GET", "/api", nil)
	req.Host = "localhost"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestVerifyRequestFiberDynamic_SkipPaths(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable:    true,
		SkipPaths: []string{"/health"},
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 5000},
		},
	})

	app := fiber.New()
	app.Use(VerifyRequestFiberDynamic(dynCfg, defaultReqFiber))
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("healthy")
	})

	req, _ := http.NewRequest("GET", "/health", nil)
	req.Host = "localhost"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestVerifyRequestFiberDynamic_MissingHeaders(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "app1", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 5000},
		},
	})

	app := fiber.New()
	app.Use(VerifyRequestFiberDynamic(dynCfg, defaultReqFiber))
	app.Post("/api", func(c *fiber.Ctx) error {
		t.Error("should not reach handler")
		return c.SendString("ok")
	})

	req, _ := http.NewRequest("POST", "/api", bytes.NewReader([]byte(`{}`)))
	req.Host = "localhost"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestVerifyRequestFiberDynamic_UnknownApp(t *testing.T) {
	dynCfg := NewDynamicRequestVerifyConfig(RequestVerifyConfig{
		Enable: true,
		Verifications: []VerificationItem{
			{App: "known", Enable: true, Method: "hmac-sha256", Salt: "s", Timeout: 300000},
		},
	})

	app := fiber.New()
	app.Use(VerifyRequestFiberDynamic(dynCfg, defaultReqFiber))
	app.Post("/api", func(c *fiber.Ctx) error {
		t.Error("should not reach handler")
		return c.SendString("ok")
	})

	req, _ := http.NewRequest("POST", "/api", bytes.NewReader([]byte(`{}`)))
	req.Host = "localhost"
	req.Header.Set(defaultReqFiber.ReqApp, "evil")
	req.Header.Set(defaultReqFiber.ReqTimestamp, "1700000000000")
	req.Header.Set(defaultReqFiber.ReqNonce, "abcdef123456")
	req.Header.Set(defaultReqFiber.ReqSign, "deadbeef")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestSignResponseFiberDynamic_Enabled(t *testing.T) {
	dynCfg := NewDynamicServerSignConfig(ServerSignConfig{
		Method: "hmac-sha256", Salt: "fdyn-rs", AppID: "fdyn-srv",
	})

	app := fiber.New()
	app.Use(SignResponseFiberDynamic(dynCfg, defaultReqFiber))
	app.Get("/api", func(c *fiber.Ctx) error {
		return c.SendString(`{"ok":true}`)
	})

	req, _ := http.NewRequest("GET", "/api", nil)
	req.Host = "localhost"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	if resp.Header.Get(defaultReqFiber.ResApp) != "fdyn-srv" {
		t.Errorf("expected fdyn-srv, got %q", resp.Header.Get(defaultReqFiber.ResApp))
	}
	if resp.Header.Get(defaultReqFiber.ResSign) == "" {
		t.Error("expected x-res-sign header")
	}
}

func TestSignResponseFiberDynamic_Disabled(t *testing.T) {
	dynCfg := NewDynamicServerSignConfig(ServerSignConfig{})

	app := fiber.New()
	app.Use(SignResponseFiberDynamic(dynCfg, defaultReqFiber))
	app.Get("/api", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req, _ := http.NewRequest("GET", "/api", nil)
	req.Host = "localhost"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	if resp.Header.Get(defaultReqFiber.ResApp) != "" {
		t.Error("should not set headers when disabled")
	}
}

func TestSignResponseFiberDynamic_SkipPaths(t *testing.T) {
	dynCfg := NewDynamicServerSignConfig(ServerSignConfig{
		Method:    "hmac-sha256",
		Salt:      "fdyn-rs",
		AppID:     "fdyn-srv",
		SkipPaths: []string{"/health"},
	})

	app := fiber.New()
	app.Use(SignResponseFiberDynamic(dynCfg, defaultReqFiber))
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("healthy")
	})

	req, _ := http.NewRequest("GET", "/health", nil)
	req.Host = "localhost"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	if resp.Header.Get(defaultReqFiber.ResApp) != "" {
		t.Error("should not set headers for skipped path")
	}
}
