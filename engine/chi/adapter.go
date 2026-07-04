package chi

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"

	interfaces "github.com/bufgot/web"
	"github.com/go-chi/chi/v5"
)

// ChiAdapter implements the WebFramework interface
type ChiAdapter struct{}

// NewChiAdapter creates a Chi adapter
func NewChiAdapter() *ChiAdapter {
	return &ChiAdapter{}
}

// Name returns the framework name
func (c *ChiAdapter) Name() string {
	return "chi"
}

// NewRouter creates a new router
func (c *ChiAdapter) NewRouter() interfaces.Router {
	return &ChiRouter{
		router: chi.NewRouter(),
		logger: interfaces.NewDefaultLogger(),
	}
}

// ChiRouter adapts Chi's router
type ChiRouter struct {
	router chi.Router
	logger interfaces.Logger
}

// GET registers a GET route
func (r *ChiRouter) GET(path string, handler interfaces.Handler) {
	r.router.Get(path, r.wrapHandler(handler))
}

// POST registers a POST route
func (r *ChiRouter) POST(path string, handler interfaces.Handler) {
	r.router.Post(path, r.wrapHandler(handler))
}

// PUT registers a PUT route
func (r *ChiRouter) PUT(path string, handler interfaces.Handler) {
	r.router.Put(path, r.wrapHandler(handler))
}

// DELETE registers a DELETE route
func (r *ChiRouter) DELETE(path string, handler interfaces.Handler) {
	r.router.Delete(path, r.wrapHandler(handler))
}

// PATCH registers a PATCH route
func (r *ChiRouter) PATCH(path string, handler interfaces.Handler) {
	r.router.Patch(path, r.wrapHandler(handler))
}

// HEAD registers a HEAD route
func (r *ChiRouter) HEAD(path string, handler interfaces.Handler) {
	r.router.Head(path, r.wrapHandler(handler))
}

// OPTIONS registers an OPTIONS route
func (r *ChiRouter) OPTIONS(path string, handler interfaces.Handler) {
	r.router.Options(path, r.wrapHandler(handler))
}

// Use adds middleware
func (r *ChiRouter) Use(middleware interfaces.Middleware) {
	r.router.Use(r.wrapMiddleware(middleware))
}

// Start starts the server
func (r *ChiRouter) Start(addr string) error {
	return http.ListenAndServe(addr, r.router)
}

// Group creates a route group
func (r *ChiRouter) Group(prefix string, middlewares ...interfaces.Middleware) interfaces.Router {
	group := r.router.Route(prefix, func(router chi.Router) {
		for _, middleware := range middlewares {
			router.Use(r.wrapMiddleware(middleware))
		}
	})
	return &ChiRouter{
		router: group,
	}
}

// Static serves static files
func (r *ChiRouter) Static(prefix, root string) {
	r.router.Handle(prefix+"/*", http.StripPrefix(prefix, http.FileServer(http.Dir(root))))
}

// SetLogger sets the logger
func (r *ChiRouter) SetLogger(logger interfaces.Logger) {
	r.logger = logger
}

// Router returns the underlying chi.Router for advanced use cases like starting the HTTP server.
func (r *ChiRouter) Router() chi.Router { return r.router }

// wrapHandler wraps the unified handler as a chi handler
func (r *ChiRouter) wrapHandler(h interfaces.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := &ChiContext{writer: w, request: req, logger: r.logger}
		h(ctx)
	}
}

// wrapMiddleware wraps the unified middleware as a chi middleware
func (r *ChiRouter) wrapMiddleware(m interfaces.Middleware) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := &ChiContext{writer: w, request: req, logger: r.logger}
			handler := m(func(c interfaces.Context) error {
				if chiCtx, ok := c.(*ChiContext); ok {
					next.ServeHTTP(w, chiCtx.request)
				} else {
					next.ServeHTTP(w, req)
				}
				return nil
			})
			handler(ctx)
		})
	}
}

// ChiContext adapts Chi's context
type ChiContext struct {
	writer  http.ResponseWriter
	request *http.Request
	logger  interfaces.Logger
}

// Request returns the HTTP request
func (c *ChiContext) Request() *http.Request {
	return c.request
}

// Method returns the request method
func (c *ChiContext) Method() string {
	return c.request.Method
}

// Path returns the request path
func (c *ChiContext) Path() string {
	return c.request.URL.Path
}

// QueryParam retrieves a query parameter
func (c *ChiContext) QueryParam(name string) string {
	return c.request.URL.Query().Get(name)
}

// Param retrieves a path parameter
func (c *ChiContext) Param(name string) string {
	return chi.URLParam(c.request, name)
}

// Status sets the status code
func (c *ChiContext) Status(code int) {
	c.writer.WriteHeader(code)
}

// JSON returns a JSON response
func (c *ChiContext) JSON(code int, obj interface{}) error {
	c.writer.Header().Set("Content-Type", "application/json")
	c.writer.WriteHeader(code)
	return writeJSON(c.writer, obj)
}

