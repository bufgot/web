package context

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bufgot/web"
	"github.com/bufgot/web/middleware/sign"
)

// ============================================================================
// Hot-reload assembly tests: default extract uses the conventional "http" key
// via mapstructure, so no concrete app Config type is required. The fake app
// struct below is intentionally NOT the web SignConfig — it merely stores its
// HTTP section under mapstructure:"http".
// ============================================================================

type fakeWebApp struct {
	Env  string     `mapstructure:"env"`
	HTTP SignConfig `mapstructure:"http"`
}

func fakeOnCfg() SignConfig {
	return SignConfig{
		Request: RequestSignConfig{
			Verifications: []VerifyEntry{
				{AppID: "app-a", Method: "md5", Salt: "salt-a"},
			},
		},
		Response: ResponseSignConfig{
			Sign: SignEntry{Enable: true, Method: "md5", PrivateKey: "resp-salt", AppID: "orion"},
		},
	}
}

func TestSignHotReload_RegistersOnChange(t *testing.T) {
	var registered func(any)
	hs := NewSignHotReload(SignConfig{}, func(cb func(any)) { registered = cb })
	if registered == nil {
		t.Fatal("expected bridge.OnChange to be registered with onChangeRegistrar")
	}
	if hs.Bridge == nil {
		t.Fatal("expected bridge to be built")
	}
	if hs.Middleware == nil {
		t.Fatal("expected middleware to be assembled")
	}
	if hs.ReqVerify == nil || hs.RespSign == nil {
		t.Fatal("expected dynamic stores to be built")
	}
}

func TestSignHotReload_DefaultExtract_StructPush_OnOff(t *testing.T) {
	hs := NewSignHotReload(SignConfig{}, func(cb func(any)) {})

	// startup snapshot: everything off
	if hs.ReqVerify.Load().Enable {
		t.Fatal("request verification should be off at startup")
	}
	if hs.RespSign.Load().Enabled() {
		t.Fatal("response signing should be off at startup")
	}

	// push a full config as an app struct (not web.SignConfig) -> request verify
	// enabled via len(Verifications)>0, response sign enabled
	hs.Bridge.OnChange(&fakeWebApp{Env: "prod", HTTP: fakeOnCfg()})

	rv := hs.ReqVerify.Load()
	if !rv.Enable {
		t.Fatal("request verification should be enabled after push")
	}
	if v := rv.LookupApp("app-a"); v == nil || v.Method != "md5" || v.Salt != "salt-a" {
		t.Fatalf("expected verification entry for app-a, got %+v", v)
	}
	rs := hs.RespSign.Load()
	if !rs.Enabled() || rs.Method != "md5" || rs.AppID != "orion" {
		t.Fatalf("response sign should be enabled with method/app, got %+v", rs)
	}

	// push full config with everything off -> both disabled again
	hs.Bridge.OnChange(&fakeWebApp{Env: "dev", HTTP: SignConfig{}})
	if hs.ReqVerify.Load().Enable {
		t.Fatal("request verification should be disabled after off push")
	}
	if hs.RespSign.Load().Enabled() {
		t.Fatal("response signing should be disabled after off push")
	}
}

func TestSignHotReload_DefaultExtract_MapPush(t *testing.T) {
	hs := NewSignHotReload(SignConfig{}, func(cb func(any)) {})

	on := map[string]any{
		"http": map[string]any{
			"request": map[string]any{
				"verifications": []any{
					map[string]any{"appid": "app-m", "method": "hmac-sha256", "salt": "s-m"},
				},
			},
			"response": map[string]any{
				"sign": map[string]any{"enable": true, "method": "hmac-sha256", "private_key": "k", "appid": "srv-m"},
			},
		},
	}
	hs.Bridge.OnChange(on)

	rv := hs.ReqVerify.Load()
	if !rv.Enable {
		t.Fatal("request verification should be enabled after map push")
	}
	if v := rv.LookupApp("app-m"); v == nil || v.Salt != "s-m" {
		t.Fatalf("expected app-m from map, got %+v", v)
	}
	rs := hs.RespSign.Load()
	if !rs.Enabled() || rs.AppID != "srv-m" {
		t.Fatalf("response sign should be enabled from map, got %+v", rs)
	}

	hs.Bridge.OnChange(map[string]any{"http": map[string]any{}})
	if hs.ReqVerify.Load().Enable || hs.RespSign.Load().Enabled() {
		t.Fatal("both should be disabled after empty map push")
	}
}

func TestSignHotReload_DefaultExtract_FallbackOnBadPayload(t *testing.T) {
	// initial snapshot: request verify on (via verifications), response off
	hs := NewSignHotReload(SignConfig{
		Request: RequestSignConfig{
			Verifications: []VerifyEntry{{AppID: "seed", Method: "md5", Salt: "seed-salt"}},
		},
	}, func(cb func(any)) {})

	// bad payload (wrong type) must not panic and must keep the snapshot
	hs.Bridge.OnChange("not-a-config")
	hs.Bridge.OnChange(nil)

	rv := hs.ReqVerify.Load()
	if !rv.Enable || rv.LookupApp("seed") == nil {
		t.Fatalf("should fall back to initial snapshot on bad payload, got %+v", rv)
	}
	if hs.RespSign.Load().Enabled() {
		t.Fatal("response sign should stay disabled (initial snapshot)")
	}
}

