package sign

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"
	"time"

	web "github.com/bufgot/web"
)

// ============================================================================
// Dynamic Sign Config Wrappers (thread-safe hot-reload)
// ============================================================================

// DynamicRequestVerifyConfig wraps RequestVerifyConfig for thread-safe hot-reload.
type DynamicRequestVerifyConfig struct {
	mu  sync.RWMutex
	cfg RequestVerifyConfig
}

// NewDynamicRequestVerifyConfig creates a DynamicRequestVerifyConfig with initial config.
func NewDynamicRequestVerifyConfig(cfg RequestVerifyConfig) *DynamicRequestVerifyConfig {
	return &DynamicRequestVerifyConfig{cfg: cfg}
}

// Load returns the current config (thread-safe).
func (d *DynamicRequestVerifyConfig) Load() RequestVerifyConfig {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.cfg
}

// Store replaces the current config (thread-safe).
func (d *DynamicRequestVerifyConfig) Store(cfg RequestVerifyConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cfg = cfg
}

// DynamicServerSignConfig wraps ServerSignConfig for thread-safe hot-reload.
type DynamicServerSignConfig struct {
	mu  sync.RWMutex
	cfg ServerSignConfig
}

// NewDynamicServerSignConfig creates a DynamicServerSignConfig with initial config.
func NewDynamicServerSignConfig(cfg ServerSignConfig) *DynamicServerSignConfig {
	return &DynamicServerSignConfig{cfg: cfg}
}

// Load returns the current config (thread-safe).
func (d *DynamicServerSignConfig) Load() ServerSignConfig {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.cfg
}

// Store replaces the current config (thread-safe).
func (d *DynamicServerSignConfig) Store(cfg ServerSignConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cfg = cfg
}

// DynamicClientSignConfig wraps ClientSignConfig for thread-safe hot-reload.
type DynamicClientSignConfig struct {
	mu  sync.RWMutex
	cfg ClientSignConfig
}

// NewDynamicClientSignConfig creates a DynamicClientSignConfig with initial config.
func NewDynamicClientSignConfig(cfg ClientSignConfig) *DynamicClientSignConfig {
	return &DynamicClientSignConfig{cfg: cfg}
}

// Load returns the current config (thread-safe).
func (d *DynamicClientSignConfig) Load() ClientSignConfig {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.cfg
}

// Store replaces the current config (thread-safe).
func (d *DynamicClientSignConfig) Store(cfg ClientSignConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cfg = cfg
}

// DynamicResponseVerifyConfig wraps ResponseVerifyConfig for thread-safe hot-reload.
type DynamicResponseVerifyConfig struct {
	mu  sync.RWMutex
	cfg ResponseVerifyConfig
}

// NewDynamicResponseVerifyConfig creates a DynamicResponseVerifyConfig with initial config.
func NewDynamicResponseVerifyConfig(cfg ResponseVerifyConfig) *DynamicResponseVerifyConfig {
	return &DynamicResponseVerifyConfig{cfg: cfg}
}

// Load returns the current config (thread-safe).
func (d *DynamicResponseVerifyConfig) Load() ResponseVerifyConfig {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.cfg
}

// Store replaces the current config (thread-safe).
func (d *DynamicResponseVerifyConfig) Store(cfg ResponseVerifyConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cfg = cfg
}

// ============================================================================
// ConfigBridge — bridges bufgot/config OnChange to dynamic sign config
// ============================================================================

// SignConfigBridge wires bufgot/config's OnChange callback to dynamic sign config stores.
//
// Usage:
//
//	bridge := &sign.SignConfigBridge{
//	    ReqVerify: sign.NewDynamicRequestVerifyConfig(initialReqVerify),
//	    RespSign:  sign.NewDynamicServerSignConfig(initialRespSign),
//	    ExtractReqVerify: func(cfg any) sign.RequestVerifyConfig {
//	        return cfg.(*MyAppConfig).HTTP.Request.Verify
//	    },
//	    ExtractRespSign: func(cfg any) sign.ServerSignConfig {
//	        return cfg.(*MyAppConfig).HTTP.Response.Sign
//	    },
//	}
//	app.OnChange(bridge.OnChange)
//
// Key design: when extractor or dynamic target is nil, that direction is silently skipped,
// so you only configure the directions your app actually uses.
type SignConfigBridge struct {
	ReqVerify  *DynamicRequestVerifyConfig
	RespSign   *DynamicServerSignConfig
	ClientSign *DynamicClientSignConfig
	RespVerify *DynamicResponseVerifyConfig

	ExtractReqVerify  func(fullCfg any) RequestVerifyConfig
	ExtractRespSign   func(fullCfg any) ServerSignConfig
	ExtractClientSign func(fullCfg any) ClientSignConfig
	ExtractRespVerify func(fullCfg any) ResponseVerifyConfig
}

// OnChange implements the config change callback. Register this with config.App.OnChange().
func (b *SignConfigBridge) OnChange(fullCfg any) {
	if b.ReqVerify != nil && b.ExtractReqVerify != nil {
		b.ReqVerify.Store(b.ExtractReqVerify(fullCfg))
	}
	if b.RespSign != nil && b.ExtractRespSign != nil {
		b.RespSign.Store(b.ExtractRespSign(fullCfg))
	}
	if b.ClientSign != nil && b.ExtractClientSign != nil {
		b.ClientSign.Store(b.ExtractClientSign(fullCfg))
	}
	if b.RespVerify != nil && b.ExtractRespVerify != nil {
		b.RespVerify.Store(b.ExtractRespVerify(fullCfg))
	}
}

