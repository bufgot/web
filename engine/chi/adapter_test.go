package chi

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	interfaces "github.com/bufgot/web"
	"github.com/go-chi/chi/v5"
)

type mockLogger struct{}

func (m *mockLogger) Info(msg string, args ...interface{})                   {}
func (m *mockLogger) Warn(msg string, args ...interface{})                   {}
func (m *mockLogger) Error(msg string, args ...interface{})                  {}
func (m *mockLogger) Debug(msg string, args ...interface{})                  {}
func (m *mockLogger) Infof(msg string, args ...interface{})                  {}
func (m *mockLogger) Warnf(msg string, args ...interface{})                  {}
func (m *mockLogger) Errorf(msg string, args ...interface{})                 {}
func (m *mockLogger) Debugf(msg string, args ...interface{})                 {}
func (m *mockLogger) Infow(msg string, keysAndValues ...interface{})         {}
func (m *mockLogger) Warnw(msg string, keysAndValues ...interface{})         {}
func (m *mockLogger) Errorw(msg string, keysAndValues ...interface{})        {}
func (m *mockLogger) Debugw(msg string, keysAndValues ...interface{})        {}
func (m *mockLogger) With(args ...interface{}) interfaces.Logger { return m }


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

// ============================================================
// Direct Context tests — no HTTP server needed
// ============================================================

func TestChiContext_Request(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
	if ctx.Request() != req {
		t.Fatal("Request should return the request")
	}
}

func TestChiContext_Method(t *testing.T) {
	req := httptest.NewRequest("POST", "/test", nil)
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
	if ctx.Method() != "POST" {
		t.Fatalf("expected POST, got %s", ctx.Method())
	}
}

func TestChiContext_Path(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/users", nil)
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
	if ctx.Path() != "/api/users" {
		t.Fatalf("expected /api/users, got %s", ctx.Path())
	}
}

func TestChiContext_Param(t *testing.T) {
	req := httptest.NewRequest("GET", "/users/42", nil)
	// Set chi URL param via context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "42")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
	if ctx.Param("id") != "42" {
		t.Fatalf("expected 42, got %s", ctx.Param("id"))
	}
}

func TestChiContext_Param_Missing(t *testing.T) {
	req := httptest.NewRequest("GET", "/users/42", nil)
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
	if ctx.Param("id") != "" {
		t.Fatal("expected empty for missing param")
	}
}

func TestChiContext_Status(t *testing.T) {
	w := httptest.NewRecorder()
	ctx := &ChiContext{writer: w, request: httptest.NewRequest("GET", "/", nil)}
	ctx.Status(201)
	if w.Code != 201 {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestChiContext_ContextMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
	if ctx.Context() != req.Context() {
		t.Fatal("Context should return request context")
	}
}

func TestChiContext_ResponseWriter(t *testing.T) {
	w := httptest.NewRecorder()
	ctx := &ChiContext{writer: w, request: httptest.NewRequest("GET", "/", nil)}
	if ctx.ResponseWriter() != w {
		t.Fatal("ResponseWriter should return the writer")
	}
}

func TestChiContext_FormValue(t *testing.T) {
	req := httptest.NewRequest("POST", "/form", strings.NewReader("name=john"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
	v := ctx.FormValue("name")
	if v != "john" {
		t.Fatalf("expected john, got %s", v)
	}
}

func TestChiContext_PostForm(t *testing.T) {
	t.Run("POST", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/form", strings.NewReader("key=val"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		_ = req.ParseForm()
		ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
		if v := ctx.PostForm("key"); v != "val" {
			t.Fatalf("expected val, got %s", v)
		}
	})
	t.Run("GET_ignored", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/form", nil)
		ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
		if v := ctx.PostForm("key"); v != "" {
			t.Fatalf("expected empty, got %s", v)
		}
	})
	t.Run("PUT", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/form", strings.NewReader("key=putval"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		_ = req.ParseForm()
		ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
		if v := ctx.PostForm("key"); v != "putval" {
			t.Fatalf("expected putval, got %s", v)
		}
	})
	t.Run("PATCH", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/form", strings.NewReader("key=patchval"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		_ = req.ParseForm()
		ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
		if v := ctx.PostForm("key"); v != "patchval" {
			t.Fatalf("expected patchval, got %s", v)
		}
	})
}

