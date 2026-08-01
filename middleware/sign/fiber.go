package sign

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
)

// VerifyRequestFiber returns a fiber.Handler that validates request headers.
func VerifyRequestFiber(cfg RequestVerifyConfig, hdr HeaderNames) fiber.Handler {
	if !cfg.Enable || len(cfg.Verifications) == 0 {
		return func(c *fiber.Ctx) error { return c.Next() }
	}
	h := hdr.Effective()

	return func(c *fiber.Ctx) error {
		if shouldSkip(c.Path(), cfg.SkipPaths) {
			return c.Next()
		}

		app := c.Get(h.ReqApp)
		ts := c.Get(h.ReqTimestamp)
		nonce := c.Get(h.ReqNonce)
		sig := c.Get(h.ReqSign)

		if app == "" || ts == "" || nonce == "" || sig == "" {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing signature headers",
			})
		}

		item := cfg.LookupApp(app)
		if item == nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error": fmt.Sprintf("unknown or disabled app %q", app),
			})
		}

		verifier, err := NewVerifier(SignMethod(item.Method), SignerOpts{
			Salt:      item.Salt,
			PublicKey: item.PublicKey,
		})
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("verifier init: %v", err),
			})
		}

		if err := VerifyRequest(VerifyRequestParams{
			AppHeader:       app,
			TimestampHeader: ts,
			NonceHeader:     nonce,
			SignHeader:      sig,
			Verifier:        verifier,
			Path:            c.Path(),
			Body:            c.Body(),
			TimeoutMS:       item.Timeout,
			HeaderNames:     hdr,
		}); err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Next()
	}
}

// SignResponseFiber returns a fiber.Handler that adds response headers.
func SignResponseFiber(cfg ServerSignConfig, hdr HeaderNames) fiber.Handler {
	if !cfg.Enabled() {
		return func(c *fiber.Ctx) error { return c.Next() }
	}

	signer, err := NewSigner(SignMethod(cfg.Method), cfg.toSignerOpts())
	if err != nil {
		return func(c *fiber.Ctx) error { return c.Next() }
	}
	h := hdr.Effective()

	return func(c *fiber.Ctx) error {
		if shouldSkip(c.Path(), cfg.SkipPaths) {
			return c.Next()
		}

		err := c.Next()

		respBody := c.Response().Body()
		statusCode := c.Response().StatusCode()

		ts := time.Now()
		headers, signErr := SignResponse(SignResponseParams{
			AppID:       cfg.AppID,
			Method:      SignMethod(cfg.Method),
			Signer:      signer,
			StatusCode:  statusCode,
			Body:        respBody,
			HeaderNames: hdr,
		}, ts)
		if signErr != nil {
			return err
		}

		c.Set(h.ResApp, headers.App)
		c.Set(h.ResTimestamp, headers.Timestamp)
		c.Set(h.ResNonce, headers.Nonce)
		c.Set(h.ResSign, headers.Sign)

		return err
	}
}

// ============================================================================
// Dynamic Fiber Middleware (hot-reload)
// ============================================================================

// VerifyRequestFiberDynamic is like VerifyRequestFiber but reads config
// from a DynamicRequestVerifyConfig on every request.
func VerifyRequestFiberDynamic(dynCfg *DynamicRequestVerifyConfig, hdr HeaderNames) fiber.Handler {
	h := hdr.Effective()

	return func(c *fiber.Ctx) error {
		cfg := dynCfg.Load()
		if !cfg.Enable || len(cfg.Verifications) == 0 {
			return c.Next()
		}

		if shouldSkip(c.Path(), cfg.SkipPaths) {
			return c.Next()
		}

		app := c.Get(h.ReqApp)
		ts := c.Get(h.ReqTimestamp)
		nonce := c.Get(h.ReqNonce)
		sig := c.Get(h.ReqSign)

		if app == "" || ts == "" || nonce == "" || sig == "" {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "missing signature headers"})
		}

		item := cfg.LookupApp(app)
		if item == nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": fmt.Sprintf("unknown or disabled app %q", app)})
		}

		verifier, err := NewVerifier(SignMethod(item.Method), SignerOpts{
			Salt:      item.Salt,
			PublicKey: item.PublicKey,
		})
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("verifier init: %v", err)})
		}

		if err := VerifyRequest(VerifyRequestParams{
			AppHeader:       app,
			TimestampHeader: ts,
			NonceHeader:     nonce,
			SignHeader:      sig,
			Verifier:        verifier,
			Path:            c.Path(),
			Body:            c.Body(),
			TimeoutMS:       item.Timeout,
			HeaderNames:     hdr,
		}); err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}

		return c.Next()
	}
}

// SignResponseFiberDynamic is like SignResponseFiber but reads config
// from a DynamicServerSignConfig on every request.
func SignResponseFiberDynamic(dynCfg *DynamicServerSignConfig, hdr HeaderNames) fiber.Handler {
	h := hdr.Effective()

	return func(c *fiber.Ctx) error {
		cfg := dynCfg.Load()
		if !cfg.Enabled() {
			return c.Next()
		}

		signer, err := NewSigner(SignMethod(cfg.Method), cfg.toSignerOpts())
		if err != nil {
			return c.Next()
		}

		if shouldSkip(c.Path(), cfg.SkipPaths) {
			return c.Next()
		}

		err = c.Next()

		respBody := c.Response().Body()
		statusCode := c.Response().StatusCode()

		ts := time.Now()
		headers, signErr := SignResponse(SignResponseParams{
			AppID:       cfg.AppID,
			Method:      SignMethod(cfg.Method),
			Signer:      signer,
			StatusCode:  statusCode,
			Body:        respBody,
			HeaderNames: hdr,
		}, ts)
		if signErr != nil {
			return err
		}

		c.Set(h.ResApp, headers.App)
		c.Set(h.ResTimestamp, headers.Timestamp)
		c.Set(h.ResNonce, headers.Nonce)
		c.Set(h.ResSign, headers.Sign)

		return err
	}
}
