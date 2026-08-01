package fiber

import (
	"bytes"
	"context"
	"net/http"

	interfaces "github.com/bufgot/web"
	"github.com/gofiber/fiber/v2"
)

// FiberAdapter implements the WebFramework interface
type FiberAdapter struct{}

// NewFiberAdapter creates a Fiber adapter
func NewFiberAdapter() *FiberAdapter {
	return &FiberAdapter{}
}

// Name returns the framework name
func (f *FiberAdapter) Name() string {
	return "fiber"
}

// NewRouter creates a new router
func (f *FiberAdapter) NewRouter() interfaces.Router {
	return &FiberRouter{
		app:    fiber.New(),
		group:  nil, // no route group initially
		logger: interfaces.NewDefaultLogger(),
	}
}

// FiberRouter adapts Fiber's router
type FiberRouter struct {
	app    *fiber.App
	group  fiber.Router // current route group; if nil, app is used
	logger interfaces.Logger
}

// currentRouter returns the currently active router
func (r *FiberRouter) currentRouter() fiber.Router {
	if r.group != nil {
		return r.group
	}
	return r.app
}

// addRoute adds a route to the current router
func (r *FiberRouter) addRoute(method, path string, handler interfaces.Handler) {
	r.currentRouter().Add(method, path, r.wrapHandler(handler))
}

// addUse adds middleware to the current router
func (r *FiberRouter) addUse(middleware interfaces.Middleware) {
	r.currentRouter().Use(r.wrapMiddleware(middleware))
}

// GET registers a GET route
func (r *FiberRouter) GET(path string, handler interfaces.Handler) {
	r.addRoute("GET", path, handler)
}

// POST registers a POST route
func (r *FiberRouter) POST(path string, handler interfaces.Handler) {
	r.addRoute("POST", path, handler)
}

// PUT registers a PUT route
func (r *FiberRouter) PUT(path string, handler interfaces.Handler) {
	r.addRoute("PUT", path, handler)
}

// DELETE registers a DELETE route
func (r *FiberRouter) DELETE(path string, handler interfaces.Handler) {
	r.addRoute("DELETE", path, handler)
}

// PATCH registers a PATCH route
func (r *FiberRouter) PATCH(path string, handler interfaces.Handler) {
	r.addRoute("PATCH", path, handler)
}

// HEAD registers a HEAD route
func (r *FiberRouter) HEAD(path string, handler interfaces.Handler) {
	r.addRoute("HEAD", path, handler)
}

// OPTIONS registers an OPTIONS route
func (r *FiberRouter) OPTIONS(path string, handler interfaces.Handler) {
	r.addRoute("OPTIONS", path, handler)
}

// Use adds middleware
func (r *FiberRouter) Use(middleware interfaces.Middleware) {
	r.addUse(middleware)
}

// Start starts the server
func (r *FiberRouter) Start(addr string) error {
	return r.app.Listen(addr)
}

// Group creates a route group
func (r *FiberRouter) Group(prefix string, middlewares ...interfaces.Middleware) interfaces.Router {
	newGroup := r.currentRouter().Group(prefix)
	for _, middleware := range middlewares {
		newGroup.Use(r.wrapMiddleware(middleware))
	}
	return &FiberRouter{
		app:    r.app,
		group:  newGroup,
		logger: r.logger,
	}
}

// Static serves static files
func (r *FiberRouter) Static(prefix, root string) {
	r.currentRouter().Static(prefix, root)
}

// SetLogger sets the logger
func (r *FiberRouter) SetLogger(logger interfaces.Logger) {
	r.logger = logger
}

// wrapHandler wraps a shim.Handler as a fiber.Handler
func (r *FiberRouter) wrapHandler(h interfaces.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := &FiberContext{context: c, logger: r.logger}
		return h(ctx)
	}
}

// wrapMiddleware wraps a shim.Middleware as a fiber.Handler
func (r *FiberRouter) wrapMiddleware(m interfaces.Middleware) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := &FiberContext{context: c, logger: r.logger}
		handler := m(func(ctx interfaces.Context) error {
			return c.Next()
		})
		return handler(ctx)
	}
}

// FiberContext adapts Fiber's context
type FiberContext struct {
	context *fiber.Ctx
	logger  interfaces.Logger
}

