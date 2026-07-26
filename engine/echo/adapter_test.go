package echo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	interfaces "github.com/bufgot/web"
)

func TestAdapter_Name(t *testing.T) {
	a := NewEchoAdapter()
	if a.Name() != "echo" {
		t.Fatalf("expected 'echo', got '%s'", a.Name())
	}
}

func TestAdapter_NewRouter(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	if r == nil {
		t.Fatal("NewRouter returned nil")
	}
}

func TestRouter_GET_RouteAndResponse(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.GET("/hello", func(c interfaces.Context) error {
		return c.Text(200, "hello world")
	})

	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRouter_MultipleMethods(t *testing.T) {
	a := NewEchoAdapter()
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

	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"}
	for _, method := range methods {
		req, _ := http.NewRequest(method, srv.URL+"/m", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("%s: expected 200, got %d", method, resp.StatusCode)
		}
	}
}

func TestRouter_Middleware(t *testing.T) {
	a := NewEchoAdapter()
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

	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/mw")
	resp.Body.Close()

	if !called {
		t.Fatal("middleware was not called")
	}
}

func TestRouter_Group(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()

	g := r.Group("/api")
	g.GET("/hello", func(c interfaces.Context) error {
		return c.Text(200, "group ok")
	})

	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/hello")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestContext_JSON(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()

	r.GET("/json", func(c interfaces.Context) error {
		return c.JSON(200, map[string]string{"key": "value"})
	})

	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/json")
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestContext_Text(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()

	r.GET("/text", func(c interfaces.Context) error {
		return c.Text(200, "plain")
	})

	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/text")
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/plain; charset=UTF-8" && ct != "text/plain" {
		t.Fatalf("expected text/plain, got %s", ct)
	}
}

func TestContext_Cookie(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()

	r.GET("/cookie", func(c interfaces.Context) error {
		c.SetCookie(&http.Cookie{Name: "test", Value: "val", Path: "/"})
		return c.Text(200, "ok")
	})

	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/cookie")
	defer resp.Body.Close()

	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected cookie in response")
	}
}

func TestContext_Redirect(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()

	r.GET("/redirect", func(c interfaces.Context) error {
		return c.Redirect(302, "/target")
	})

	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, _ := client.Get(srv.URL + "/redirect")
	resp.Body.Close()

	if resp.StatusCode != 302 {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
}

func TestContext_SetGet(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()

	r.GET("/setget", func(c interfaces.Context) error {
		c.Set("key", "val")
		if v := c.Get("key"); v != "val" {
			t.Errorf("expected 'val', got '%v'", v)
		}
		return c.Text(200, "ok")
	})

	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()

	_, _ = http.Get(srv.URL + "/setget")
}

func TestRouter_SetLogger(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.SetLogger(nil)
}

func TestRouter_Static(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.Static("/static", ".")
}
