// Package sign provides request/response signing and verification.
//
// Four directions are supported:
//   - Client request signing  → SignRequest
//   - Server request verification → VerifyRequest
//   - Server response signing → SignResponse
//   - Client response verification → VerifyResponse
//
// Supported algorithms: md5, ed25519, hmac-sha256 (configurable per direction).
//
// Header format (request):  x-req-app / x-req-timestamp / x-req-nonce / x-req-sign
// Header format (response): x-res-app / x-res-timestamp / x-res-nonce / x-res-sign
//
// Signature payload (request):
//
//	app + timestamp + nonce + reqPath + base64(reqBody)
//
// Signature payload (response):
//
//	app + timestamp + nonce + strconv.Itoa(statusCode) + base64(respBody)
package sign

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"time"
)

// HeaderNames holds configurable header key names (request and response).
type HeaderNames struct {
	ReqApp       string
	ReqTimestamp string
	ReqNonce     string
	ReqSign      string
	ResApp       string
	ResTimestamp string
	ResNonce     string
	ResSign      string
}

// DefaultHeaderNames is the default set of header keys.
var DefaultHeaderNames = HeaderNames{
	ReqApp:       "x-req-app",
	ReqTimestamp: "x-req-timestamp",
	ReqNonce:     "x-req-nonce",
	ReqSign:      "x-req-sign",
	ResApp:       "x-res-app",
	ResTimestamp: "x-res-timestamp",
	ResNonce:     "x-res-nonce",
	ResSign:      "x-res-sign",
}

// Effective merges zero-value fields with DefaultHeaderNames.
func (h HeaderNames) Effective() HeaderNames {
	out := h
	if out.ReqApp == "" {
		out.ReqApp = DefaultHeaderNames.ReqApp
	}
	if out.ReqTimestamp == "" {
		out.ReqTimestamp = DefaultHeaderNames.ReqTimestamp
	}
	if out.ReqNonce == "" {
		out.ReqNonce = DefaultHeaderNames.ReqNonce
	}
	if out.ReqSign == "" {
		out.ReqSign = DefaultHeaderNames.ReqSign
	}
	if out.ResApp == "" {
		out.ResApp = DefaultHeaderNames.ResApp
	}
	if out.ResTimestamp == "" {
		out.ResTimestamp = DefaultHeaderNames.ResTimestamp
	}
	if out.ResNonce == "" {
		out.ResNonce = DefaultHeaderNames.ResNonce
	}
	if out.ResSign == "" {
		out.ResSign = DefaultHeaderNames.ResSign
	}
	return out
}

// SignRequestParams holds inputs for signing an outgoing request.
type SignRequestParams struct {
	AppID       string
	Method      SignMethod
	Signer      Signer
	Path        string
	Body        []byte
	HeaderNames HeaderNames
}

// SignedRequestHeaders contains the 4 headers to inject into the outgoing request.
type SignedRequestHeaders struct {
	App       string
	Timestamp string
	Nonce     string
	Sign      string
}

// SignRequest computes the 4 header values for an outgoing request.
// ts is the clock value used for the timestamp header and payload.
func SignRequest(params SignRequestParams, ts time.Time) (SignedRequestHeaders, error) {
	if params.AppID == "" {
		return SignedRequestHeaders{}, fmt.Errorf("sign: AppID is required")
	}
	if params.Signer == nil {
		return SignedRequestHeaders{}, fmt.Errorf("sign: Signer is nil")
	}

	timestamp := strconv.FormatInt(ts.UnixMilli(), 10)
	nonce, err := generateNonce(DefaultNonceLength)
	if err != nil {
		return SignedRequestHeaders{}, fmt.Errorf("sign: generate nonce: %w", err)
	}

	payload := params.AppID + timestamp + nonce + params.Path + base64.StdEncoding.EncodeToString(params.Body)
	sigBytes, err := params.Signer.Sign([]byte(payload))
	if err != nil {
		return SignedRequestHeaders{}, fmt.Errorf("sign: %w", err)
	}

	return SignedRequestHeaders{
		App:       params.AppID,
		Timestamp: timestamp,
		Nonce:     nonce,
		Sign:      string(sigBytes),
	}, nil
}

// VerifyRequestParams holds inputs for verifying an incoming request.
type VerifyRequestParams struct {
	AppHeader       string
	TimestampHeader string
	NonceHeader     string
	SignHeader      string

	Verifier    Signer
	Path        string
	Body        []byte
	TimeoutMS   int64 // allowed clock skew in milliseconds (0 = use default)
	HeaderNames HeaderNames
}