// ============================================================================
// Dynamic Middleware — net/http (chi / echo / gin)
// ============================================================================

// VerifyRequestMiddlewareDynamic is like VerifyRequestMiddleware but reads config
// from a DynamicRequestVerifyConfig on every request, enabling hot-reload.
func VerifyRequestMiddlewareDynamic(dynCfg *DynamicRequestVerifyConfig, hdr HeaderNames) web.Middleware {
	h := hdr.Effective()

	return func(next web.Handler) web.Handler {
		return func(ctx web.Context) error {
			cfg := dynCfg.Load()
			if !cfg.Enable || len(cfg.Verifications) == 0 {
				return next(ctx)
			}

			req := ctx.Request()
			if req == nil {
				return fmt.Errorf("sign: nil request")
			}

			if shouldSkip(req.URL.Path, cfg.SkipPaths) {
				return next(ctx)
			}

			app := req.Header.Get(h.ReqApp)
			ts := req.Header.Get(h.ReqTimestamp)
			nonce := req.Header.Get(h.ReqNonce)
			sig := req.Header.Get(h.ReqSign)

			if app == "" || ts == "" || nonce == "" || sig == "" {
				ctx.Status(http.StatusUnauthorized)
				return ctx.JSON(http.StatusUnauthorized, map[string]string{
					"error": "missing signature headers",
				})
			}

			item := cfg.LookupApp(app)
			if item == nil {
				ctx.Status(http.StatusUnauthorized)
				return ctx.JSON(http.StatusUnauthorized, map[string]string{
					"error": fmt.Sprintf("unknown or disabled app %q", app),
				})
			}

			verifier, err := NewVerifier(SignMethod(item.Method), SignerOpts{
				Salt:      item.Salt,
				PublicKey: item.PublicKey,
			})
			if err != nil {
				ctx.Status(http.StatusInternalServerError)
				return ctx.JSON(http.StatusInternalServerError, map[string]string{
					"error": fmt.Sprintf("verifier init: %v", err),
				})
			}

			body, err := readAndRestoreBody(req)
			if err != nil {
				ctx.Status(http.StatusInternalServerError)
				return ctx.JSON(http.StatusInternalServerError, map[string]string{
					"error": "failed to read request body",
				})
			}

			if err := VerifyRequest(VerifyRequestParams{
				AppHeader:       app,
				TimestampHeader: ts,
				NonceHeader:     nonce,
				SignHeader:      sig,
				Verifier:        verifier,
				Path:            req.URL.Path,
				Body:            body,
				TimeoutMS:       item.Timeout,
				HeaderNames:     hdr,
			}); err != nil {
				ctx.Status(http.StatusUnauthorized)
				return ctx.JSON(http.StatusUnauthorized, map[string]string{
					"error": err.Error(),
				})
			}

			return next(ctx)
		}
	}
}

// SignResponseMiddlewareDynamic is like SignResponseMiddleware but reads config
// from a DynamicServerSignConfig on every request, enabling hot-reload.
func SignResponseMiddlewareDynamic(dynCfg *DynamicServerSignConfig, hdr HeaderNames) web.Middleware {
	h := hdr.Effective()

	return func(next web.Handler) web.Handler {
		return func(ctx web.Context) error {
			cfg := dynCfg.Load()
			if !cfg.Enabled() {
				return next(ctx)
			}

			signer, err := NewSigner(SignMethod(cfg.Method), cfg.toSignerOpts())
			if err != nil {
				return next(ctx)
			}

			if shouldSkip(ctx.Path(), cfg.SkipPaths) {
				return next(ctx)
			}

			w := ctx.ResponseWriter()
			if w == nil {
				return next(ctx)
			}

			rw := &responseRecorder{ResponseWriter: w, buf: &bytes.Buffer{}}
			type writerSetter interface {
				SetResponseWriter(http.ResponseWriter)
			}
			if ws, ok := ctx.(writerSetter); ok {
				ws.SetResponseWriter(rw)
			}

			nextErr := next(ctx)

			if ws, ok := ctx.(writerSetter); ok {
				ws.SetResponseWriter(w)
			}

			body := rw.buf.Bytes()
			statusCode := rw.statusCode
			if statusCode == 0 {
				statusCode = http.StatusOK
			}

			ts := time.Now()
			headers, signErr := SignResponse(SignResponseParams{
				AppID:       cfg.AppID,
				Method:      SignMethod(cfg.Method),
				Signer:      signer,
				StatusCode:  statusCode,
				Body:        body,
				HeaderNames: hdr,
			}, ts)
			if signErr != nil {
				return nextErr
			}

			w.Header().Set(h.ResApp, headers.App)
			w.Header().Set(h.ResTimestamp, headers.Timestamp)
			w.Header().Set(h.ResNonce, headers.Nonce)
			w.Header().Set(h.ResSign, headers.Sign)

			return nextErr
		}
	}
}
