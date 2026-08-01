package fiber

import (
	"net/http"
	"strings"
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

// ============================================================
// Additional router & context coverage tests
// ============================================================

func TestRouter_Group_Chained(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	g1 := r.Group("/v1")
	g2 := g1.Group("/users")
	g2.GET("/list", func(c interfaces.Context) error {
		return c.Text(200, "ok")
	})

	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/v1/users/list", nil)
	req.Host = "example.com"
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRouter_Group_WithMiddleware(t *testing.T) {
	a := NewFiberAdapter()
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

	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/v2/test", nil)
	req.Host = "example.com"
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestContext_HTML(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.GET("/html", func(c interfaces.Context) error {
		return c.HTML(200, "<h1>hello</h1>")
	})
	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/html", nil)
	req.Host = "example.com"
	app.Test(req, -1)
}

func TestContext_XML(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.GET("/xml", func(c interfaces.Context) error {
		type data struct {
			Name string `xml:"name"`
		}
		return c.XML(200, data{Name: "test"})
	})
	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/xml", nil)
	req.Host = "example.com"
	app.Test(req, -1)
}

func TestContext_RequestMethodPath(t *testing.T) {
	a := NewFiberAdapter()
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
	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/req-test", nil)
	req.Host = "example.com"
	app.Test(req, -1)
}

func TestContext_QueryParam(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.GET("/qp", func(c interfaces.Context) error {
		if v := c.QueryParam("name"); v != "j" {
			t.Errorf("expected j, got %s", v)
		}
		return c.Text(200, "ok")
	})
	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/qp?name=j", nil)
	req.Host = "example.com"
	app.Test(req, -1)
}

func TestContext_Param(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.GET("/users/:id", func(c interfaces.Context) error {
		_ = c.Param("id")
		return c.Text(200, "ok")
	})
	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/users/42", nil)
	req.Host = "example.com"
	app.Test(req, -1)
}

func TestContext_Status(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.GET("/status", func(c interfaces.Context) error {
		c.Status(201)
		return c.Text(200, "ok")
	})
	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/status", nil)
	req.Host = "example.com"
	resp, _ := app.Test(req, -1)
	// Status then Text: the Text override wins
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestContext_ContextMethod(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.GET("/ctx", func(c interfaces.Context) error {
		if c.Context() == nil {
			t.Error("Context nil")
		}
		return c.Text(200, "ok")
	})
	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/ctx", nil)
	req.Host = "example.com"
	app.Test(req, -1)
}

func TestContext_ResponseWriter(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.GET("/rw", func(c interfaces.Context) error {
		_ = c.ResponseWriter()
		return c.Text(200, "ok")
	})
	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/rw", nil)
	req.Host = "example.com"
	app.Test(req, -1)
}

func TestContext_BindJSON(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.POST("/bind", func(c interfaces.Context) error {
		var data map[string]string
		_ = c.BindJSON(&data)
		return c.Text(200, "ok")
	})
	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("POST", "/bind", strings.NewReader(`{"x":"y"}`))
	req.Host = "example.com"
	req.Header.Set("Content-Type", "application/json")
	app.Test(req, -1)
}

func TestContext_BindXML(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.POST("/bindxml", func(c interfaces.Context) error {
		var data map[string]string
		_ = c.BindXML(&data)
		return c.Text(200, "ok")
	})
	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("POST", "/bindxml", strings.NewReader("<root><name>x</name></root>"))
	req.Host = "example.com"
	req.Header.Set("Content-Type", "application/xml")
	app.Test(req, -1)
}

func TestContext_BindQuery(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.GET("/bindq", func(c interfaces.Context) error {
		_ = c.BindQuery(nil)
		return c.Text(200, "ok")
	})
	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/bindq?a=1", nil)
	req.Host = "example.com"
	app.Test(req, -1)
}

func TestContext_Cookie_Read(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.GET("/cookier", func(c interfaces.Context) error {
		_, _ = c.Cookie("test")
		return c.Text(200, "ok")
	})
	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/cookier", nil)
	req.Host = "example.com"
	req.AddCookie(&http.Cookie{Name: "test", Value: "v"})
	app.Test(req, -1)
}

func TestContext_Logger(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.GET("/logger", func(c interfaces.Context) error {
		if c.Logger() == nil {
			t.Error("Logger nil")
		}
		return c.Text(200, "ok")
	})
	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("GET", "/logger", nil)
	req.Host = "example.com"
	app.Test(req, -1)
}

func TestContext_FormValue(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.POST("/formval", func(c interfaces.Context) error {
		_ = c.FormValue("field")
		return c.Text(200, "ok")
	})
	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("POST", "/formval", strings.NewReader("field=hello"))
	req.Host = "example.com"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.Test(req, -1)
}

func TestContext_PostForm(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.POST("/postform", func(c interfaces.Context) error {
		_ = c.PostForm("field")
		return c.Text(200, "ok")
	})
	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("POST", "/postform", strings.NewReader("field=world"))
	req.Host = "example.com"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.Test(req, -1)
}

func TestContext_ParseForm(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.POST("/pf", func(c interfaces.Context) error {
		return c.ParseForm()
	})
	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("POST", "/pf", nil)
	req.Host = "example.com"
	app.Test(req, -1)
}

func TestContext_ParseMultipartForm(t *testing.T) {
	a := NewFiberAdapter()
	r := a.NewRouter()
	r.POST("/pmf", func(c interfaces.Context) error {
		return c.ParseMultipartForm(32 << 20)
	})
	app := r.(*FiberRouter).app
	req, _ := http.NewRequest("POST", "/pmf", nil)
	req.Host = "example.com"
	app.Test(req, -1)
}
