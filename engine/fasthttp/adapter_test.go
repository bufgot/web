package fasthttp

import (
	"testing"

	interfaces "github.com/bufgot/web"
	"github.com/valyala/fasthttp"
)

func TestAdapter_Name(t *testing.T) {
	a := NewFasthttpAdapter()
	if a.Name() != "fasthttp" {
		t.Fatalf("expected 'fasthttp', got '%s'", a.Name())
	}
}

func TestAdapter_NewRouter(t *testing.T) {
	a := NewFasthttpAdapter()
	r := a.NewRouter()
	if r == nil {
		t.Fatal("NewRouter returned nil")
	}
}

func TestRouter_RouteRegistration(t *testing.T) {
	a := NewFasthttpAdapter()
	r := a.NewRouter()

	ok := func(c interfaces.Context) error {
		return c.Text(200, "ok")
	}

	methods := []struct {
		name string
		fn   func(path string, h interfaces.Handler)
	}{
		{"GET", r.GET},
		{"POST", r.POST},
		{"PUT", r.PUT},
		{"DELETE", r.DELETE},
		{"PATCH", r.PATCH},
		{"HEAD", r.HEAD},
		{"OPTIONS", r.OPTIONS},
	}

	for _, m := range methods {
		m.fn("/"+m.name, ok)
	}

	fr := r.(*FasthttpRouter)
	for _, m := range methods {
		if fr.routes[m.name] == nil || fr.routes[m.name]["/"+m.name] == nil {
			t.Errorf("route not registered for %s /%s", m.name, m.name)
		}
	}
}

func TestRouter_Middleware(t *testing.T) {
	a := NewFasthttpAdapter()
	r := a.NewRouter()

	r.Use(func(next interfaces.Handler) interfaces.Handler {
		return func(c interfaces.Context) error {
			return next(c)
		}
	})

	fr := r.(*FasthttpRouter)
	if len(fr.middlewares) != 1 {
		t.Fatalf("expected 1 middleware, got %d", len(fr.middlewares))
	}
}

func TestRouter_Group(t *testing.T) {
	a := NewFasthttpAdapter()
	r := a.NewRouter()
	g := r.Group("/api")

	gr := g.(*FasthttpRouter)
	if gr.prefix != "/api" {
		t.Fatalf("expected prefix '/api', got '%s'", gr.prefix)
	}
}

func TestRouter_HandleRequest(t *testing.T) {
	a := NewFasthttpAdapter()
	r := a.NewRouter()

	r.GET("/hello", func(c interfaces.Context) error {
		return c.Text(200, "hello")
	})

	fr := r.(*FasthttpRouter)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/hello")
	ctx.Request.Header.SetMethod("GET")

	fr.handleRequest(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", ctx.Response.StatusCode())
	}
	if string(ctx.Response.Body()) != "hello" {
		t.Fatalf("expected 'hello', got '%s'", string(ctx.Response.Body()))
	}
}

func TestRouter_HandleRequest_NotFound(t *testing.T) {
	a := NewFasthttpAdapter()
	r := a.NewRouter()
	fr := r.(*FasthttpRouter)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/nonexistent")
	ctx.Request.Header.SetMethod("GET")

	fr.handleRequest(ctx)

	if ctx.Response.StatusCode() != 404 {
		t.Fatalf("expected 404, got %d", ctx.Response.StatusCode())
	}
}

func TestContext_JSON(t *testing.T) {
	a := NewFasthttpAdapter()
	r := a.NewRouter()
	r.GET("/json", func(c interfaces.Context) error {
		return c.JSON(200, map[string]string{"key": "value"})
	})

	fr := r.(*FasthttpRouter)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/json")
	ctx.Request.Header.SetMethod("GET")
	fr.handleRequest(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", ctx.Response.StatusCode())
	}
}

func TestContext_Text(t *testing.T) {
	a := NewFasthttpAdapter()
	r := a.NewRouter()
	r.GET("/text", func(c interfaces.Context) error {
		return c.Text(200, "plain")
	})

	fr := r.(*FasthttpRouter)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/text")
	ctx.Request.Header.SetMethod("GET")
	fr.handleRequest(ctx)

	if ct := string(ctx.Response.Header.ContentType()); ct != "text/plain" {
		t.Fatalf("expected text/plain, got %s", ct)
	}
}

func TestRouter_SetLogger(t *testing.T) {
	a := NewFasthttpAdapter()
	r := a.NewRouter()
	r.SetLogger(nil)
}

func TestRouter_Static(t *testing.T) {
	a := NewFasthttpAdapter()
	r := a.NewRouter()
	r.Static("/static", ".")
}
