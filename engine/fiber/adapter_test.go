package fiber

import (
	"net/http"
	"testing"

	interfaces "github.com/bufgot/web"
)

func TestAdapter_Name(t *testing.T) {
	a := NewFiberAdapter()
	if a.Name() != "fiber" {
		t.Fatalf("expected 'fiber', got '%s'", a.Name())
	}
}

func TestAdapter_NewRouter(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	if r == nil {
		t.Fatal("NewRouter returned nil")
	}
}

func TestRouter_GET_RouteAndResponse(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.GET("/hello", func(c interfaces.Context) error {
		return c.Text(200, "hello world")
	})

	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/hello", nil)
	req.Host = "example.com"
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRouter_MultipleMethods(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()

	ok := func(c interfaces.Context) error {
		return c.Text(200, "ok")
	}

	r.GET("/m", ok)
	r.POST("/m", ok)
	r.PUT("/m", ok)
	r.DELETE("/m", ok)
	r.PATCH("/m", ok)
	r.HEAD("/m", ok)
	r.OPTIONS("/m", ok)

	app := r.(*FiberRouter).app
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"}
	for _, method := range methods {
		req, _ := http.NewRequest(method, "/m", nil)
		req.Host = "example.com"
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("%s: expected 200, got %d", method, resp.StatusCode)
		}
	}
}

func TestRouter_Middleware(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()

	var called bool
	r.Use(func(next interfaces.Handler) interfaces.Handler {
		return func(c interfaces.Context) error {
			called = true
			return next(c)
		}
	})

	r.GET("/mw", func(c interfaces.Context) error {
		return c.Text(200, "ok")
	})

	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/mw", nil)
	req.Host = "example.com"
	_, _ = app.Test(req, -1)

	if !called {
		t.Fatal("middleware was not called")
	}
}

func TestRouter_Group(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()

	g := r.Group("/api")
	g.GET("/hello", func(c interfaces.Context) error {
		return c.Text(200, "group ok")
	})

	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/api/hello", nil)
	req.Host = "example.com"
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestContext_JSON(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()

	r.GET("/json", func(c interfaces.Context) error {
		return c.JSON(200, map[string]string{"key": "value"})
	})

	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/json", nil)
	req.Host = "example.com"
	req.Header.Set("Accept", "application/json")
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestContext_Text(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()

	r.GET("/text", func(c interfaces.Context) error {
		return c.Text(200, "plain")
	})

	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/text", nil)
	req.Host = "example.com"
	resp, _ := app.Test(req, -1)

	ct := resp.Header.Get("Content-Type")
	if ct != "text/plain" && ct != "text/plain; charset=utf-8" {
		t.Fatalf("expected text/plain, got %s", ct)
	}
}

func TestContext_Cookie(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()

	r.GET("/cookie", func(c interfaces.Context) error {
		c.SetCookie(&http.Cookie{Name: "test", Value: "val", Path: "/"})
		return c.Text(200, "ok")
	})

	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/cookie", nil)
	req.Host = "example.com"
	resp, _ := app.Test(req, -1)

	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected cookie in response")
	}
}

func TestContext_Redirect(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()

	r.GET("/redirect", func(c interfaces.Context) error {
		return c.Redirect(302, "/target")
	})

	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/redirect", nil)
	req.Host = "example.com"
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != 302 {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
}

func TestContext_SetGet(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()

	r.GET("/setget", func(c interfaces.Context) error {
		c.Set("key", "val")
		v := c.Get("key")
		if v != "val" {
			t.Errorf("expected 'val', got '%v'", v)
		}
		return c.Text(200, "ok")
	})

	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/setget", nil)
	req.Host = "example.com"
	_, _ = app.Test(req, -1)
}

func TestRouter_SetLogger(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.SetLogger(nil)
}

func TestRouter_Static(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.Static("/static", ".")
}
