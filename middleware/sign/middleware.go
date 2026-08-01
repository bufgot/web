package sign

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	web "github.com/bufgot/web"
)

// VerifyRequestMiddleware returns a web.Middleware that validates request headers on incoming requests.
//
// Works with net/http-based engines (chi, echo, gin).
func VerifyRequestMiddleware(cfg RequestVerifyConfig, hdr HeaderNames) web.Middleware {
	if !cfg.Enable || len(cfg.Verifications) == 0 {
		return func(next web.Handler) web.Handler { return next }
	}
	h := hdr.Effective()

	return func(next web.Handler) web.Handler {
		return func(ctx web.Context) error {
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

// SignResponseMiddleware returns a web.Middleware that adds response headers to outgoing responses.
//
// Works with net/http-based engines (chi, echo, gin).
// Uses ctx.SetResponseWriter to wrap the ResponseWriter and capture
// the response body written by downstream handlers (e.g. gin c.JSON).
func SignResponseMiddleware(cfg ServerSignConfig, hdr HeaderNames) web.Middleware {
	if !cfg.Enabled() {
		return func(next web.Handler) web.Handler { return next }
	}

	signer, err := NewSigner(SignMethod(cfg.Method), cfg.toSignerOpts())
	if err != nil {
		return func(next web.Handler) web.Handler { return next }
	}
	h := hdr.Effective()

	return func(next web.Handler) web.Handler {
		return func(ctx web.Context) error {
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

			// Restore original writer (best-effort).
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

// readAndRestoreBody reads the entire request body and restores it for downstream handlers.
func readAndRestoreBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

// responseRecorder wraps http.ResponseWriter to capture the status code and body.
type responseRecorder struct {
	http.ResponseWriter
	buf        *bytes.Buffer
	statusCode int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.buf.Write(b)
	return r.ResponseWriter.Write(b)
}

// Unwrap returns the underlying ResponseWriter.
func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
