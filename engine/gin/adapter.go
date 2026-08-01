package gin

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"

	interfaces "github.com/bufgot/web"
	"github.com/gin-gonic/gin"
)

// GinAdapter implements the WebFramework interface
type GinAdapter struct{}

// NewGinAdapter creates a Gin adapter
func NewGinAdapter() *GinAdapter {
	return &GinAdapter{}
}

// Name returns the framework name
func (g *GinAdapter) Name() string {
	return "gin"
}

// NewRouter creates a new router
func (g *GinAdapter) NewRouter() interfaces.Router {
	return &GinRouter{
		gin:    gin.New(),
		group:  nil, // no route group initially
		logger: interfaces.NewDefaultLogger(),
	}
}

// GinRouter adapts Gin's router
type GinRouter struct {
	gin    *gin.Engine
	group  *gin.RouterGroup // current route group; if nil, gin.Engine is used
	logger interfaces.Logger
}

// currentRouter returns the currently active router
func (r *GinRouter) currentRouter() interface{} {
	if r.group != nil {
		return r.group
	}
	return r.gin
}

// addRoute adds a route to the current router
func (r *GinRouter) addRoute(method, path string, handler interfaces.Handler) {
	if r.group != nil {
		r.group.Handle(method, path, r.wrapHandler(handler))
	} else {
		r.gin.Handle(method, path, r.wrapHandler(handler))
	}
}

// addUse adds middleware to the current router
func (r *GinRouter) addUse(middleware interfaces.Middleware) {
	if r.group != nil {
		r.group.Use(r.wrapMiddleware(middleware))
	} else {
		r.gin.Use(r.wrapMiddleware(middleware))
	}
}

// GET registers a GET route
func (r *GinRouter) GET(path string, handler interfaces.Handler) {
	r.addRoute("GET", path, handler)
}

// POST registers a POST route
func (r *GinRouter) POST(path string, handler interfaces.Handler) {
	r.addRoute("POST", path, handler)
}

// PUT registers a PUT route
func (r *GinRouter) PUT(path string, handler interfaces.Handler) {
	r.addRoute("PUT", path, handler)
}

// DELETE registers a DELETE route
func (r *GinRouter) DELETE(path string, handler interfaces.Handler) {
	r.addRoute("DELETE", path, handler)
}

// PATCH registers a PATCH route
func (r *GinRouter) PATCH(path string, handler interfaces.Handler) {
	r.addRoute("PATCH", path, handler)
}

// HEAD registers a HEAD route
func (r *GinRouter) HEAD(path string, handler interfaces.Handler) {
	r.addRoute("HEAD", path, handler)
}

// OPTIONS registers an OPTIONS route
func (r *GinRouter) OPTIONS(path string, handler interfaces.Handler) {
	r.addRoute("OPTIONS", path, handler)
}

// Use adds middleware
func (r *GinRouter) Use(middleware interfaces.Middleware) {
	r.addUse(middleware)
}

// Start starts the server
func (r *GinRouter) Start(addr string) error {
	return r.gin.Run(addr)
}

// Group creates a route group
func (r *GinRouter) Group(prefix string, middlewares ...interfaces.Middleware) interfaces.Router {
	var newGroup *gin.RouterGroup
	if r.group != nil {
		newGroup = r.group.Group(prefix)
	} else {
		newGroup = r.gin.Group(prefix)
	}
	for _, middleware := range middlewares {
		newGroup.Use(r.wrapMiddleware(middleware))
	}
	return &GinRouter{
		gin:    r.gin,
		group:  newGroup,
		logger: r.logger,
	}
}

// Static serves static files
func (r *GinRouter) Static(prefix, root string) {
	if r.group != nil {
		r.group.Static(prefix, root)
	} else {
		r.gin.Static(prefix, root)
	}
}

// SetLogger sets the logger
func (r *GinRouter) SetLogger(logger interfaces.Logger) {
	r.logger = logger
}

// GetGinEngine returns the underlying gin.Engine
func (r *GinRouter) GetGinEngine() *gin.Engine {
	return r.gin
}

// wrapHandler wraps a shim.Handler as a gin.HandlerFunc
func (r *GinRouter) wrapHandler(h interfaces.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := &GinContext{context: c, logger: r.logger}
		h(ctx)
	}
}

// wrapMiddleware wraps a shim.Middleware as a gin.HandlerFunc
func (r *GinRouter) wrapMiddleware(m interfaces.Middleware) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := &GinContext{context: c}
		handler := m(func(ctx interfaces.Context) error {
			c.Next()
			return nil
		})
		handler(ctx)
	}
}

// GinContext adapts Gin's context
type GinContext struct {
	context *gin.Context
	logger  interfaces.Logger
}

// Request returns the HTTP request
func (c *GinContext) Request() *http.Request {
	return c.context.Request
}

// Method returns the request method
func (c *GinContext) Method() string {
	return c.context.Request.Method
}

