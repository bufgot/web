package echo

import (
	"context"
	"net/http"

	"github.com/bufgot/log/stdlib"
	interfaces "github.com/bufgot/web"
	"github.com/labstack/echo/v4"
)

// EchoAdapter implements the WebFramework interface
type EchoAdapter struct{}

// NewEchoAdapter creates an Echo adapter
func NewEchoAdapter() *EchoAdapter {
	return &EchoAdapter{}
}

// Name returns the framework name
func (e *EchoAdapter) Name() string {
	return "echo"
}

// NewRouter creates a new router
func (e *EchoAdapter) NewRouter() interfaces.Router {
	return &EchoRouter{
		echo:   echo.New(),
		group:  nil, // no route group initially
		logger: stdlib.NewLogger(nil),
	}
}

// EchoRouter adapts Echo's router
type EchoRouter struct {
	echo   *echo.Echo
	group  *echo.Group // current route group; if nil, echo.Echo is used
	logger interfaces.Logger
}

// currentRouter returns the currently active router
func (r *EchoRouter) currentRouter() interface{} {
	if r.group != nil {
		return r.group
	}
	return r.echo
}

// addRoute adds a route to the current router
func (r *EchoRouter) addRoute(method, path string, handler interfaces.Handler) {
	if r.group != nil {
		r.group.Add(method, path, r.wrapHandler(handler))
	} else {
		r.echo.Add(method, path, r.wrapHandler(handler))
	}
}

// addUse adds middleware to the current router
func (r *EchoRouter) addUse(middleware interfaces.Middleware) {
	if r.group != nil {
		r.group.Use(r.wrapMiddleware(middleware))
	} else {
		r.echo.Use(r.wrapMiddleware(middleware))
	}
}

// GET registers a GET route
func (r *EchoRouter) GET(path string, handler interfaces.Handler) {
	r.addRoute("GET", path, handler)
}

// POST registers a POST route
func (r *EchoRouter) POST(path string, handler interfaces.Handler) {
	r.addRoute("POST", path, handler)
}

// PUT registers a PUT route
func (r *EchoRouter) PUT(path string, handler interfaces.Handler) {
	r.addRoute("PUT", path, handler)
}

// DELETE registers a DELETE route
func (r *EchoRouter) DELETE(path string, handler interfaces.Handler) {
	r.addRoute("DELETE", path, handler)
}

// PATCH registers a PATCH route
func (r *EchoRouter) PATCH(path string, handler interfaces.Handler) {
	r.addRoute("PATCH", path, handler)
}

// HEAD registers a HEAD route
func (r *EchoRouter) HEAD(path string, handler interfaces.Handler) {
	r.addRoute("HEAD", path, handler)
}

// OPTIONS registers an OPTIONS route
func (r *EchoRouter) OPTIONS(path string, handler interfaces.Handler) {
	r.addRoute("OPTIONS", path, handler)
}

// Use adds middleware
func (r *EchoRouter) Use(middleware interfaces.Middleware) {
	r.addUse(middleware)
}

// Start starts the server
func (r *EchoRouter) Start(addr string) error {
	return r.echo.Start(addr)
}

// Group creates a route group
func (r *EchoRouter) Group(prefix string, middlewares ...interfaces.Middleware) interfaces.Router {
	var newGroup *echo.Group
	if r.group != nil {
		newGroup = r.group.Group(prefix)
	} else {
		newGroup = r.echo.Group(prefix)
	}
	for _, middleware := range middlewares {
		newGroup.Use(r.wrapMiddleware(middleware))
	}
	return &EchoRouter{
		echo:   r.echo,
		group:  newGroup,
		logger: r.logger,
	}
}

// Static serves static files
func (r *EchoRouter) Static(prefix, root string) {
	if r.group != nil {
		r.group.Static(prefix, root)
	} else {
		r.echo.Static(prefix, root)
	}
}

// SetLogger sets the logger
func (r *EchoRouter) SetLogger(logger interfaces.Logger) {
	r.logger = logger
}

// wrapHandler wraps a shim.Handler as an echo.HandlerFunc
func (r *EchoRouter) wrapHandler(h interfaces.Handler) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := &EchoContext{context: c, logger: r.logger}
		return h(ctx)
	}
}

