package context

import (
	"github.com/mitchellh/mapstructure"

	"github.com/bufgot/web"
	"github.com/bufgot/web/middleware/sign"
)

// ============================================================================
// Configuration types
// ============================================================================

// SignEntry configures signing for one direction (we sign our own messages).
type SignEntry struct {
	Enable     bool   `mapstructure:"enable"`
	AppID      string `mapstructure:"appid"`
	Method     string `mapstructure:"method"`
	PrivateKey string `mapstructure:"privateKey"` // ed25519 private key, or salt for md5/hmac
}

// VerifyEntry configures verification of a single caller's signatures.
type VerifyEntry struct {
	AppID     string `mapstructure:"appid"`
	Method    string `mapstructure:"method"`
	Salt      string `mapstructure:"salt"`      // shared secret for md5/hmac-sha256
	PublicKey string `mapstructure:"publicKey"` // ed25519 public key
}

// RequestSignConfig controls signing and verification for the request direction.
type RequestSignConfig struct {
	Sign          SignEntry     `mapstructure:"sign"`
	Verifications []VerifyEntry `mapstructure:"verifications"`
}

// ResponseSignConfig controls signing and verification for the response direction.
type ResponseSignConfig struct {
	Sign          SignEntry     `mapstructure:"sign"`
	Verifications []VerifyEntry `mapstructure:"verifications"`
}

// SignConfig is the top-level HTTP signing configuration.
type SignConfig struct {
	Request  RequestSignConfig  `mapstructure:"request"`
	Response ResponseSignConfig `mapstructure:"response"`
}

// DefaultSignConfig returns a SignConfig with sensible empty defaults.
func DefaultSignConfig() SignConfig {
	return SignConfig{
		Request: RequestSignConfig{
			Sign: SignEntry{
				Method: "md5",
			},
		},
		Response: ResponseSignConfig{
			Sign: SignEntry{
				Method: "md5",
			},
		},
	}
}

// ============================================================================
// Config conversion (shared by static and dynamic assembly)
// ============================================================================

// ConvertRequestVerifyConfig maps a RequestSignConfig into a sign.RequestVerifyConfig.
// The master Enable switch is derived from verifications list presence: when the
// list is empty, request verification is disabled regardless of cfg.Sign.Enable.
func ConvertRequestVerifyConfig(cfg RequestSignConfig) sign.RequestVerifyConfig {
	items := make([]sign.VerificationItem, 0, len(cfg.Verifications))
	for _, v := range cfg.Verifications {
		items = append(items, sign.VerificationItem{
			App:       v.AppID,
			Enable:    true,
			Method:    v.Method,
			Salt:      v.Salt,
			PublicKey: v.PublicKey,
		})
	}

	return sign.RequestVerifyConfig{
		Enable:        len(cfg.Verifications) > 0,
		Verifications: items,
		SkipPaths:     []string{"/healthz"},
	}
}

// ConvertResponseSignConfig maps a ResponseSignConfig into a sign.ServerSignConfig.
// The response sign switch is cfg.Sign.Enable; when disabled an empty config
// (no method) is returned so that sign.Enabled() yields false.
func ConvertResponseSignConfig(cfg ResponseSignConfig) sign.ServerSignConfig {
	if !cfg.Sign.Enable {
		return sign.ServerSignConfig{}
	}
	return sign.ServerSignConfig{
		Method:     cfg.Sign.Method,
		Salt:       cfg.Sign.PrivateKey, // md5: privateKey field holds the salt
		PrivateKey: cfg.Sign.PrivateKey, // ed25519: privateKey field holds the key
		AppID:      cfg.Sign.AppID,
	}
}

// ============================================================================
// Middleware
// ============================================================================

