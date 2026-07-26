package chi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	interfaces "github.com/bufgot/web"
)

func TestAdapter_Name(t *testing.T) {
	a := NewChiAdapter()
	if a.Name() != "chi" {
		t.Fatalf("expected 'chi', got '%s'", a.Name())
	}
}

func TestAdapter_NewRouter(t *testing.T) {
	a := NewChiAdapter()
	r := a.NewRouter()
	if r == nil {
		t.Fatal("NewRouter returned nil")
	}
}

func TestRouter_GET_RouteAndResponse(t *testing.T) {
	a := NewChiAdapter()
	r := a.NewRouter()
	r.GET("/hello", func(c interfaces.Context) error {
		return c.Text(200, "hello world")
	})

	router := r.(*ChiRouter).Router()
	srv := httptest.NewServer(router)
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
	a := NewChiAdapter()
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

	router := r.(*ChiRouter).Router()
	srv := httptest.NewServer(router)
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
	a := NewChiAdapter()
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

	router := r.(*ChiRouter).Router()
	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/mw")
	resp.Body.Close()

	if !called {
		t.Fatal("middleware was not called")
	}
}

func TestRouter_Group(t *testing.T) {
	a := NewChiAdapter()
	r := a.NewRouter()

	g := r.Group("/api")
	g.GET("/hello", func(c interfaces.Context) error {
		return c.Text(200, "group ok")
	})

	router := r.(*ChiRouter).Router()
	srv := httptest.NewServer(router)
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
	a := NewChiAdapter()
	r := a.NewRouter()

	r.GET("/json", func(c interfaces.Context) error {
		return c.JSON(200, map[string]string{"key": "value"})
	})

	router := r.(*ChiRouter).Router()
	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/json")
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}
}

func TestContext_Text(t *testing.T) {
	a := NewChiAdapter()
	r := a.NewRouter()

	r.GET("/text", func(c interfaces.Context) error {
		return c.Text(200, "plain")
	})

	router := r.(*ChiRouter).Router()
	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/text")
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/plain" {
		t.Fatalf("expected text/plain, got %s", ct)
	}
}

func TestContext_Cookie(t *testing.T) {
	a := NewChiAdapter()
	r := a.NewRouter()

	r.GET("/cookie", func(c interfaces.Context) error {
		c.SetCookie(&http.Cookie{Name: "test", Value: "val"})
		return c.Text(200, "ok")
	})

	router := r.(*ChiRouter).Router()
	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/cookie")
	defer resp.Body.Close()

	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected cookie in response")
	}
	if cookies[0].Name != "test" || cookies[0].Value != "val" {
		t.Fatalf("unexpected cookie: %+v", cookies[0])
	}
}

func TestContext_Redirect(t *testing.T) {
	a := NewChiAdapter()
	r := a.NewRouter()

	r.GET("/redirect", func(c interfaces.Context) error {
		return c.Redirect(302, "/target")
	})

	router := r.(*ChiRouter).Router()
	srv := httptest.NewServer(router)
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

func TestContext_HTML(t *testing.T) {
	a := NewChiAdapter()
	r := a.NewRouter()

	r.GET("/html", func(c interfaces.Context) error {
		return c.HTML(200, "<h1>hello</h1>")
	})

	router := r.(*ChiRouter).Router()
	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/html")
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/html" {
		t.Fatalf("expected text/html, got %s", ct)
	}
}

func TestContext_SetGet(t *testing.T) {
	a := NewChiAdapter()
	r := a.NewRouter()

	r.GET("/setget", func(c interfaces.Context) error {
		c.Set("key", "val")
		if v := c.Get("key"); v != "val" {
			t.Errorf("expected 'val', got '%v'", v)
		}
		return c.Text(200, "ok")
	})

	router := r.(*ChiRouter).Router()
	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/setget")
	resp.Body.Close()
}

func TestContext_QueryParam(t *testing.T) {
	a := NewChiAdapter()
	r := a.NewRouter()

	r.GET("/query", func(c interfaces.Context) error {
		return c.Text(200, c.QueryParam("q"))
	})

	router := r.(*ChiRouter).Router()
	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/query?q=testval")
	defer resp.Body.Close()
}

func TestContext_BindJSON(t *testing.T) {
	a := NewChiAdapter()
	r := a.NewRouter()

	type body struct {
		Name string `json:"name"`
	}

	r.POST("/bind", func(c interfaces.Context) error {
		var b body
		if err := c.BindJSON(&b); err != nil {
			return c.Text(500, err.Error())
		}
		return c.JSON(200, b)
	})

	router := r.(*ChiRouter).Router()
	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, _ := http.Post(srv.URL+"/bind", "application/json", nil) // just test no panic
	resp.Body.Close()
}

func TestRouter_SetLogger(t *testing.T) {
	a := NewChiAdapter()
	r := a.NewRouter()
	cr := r.(*ChiRouter)
	oldLogger := cr.logger
	cr.SetLogger(nil)
	cr.SetLogger(oldLogger)
}

func TestRouter_Static(t *testing.T) {
	a := NewChiAdapter()
	r := a.NewRouter()
	r.Static("/static", ".")
}
