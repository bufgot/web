package sign

import "strings"

// ============================================================================
// Request direction: client sign config
// ============================================================================

// ClientSignConfig controls how the client signs outgoing requests.
// Configuration key: http.request.sign
type ClientSignConfig struct {
	// Method is the signing algorithm: "md5", "ed25519", or "hmac-sha256".
	Method string `mapstructure:"method" json:"method" yaml:"method"`

	// Salt is the shared secret for md5 and hmac-sha256.
	Salt string `mapstructure:"salt" json:"salt" yaml:"salt"`

	// PrivateKey is the Ed25519 private key (hex/base64, 64-byte seed).
	PrivateKey string `mapstructure:"privateKey" json:"privateKey" yaml:"privateKey"`

	// AppID is the calling application identifier (goes into request app header).
	AppID string `mapstructure:"app" json:"app" yaml:"app"`
}

func (c ClientSignConfig) toSignerOpts() SignerOpts {
	return SignerOpts{Salt: c.Salt, PrivateKey: c.PrivateKey}
}

// Enabled returns true when Method is set (config presence implies enablement, no extra switch).
func (c ClientSignConfig) Enabled() bool {
	return c.Method != ""
}

// ============================================================================
// Request direction: server verify config
// ============================================================================

// VerificationItem defines verification rules for one caller app.
type VerificationItem struct {
	// App is the caller identifier (matches request app header).
	App string `mapstructure:"app" json:"app" yaml:"app"`

	// Enable toggles verification for this app.
	Enable bool `mapstructure:"enable" json:"enable" yaml:"enable"`

	// Method is the signing algorithm: "md5", "ed25519", or "hmac-sha256".
	Method string `mapstructure:"method" json:"method" yaml:"method"`

	// Timeout is the allowed timestamp skew in milliseconds.
	Timeout int64 `mapstructure:"timeout" json:"timeout" yaml:"timeout"`

	// Salt is the shared secret for md5 and hmac-sha256.
	Salt string `mapstructure:"salt" json:"salt" yaml:"salt"`

	// PublicKey is the Ed25519 public key (hex/base64, 32 bytes).
	PublicKey string `mapstructure:"publicKey" json:"publicKey" yaml:"publicKey"`
}

// RequestVerifyConfig is the server-side request verification configuration.
// Configuration keys: http.request.verify (master switch) + http.request.verifications (app list).
type RequestVerifyConfig struct {
	// Enable is the master switch for request verification.
	Enable bool `mapstructure:"enable" json:"enable" yaml:"enable"`

	// Verifications is the per-app verification rules list.
	Verifications []VerificationItem `mapstructure:"verifications" json:"verifications" yaml:"verifications"`

	// SkipPaths is a list of path patterns to skip verification for.
	// Supports exact match and prefix match (path ending with "/").
	SkipPaths []string `mapstructure:"skipPaths" json:"skipPaths" yaml:"skipPaths"`
}

// LookupApp returns the VerificationItem for the given app, or nil if not found/disabled.
func (c RequestVerifyConfig) LookupApp(app string) *VerificationItem {
	for i := range c.Verifications {
		if c.Verifications[i].App == app && c.Verifications[i].Enable {
			return &c.Verifications[i]
		}
	}
	return nil
}

// ============================================================================
// Response direction: server sign config
// ============================================================================

// ServerSignConfig controls how the server signs outgoing responses.
// Configuration key: http.response.sign
type ServerSignConfig struct {
	// Method is the signing algorithm: "md5", "ed25519", or "hmac-sha256".
	Method string `mapstructure:"method" json:"method" yaml:"method"`

	// Salt is the shared secret for md5 and hmac-sha256.
	Salt string `mapstructure:"salt" json:"salt" yaml:"salt"`

	// PrivateKey is the Ed25519 private key (hex/base64, 64-byte seed).
	PrivateKey string `mapstructure:"privateKey" json:"privateKey" yaml:"privateKey"`

	// AppID is the server application identifier (goes into response app header).
	AppID string `mapstructure:"app" json:"app" yaml:"app"`

	// SkipPaths is a list of path patterns to skip signing for.
	// Supports exact match and prefix match (path ending with "/").
	SkipPaths []string `mapstructure:"skipPaths" json:"skipPaths" yaml:"skipPaths"`
}

func (c ServerSignConfig) toSignerOpts() SignerOpts {
	return SignerOpts{Salt: c.Salt, PrivateKey: c.PrivateKey}
}

// Enabled returns true when Method is set.
func (c ServerSignConfig) Enabled() bool {
	return c.Method != ""
}

// ============================================================================
// Response direction: client verify config
// ============================================================================

// ResponseVerifyConfig is the client-side response verification configuration.
// Configuration keys: http.response.verify (master switch) + http.response.verifications (app list).
type ResponseVerifyConfig struct {
	// Enable is the master switch for response verification.
	Enable bool `mapstructure:"enable" json:"enable" yaml:"enable"`

	// Verifications is the per-app verification rules list.
	Verifications []VerificationItem `mapstructure:"verifications" json:"verifications" yaml:"verifications"`
}

// LookupApp returns the VerificationItem for the given app, or nil if not found/disabled.
func (c ResponseVerifyConfig) LookupApp(app string) *VerificationItem {
	for i := range c.Verifications {
		if c.Verifications[i].App == app && c.Verifications[i].Enable {
			return &c.Verifications[i]
		}
	}
	return nil
}

// ============================================================================
// skip path matching
// ============================================================================

// shouldSkip returns true if path matches any entry in skipPaths.
// Matching rules:
//   - Exact match on the raw path.
//   - If a skip entry ends with "/", it is treated as a prefix match.
func shouldSkip(path string, skipPaths []string) bool {
	for _, sp := range skipPaths {
		if sp == path {
			return true
		}
		if strings.HasSuffix(sp, "/") && strings.HasPrefix(path, sp) {
			return true
		}
	}
	return false
}

// ============================================================================
// Defaults
// ============================================================================

// DefaultTimeoutMS is the default timestamp tolerance in milliseconds.
const DefaultTimeoutMS int64 = 5000

// DefaultNonceLength is the length of generated random nonce strings.
const DefaultNonceLength = 16