// VerifyRequest validates the 4 request headers.
func VerifyRequest(params VerifyRequestParams) error {
	h := params.HeaderNames.Effective()

	if params.AppHeader == "" {
		return fmt.Errorf("sign: missing %s header", h.ReqApp)
	}
	if params.TimestampHeader == "" {
		return fmt.Errorf("sign: missing %s header", h.ReqTimestamp)
	}
	if params.NonceHeader == "" {
		return fmt.Errorf("sign: missing %s header", h.ReqNonce)
	}
	if params.SignHeader == "" {
		return fmt.Errorf("sign: missing %s header", h.ReqSign)
	}
	if params.Verifier == nil {
		return fmt.Errorf("sign: Verifier is nil")
	}

	// Timestamp freshness check.
	tsMS, err := strconv.ParseInt(params.TimestampHeader, 10, 64)
	if err != nil {
		return fmt.Errorf("sign: invalid %s: %w", h.ReqTimestamp, err)
	}
	timeout := params.TimeoutMS
	if timeout <= 0 {
		timeout = DefaultTimeoutMS
	}
	nowMS := time.Now().UnixMilli()
	if absDiff(nowMS, tsMS) > timeout {
		return fmt.Errorf("sign: timestamp expired (now=%d, ts=%d, timeout=%dms)", nowMS, tsMS, timeout)
	}

	// Rebuild payload and verify.
	payload := params.AppHeader + params.TimestampHeader + params.NonceHeader + params.Path + base64.StdEncoding.EncodeToString(params.Body)
	if !params.Verifier.Verify([]byte(payload), []byte(params.SignHeader)) {
		return fmt.Errorf("sign: signature mismatch for app %q", params.AppHeader)
	}

	return nil
}

// --- Response Direction ---

// SignResponseParams holds inputs for signing an outgoing response.
type SignResponseParams struct {
	AppID       string
	Method      SignMethod
	Signer      Signer
	StatusCode  int
	Body        []byte
	HeaderNames HeaderNames
}

// SignedResponseHeaders contains the 4 headers to inject into the outgoing response.
type SignedResponseHeaders struct {
	App       string
	Timestamp string
	Nonce     string
	Sign      string
}

// SignResponse computes the 4 header values for an outgoing response.
func SignResponse(params SignResponseParams, ts time.Time) (SignedResponseHeaders, error) {
	if params.AppID == "" {
		return SignedResponseHeaders{}, fmt.Errorf("sign: AppID is required")
	}
	if params.Signer == nil {
		return SignedResponseHeaders{}, fmt.Errorf("sign: Signer is nil")
	}

	timestamp := strconv.FormatInt(ts.UnixMilli(), 10)
	nonce, err := generateNonce(DefaultNonceLength)
	if err != nil {
		return SignedResponseHeaders{}, fmt.Errorf("sign: generate nonce: %w", err)
	}

	payload := params.AppID + timestamp + nonce + strconv.Itoa(params.StatusCode) + base64.StdEncoding.EncodeToString(params.Body)
	sigBytes, err := params.Signer.Sign([]byte(payload))
	if err != nil {
		return SignedResponseHeaders{}, fmt.Errorf("sign: %w", err)
	}

	return SignedResponseHeaders{
		App:       params.AppID,
		Timestamp: timestamp,
		Nonce:     nonce,
		Sign:      string(sigBytes),
	}, nil
}

// VerifyResponseParams holds inputs for verifying an incoming response.
type VerifyResponseParams struct {
	AppHeader       string
	TimestampHeader string
	NonceHeader     string
	SignHeader      string

	Verifier    Signer
	StatusCode  int
	Body        []byte
	TimeoutMS   int64
	HeaderNames HeaderNames
}

// VerifyResponse validates the 4 response headers.
func VerifyResponse(params VerifyResponseParams) error {
	h := params.HeaderNames.Effective()

	if params.AppHeader == "" {
		return fmt.Errorf("sign: missing %s header", h.ResApp)
	}
	if params.TimestampHeader == "" {
		return fmt.Errorf("sign: missing %s header", h.ResTimestamp)
	}
	if params.NonceHeader == "" {
		return fmt.Errorf("sign: missing %s header", h.ResNonce)
	}
	if params.SignHeader == "" {
		return fmt.Errorf("sign: missing %s header", h.ResSign)
	}
	if params.Verifier == nil {
		return fmt.Errorf("sign: Verifier is nil")
	}

	tsMS, err := strconv.ParseInt(params.TimestampHeader, 10, 64)
	if err != nil {
		return fmt.Errorf("sign: invalid %s: %w", h.ResTimestamp, err)
	}
	timeout := params.TimeoutMS
	if timeout <= 0 {
		timeout = DefaultTimeoutMS
	}
	nowMS := time.Now().UnixMilli()
	if absDiff(nowMS, tsMS) > timeout {
		return fmt.Errorf("sign: timestamp expired (now=%d, ts=%d, timeout=%dms)", nowMS, tsMS, timeout)
	}

	payload := params.AppHeader + params.TimestampHeader + params.NonceHeader + strconv.Itoa(params.StatusCode) + base64.StdEncoding.EncodeToString(params.Body)
	if !params.Verifier.Verify([]byte(payload), []byte(params.SignHeader)) {
		return fmt.Errorf("sign: signature mismatch for app %q", params.AppHeader)
	}

	return nil
}

func absDiff(a, b int64) int64 {
	if a > b {
		return a - b
	}
	return b - a
}