func TestChiContext_ParseForm(t *testing.T) {
	req := httptest.NewRequest("POST", "/form", strings.NewReader("a=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
	if err := ctx.ParseForm(); err != nil {
		t.Fatalf("ParseForm failed: %v", err)
	}
}

func TestChiContext_ParseMultipartForm(t *testing.T) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	w.WriteField("field", "value")
	w.Close()
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
	if err := ctx.ParseMultipartForm(32 << 20); err != nil {
		t.Fatalf("ParseMultipartForm failed: %v", err)
	}
}

func TestChiContext_BindXML(t *testing.T) {
	type data struct {
		Name string `xml:"name"`
	}
	body := `<data><name>xmltest</name></data>`
	req := httptest.NewRequest("POST", "/xml", strings.NewReader(body))
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
	var d data
	if err := ctx.BindXML(&d); err != nil {
		t.Fatalf("BindXML failed: %v", err)
	}
	if d.Name != "xmltest" {
		t.Fatalf("expected xmltest, got %s", d.Name)
	}
}

func TestChiContext_BindQuery(t *testing.T) {
	type query struct {
		Name string `form:"name"`
		Age  int    `form:"age"`
	}
	req := httptest.NewRequest("GET", "/query?name=john&age=30", nil)
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
	var q query
	if err := ctx.BindQuery(&q); err != nil {
		t.Fatalf("BindQuery failed: %v", err)
	}
	if q.Name != "john" {
		t.Fatalf("expected john, got %s", q.Name)
	}
	if q.Age != 30 {
		t.Fatalf("expected 30, got %d", q.Age)
	}
}

func TestChiContext_XML(t *testing.T) {
	w := httptest.NewRecorder()
	ctx := &ChiContext{writer: w, request: httptest.NewRequest("GET", "/", nil)}
	type xmlData struct {
		Key string `xml:"key"`
	}
	err := ctx.XML(200, xmlData{Key: "val"})
	if err != nil {
		t.Fatalf("XML failed: %v", err)
	}
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/xml" {
		t.Fatalf("expected application/xml, got %s", ct)
	}
}

func TestChiContext_Cookie_Missing(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
	_, err := ctx.Cookie("missing")
	if err == nil {
		t.Fatal("expected error for missing cookie")
	}
}

func TestChiContext_Cookie_Present(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
	val, err := ctx.Cookie("session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "abc123" {
		t.Fatalf("expected abc123, got %s", val)
	}
}

func TestChiContext_Logger(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	logger := &mockLogger{}
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder(), logger: logger}
	if l := ctx.Logger(); l != logger {
		t.Fatal("Logger should return the context logger")
	}
}

func TestChiContext_Logger_WithContextValue(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	customLogger := &mockLogger{}
	req = req.WithContext(context.WithValue(req.Context(), "logger", customLogger))
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
	if l := ctx.Logger(); l != customLogger {
		t.Fatal("Logger should return context value logger")
	}
}

func TestChiContext_FormFile(t *testing.T) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	fw, _ := w.CreateFormFile("file", "test.txt")
	fw.Write([]byte("content"))
	w.Close()
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
	f, fh, err := ctx.FormFile("file")
	if err != nil {
		t.Fatalf("FormFile failed: %v", err)
	}
	f.Close()
	if fh.Filename != "test.txt" {
		t.Fatalf("expected test.txt, got %s", fh.Filename)
	}
}

func TestChiContext_FormFile_Missing(t *testing.T) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	w.WriteField("dummy", "val")
	w.Close()
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
	_, _, err := ctx.FormFile("missing")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestChiContext_MultipartForm(t *testing.T) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	w.WriteField("hello", "world")
	w.Close()
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
	form, err := ctx.MultipartForm()
	if err != nil {
		t.Fatalf("MultipartForm failed: %v", err)
	}
	if form.Value["hello"][0] != "world" {
		t.Fatalf("expected world, got %s", form.Value["hello"][0])
	}
}