// Path returns the request path
func (c *GinContext) Path() string {
	return c.context.Request.URL.Path
}

// QueryParam retrieves a query parameter
func (c *GinContext) QueryParam(name string) string {
	return c.context.Query(name)
}

// Param retrieves a path parameter
func (c *GinContext) Param(name string) string {
	return c.context.Param(name)
}

// Status sets the status code
func (c *GinContext) Status(code int) {
	c.context.Status(code)
}

// JSON returns a JSON response
func (c *GinContext) JSON(code int, obj interface{}) error {
	c.context.JSON(code, obj)
	return nil
}

// Text returns a text response
func (c *GinContext) Text(code int, text string) error {
	c.context.String(code, text)
	return nil
}

// HTML returns an HTML response
func (c *GinContext) HTML(code int, html string) error {
	c.context.HTML(code, html, nil)
	return nil
}

// Redirect performs a redirect
func (c *GinContext) Redirect(code int, url string) error {
	c.context.Redirect(code, url)
	return nil
}

// Set stores a key-value pair�
func (c *GinContext) Set(key string, value interface{}) {
	c.context.Set(key, value)
}

// Get retrieves a stored value�?
func (c *GinContext) Get(key string) interface{} {
	val, _ := c.context.Get(key)
	return val
}

// Context returns the Go context
func (c *GinContext) Context() context.Context {
	return c.context.Request.Context()
}

// ResponseWriter returns the underlying http.ResponseWriter.
func (c *GinContext) ResponseWriter() http.ResponseWriter { return c.context.Writer }

// SetResponseWriter replaces the underlying http.ResponseWriter.
func (c *GinContext) SetResponseWriter(w http.ResponseWriter) {
	if gw, ok := w.(gin.ResponseWriter); ok {
		c.context.Writer = gw
		return
	}
	// Wrap plain http.ResponseWriter into gin.ResponseWriter.
	c.context.Writer = &ginWriterAdapter{ResponseWriter: w}
}

// BindJSON binds the JSON request�?
func (c *GinContext) BindJSON(obj interface{}) error {
	return c.context.ShouldBindJSON(obj)
}

// BindXML binds the XML request�?
func (c *GinContext) BindXML(obj interface{}) error {
	return c.context.ShouldBindXML(obj)
}

// BindQuery binds query parameters to a struct
func (c *GinContext) BindQuery(obj interface{}) error {
	return c.context.ShouldBindQuery(obj)
}

// Cookie retrieves a cookie
func (c *GinContext) Cookie(name string) (string, error) {
	return c.context.Cookie(name)
}

// SetCookie sets a cookie
func (c *GinContext) SetCookie(cookie *http.Cookie) {
	c.context.SetCookie(cookie.Name, cookie.Value, cookie.MaxAge, cookie.Path, cookie.Domain, cookie.Secure, cookie.HttpOnly)
}

// Logger returns the logger
func (c *GinContext) Logger() interfaces.Logger {
	if logger, ok := c.Get("logger").(interfaces.Logger); ok && logger != nil {
		return logger
	}
	return c.logger
}

// XML returns an XML response
func (c *GinContext) XML(code int, obj interface{}) error {
	c.context.XML(code, obj)
	return nil
}

// FormValue retrieves a form field value
func (c *GinContext) FormValue(key string) string {
	return c.context.Request.FormValue(key)
}

// PostForm retrieves a POST form field value
func (c *GinContext) PostForm(key string) string {
	return c.context.PostForm(key)
}

// ParseForm parses the form
func (c *GinContext) ParseForm() error {
	return c.context.Request.ParseForm()
}

// ParseMultipartForm parses the multipart form
func (c *GinContext) ParseMultipartForm(maxMemory int64) error {
	return c.context.Request.ParseMultipartForm(maxMemory)
}

// ginWriterAdapter wraps an http.ResponseWriter to satisfy gin.ResponseWriter.
type ginWriterAdapter struct {
	http.ResponseWriter
	status int
	size   int
}

func (w *ginWriterAdapter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *ginWriterAdapter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.size += n
	return n, err
}

func (w *ginWriterAdapter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *ginWriterAdapter) WriteHeaderNow() {}

func (w *ginWriterAdapter) Status() int {
	if w.status == 0 {
		return 200
	}
	return w.status
}

func (w *ginWriterAdapter) Size() int     { return w.size }
func (w *ginWriterAdapter) Written() bool { return w.size > 0 || w.status != 0 }

func (w *ginWriterAdapter) Pusher() http.Pusher {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p
	}
	return nil
}

func (w *ginWriterAdapter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("ginWriterAdapter: underlying writer does not support Hijack")
}

func (w *ginWriterAdapter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *ginWriterAdapter) CloseNotify() <-chan bool {
	if cn, ok := w.ResponseWriter.(http.CloseNotifier); ok {
		return cn.CloseNotify()
	}
	return nil
}
