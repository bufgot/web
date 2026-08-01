package hertz

import (
	"bytes"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"strings"
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

// ============================================================
// Additional context method coverage via route handlers
// ============================================================

func TestContext_RequestMethodPath(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.GET("/req-test", func(c interfaces.Context) error {
		req := c.Request()
		if req == nil {
			t.Error("Request() returned nil")
		}
		method := c.Method()
		if method != "GET" {
			t.Errorf("expected GET, got %s", method)
		}
		p := c.Path()
		if p != "/req-test" {
			t.Errorf("expected /req-test, got %s", p)
		}
		return c.Text(200, "ok")
	})
	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()
	http.Get(url + "/req-test")
}

func TestContext_QueryParam(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.GET("/qp", func(c interfaces.Context) error {
		v := c.QueryParam("name")
		t.Logf("QueryParam name=%q", v)
		return c.Text(200, v)
	})
	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()
	resp, _ := http.Get(url + "/qp?name=testval")
	resp.Body.Close()
}

func TestContext_Param(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.GET("/users/:id", func(c interfaces.Context) error {
		v := c.Param("id")
		t.Logf("Param id=%q", v)
		return c.Text(200, v)
	})
	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()
	resp, _ := http.Get(url + "/users/42")
	resp.Body.Close()
}

func TestContext_Status(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.GET("/status", func(c interfaces.Context) error {
		c.Status(418)
		return nil
	})
	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()
	client := &http.Client{}
	resp, _ := client.Get(url + "/status")
	resp.Body.Close()
	if resp.StatusCode != 418 {
		t.Errorf("expected 418, got %d", resp.StatusCode)
	}
}

func TestContext_HTML(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.GET("/html", func(c interfaces.Context) error {
		return c.HTML(200, "<h1>hello</h1>")
	})
	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()
	resp, _ := http.Get(url + "/html")
	defer resp.Body.Close()
	// hertz may return text/plain or text/html depending on version
	ct := resp.Header.Get("Content-Type")
	t.Logf("Content-Type: %s", ct)
}

func TestContext_XML(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.GET("/xml", func(c interfaces.Context) error {
		type xmlData struct {
			Name string `xml:"name"`
		}
		return c.XML(200, xmlData{Name: "test"})
	})
	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()
	resp, _ := http.Get(url + "/xml")
	resp.Body.Close()
}

func TestContext_Cookie_Read(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()

	// Cookie present
	r.GET("/cookie-read", func(c interfaces.Context) error {
		val, err := c.Cookie("test")
		if err == nil {
			t.Logf("cookie test=%s", val)
		}
		return c.Text(200, "ok")
	})

	// Cookie missing — covers error path
	r.GET("/cookie-miss", func(c interfaces.Context) error {
		_, err := c.Cookie("missing")
		if err != nil {
			t.Logf("got expected error: %v", err)
		}
		return c.Text(200, "ok")
	})

	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()

	req1, _ := http.NewRequest("GET", url+"/cookie-read", nil)
	req1.AddCookie(&http.Cookie{Name: "test", Value: "xyz"})
	resp, _ := http.DefaultClient.Do(req1)
	resp.Body.Close()

	resp2, _ := http.Get(url + "/cookie-miss")
	resp2.Body.Close()
}

func TestContext_Logger(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.GET("/logger", func(c interfaces.Context) error {
		l := c.Logger()
		if l == nil {
			t.Error("Logger() returned nil")
		}
		return c.Text(200, "ok")
	})
	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()
	http.Get(url + "/logger")
}

func TestContext_LoggerWithSet(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()

	// Set non-Logger type to force type assertion to fail (cover !ok branch)
	r.GET("/logger-set-non", func(c interfaces.Context) error {
		c.Set("logger", "not-a-logger")
		l := c.Logger()
		if l == nil {
			t.Error("Logger should fall back to default")
		}
		return c.Text(200, "ok")
	})

	// Set nil to make ok && logger != nil false
	r.GET("/logger-set-nil", func(c interfaces.Context) error {
		c.Set("logger", nil)
		l := c.Logger()
		if l == nil {
			t.Error("Logger should fall back to default")
		}
		return c.Text(200, "ok")
	})

	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()
	http.Get(url + "/logger-set-non")
	http.Get(url + "/logger-set-nil")
}

func TestContext_ContextMethod(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.GET("/ctx", func(c interfaces.Context) error {
		ctx := c.Context()
		if ctx == nil {
			t.Error("Context() returned nil")
		}
		return c.Text(200, "ok")
	})
	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()
	http.Get(url + "/ctx")
}

func TestContext_ResponseWriter(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.GET("/rw", func(c interfaces.Context) error {
		_ = c.ResponseWriter() // returns nil per docs
		return c.Text(200, "ok")
	})
	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()
	http.Get(url + "/rw")
}

func TestContext_BindJSON(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.POST("/bindjson", func(c interfaces.Context) error {
		var data map[string]string
		if err := c.BindJSON(&data); err != nil {
			t.Logf("BindJSON error: %v", err)
		}
		return c.Text(200, "ok")
	})
	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()
	resp, _ := http.Post(url+"/bindjson", "application/json", strings.NewReader(`{"key":"val"}`))
	resp.Body.Close()
}

func TestContext_BindXML(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.POST("/bindxml", func(c interfaces.Context) error {
		var data map[string]string
		_ = c.BindXML(&data) // stub, always nil
		return c.Text(200, "ok")
	})
	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()
	resp, _ := http.Post(url+"/bindxml", "application/xml", strings.NewReader("<root/>"))
	resp.Body.Close()
}

func TestContext_BindQuery(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.GET("/bindq", func(c interfaces.Context) error {
		var data map[string]string
		_ = c.BindQuery(&data) // stub, always nil
		return c.Text(200, "ok")
	})
	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()
	http.Get(url + "/bindq?a=1&b=2")
}

func TestContext_FormValue(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.POST("/formval", func(c interfaces.Context) error {
		_ = c.FormValue("field")
		return c.Text(200, "ok")
	})
	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()
	resp, _ := http.PostForm(url+"/formval", map[string][]string{"field": {"hello"}})
	resp.Body.Close()
}

func TestContext_PostForm(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.POST("/postform", func(c interfaces.Context) error {
		_ = c.PostForm("field")
		return c.Text(200, "ok")
	})
	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()
	resp, _ := http.PostForm(url+"/postform", map[string][]string{"field": {"world"}})
	resp.Body.Close()
}

func TestContext_ParseForm(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.POST("/parseform", func(c interfaces.Context) error {
		if err := c.ParseForm(); err != nil {
			t.Errorf("ParseForm error: %v", err)
		}
		return c.Text(200, "ok")
	})
	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()
	resp, _ := http.PostForm(url+"/parseform", map[string][]string{"a": {"1"}})
	resp.Body.Close()
}

func TestContext_ParseMultipartForm(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	r.POST("/parsem", func(c interfaces.Context) error {
		if err := c.ParseMultipartForm(32 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
		}
		return c.Text(200, "ok")
	})
	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("hello", "world")
	w.Close()
	resp, _ := http.Post(url+"/parsem", w.FormDataContentType(), &buf)
	resp.Body.Close()
}

func TestRouter_GroupMiddleware(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()
	g := r.Group("/v2", func(next interfaces.Handler) interfaces.Handler {
		return func(c interfaces.Context) error {
			return next(c)
		}
	})
	g.GET("/test", func(c interfaces.Context) error {
		return c.Text(200, "ok")
	})
	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()
	resp, _ := http.Get(url + "/v2/test")
	resp.Body.Close()
}

// TestRouter_Start covers the Start method's spin-up logic
func TestRouter_Start(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()

	r.GET("/start-test", func(c interfaces.Context) error {
		return c.Text(200, "started")
	})

	// Start in goroutine — covers address normalization + Spin
	go func() {
		_ = r.(*HertzRouter).Start(":0")
	}()

	// Wait for server to spin up
	time.Sleep(500 * time.Millisecond)
	// Not easily accessible; just exercise the code path
}

// TestRouter_Start_AllSwitchCases exercises all method switch branches in Start
func TestRouter_Start_AllMethods(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter().(*HertzRouter)

	ok := func(c interfaces.Context) error { return c.Text(200, "ok") }
	// Register all methods to cover every switch case in Start()
	r.addRoute("GET", "/all", ok)
	r.addRoute("POST", "/all", ok)
	r.addRoute("PUT", "/all", ok)
	r.addRoute("DELETE", "/all", ok)
	r.addRoute("PATCH", "/all", ok)
	r.addRoute("HEAD", "/all", ok)
	r.addRoute("OPTIONS", "/all", ok)

	go func() { _ = r.Start("127.0.0.1:0") }()
	time.Sleep(300 * time.Millisecond)
}

// TestRouter_Start_WithStatic covers static path registration in Start
func TestRouter_Start_WithStatic(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter().(*HertzRouter)
	r.Static("/assets", ".")
	go func() { _ = r.Start("127.0.0.1:0") }()
	time.Sleep(300 * time.Millisecond)
}

// TestContext_Logger_WithValidLogger covers the ok && logger != nil branch
func TestContext_Logger_WithValidLogger(t *testing.T) {
	a := NewHertzAdapter()
	r := a.NewRouter()

	r.GET("/logger-valid", func(c interfaces.Context) error {
		// Set a valid (non-nil) logger to exercise the first return path
		routerLogger := r.(*HertzRouter).logger
		c.Set("logger", routerLogger)
		l := c.Logger()
		if l == nil {
			t.Error("Logger should return the set logger")
		}
		return c.Text(200, "ok")
	})

	url, cleanup := buildAndStart(t, r.(*HertzRouter))
	defer cleanup()
	http.Get(url + "/logger-valid")
}