// Text returns a text response
func (c *ChiContext) Text(code int, text string) error {
	c.writer.Header().Set("Content-Type", "text/plain")
	c.writer.WriteHeader(code)
	_, err := c.writer.Write([]byte(text))
	return err
}

// HTML returns an HTML response
func (c *ChiContext) HTML(code int, html string) error {
	c.writer.Header().Set("Content-Type", "text/html")
	c.writer.WriteHeader(code)
	_, err := c.writer.Write([]byte(html))
	return err
}

// Redirect performs a redirect
func (c *ChiContext) Redirect(code int, url string) error {
	http.Redirect(c.writer, c.request, url, code)
	return nil
}

// Set stores a key-value pair
func (c *ChiContext) Set(key string, value interface{}) {
	c.request = c.request.WithContext(context.WithValue(c.request.Context(), key, value))
}

// Get retrieves a stored key-value
func (c *ChiContext) Get(key string) interface{} {
	return c.request.Context().Value(key)
}

// Context returns the Go context
func (c *ChiContext) Context() context.Context {
	return c.request.Context()
}

// ResponseWriter returns the underlying http.ResponseWriter.
func (c *ChiContext) ResponseWriter() http.ResponseWriter { return c.writer }

// FormValue retrieves a form field value
func (c *ChiContext) FormValue(key string) string {
	return c.request.FormValue(key)
}

// PostForm retrieves a POST form field
func (c *ChiContext) PostForm(key string) string {
	if c.request.Method == "POST" || c.request.Method == "PUT" || c.request.Method == "PATCH" {
		return c.request.PostFormValue(key)
	}
	return ""
}

// ParseForm parses the form
func (c *ChiContext) ParseForm() error {
	return c.request.ParseForm()
}

// ParseMultipartForm parses the multipart form
func (c *ChiContext) ParseMultipartForm(maxMemory int64) error {
	return c.request.ParseMultipartForm(maxMemory)
}

// BindJSON binds the JSON request
func (c *ChiContext) BindJSON(obj interface{}) error {
	return json.NewDecoder(c.request.Body).Decode(obj)
}

// BindXML binds the XML request
func (c *ChiContext) BindXML(obj interface{}) error {
	return xml.NewDecoder(c.request.Body).Decode(obj)
}

// BindQuery binds query parameters to a struct
func (c *ChiContext) BindQuery(obj interface{}) error {
	values := c.request.URL.Query()
	return mapForm(values, obj)
}

// FormFile retrieves an uploaded file
func (c *ChiContext) FormFile(key string) (multipart.File, *multipart.FileHeader, error) {
	if c.request.MultipartForm == nil {
		c.ParseMultipartForm(32 << 20) // 32MB
	}
	if c.request.MultipartForm != nil && c.request.MultipartForm.File != nil {
		if files, ok := c.request.MultipartForm.File[key]; ok && len(files) > 0 {
			file, err := files[0].Open()
			return file, files[0], err
		}
	}
	return nil, nil, http.ErrMissingFile
}

// MultipartForm retrieves the multipart form
func (c *ChiContext) MultipartForm() (*multipart.Form, error) {
	err := c.ParseMultipartForm(32 << 20)
	return c.request.MultipartForm, err
}

// SaveUploadedFile saves an uploaded file
func (c *ChiContext) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

// Cookie retrieves a cookie
func (c *ChiContext) Cookie(name string) (string, error) {
	cookie, err := c.request.Cookie(name)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// SetCookie sets a cookie
func (c *ChiContext) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.writer, cookie)
}

// Logger returns the logger
func (c *ChiContext) Logger() interfaces.Logger {
	if logger, ok := c.Get("logger").(interfaces.Logger); ok && logger != nil {
		return logger
	}
	return c.logger
}

// XML returns an XML response
func (c *ChiContext) XML(code int, obj interface{}) error {
	c.writer.Header().Set("Content-Type", "application/xml")
	c.writer.WriteHeader(code)
	return xml.NewEncoder(c.writer).Encode(obj)
}

func writeJSON(w http.ResponseWriter, obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// mapForm maps form values to struct fields
func mapForm(values map[string][]string, obj interface{}) error {
	v := reflect.ValueOf(obj).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		tag := field.Tag.Get("form")
		if tag == "" {
			tag = strings.ToLower(field.Name)
		}

		if vals, ok := values[tag]; ok && len(vals) > 0 {
			if fieldValue.CanSet() {
				switch fieldValue.Kind() {
				case reflect.String:
					fieldValue.SetString(vals[0])
				case reflect.Int, reflect.Int64:
					if intVal, err := strconv.Atoi(vals[0]); err == nil {
						fieldValue.SetInt(int64(intVal))
					}
				case reflect.Bool:
					if boolVal, err := strconv.ParseBool(vals[0]); err == nil {
						fieldValue.SetBool(boolVal)
					}
				}
			}
		}
	}
	return nil
}
