package context

import (
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
	Salt      string `mapstructure:"salt"`     // shared secret for md5/hmac-sha256
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
	if len(cfg.Verifications) == 0 {
		return func(next web.Handler) web.Handler { return next }
	}

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

	verifyCfg := sign.RequestVerifyConfig{
		Enable:        true,
		Verifications: items,
		SkipPaths:     []string{"/healthz"},
	}

	return sign.VerifyRequestMiddleware(verifyCfg, sign.DefaultHeaderNames)
}

func buildResponseSignMW(cfg ResponseSignConfig) web.Middleware {
	if !cfg.Sign.Enable {
		return func(next web.Handler) web.Handler { return next }
	}

	signCfg := sign.ServerSignConfig{
		Method:     cfg.Sign.Method,
		Salt:       cfg.Sign.PrivateKey, // md5: privateKey field holds the salt
		PrivateKey: cfg.Sign.PrivateKey, // ed25519: privateKey field holds the key
		AppID:      cfg.Sign.AppID,
	}

	return sign.SignResponseMiddleware(signCfg, sign.DefaultHeaderNames)
}
