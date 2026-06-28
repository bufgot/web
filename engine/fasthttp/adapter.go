package fasthttp

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"mime/multipart"
	"net/http"
	"strings"

	interfaces "github.com/bufgot/web"
	"github.com/valyala/fasthttp"
)

// FasthttpAdapter implements the WebFramework interface
type FasthttpAdapter struct{}

// NewFasthttpAdapter creates a Fasthttp adapter
func NewFasthttpAdapter() *FasthttpAdapter {
	return &FasthttpAdapter{}
}

// Name returns the framework name
func (f *FasthttpAdapter) Name() string {
	return "fasthttp"
}

// NewRouter creates a new router
func (f *FasthttpAdapter) NewRouter() interfaces.Router {
	return New()
}

// FasthttpRouter adapts Fasthttp's router
type FasthttpRouter struct {
	router      *fasthttp.Server
	routes      map[string]map[string]interfaces.Handler
	middlewares []interfaces.Middleware
	prefix      string            // route group prefix
	staticPaths map[string]string // static file path mappings
	logger      interfaces.Logger
}

// New creates a new router instance
func New() *FasthttpRouter {
	return &FasthttpRouter{
		routes:      make(map[string]map[string]interfaces.Handler),
		staticPaths: make(map[string]string),
		logger:      interfaces.NewDefaultLogger(),
	}
}

// addRoute registers a route
func (r *FasthttpRouter) addRoute(method string, path string, handler interfaces.Handler) {
	if r.routes[method] == nil {
		r.routes[method] = make(map[string]interfaces.Handler)
	}
	// apply route group prefix
	fullPath := r.prefix + path
	r.routes[method][fullPath] = handler
}

// GET registers a GET route
func (r *FasthttpRouter) GET(path string, handler interfaces.Handler) {
	r.addRoute("GET", path, handler)
}

// POST registers a POST route
func (r *FasthttpRouter) POST(path string, handler interfaces.Handler) {
	r.addRoute("POST", path, handler)
}

// PUT registers a PUT route
func (r *FasthttpRouter) PUT(path string, handler interfaces.Handler) {
	r.addRoute("PUT", path, handler)
}

// DELETE registers a DELETE route
func (r *FasthttpRouter) DELETE(path string, handler interfaces.Handler) {
	r.addRoute("DELETE", path, handler)
}

// PATCH registers a PATCH route
func (r *FasthttpRouter) PATCH(path string, handler interfaces.Handler) {
	r.addRoute("PATCH", path, handler)
}

// HEAD registers a HEAD route
func (r *FasthttpRouter) HEAD(path string, handler interfaces.Handler) {
	r.addRoute("HEAD", path, handler)
}

// OPTIONS registers an OPTIONS route
func (r *FasthttpRouter) OPTIONS(path string, handler interfaces.Handler) {
	r.addRoute("OPTIONS", path, handler)
}

// Use adds middleware
func (r *FasthttpRouter) Use(middleware interfaces.Middleware) {
	r.middlewares = append(r.middlewares, middleware)
}

// Start starts the server
func (r *FasthttpRouter) Start(addr string) error {
	server := &fasthttp.Server{
		Handler: r.handleRequest,
	}
	return server.ListenAndServe(addr)
}

// Group creates a route group
func (r *FasthttpRouter) Group(prefix string, middlewares ...interfaces.Middleware) interfaces.Router {
	group := &FasthttpRouter{
		routes:      r.routes, // share the root router's route table
		middlewares: append(r.middlewares, middlewares...),
		prefix:      r.prefix + prefix, // concatenate prefix
		logger:      r.logger,
	}
	return group
}

// Static serves static files
func (r *FasthttpRouter) Static(prefix, root string) {
	r.staticPaths[prefix] = root
}

// SetLogger sets the logger
func (r *FasthttpRouter) SetLogger(logger interfaces.Logger) {
	r.logger = logger
}

// handleRequest handles requests
func (r *FasthttpRouter) handleRequest(ctx *fasthttp.RequestCtx) {
	method := string(ctx.Method())
	path := string(ctx.Path())

	for prefix, root := range r.staticPaths {
		if strings.HasPrefix(path, prefix) {
			filePath := root + path[len(prefix):]
			fasthttp.ServeFile(ctx, filePath)
			return
		}
	}

	handler, exists := r.routes[method][path]
	if !exists {
		ctx.SetStatusCode(404)
		ctx.WriteString("Not Found")
		return
	}

	shimCtx := &FasthttpContext{
		context: ctx,
		store:   make(map[string]interface{}),
		logger:  r.logger,
	}

	// apply middlewares
	for _, middleware := range r.middlewares {
		handler = middleware(handler)
	}

	handler(shimCtx)
}

// FasthttpContext adapts Fasthttp's context
type FasthttpContext struct {
	context *fasthttp.RequestCtx
	store   map[string]interface{} // stores middleware data
	logger  interfaces.Logger
}

// Request returns the HTTP request (builds an http.Request object)
func (c *FasthttpContext) Request() *http.Request {
	req, _ := http.NewRequest(
		string(c.context.Method()),
		c.context.URI().String(),
		bytes.NewReader(c.context.Request.Body()),
	)
	c.context.Request.Header.VisitAll(func(key, value []byte) {
		req.Header.Add(string(key), string(value))
	})
	return req
}