// Request returns the HTTP request
func (c *FiberContext) Request() *http.Request {
	req, _ := http.NewRequest(
		c.context.Method(),
		c.context.OriginalURL(),
		bytes.NewReader(c.context.Body()),
	)
	c.context.Request().Header.VisitAll(func(key, value []byte) {
		req.Header.Add(string(key), string(value))
	})
	return req
}

// Method returns the request method
func (c *FiberContext) Method() string {
	return c.context.Method()
}

// Path returns the request path
func (c *FiberContext) Path() string {
	return c.context.Path()
}

// QueryParam retrieves a query parameter
func (c *FiberContext) QueryParam(name string) string {
	return c.context.Query(name)
}

// Param retrieves a path parameter
func (c *FiberContext) Param(name string) string {
	return c.context.Params(name)
}

// Status sets the status code
func (c *FiberContext) Status(code int) {
	c.context.Status(code)
}

// JSON returns a JSON response
func (c *FiberContext) JSON(code int, obj interface{}) error {
	return c.context.Status(code).JSON(obj)
}

// Text returns a text response
func (c *FiberContext) Text(code int, text string) error {
	return c.context.Status(code).SendString(text)
}

// HTML returns an HTML response
func (c *FiberContext) HTML(code int, html string) error {
	return c.context.Status(code).SendString(html)
}

// Redirect performs a redirect
func (c *FiberContext) Redirect(code int, url string) error {
	return c.context.Status(code).Redirect(url)
}

// Set stores a key-value pair
func (c *FiberContext) Set(key string, value interface{}) {
	c.context.Locals(key, value)
}

// Get retrieves a stored value
func (c *FiberContext) Get(key string) interface{} {
	return c.context.Locals(key)
}

// Context returns the Go context
func (c *FiberContext) Context() context.Context {
	return c.context.Context()
}

// ResponseWriter returns the underlying http.ResponseWriter.
// Fiber uses fasthttp internally; returns nil.
func (c *FiberContext) ResponseWriter() http.ResponseWriter { return nil }

// SetResponseWriter is a no-op for fiber (no http.ResponseWriter).
func (c *FiberContext) SetResponseWriter(w http.ResponseWriter) {}

// BindJSON binds the JSON request body
func (c *FiberContext) BindJSON(obj interface{}) error {
	return c.context.BodyParser(obj)
}

// BindXML binds the XML request body
func (c *FiberContext) BindXML(obj interface{}) error {
	return c.context.BodyParser(obj)
}

// BindQuery binds query parameters to a struct
func (c *FiberContext) BindQuery(obj interface{}) error {
	// Fiber has no built-in BindQuery; simplified implementation
	return nil
}

// Cookie retrieves a cookie
func (c *FiberContext) Cookie(name string) (string, error) {
	return c.context.Cookies(name), nil
}

// SetCookie sets a cookie
func (c *FiberContext) SetCookie(cookie *http.Cookie) {
	fiberCookie := &fiber.Cookie{
		Name:     cookie.Name,
		Value:    cookie.Value,
		Path:     cookie.Path,
		Domain:   cookie.Domain,
		MaxAge:   cookie.MaxAge,
		Secure:   cookie.Secure,
		HTTPOnly: cookie.HttpOnly,
	}
	c.context.Cookie(fiberCookie)
}

// Logger returns the logger
func (c *FiberContext) Logger() interfaces.Logger {
	if logger, ok := c.Get("logger").(interfaces.Logger); ok && logger != nil {
		return logger
	}
	return c.logger
}

// XML returns an XML response
func (c *FiberContext) XML(code int, obj interface{}) error {
	return c.context.Status(code).XML(obj)
}

// FormValue retrieves a form field value
func (c *FiberContext) FormValue(key string) string {
	return c.context.FormValue(key)
}

// PostForm retrieves a POST form field value
func (c *FiberContext) PostForm(key string) string {
	return c.context.FormValue(key) // Fiber's FormValue handles POST data
}

// ParseForm parses the form
func (c *FiberContext) ParseForm() error {
	// Fiber auto-parses; no extra action needed
	return nil
}

// ParseMultipartForm parses the multipart form
func (c *FiberContext) ParseMultipartForm(maxMemory int64) error {
	// Fiber auto-handles multipart forms; no extra action needed
	return nil
}