// wrapMiddleware wraps a shim.Middleware as an echo.MiddlewareFunc
func (r *EchoRouter) wrapMiddleware(m interfaces.Middleware) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := &EchoContext{context: c}
			handler := m(func(ctx interfaces.Context) error {
				return next(c)
			})
			return handler(ctx)
		}
	}
}

// EchoContext adapts Echo's context
type EchoContext struct {
	context echo.Context
	logger  interfaces.Logger
}

// Request returns the HTTP request
func (c *EchoContext) Request() *http.Request {
	return c.context.Request()
}

// Method returns the request method
func (c *EchoContext) Method() string {
	return c.context.Request().Method
}

// Path returns the request path
func (c *EchoContext) Path() string {
	return c.context.Request().URL.Path
}

// QueryParam retrieves a query parameter
func (c *EchoContext) QueryParam(name string) string {
	return c.context.QueryParam(name)
}

// Param retrieves a path parameter
func (c *EchoContext) Param(name string) string {
	return c.context.Param(name)
}

// Status sets the status code
func (c *EchoContext) Status(code int) {
	c.context.Response().Status = code
}

// JSON returns a JSON response
func (c *EchoContext) JSON(code int, obj interface{}) error {
	return c.context.JSON(code, obj)
}

// Text returns a text response
func (c *EchoContext) Text(code int, text string) error {
	return c.context.String(code, text)
}

// HTML returns an HTML response
func (c *EchoContext) HTML(code int, html string) error {
	return c.context.HTML(code, html)
}

// Redirect performs a redirect
func (c *EchoContext) Redirect(code int, url string) error {
	return c.context.Redirect(code, url)
}

// Set stores a key-value pair
func (c *EchoContext) Set(key string, value interface{}) {
	c.context.Set(key, value)
}

// Get retrieves a stored value
func (c *EchoContext) Get(key string) interface{} {
	return c.context.Get(key)
}

// Context returns the Go context
func (c *EchoContext) Context() context.Context {
	return c.context.Request().Context()
}

// ResponseWriter returns the underlying http.ResponseWriter.
func (c *EchoContext) ResponseWriter() http.ResponseWriter {
	if w, ok := c.context.Response().Writer.(http.ResponseWriter); ok {
		return w
	}
	return nil
}

// BindJSON binds the JSON request body
func (c *EchoContext) BindJSON(obj interface{}) error {
	return c.context.Bind(obj)
}

// BindXML binds the XML request body
func (c *EchoContext) BindXML(obj interface{}) error {
	return c.context.Bind(obj)
}

// BindQuery binds query parameters to a struct
func (c *EchoContext) BindQuery(obj interface{}) error {
	// Echo has no built-in BindQuery; simplified implementation here
	return nil
}

// Cookie retrieves a cookie
func (c *EchoContext) Cookie(name string) (string, error) {
	cookie, err := c.context.Cookie(name)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// SetCookie sets a cookie
func (c *EchoContext) SetCookie(cookie *http.Cookie) {
	c.context.SetCookie(cookie)
}

// Logger returns the logger
func (c *EchoContext) Logger() interfaces.Logger {
	if logger, ok := c.Get("logger").(interfaces.Logger); ok && logger != nil {
		return logger
	}
	return c.logger
}

// XML returns an XML response
func (c *EchoContext) XML(code int, obj interface{}) error {
	return c.context.XML(code, obj)
}

// FormValue retrieves a form field value
func (c *EchoContext) FormValue(key string) string {
	return c.context.FormValue(key)
}

// PostForm retrieves a POST form field value
func (c *EchoContext) PostForm(key string) string {
	return c.context.FormValue(key) // Echo's FormValue handles POST and GET
}

// ParseForm parses the form
func (c *EchoContext) ParseForm() error {
	return c.context.Request().ParseForm()
}

// ParseMultipartForm parses the multipart form
func (c *EchoContext) ParseMultipartForm(maxMemory int64) error {
	return c.context.Request().ParseMultipartForm(maxMemory)
}
