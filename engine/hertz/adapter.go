package hertz

import (
	"bytes"
	"context"
	"net/http"
	"strings"

	interfaces "github.com/bufgot/web"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// HertzAdapter implements the WebFramework interface
type HertzAdapter struct{}

// NewHertzAdapter creates a Hertz adapter
func NewHertzAdapter() *HertzAdapter {
	return &HertzAdapter{}
}

// Name returns the framework name
func (h *HertzAdapter) Name() string {
	return "hertz"
}

// NewRouter creates a new router
func (h *HertzAdapter) NewRouter() interfaces.Router {
	return New()
}

// HertzRouter adapts Hertz's router
type HertzRouter struct {
	routes      map[string]map[string]interfaces.Handler
	middlewares []interfaces.Middleware
	server      *server.Hertz
	prefix      string            // route group prefix
	staticPaths map[string]string // static file path mappings
	logger      interfaces.Logger
}

// New creates a new router instance
func New() *HertzRouter {
	return &HertzRouter{
		routes:      make(map[string]map[string]interfaces.Handler),
		staticPaths: make(map[string]string),
		logger:      interfaces.NewDefaultLogger(),
	}
}

// GET registers a GET route
func (r *HertzRouter) addRoute(method string, path string, handler interfaces.Handler) {
	if r.routes[method] == nil {
		r.routes[method] = make(map[string]interfaces.Handler)
	}
	// apply route group prefix
	fullPath := r.prefix + path
	r.routes[method][fullPath] = handler
}

// GET registers a GET route
func (r *HertzRouter) GET(path string, handler interfaces.Handler) {
	r.addRoute("GET", path, handler)
}

// POST registers a POST route
func (r *HertzRouter) POST(path string, handler interfaces.Handler) {
	r.addRoute("POST", path, handler)
}

// PUT registers a PUT route
func (r *HertzRouter) PUT(path string, handler interfaces.Handler) {
	r.addRoute("PUT", path, handler)
}

// DELETE registers a DELETE route
func (r *HertzRouter) DELETE(path string, handler interfaces.Handler) {
	r.addRoute("DELETE", path, handler)
}

// PATCH registers a PATCH route
func (r *HertzRouter) PATCH(path string, handler interfaces.Handler) {
	r.addRoute("PATCH", path, handler)
}

// HEAD registers a HEAD route
func (r *HertzRouter) HEAD(path string, handler interfaces.Handler) {
	r.addRoute("HEAD", path, handler)
}

// OPTIONS registers an OPTIONS route
func (r *HertzRouter) OPTIONS(path string, handler interfaces.Handler) {
	r.addRoute("OPTIONS", path, handler)
}

// Use adds middleware
func (r *HertzRouter) Use(middleware interfaces.Middleware) {
	r.middlewares = append(r.middlewares, middleware)
}

// Start starts the server
func (r *HertzRouter) Start(addr string) error {
	if strings.HasPrefix(addr, ":") {
		addr = "0.0.0.0" + addr
	}
	r.server = server.New(server.WithHostPorts(addr))

	for _, middleware := range r.middlewares {
		r.server.Use(r.wrapMiddleware(middleware))
	}

	for method, handlers := range r.routes {
		for path, handler := range handlers {
			switch method {
			case "GET":
				r.server.GET(path, r.wrapHandler(handler))
			case "POST":
				r.server.POST(path, r.wrapHandler(handler))
			case "PUT":
				r.server.PUT(path, r.wrapHandler(handler))
			case "DELETE":
				r.server.DELETE(path, r.wrapHandler(handler))
			case "PATCH":
				r.server.PATCH(path, r.wrapHandler(handler))
			case "HEAD":
				r.server.HEAD(path, r.wrapHandler(handler))
			case "OPTIONS":
				r.server.OPTIONS(path, r.wrapHandler(handler))
			}
		}
	}

	for prefix, root := range r.staticPaths {
		r.server.GET(prefix+"/*filepath", func(ctx context.Context, c *app.RequestContext) {
			filepath := c.Param("filepath")
			c.File(root + "/" + filepath)
		})
	}

	r.server.Spin()
	return nil
}

// Group creates a route group
func (r *HertzRouter) Group(prefix string, middlewares ...interfaces.Middleware) interfaces.Router {
	group := &HertzRouter{
		routes:      r.routes, // share the root router's route table
		middlewares: append(r.middlewares, middlewares...),
		prefix:      r.prefix + prefix, // concatenate prefix
		logger:      r.logger,
	}
	return group
}

// Static serves static files
func (r *HertzRouter) Static(prefix, root string) {
	r.staticPaths[prefix] = root
}

// SetLogger sets the logger
func (r *HertzRouter) SetLogger(logger interfaces.Logger) {
	r.logger = logger
}

// wrapHandler wraps the unified handler as a Hertz handler
func (r *HertzRouter) wrapHandler(h interfaces.Handler) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		shimCtx := &HertzContext{context: c, ctx: ctx, logger: r.logger}
		h(shimCtx)
	}
}