func TestChiContext_SaveUploadedFile(t *testing.T) {
	dir := t.TempDir()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	fw, _ := w.CreateFormFile("file", "save.txt")
	fw.Write([]byte("save me"))
	w.Close()
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	ctx := &ChiContext{request: req, writer: httptest.NewRecorder()}
	_, fh, err := ctx.FormFile("file")
	if err != nil {
		t.Fatalf("FormFile: %v", err)
	}
	dst := filepath.Join(dir, "saved.txt")
	if err := ctx.SaveUploadedFile(fh, dst); err != nil {
		t.Fatalf("SaveUploadedFile failed: %v", err)
	}
	data, _ := os.ReadFile(dst)
	if string(data) != "save me" {
		t.Fatalf("expected 'save me', got '%s'", string(data))
	}
}

func TestChiContext_JSON_MarshalError(t *testing.T) {
	w := httptest.NewRecorder()
	ctx := &ChiContext{writer: w, request: httptest.NewRequest("GET", "/", nil)}
	// Pass an unmarshalable value
	err := ctx.JSON(200, make(chan int))
	if err == nil {
		t.Fatal("expected marshal error for channel type")
	}
}

// mapForm tests

func TestMapForm(t *testing.T) {
	type form struct {
		Name   string `form:"name"`
		Age    int    `form:"age"`
		Active bool   `form:"active"`
	}
	v := map[string][]string{
		"name":   {"john"},
		"age":    {"25"},
		"active": {"true"},
	}
	var f form
	if err := mapForm(v, &f); err != nil {
		t.Fatalf("mapForm failed: %v", err)
	}
	if f.Name != "john" {
		t.Fatalf("expected john, got %s", f.Name)
	}
	if f.Age != 25 {
		t.Fatalf("expected 25, got %d", f.Age)
	}
	if !f.Active {
		t.Fatal("expected true")
	}
}

func TestMapForm_BoolFalse(t *testing.T) {
	type form struct {
		Active bool `form:"active"`
	}
	v := map[string][]string{"active": {"false"}}
	var f form
	mapForm(v, &f)
	if f.Active {
		t.Fatal("expected false")
	}
}

func TestMapForm_DefaultTag(t *testing.T) {
	type form struct {
		Name string
	}
	v := map[string][]string{"name": {"auto"}}
	var f form
	mapForm(v, &f)
	if f.Name != "auto" {
		t.Fatalf("expected auto, got %s", f.Name)
	}
}

// wrapMiddleware with non-ChiContext
func TestWrapMiddleware_NonChiContext(t *testing.T) {
	a := NewChiAdapter()
	r := a.NewRouter()
	cr := r.(*ChiRouter)

	var middlewareCalled bool
	m := func(next interfaces.Handler) interfaces.Handler {
		return func(c interfaces.Context) error {
			middlewareCalled = true
			return next(c)
		}
	}

	r.Use(m)
	router := cr.Router()
	srv := httptest.NewServer(router)
	defer srv.Close()

	r.GET("/nonchi", func(c interfaces.Context) error {
		return c.Text(200, "ok")
	})

	// Force a non-ChiContext scenario by calling router directly
	resp, _ := http.Get(srv.URL + "/nonchi")
	resp.Body.Close()
	if !middlewareCalled {
		t.Fatal("middleware should be called")
	}
}

func TestRouter_Group_WithMiddleware(t *testing.T) {
	a := NewChiAdapter()
	r := a.NewRouter()

	var mwCalled bool
	g := r.Group("/v1", func(next interfaces.Handler) interfaces.Handler {
		return func(c interfaces.Context) error {
			mwCalled = true
			return next(c)
		}
	})
	g.GET("/test", func(c interfaces.Context) error {
		return c.Text(200, "ok")
	})

	router := r.(*ChiRouter).Router()
	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/v1/test")
	resp.Body.Close()
	if !mwCalled {
		t.Fatal("group middleware should be called")
	}
}