// Method returns the request method
func (c *FasthttpContext) Method() string {
	return string(c.context.Method())
}

// Path returns the request path
func (c *FasthttpContext) Path() string {
	return string(c.context.Path())
}

// QueryParam retrieves a query parameter
func (c *FasthttpContext) QueryParam(name string) string {
	return string(c.context.QueryArgs().Peek(name))
}

// Param retrieves a path parameter (fasthttp has no path param support; simplified implementation)
func (c *FasthttpContext) Param(name string) string {
	// simplified implementation; a real project would need a routing library for path params
	return ""
}

// Status sets the status code
func (c *FasthttpContext) Status(code int) {
	c.context.SetStatusCode(code)
}

// JSON returns a JSON response
func (c *FasthttpContext) JSON(code int, obj interface{}) error {
	c.context.SetStatusCode(code)
	c.context.SetContentType("application/json")
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	c.context.Write(data)
	return nil
}

// Text returns a text response
func (c *FasthttpContext) Text(code int, text string) error {
	c.context.SetStatusCode(code)
	c.context.SetContentType("text/plain")
	c.context.WriteString(text)
	return nil
}

// HTML returns an HTML response
func (c *FasthttpContext) HTML(code int, html string) error {
	c.context.SetStatusCode(code)
	c.context.SetContentType("text/html")
	c.context.WriteString(html)
	return nil
}

// Redirect performs a redirect
func (c *FasthttpContext) Redirect(code int, url string) error {
	c.context.SetStatusCode(code)
	c.context.Redirect(url, code)
	return nil
}

// Get retrieves a stored value
func (c *FasthttpContext) Get(key string) interface{} {
	return c.store[key]
}

// Set stores a key-value pair
func (c *FasthttpContext) Set(key string, value interface{}) {
	c.store[key] = value
}

// Context returns the Go context
func (c *FasthttpContext) Context() context.Context {
	return context.Background()
}

// BindJSON binds the JSON request body
func (c *FasthttpContext) BindJSON(obj interface{}) error {
	return json.Unmarshal(c.context.Request.Body(), obj)
}

// BindXML binds the XML request body
func (c *FasthttpContext) BindXML(obj interface{}) error {
	return xml.Unmarshal(c.context.Request.Body(), obj)
}

// BindQuery binds query parameters to a struct
func (c *FasthttpContext) BindQuery(obj interface{}) error {
	// simplified stub
	return nil
}

// FormFile retrieves an uploaded file
func (c *FasthttpContext) FormFile(key string) (multipart.File, *multipart.FileHeader, error) {
	// Fasthttp file upload handling is complex; simplified here
	return nil, nil, http.ErrMissingFile
}

// MultipartForm retrieves the multipart form
func (c *FasthttpContext) MultipartForm() (*multipart.Form, error) {
	return nil, nil
}

// SaveUploadedFile saves an uploaded file
func (c *FasthttpContext) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	return nil
}

// Cookie retrieves a cookie
func (c *FasthttpContext) Cookie(name string) (string, error) {
	return string(c.context.Request.Header.Cookie(name)), nil
}

// SetCookie sets a cookie
func (c *FasthttpContext) SetCookie(cookie *http.Cookie) {
	fasthttpCookie := fasthttp.Cookie{}
	fasthttpCookie.SetKey(cookie.Name)
	fasthttpCookie.SetValue(cookie.Value)
	fasthttpCookie.SetMaxAge(cookie.MaxAge)
	fasthttpCookie.SetPath(cookie.Path)
	fasthttpCookie.SetDomain(cookie.Domain)
	fasthttpCookie.SetSecure(cookie.Secure)
	fasthttpCookie.SetHTTPOnly(cookie.HttpOnly)
	c.context.Response.Header.SetCookie(&fasthttpCookie)
}

// Logger returns the logger
func (c *FasthttpContext) Logger() interfaces.Logger {
	if logger, ok := c.Get("logger").(interfaces.Logger); ok && logger != nil {
		return logger
	}
	return c.logger
}

// XML returns an XML response
func (c *FasthttpContext) XML(code int, obj interface{}) error {
	c.context.SetStatusCode(code)
	c.context.SetContentType("application/xml")
	data, err := xml.Marshal(obj)
	if err != nil {
		return err
	}
	c.context.Write(data)
	return nil
}

// FormValue retrieves a form field value
func (c *FasthttpContext) FormValue(key string) string {
	return string(c.context.FormValue(key))
}

// PostForm retrieves a POST form field value
func (c *FasthttpContext) PostForm(key string) string {
	return string(c.context.PostArgs().Peek(key))
}

// ParseForm parses the form
func (c *FasthttpContext) ParseForm() error {
	// Fasthttp auto-parses; no extra action needed
	return nil
}

// ParseMultipartForm parses the multipart form
func (c *FasthttpContext) ParseMultipartForm(maxMemory int64) error {
	// Fasthttp auto-handles multipart forms; no extra action needed
	return nil
}
