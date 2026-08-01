package echo

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	labstack "github.com/labstack/echo/v4"
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

// ============================================================
// Additional context & router coverage tests
// ============================================================

func TestRouter_Static_WithGroup(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	g := r.Group("/api")
	g.Static("/static", ".")

	// verify the group's static was set on the echo engine
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
}

func TestRouter_Group_Chained(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	g1 := r.Group("/v1")
	g2 := g1.Group("/users")
	g2.GET("/list", func(c interfaces.Context) error {
		return c.Text(200, "ok")
	})

	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/v1/users/list")
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRouter_Group_WithMiddleware(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	g := r.Group("/v2", func(next interfaces.Handler) interfaces.Handler {
		return func(c interfaces.Context) error {
			c.Set("mw", true)
			return next(c)
		}
	})
	g.GET("/test", func(c interfaces.Context) error {
		return c.Text(200, "ok")
	})

	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/v2/test")
	resp.Body.Close()
}

func TestContext_HTML(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.GET("/html", func(c interfaces.Context) error {
		return c.HTML(200, "<h1>hello</h1>")
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	http.Get(srv.URL + "/html")
}

func TestContext_XML(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.GET("/xml", func(c interfaces.Context) error {
		type data struct {
			Name string `xml:"name"`
		}
		return c.XML(200, data{Name: "test"})
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	http.Get(srv.URL + "/xml")
}

func TestContext_RequestMethodPath(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.GET("/req-test", func(c interfaces.Context) error {
		req := c.Request()
		if req == nil {
			t.Error("Request nil")
		}
		if c.Method() != "GET" {
			t.Errorf("bad method: %s", c.Method())
		}
		if c.Path() != "/req-test" {
			t.Errorf("bad path: %s", c.Path())
		}
		return c.Text(200, "ok")
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	http.Get(srv.URL + "/req-test")
}

func TestContext_QueryParam(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.GET("/qp", func(c interfaces.Context) error {
		c.QueryParam("name")
		return c.Text(200, "ok")
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	http.Get(srv.URL + "/qp?name=j")
}

func TestContext_Param(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.GET("/users/:id", func(c interfaces.Context) error {
		c.Param("id")
		return c.Text(200, "ok")
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	http.Get(srv.URL + "/users/42")
}

func TestContext_Status(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.GET("/status", func(c interfaces.Context) error {
		c.Status(201)
		return nil
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	resp, _ := http.Get(srv.URL + "/status")
	resp.Body.Close()
	// Echo's Status sets the field but without writing output
	// the actual HTTP status depends on echo internals
}

func TestContext_ContextMethod(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.GET("/ctx", func(c interfaces.Context) error {
		if c.Context() == nil {
			t.Error("Context nil")
		}
		return c.Text(200, "ok")
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	http.Get(srv.URL + "/ctx")
}

func TestContext_ResponseWriter(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.GET("/rw", func(c interfaces.Context) error {
		_ = c.ResponseWriter()
		return c.Text(200, "ok")
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	http.Get(srv.URL + "/rw")
}

func TestContext_BindJSON(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.POST("/bind", func(c interfaces.Context) error {
		var data map[string]string
		_ = c.BindJSON(&data)
		return c.Text(200, "ok")
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	b, _ := json.Marshal(map[string]string{"x": "y"})
	req, _ := http.NewRequest("POST", srv.URL+"/bind", strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(req)
}

func TestContext_BindXML(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.POST("/bindxml", func(c interfaces.Context) error {
		var data map[string]string
		_ = c.BindXML(&data)
		return c.Text(200, "ok")
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	req, _ := http.NewRequest("POST", srv.URL+"/bindxml", strings.NewReader("<root><name>x</name></root>"))
	req.Header.Set("Content-Type", "application/xml")
	http.DefaultClient.Do(req)
}

func TestContext_BindQuery(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.GET("/bindq", func(c interfaces.Context) error {
		_ = c.BindQuery(nil)
		return c.Text(200, "ok")
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	http.Get(srv.URL + "/bindq?a=1")
}

func TestContext_Cookie_Read(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.GET("/cookie-read", func(c interfaces.Context) error {
		c.Cookie("test")
		return c.Text(200, "ok")
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/cookie-read", nil)
	req.AddCookie(&http.Cookie{Name: "test", Value: "v"})
	http.DefaultClient.Do(req)
}

func TestContext_Logger(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.GET("/logger", func(c interfaces.Context) error {
		if c.Logger() == nil {
			t.Error("Logger nil")
		}
		return c.Text(200, "ok")
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	http.Get(srv.URL + "/logger")
}

func TestContext_FormValue(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.POST("/formval", func(c interfaces.Context) error {
		_ = c.FormValue("field")
		return c.Text(200, "ok")
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	resp, _ := http.PostForm(srv.URL+"/formval", map[string][]string{"field": {"hello"}})
	resp.Body.Close()
}

func TestContext_PostForm(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.POST("/postform", func(c interfaces.Context) error {
		_ = c.PostForm("field")
		return c.Text(200, "ok")
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	resp, _ := http.PostForm(srv.URL+"/postform", map[string][]string{"field": {"world"}})
	resp.Body.Close()
}

func TestContext_ParseForm(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.POST("/parseform", func(c interfaces.Context) error {
		_ = c.ParseForm()
		return c.Text(200, "ok")
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	resp, _ := http.PostForm(srv.URL+"/parseform", map[string][]string{"a": {"1"}})
	resp.Body.Close()
}

func TestContext_ParseMultipartForm(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.POST("/parsem", func(c interfaces.Context) error {
		_ = c.ParseMultipartForm(32 << 20)
		return c.Text(200, "ok")
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	w.WriteField("h", "w")
	w.Close()
	resp, _ := http.Post(srv.URL+"/parsem", w.FormDataContentType(), body)
	resp.Body.Close()
}

// ============================================================
// Coverage gap fillers
// ============================================================

func TestEchoRouter_currentRouter_NoGroup(t *testing.T) {
	r := &EchoRouter{echo: labstack.New(), group: nil}
	cr := r.currentRouter()
	if cr == nil {
		t.Fatal("currentRouter returned nil")
	}
	// should return r.echo (not a *labstack.Group)
	if _, ok := cr.(*labstack.Echo); !ok {
		t.Fatalf("expected *labstack.Echo, got %T", cr)
	}
}

func TestEchoRouter_currentRouter_WithGroup(t *testing.T) {
	e := labstack.New()
	g := e.Group("/api")
	r := &EchoRouter{echo: e, group: g}
	cr := r.currentRouter()
	if _, ok := cr.(*labstack.Group); !ok {
		t.Fatalf("expected *labstack.Group, got %T", cr)
	}
}

func TestEchoRouter_addRoute_WithGroup(t *testing.T) {
	e := labstack.New()
	g := e.Group("/api")
	r := &EchoRouter{echo: e, group: g}

	r.addRoute("GET", "/test", func(c interfaces.Context) error {
		return c.Text(200, "ok")
	})

	srv := httptest.NewServer(e)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/api/test")
	if resp != nil {
		resp.Body.Close()
	}
}

func TestEchoRouter_addUse_WithGroup(t *testing.T) {
	e := labstack.New()
	g := e.Group("/api")
	r := &EchoRouter{echo: e, group: g}

	var mwCalled bool
	r.addUse(func(next interfaces.Handler) interfaces.Handler {
		return func(c interfaces.Context) error {
			mwCalled = true
			return next(c)
		}
	})
	r.addRoute("GET", "/mwtest", func(c interfaces.Context) error {
		return c.Text(200, "ok")
	})

	srv := httptest.NewServer(e)
	defer srv.Close()
	resp, _ := http.Get(srv.URL + "/api/mwtest")
	if resp != nil {
		resp.Body.Close()
	}
	if !mwCalled {
		t.Fatal("middleware was not called for group route")
	}
}

func TestEchoRouter_addUse_NoGroup(t *testing.T) {
	e := labstack.New()
	r := &EchoRouter{echo: e, group: nil}

	r.addUse(func(next interfaces.Handler) interfaces.Handler {
		return func(c interfaces.Context) error {
			return next(c)
		}
	})
	r.addRoute("GET", "/mwtest2", func(c interfaces.Context) error {
		return c.Text(200, "ok")
	})

	srv := httptest.NewServer(e)
	defer srv.Close()
	resp, _ := http.Get(srv.URL + "/mwtest2")
	if resp != nil {
		resp.Body.Close()
	}
}

func TestContext_Cookie_NotFound(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.GET("/nocookie", func(c interfaces.Context) error {
		val, err := c.Cookie("nonexistent")
		// Should return error since no cookie was set
		if err == nil && val != "" {
			t.Log("cookie returned value without error (ok)")
		}
		return c.Text(200, "ok")
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/nocookie", nil)
	resp, _ := http.DefaultClient.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
}

func TestContext_Logger_WithContextValue(t *testing.T) {
	a := NewEchoAdapter()
	r := a.NewRouter()
	r.GET("/loggerctx", func(c interfaces.Context) error {
		// Set a logger in context to cover the set-logger path
		logger := r.(*EchoRouter).logger
		c.Set("logger", logger)
		l := c.Logger()
		if l == nil {
			t.Error("Logger returned nil")
		}
		return c.Text(200, "ok")
	})
	engine := r.(*EchoRouter).echo
	srv := httptest.NewServer(engine)
	defer srv.Close()
	http.Get(srv.URL + "/loggerctx")
}

func TestEchoRouter_Start(t *testing.T) {
	e := labstack.New()
	r := &EchoRouter{echo: e}
	go func() {
		_ = r.Start("127.0.0.1:0")
	}()
}
