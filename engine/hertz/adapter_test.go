package hertz

import (
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	interfaces "github.com/bufgot/web"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// buildAndStart creates a hertz server on a random port and returns URL + cleanup
func buildAndStart(t *testing.T, r *HertzRouter) (string, func()) {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()

	h := server.Default(server.WithHostPorts(addr))
	for _, mw := range r.middlewares {
		h.Use(r.wrapMiddleware(mw))
	}
	for method, handlers := range r.routes {
		for path, handler := range handlers {
			wrapped := r.wrapHandler(handler)
			switch method {
			case "GET":
				h.GET(path, wrapped)
			case "POST":
				h.POST(path, wrapped)
			case "PUT":
				h.PUT(path, wrapped)
			case "DELETE":
				h.DELETE(path, wrapped)
			case "PATCH":
				h.PATCH(path, wrapped)
			case "HEAD":
				h.HEAD(path, wrapped)
			case "OPTIONS":
				h.OPTIONS(path, wrapped)
			}
		}
	}

	go h.Spin()
	time.Sleep(200 * time.Millisecond)
	return "http://" + addr, func() {}
}

func TestAdapter_Name(t *testing.T) {
	a := NewHertzAdapter()
	if a.Name() != "hertz" {
		t.Fatalf("expected 'hertz', got '%s'", a.Name())
	}
}

func TestAdapter_NewRouter(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	if r == nil {
		t.Fatal("NewRouter returned nil")
	}
}

func TestRouter_GET_RouteAndResponse(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.GET("/hello", func(c interfaces.Context) error {
		return c.Text(200, "hello world")
	})

	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()

	resp, err := http.Get(url + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello world" {
		t.Fatalf("expected 'hello world', got '%s'", string(body))
	}
}

func TestRouter_MultipleMethods(t *testing.T) {
	a := NewHertzAdapter()
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

	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"}
	for _, method := range methods {
		req, _ := http.NewRequest(method, url+"/m", nil)
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
	a := NewHertzAdapter()
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

	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()

	resp, err := http.Get(url + "/mw")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if !called {
		t.Fatal("middleware was not called")
	}
}

func TestRouter_Group(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()

	g := r.Group("/api")
	g.GET("/hello", func(c interfaces.Context) error {
		return c.Text(200, "group ok")
	})

	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()

	resp, err := http.Get(url + "/api/hello")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "group ok" {
		t.Fatalf("expected 'group ok', got '%s'", string(body))
	}
}

func TestContext_JSON(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()

	r.GET("/json", func(c interfaces.Context) error {
		return c.JSON(200, map[string]string{"key": "value"})
	})

	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()

	resp, err := http.Get(url + "/json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestContext_Text(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()

	r.GET("/text", func(c interfaces.Context) error {
		return c.Text(200, "plain")
	})

	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()

	resp, err := http.Get(url + "/text")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/plain" && ct != "text/plain; charset=utf-8" {
		t.Fatalf("expected text/plain, got %s", ct)
	}
}

func TestContext_Cookie(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()

	r.GET("/cookie", func(c interfaces.Context) error {
		c.SetCookie(&http.Cookie{Name: "test", Value: "val", Path: "/"})
		return c.Text(200, "ok")
	})

	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()

	resp, err := http.Get(url + "/cookie")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected cookie in response")
	}
}

func TestContext_Redirect(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()

	r.GET("/redirect", func(c interfaces.Context) error {
		return c.Redirect(302, "/target")
	})

	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(url + "/redirect")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != 302 {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
}

func TestContext_SetGet(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()

	r.GET("/setget", func(c interfaces.Context) error {
		c.Set("key", "val")
		v := c.Get("key")
		if v != "val" {
			t.Errorf("expected 'val', got '%v'", v)
		}
		return c.Text(200, "ok")
	})

	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()

	http.Get(url + "/setget")
}

func TestRouter_SetLogger(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.SetLogger(nil)
}

func TestRouter_Static(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.Static("/static", ".")
}