// SignMiddleware returns a combined HTTP server sign middleware:
//   - Verifies incoming request signatures (from Request.Verifications)
//   - Signs outgoing response signatures (from Response.Sign)
func SignMiddleware(cfg SignConfig) web.Middleware {
	verifyMW := buildRequestVerifyMW(cfg.Request)
	signMW := buildResponseSignMW(cfg.Response)

	return func(next web.Handler) web.Handler {
		return func(c web.Context) error {
			return verifyMW(signMW(next))(c)
		}
	}
}

func buildRequestVerifyMW(cfg RequestSignConfig) web.Middleware {
	verifyCfg := ConvertRequestVerifyConfig(cfg)
	if !verifyCfg.Enable {
		return func(next web.Handler) web.Handler { return next }
	}
	return sign.VerifyRequestMiddleware(verifyCfg, sign.DefaultHeaderNames)
}

func buildResponseSignMW(cfg ResponseSignConfig) web.Middleware {
	signCfg := ConvertResponseSignConfig(cfg)
	if !signCfg.Enabled() {
		return func(next web.Handler) web.Handler { return next }
	}
	return sign.SignResponseMiddleware(signCfg, sign.DefaultHeaderNames)
}

// ============================================================================
// Dynamic (hot-reload) middleware
// ============================================================================

// SignMiddlewareDynamic returns a combined HTTP server sign middleware that
// reads its config from sign.DynamicRequestVerifyConfig / sign.DynamicServerSignConfig
// on every request, enabling config hot-reload (e.g. Apollo push) without restart.
func SignMiddlewareDynamic(reqVerify *sign.DynamicRequestVerifyConfig, respSign *sign.DynamicServerSignConfig) web.Middleware {
	verifyMW := sign.VerifyRequestMiddlewareDynamic(reqVerify, sign.DefaultHeaderNames)
	signMW := sign.SignResponseMiddlewareDynamic(respSign, sign.DefaultHeaderNames)

	return func(next web.Handler) web.Handler {
		return func(c web.Context) error {
			return verifyMW(signMW(next))(c)
		}
	}
}

// NewSignConfigBridge assembles a sign.SignConfigBridge wired to the given
// dynamic stores. Register the returned bridge with config.App.OnChange; the
// extract functions convert the full config (e.g. *app.Config) into
// sign.RequestVerifyConfig / sign.ServerSignConfig on every notification.
func NewSignConfigBridge(
	reqVerify *sign.DynamicRequestVerifyConfig,
	respSign *sign.DynamicServerSignConfig,
	extractReq func(any) sign.RequestVerifyConfig,
	extractResp func(any) sign.ServerSignConfig,
) *sign.SignConfigBridge {
	return &sign.SignConfigBridge{
		ReqVerify:        reqVerify,
		RespSign:         respSign,
		ExtractReqVerify: extractReq,
		ExtractRespSign:  extractResp,
	}
}

// ============================================================================
// Hot-reload assembly (app-side one-liner)
// ============================================================================

// SignConfigKey is the conventional root mapstructure key under which a full
// config snapshot carries the HTTP sign section (webctx.SignConfig). The
// default extractor reads sign/verify settings from this key, so any app config
// struct that stores its HTTP section under mapstructure:"http" — or a flat map
// with an "http" key — is supported without knowing the concrete type.
const SignConfigKey = "http"

// SignHotReloadValue is the complete hot-reload sign assembly built by
// NewSignHotReload.
type SignHotReloadValue struct {
	// Middleware is the combined dynamic sign middleware (request verify +
	// response sign), reading config from the dynamic stores on every request.
	// Register it with the router, e.g. r.Use(hs.Middleware).
	Middleware web.Middleware

	// Bridge wires config.App.OnChange notifications into the dynamic stores.
	// NewSignHotReload registers bridge.OnChange automatically when an
	// onChangeRegistrar was provided; it is exposed here for tests and for
	// callers that prefer to drive hot-reload manually.
	Bridge *sign.SignConfigBridge

	// ReqVerify / RespSign expose the underlying dynamic stores for advanced
	// use and tests.
	ReqVerify *sign.DynamicRequestVerifyConfig
	RespSign  *sign.DynamicServerSignConfig
}

