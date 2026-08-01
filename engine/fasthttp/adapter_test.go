package fasthttp

import (
	"encoding/xml"
	"net/http"
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

// ============================================================
// Direct Context tests — construct FasthttpContext directly
// ============================================================

func makeCtx(method, uri string) *FasthttpContext {
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI(uri)
	fctx.Request.Header.SetMethod(method)
	return &FasthttpContext{
		context: fctx,
		store:   make(map[string]interface{}),
		logger:  interfaces.NewDefaultLogger(),
	}
}

func TestContext_Request(t *testing.T) {
	c := makeCtx("GET", "/test")
	req := c.Request()
	if req == nil {
		t.Fatal("Request() returned nil")
	}
	if req.Method != "GET" {
		t.Fatalf("expected GET, got %s", req.Method)
	}
}

func TestContext_Method(t *testing.T) {
	c := makeCtx("POST", "/test")
	if c.Method() != "POST" {
		t.Fatalf("expected POST, got %s", c.Method())
	}
}

func TestContext_Path(t *testing.T) {
	c := makeCtx("GET", "/api/users")
	if c.Path() != "/api/users" {
		t.Fatalf("expected /api/users, got %s", c.Path())
	}
}

func TestContext_QueryParam(t *testing.T) {
	c := makeCtx("GET", "/search?q=hello&page=1")
	v := c.QueryParam("q")
	if v != "hello" {
		t.Fatalf("expected hello, got %s", v)
	}
}

func TestContext_Param(t *testing.T) {
	c := makeCtx("GET", "/users/42")
	// fasthttp Param returns "" always (simplified impl)
	v := c.Param("id")
	if v != "" {
		t.Fatalf("expected empty string, got %s", v)
	}
}

func TestContext_Status(t *testing.T) {
	c := makeCtx("GET", "/")
	c.Status(418)
	if c.context.Response.StatusCode() != 418 {
		t.Fatalf("expected 418, got %d", c.context.Response.StatusCode())
	}
}

func TestContext_HTML(t *testing.T) {
	c := makeCtx("GET", "/")
	c.HTML(200, "<h1>Title</h1>")
	if ct := string(c.context.Response.Header.ContentType()); ct != "text/html" {
		t.Fatalf("expected text/html, got %s", ct)
	}
}

func TestContext_Redirect(t *testing.T) {
	c := makeCtx("GET", "/")
	c.Redirect(302, "/target")
	if c.context.Response.StatusCode() != 302 {
		t.Fatalf("expected 302, got %d", c.context.Response.StatusCode())
	}
}

func TestContext_SetGet(t *testing.T) {
	c := makeCtx("GET", "/")
	c.Set("key", "value")
	if c.Get("key") != "value" {
		t.Fatal("Get should return set value")
	}
}

func TestContext_Context(t *testing.T) {
	c := makeCtx("GET", "/")
	ctx := c.Context()
	if ctx == nil {
		t.Fatal("Context() returned nil")
	}
}

func TestContext_ResponseWriter(t *testing.T) {
	c := makeCtx("GET", "/")
	_ = c.ResponseWriter() // returns nil
}

func TestContext_BindJSON(t *testing.T) {
	c := makeCtx("POST", "/")
	c.context.Request.SetBody([]byte(`{"name":"john"}`))
	var data map[string]string
	if err := c.BindJSON(&data); err != nil {
		t.Fatalf("BindJSON: %v", err)
	}
	if data["name"] != "john" {
		t.Fatalf("expected john, got %s", data["name"])
	}
}

func TestContext_BindXML(t *testing.T) {
	type xmlData struct {
		XMLName xml.Name `xml:"data"`
		Name    string   `xml:"name"`
	}
	c := makeCtx("POST", "/")
	c.context.Request.SetBody([]byte(`<data><name>xmltest</name></data>`))
	var data xmlData
	if err := c.BindXML(&data); err != nil {
		t.Fatalf("BindXML: %v", err)
	}
	if data.Name != "xmltest" {
		t.Fatalf("expected xmltest, got %s", data.Name)
	}
}

func TestContext_BindQuery(t *testing.T) {
	c := makeCtx("GET", "/?a=1&b=2")
	_ = c.BindQuery(nil) // stub, always nil
}

func TestContext_FormFile(t *testing.T) {
	c := makeCtx("POST", "/upload")
	_, _, err := c.FormFile("file")
	if err == nil {
		t.Fatal("expected error for FormFile")
	}
}

func TestContext_MultipartForm(t *testing.T) {
	c := makeCtx("POST", "/upload")
	form, err := c.MultipartForm()
	if err != nil || form != nil {
		t.Fatal("expected nil, nil from MultipartForm")
	}
}

func TestContext_SaveUploadedFile(t *testing.T) {
	c := makeCtx("POST", "/upload")
	err := c.SaveUploadedFile(nil, "/tmp/out.txt")
	if err != nil {
		t.Fatal("SaveUploadedFile should return nil")
	}
}

func TestContext_Cookie(t *testing.T) {
	c := makeCtx("GET", "/")
	c.context.Request.Header.SetCookie("session", "abc123")
	val, err := c.Cookie("session")
	if err != nil {
		t.Fatalf("Cookie: %v", err)
	}
	if val != "abc123" {
		t.Fatalf("expected abc123, got %s", val)
	}
}

func TestContext_SetCookie(t *testing.T) {
	c := makeCtx("GET", "/")
	c.SetCookie(&http.Cookie{
		Name:     "token",
		Value:    "xyz",
		Path:     "/",
		MaxAge:   3600,
		Secure:   true,
		HttpOnly: true,
	})
}

func TestContext_Logger(t *testing.T) {
	c := makeCtx("GET", "/")
	l := c.Logger()
	if l == nil {
		t.Fatal("Logger() returned nil")
	}
}

func TestContext_Logger_WithSet(t *testing.T) {
	c := makeCtx("GET", "/")
	c.Set("logger", "not-a-logger")
	l := c.Logger()
	if l == nil {
		t.Fatal("Logger() should fall back to default")
	}
}

func TestContext_XML(t *testing.T) {
	type xmlResp struct {
		XMLName xml.Name `xml:"response"`
		Status  string   `xml:"status"`
	}
	c := makeCtx("GET", "/xml")
	c.XML(200, xmlResp{Status: "ok"})
	if ct := string(c.context.Response.Header.ContentType()); ct != "application/xml" {
		t.Fatalf("expected application/xml, got %s", ct)
	}
}

func TestContext_FormValue(t *testing.T) {
	c := makeCtx("POST", "/form")
	c.context.Request.Header.SetMethod("POST")
	c.context.PostArgs().Set("field", "hello")
	v := c.FormValue("field")
	// In fasthttp, FormValue reads from both query and post args
	t.Logf("FormValue field=%q", v)
}

func TestContext_PostForm(t *testing.T) {
	c := makeCtx("POST", "/form")
	c.context.Request.Header.SetMethod("POST")
	c.context.PostArgs().Set("key", "val")
	v := c.PostForm("key")
	if v != "val" {
		t.Fatalf("expected val, got %s", v)
	}
}

func TestContext_ParseForm(t *testing.T) {
	c := makeCtx("POST", "/form")
	if err := c.ParseForm(); err != nil {
		t.Fatalf("ParseForm: %v", err)
	}
}

func TestContext_ParseMultipartForm(t *testing.T) {
	c := makeCtx("POST", "/upload")
	if err := c.ParseMultipartForm(32 << 20); err != nil {
		t.Fatalf("ParseMultipartForm: %v", err)
	}
}

func TestContext_JSON_Error(t *testing.T) {
	c := makeCtx("GET", "/")
	err := c.JSON(200, make(chan int))
	if err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestContext_XML_Error(t *testing.T) {
	c := makeCtx("GET", "/")
	err := c.XML(200, make(chan int))
	if err == nil {
		t.Fatal("expected marshal error")
	}
}

// Test handleRequest with middleware and static
func TestRouter_HandleRequest_Middleware(t *testing.T) {
	a := NewFasthttpAdapter()
	r := a.NewRouter()

	var order []string
	r.Use(func(next interfaces.Handler) interfaces.Handler {
		return func(c interfaces.Context) error {
			order = append(order, "mw")
			return next(c)
		}
	})
	r.GET("/mw", func(c interfaces.Context) error {
		order = append(order, "handler")
		return c.Text(200, "ok")
	})

	fr := r.(*FasthttpRouter)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/mw")
	ctx.Request.Header.SetMethod("GET")
	fr.handleRequest(ctx)

	if len(order) != 2 || order[0] != "mw" || order[1] != "handler" {
		t.Fatalf("expected [mw handler], got %v", order)
	}
}