// wrapMiddleware wraps the unified middleware as a Hertz middleware
func (r *HertzRouter) wrapMiddleware(m interfaces.Middleware) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		shimCtx := &HertzContext{context: c, ctx: ctx, logger: r.logger}
		handler := m(func(inner interfaces.Context) error {
			c.Next(ctx)
			return nil
		})
		handler(shimCtx)
	}
}

// HertzContext adapts Hertz's context
type HertzContext struct {
	context *app.RequestContext
	ctx     context.Context
	logger  interfaces.Logger
}

// Request returns the HTTP request
func (c *HertzContext) Request() *http.Request {
	method := string(c.context.Method())
	uri := c.context.URI().String()
	body := c.context.GetRawData()
	req, _ := http.NewRequest(method, uri, bytes.NewReader(body))
	c.context.Request.Header.VisitAll(func(key, value []byte) {
		req.Header.Add(string(key), string(value))
	})
	return req
}

// Method returns the request method
func (c *HertzContext) Method() string {
	return string(c.context.Method())
}

// Path returns the request path
func (c *HertzContext) Path() string {
	return string(c.context.Path())
}

// QueryParam retrieves a query parameter
func (c *HertzContext) QueryParam(name string) string {
	return string(c.context.Query(name))
}

// Param retrieves a path parameter
func (c *HertzContext) Param(name string) string {
	return string(c.context.Param(name))
}

// Status sets the status code
func (c *HertzContext) Status(code int) {
	c.context.SetStatusCode(code)
}

// JSON returns a JSON response
func (c *HertzContext) JSON(code int, obj interface{}) error {
	c.context.JSON(code, obj)
	return nil
}

// Text returns a text response
func (c *HertzContext) Text(code int, text string) error {
	c.context.String(code, text)
	return nil
}

// HTML returns an HTML response
func (c *HertzContext) HTML(code int, html string) error {
	c.context.SetContentType("text/html")
	c.context.String(code, html)
	return nil
}

// Redirect performs a redirect
func (c *HertzContext) Redirect(code int, url string) error {
	c.context.Redirect(code, []byte(url))
	return nil
}

// Set stores a key-value pair
func (c *HertzContext) Set(key string, value interface{}) {
	c.context.Set(key, value)
}

// Logger returns the logger
func (c *HertzContext) Logger() interfaces.Logger {
	if logger, ok := c.Get("logger").(interfaces.Logger); ok && logger != nil {
		return logger
	}
	return c.logger
}

// Get retrieves a stored value
func (c *HertzContext) Get(key string) interface{} {
	val, _ := c.context.Get(key)
	return val
}

// Context returns the Go context
func (c *HertzContext) Context() context.Context {
	return c.ctx
}

// ResponseWriter returns the underlying http.ResponseWriter.
// Hertz does not provide http.ResponseWriter; returns nil.
func (c *HertzContext) ResponseWriter() http.ResponseWriter { return nil }

// SetResponseWriter is a no-op for hertz (no http.ResponseWriter).
func (c *HertzContext) SetResponseWriter(w http.ResponseWriter) {}

// BindJSON binds the JSON request body
func (c *HertzContext) BindJSON(obj interface{}) error {
	return c.context.BindJSON(obj)
}

// BindXML binds the XML request body
func (c *HertzContext) BindXML(obj interface{}) error {
	// Hertz has no built-in BindXML; simplified implementation
	return nil
}

// BindQuery binds query parameters to a struct
func (c *HertzContext) BindQuery(obj interface{}) error {
	// Hertz has no built-in BindQuery; simplified implementation
	return nil
}

// Cookie retrieves a cookie
func (c *HertzContext) Cookie(name string) (string, error) {
	value := c.context.Cookie(name)
	if len(value) == 0 {
		return "", http.ErrNoCookie
	}
	return string(value), nil
}

// SetCookie sets a cookie
func (c *HertzContext) SetCookie(cookie *http.Cookie) {
	c.context.SetCookie(cookie.Name, cookie.Value, cookie.MaxAge, cookie.Path, cookie.Domain, 0, cookie.Secure, cookie.HttpOnly)
}

// XML returns an XML response
func (c *HertzContext) XML(code int, obj interface{}) error {
	c.context.XML(code, obj)
	return nil
}

// FormValue retrieves a form field value
func (c *HertzContext) FormValue(key string) string {
	return string(c.context.FormValue(key))
}

// PostForm retrieves a POST form field value
func (c *HertzContext) PostForm(key string) string {
	return string(c.context.PostForm(key))
}

// ParseForm parses the form
func (c *HertzContext) ParseForm() error {
	// Hertz auto-parses forms; no extra action needed
	return nil
}

// ParseMultipartForm parses the multipart form
func (c *HertzContext) ParseMultipartForm(maxMemory int64) error {
	// Hertz auto-handles multipart forms; no extra action needed
	return nil
}
