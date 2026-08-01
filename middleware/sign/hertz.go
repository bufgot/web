package sign

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
)

// VerifyRequestHertz returns a hertz app.HandlerFunc that validates request headers.
func VerifyRequestHertz(cfg RequestVerifyConfig, hdr HeaderNames) app.HandlerFunc {
	if !cfg.Enable || len(cfg.Verifications) == 0 {
		return func(c context.Context, reqCtx *app.RequestContext) {
			reqCtx.Next(c)
		}
	}
	h := hdr.Effective()

	return func(c context.Context, reqCtx *app.RequestContext) {
		if shouldSkip(string(reqCtx.Path()), cfg.SkipPaths) {
			reqCtx.Next(c)
			return
		}

		appStr := reqCtx.Request.Header.Get(h.ReqApp)
		ts := reqCtx.Request.Header.Get(h.ReqTimestamp)
		nonce := reqCtx.Request.Header.Get(h.ReqNonce)
		sig := reqCtx.Request.Header.Get(h.ReqSign)

		if appStr == "" || ts == "" || nonce == "" || sig == "" {
			reqCtx.AbortWithStatusJSON(http.StatusUnauthorized, map[string]string{
				"error": "missing signature headers",
			})
			return
		}

		item := cfg.LookupApp(appStr)
		if item == nil {
			reqCtx.AbortWithStatusJSON(http.StatusUnauthorized, map[string]string{
				"error": fmt.Sprintf("unknown or disabled app %q", appStr),
			})
			return
		}

		verifier, err := NewVerifier(SignMethod(item.Method), SignerOpts{
			Salt:      item.Salt,
			PublicKey: item.PublicKey,
		})
		if err != nil {
			reqCtx.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("verifier init: %v", err),
			})
			return
		}

		if err := VerifyRequest(VerifyRequestParams{
			AppHeader:       appStr,
			TimestampHeader: ts,
			NonceHeader:     nonce,
			SignHeader:      sig,
			Verifier:        verifier,
			Path:            string(reqCtx.Path()),
			Body:            reqCtx.Request.Body(),
			TimeoutMS:       item.Timeout,
			HeaderNames:     hdr,
		}); err != nil {
			reqCtx.AbortWithStatusJSON(http.StatusUnauthorized, map[string]string{
				"error": err.Error(),
			})
			return
		}

		reqCtx.Next(c)
	}
}

// SignResponseHertz returns a hertz app.HandlerFunc that adds response headers.
func SignResponseHertz(cfg ServerSignConfig, hdr HeaderNames) app.HandlerFunc {
	if !cfg.Enabled() {
		return func(c context.Context, reqCtx *app.RequestContext) {
			reqCtx.Next(c)
		}
	}

	signer, err := NewSigner(SignMethod(cfg.Method), cfg.toSignerOpts())
	if err != nil {
		return func(c context.Context, reqCtx *app.RequestContext) {
			reqCtx.Next(c)
		}
	}
	h := hdr.Effective()

	return func(c context.Context, reqCtx *app.RequestContext) {
		if shouldSkip(string(reqCtx.Path()), cfg.SkipPaths) {
			reqCtx.Next(c)
			return
		}

		bodyLen := len(reqCtx.Response.Body())

		reqCtx.Next(c)

		respBody := reqCtx.Response.Body()[bodyLen:]
		statusCode := reqCtx.Response.StatusCode()

		ts := time.Now()
		headers, err := SignResponse(SignResponseParams{
			AppID:       cfg.AppID,
			Method:      SignMethod(cfg.Method),
			Signer:      signer,
			StatusCode:  statusCode,
			Body:        respBody,
			HeaderNames: hdr,
		}, ts)
		if err != nil {
			return
		}

		reqCtx.Response.Header.Set(h.ResApp, headers.App)
		reqCtx.Response.Header.Set(h.ResTimestamp, headers.Timestamp)
		reqCtx.Response.Header.Set(h.ResNonce, headers.Nonce)
		reqCtx.Response.Header.Set(h.ResSign, headers.Sign)
	}
}