func TestSignHotReload_MiddlewareTogglesWithPush(t *testing.T) {
	hs := NewSignHotReload(SignConfig{}, func(cb func(any)) {})
	mw := hs.Middleware
	handlerBody := []byte(`{"ok":true}`)

	// --- startup: off -> request without signature passes, response carries no sign
	rec := runStub(mw, "POST", "/api", handlerBody)
	if rec.Code != 200 {
		t.Fatalf("expected pass-through when off, got %d", rec.Code)
	}
	if rec.Header().Get(sign.DefaultHeaderNames.ResSign) != "" {
		t.Fatal("no response sign header expected when off")
	}

	// --- push: request verify off, response sign on -> passes unsigned, body signed
	hs.Bridge.OnChange(&fakeWebApp{HTTP: SignConfig{
		Response: ResponseSignConfig{
			Sign: SignEntry{Enable: true, Method: "md5", PrivateKey: "resp-salt", AppID: "orion"},
		},
	}})
	rec = runStub(mw, "POST", "/api", handlerBody)
	if rec.Code != 200 {
		t.Fatalf("expected 200 with request verify off, got %d", rec.Code)
	}
	if rec.Header().Get(sign.DefaultHeaderNames.ResSign) == "" {
		t.Fatal("expected response sign header after enable push")
	}

	// --- push: request verify on -> unsigned request rejected
	hs.Bridge.OnChange(&fakeWebApp{HTTP: fakeOnCfg()})
	rec = runStub(mw, "POST", "/api", handlerBody)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when verification enabled but signature missing, got %d", rec.Code)
	}

	// --- push: everything off -> unsigned passes again, no response sign
	hs.Bridge.OnChange(&fakeWebApp{HTTP: SignConfig{}})
	rec = runStub(mw, "POST", "/api", handlerBody)
	if rec.Code != 200 {
		t.Fatalf("expected 200 after off push, got %d", rec.Code)
	}
	if rec.Header().Get(sign.DefaultHeaderNames.ResSign) != "" {
		t.Fatal("no response sign header expected after off push")
	}
}

func TestSignHotReload_OneLinerReturnsMiddleware(t *testing.T) {
	mw := SignHotReload(SignConfig{}, func(cb func(any)) {})
	if mw == nil {
		t.Fatal("one-liner SignHotReload should return assembled middleware")
	}
	rec := runStub(mw, "GET", "/ping", nil)
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

// ============================================================================
// test harness
// ============================================================================

func runStub(mw web.Middleware, method, path string, body []byte) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	rec := httptest.NewRecorder()
	h := mw(func(ctx web.Context) error {
		ctx.ResponseWriter().Header().Set("Content-Type", "application/json")
		ctx.ResponseWriter().WriteHeader(200)
		_, _ = ctx.ResponseWriter().Write(body)
		return nil
	})
	_ = h(&stubCtx{req: req, w: rec})
	return rec
}

type stubCtx struct {
	req *http.Request
	w   http.ResponseWriter
}

func (c *stubCtx) Request() *http.Request             { return c.req }
func (c *stubCtx) Method() string                     { return c.req.Method }
func (c *stubCtx) Path() string                       { return c.req.URL.Path }
func (c *stubCtx) QueryParam(name string) string      { return c.req.URL.Query().Get(name) }
func (c *stubCtx) Param(name string) string           { return "" }
func (c *stubCtx) FormValue(key string) string        { return "" }
func (c *stubCtx) PostForm(key string) string         { return "" }
func (c *stubCtx) ParseForm() error                   { return nil }
func (c *stubCtx) ParseMultipartForm(max int64) error { return nil }
func (c *stubCtx) Status(code int)                    { c.w.WriteHeader(code) }
func (c *stubCtx) JSON(code int, obj interface{}) error {
	c.w.Header().Set("Content-Type", "application/json")
	c.w.WriteHeader(code)
	data, _ := json.Marshal(obj)
	_, _ = c.w.Write(data)
	return nil
}
func (c *stubCtx) XML(code int, obj interface{}) error { return nil }
func (c *stubCtx) Text(code int, text string) error {
	c.w.WriteHeader(code)
	_, _ = c.w.Write([]byte(text))
	return nil
}
func (c *stubCtx) HTML(code int, html string) error        { return nil }
func (c *stubCtx) Redirect(code int, url string) error     { return nil }
func (c *stubCtx) Cookie(name string) (string, error)      { return "", nil }
func (c *stubCtx) SetCookie(cookie *http.Cookie)           {}
func (c *stubCtx) Logger() web.Logger                      { return nil }
func (c *stubCtx) BindJSON(obj interface{}) error          { return nil }
func (c *stubCtx) BindXML(obj interface{}) error           { return nil }
func (c *stubCtx) BindQuery(obj interface{}) error         { return nil }
func (c *stubCtx) Set(key string, value interface{})       {}
func (c *stubCtx) Get(key string) interface{}              { return nil }
func (c *stubCtx) Context() context.Context                { return c.req.Context() }
func (c *stubCtx) ResponseWriter() http.ResponseWriter     { return c.w }
func (c *stubCtx) SetResponseWriter(w http.ResponseWriter) { c.w = w }
