package sign

import (
	"fmt"
	"net/http"
	"time"

	"github.com/valyala/fasthttp"
)

// VerifyRequestFasthttp returns a fasthttp middleware that validates request headers.
func VerifyRequestFasthttp(cfg RequestVerifyConfig, hdr HeaderNames) func(fasthttp.RequestHandler) fasthttp.RequestHandler {
	if !cfg.Enable || len(cfg.Verifications) == 0 {
		return func(next fasthttp.RequestHandler) fasthttp.RequestHandler { return next }
	}
	h := hdr.Effective()

	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			if shouldSkip(string(ctx.Path()), cfg.SkipPaths) {
				next(ctx)
				return
			}

			app := string(ctx.Request.Header.Peek(h.ReqApp))
			ts := string(ctx.Request.Header.Peek(h.ReqTimestamp))
			nonce := string(ctx.Request.Header.Peek(h.ReqNonce))
			sig := string(ctx.Request.Header.Peek(h.ReqSign))

			if app == "" || ts == "" || nonce == "" || sig == "" {
				ctx.SetStatusCode(http.StatusUnauthorized)
				ctx.SetContentType("application/json")
				ctx.WriteString(`{"error":"missing signature headers"}`)
				return
			}

			item := cfg.LookupApp(app)
			if item == nil {
				ctx.SetStatusCode(http.StatusUnauthorized)
				ctx.SetContentType("application/json")
				ctx.WriteString(fmt.Sprintf(`{"error":"unknown or disabled app %q"}`, app))
				return
			}

			verifier, err := NewVerifier(SignMethod(item.Method), SignerOpts{
				Salt:      item.Salt,
				PublicKey: item.PublicKey,
			})
			if err != nil {
				ctx.SetStatusCode(http.StatusInternalServerError)
				ctx.SetContentType("application/json")
				ctx.WriteString(fmt.Sprintf(`{"error":"verifier init: %v"}`, err))
				return
			}

			if err := VerifyRequest(VerifyRequestParams{
				AppHeader:       app,
				TimestampHeader: ts,
				NonceHeader:     nonce,
				SignHeader:      sig,
				Verifier:        verifier,
				Path:            string(ctx.Path()),
				Body:            ctx.Request.Body(),
				TimeoutMS:       item.Timeout,
				HeaderNames:     hdr,
			}); err != nil {
				ctx.SetStatusCode(http.StatusUnauthorized)
				ctx.SetContentType("application/json")
				ctx.WriteString(fmt.Sprintf(`{"error":"%s"}`, err.Error()))
				return
			}

			next(ctx)
		}
	}
}

// SignResponseFasthttp returns a fasthttp middleware that adds response headers.
func SignResponseFasthttp(cfg ServerSignConfig, hdr HeaderNames) func(fasthttp.RequestHandler) fasthttp.RequestHandler {
	if !cfg.Enabled() {
		return func(next fasthttp.RequestHandler) fasthttp.RequestHandler { return next }
	}

	signer, err := NewSigner(SignMethod(cfg.Method), cfg.toSignerOpts())
	if err != nil {
		return func(next fasthttp.RequestHandler) fasthttp.RequestHandler { return next }
	}
	h := hdr.Effective()

	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			if shouldSkip(string(ctx.Path()), cfg.SkipPaths) {
				next(ctx)
				return
			}

			bodyLen := len(ctx.Response.Body())

			next(ctx)

			respBody := ctx.Response.Body()[bodyLen:]
			statusCode := ctx.Response.StatusCode()

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

			ctx.Response.Header.Set(h.ResApp, headers.App)
			ctx.Response.Header.Set(h.ResTimestamp, headers.Timestamp)
			ctx.Response.Header.Set(h.ResNonce, headers.Nonce)
			ctx.Response.Header.Set(h.ResSign, headers.Sign)
		}
	}
}