// NewSignHotReload assembles the full hot-reload sign stack out of an initial
// SignConfig snapshot, with no app-specific code required:
//
//   - creates the dynamic stores seeded with the initial snapshot
//   - wires a sign.SignConfigBridge whose default extractors locate the sign
//     section of any full config snapshot at the conventional SignConfigKey and
//     deserialize it via mapstructure, so no concrete *MyConfig type has to be
//     known inside the library
//   - when onChangeRegistrar is non-nil, registers bridge.OnChange with it
//     (e.g. config.App.OnChange) so remote pushes take effect on the next
//     request without a restart
//
// Apps that want a one-liner should use SignHotReload instead.
func NewSignHotReload(initial SignConfig, onChangeRegistrar func(cb func(any))) *SignHotReloadValue {
	reqVerify := sign.NewDynamicRequestVerifyConfig(ConvertRequestVerifyConfig(initial.Request))
	respSign := sign.NewDynamicServerSignConfig(ConvertResponseSignConfig(initial.Response))

	extractReq, extractResp := newDefaultSignExtractors(initial)
	bridge := NewSignConfigBridge(reqVerify, respSign, extractReq, extractResp)

	if onChangeRegistrar != nil {
		onChangeRegistrar(bridge.OnChange)
	}

	return &SignHotReloadValue{
		Middleware: SignMiddlewareDynamic(reqVerify, respSign),
		Bridge:     bridge,
		ReqVerify:  reqVerify,
		RespSign:   respSign,
	}
}

// SignHotReload is the one-liner convenient form of NewSignHotReload: it
// assembles the dynamic sign stack (stores, bridge, default extractors, OnChange
// registration) and returns the combined middleware. A router enables default
// sign/verify hot-reload with a single line:
//
//	r.Use(webctx.SignHotReload(cfg.HTTP, func(cb func(any)) { appCfg.OnChange(cb) }))
func SignHotReload(initial SignConfig, onChangeRegistrar func(cb func(any))) web.Middleware {
	return NewSignHotReload(initial, onChangeRegistrar).Middleware
}

// newDefaultSignExtractors builds the default full-config -> sign config
// extractors. Each extractor decodes the sign section out of the pushed
// snapshot and converts it to the middleware config; on any decode failure it
// falls back to the assembly-time snapshot so the stores never go stale.
func newDefaultSignExtractors(initial SignConfig) (func(any) sign.RequestVerifyConfig, func(any) sign.ServerSignConfig) {
	initReq := ConvertRequestVerifyConfig(initial.Request)
	initResp := ConvertResponseSignConfig(initial.Response)

	extractReq := func(full any) sign.RequestVerifyConfig {
		sc, ok := decodeSignSnapshot(full)
		if !ok {
			return initReq
		}
		return ConvertRequestVerifyConfig(sc.Request)
	}
	extractResp := func(full any) sign.ServerSignConfig {
		sc, ok := decodeSignSnapshot(full)
		if !ok {
			return initResp
		}
		return ConvertResponseSignConfig(sc.Response)
	}
	return extractReq, extractResp
}

// decodeSignSnapshot locates the HTTP sign section inside an arbitrary full
// config snapshot (struct or map) at the conventional SignConfigKey and
// deserializes it into a SignConfig via mapstructure. It reports ok=false when
// the snapshot is nil, unsupported, or carries no such section, letting callers
// fall back to their initial snapshot.
func decodeSignSnapshot(full any) (SignConfig, bool) {
	if full == nil {
		return SignConfig{}, false
	}
	var holder struct {
		HTTP *SignConfig `mapstructure:"http"`
	}
	if err := mapstructure.Decode(full, &holder); err != nil {
		return SignConfig{}, false
	}
	if holder.HTTP == nil {
		return SignConfig{}, false
	}
	return *holder.HTTP, true
}
