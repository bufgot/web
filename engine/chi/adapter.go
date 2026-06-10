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

	"github.com/bufgot/web"
	""
	"github.com/go-chi/chi/v5"
)

// ChiAdapter 实现WebFramework接口
type ChiAdapter struct{}

// NewChiAdapter 创建Chi适配�?func NewChiAdapter() *ChiAdapter {
	return &ChiAdapter{}
}

// Name 返回框架名称
func (c *ChiAdapter) Name() string {
	return "chi"
}

// NewRouter 创建新的路由�?func (c *ChiAdapter) NewRouter() interfaces.Router {
	return &ChiRouter{
		router: chi.NewRouter(),
		logger: default.NewDefaultLogger(),
	}
}

// ChiRouter 适配Chi路由�?type ChiRouter struct {
	router chi.Router
	logger interfaces.Logger
}

// GET 注册GET路由
func (r *ChiRouter) GET(path string, handler interfaces.Handler) {
	r.router.Get(path, r.wrapHandler(handler))
}

// POST 注册POST路由
func (r *ChiRouter) POST(path string, handler interfaces.Handler) {
	r.router.Post(path, r.wrapHandler(handler))
}

// PUT 注册PUT路由
func (r *ChiRouter) PUT(path string, handler interfaces.Handler) {
	r.router.Put(path, r.wrapHandler(handler))
}

// DELETE 注册DELETE路由
func (r *ChiRouter) DELETE(path string, handler interfaces.Handler) {
	r.router.Delete(path, r.wrapHandler(handler))
}

// PATCH 注册PATCH路由
func (r *ChiRouter) PATCH(path string, handler interfaces.Handler) {
	r.router.Patch(path, r.wrapHandler(handler))
}

// HEAD 注册HEAD路由
func (r *ChiRouter) HEAD(path string, handler interfaces.Handler) {
	r.router.Head(path, r.wrapHandler(handler))
}

// OPTIONS 注册OPTIONS路由
func (r *ChiRouter) OPTIONS(path string, handler interfaces.Handler) {
	r.router.Options(path, r.wrapHandler(handler))
}

// Use 添加中间�?func (r *ChiRouter) Use(middleware interfaces.Middleware) {
	r.router.Use(r.wrapMiddleware(middleware))
}

// Start 启动服务�?func (r *ChiRouter) Start(addr string) error {
	return http.ListenAndServe(addr, r.router)
}

// Group 创建路由�?func (r *ChiRouter) Group(prefix string, middlewares ...interfaces.Middleware) interfaces.Router {
	group := r.router.Route(prefix, func(router chi.Router) {
		for _, middleware := range middlewares {
			router.Use(r.wrapMiddleware(middleware))
		}
	})
	return &ChiRouter{
		router: group,
	}
}

// Static 服务静态文�?func (r *ChiRouter) Static(prefix, root string) {
	r.router.Handle(prefix+"/*", http.StripPrefix(prefix, http.FileServer(http.Dir(root))))
}

// SetLogger 设置日志�?func (r *ChiRouter) SetLogger(logger interfaces.Logger) {
	r.logger = logger
}

// wrapHandler 将统一处理器包装为chi handler
func (r *ChiRouter) wrapHandler(h interfaces.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := &ChiContext{writer: w, request: req, logger: r.logger}
		h(ctx)
	}
}

// wrapMiddleware 将统一中间件包装为chi中间�?func (r *ChiRouter) wrapMiddleware(m interfaces.Middleware) func(http.Handler) http.Handler {
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

// ChiContext 适配Chi上下�?type ChiContext struct {
	writer  http.ResponseWriter
	request *http.Request
	logger  interfaces.Logger
}

// Request 返回HTTP请求
func (c *ChiContext) Request() *http.Request {
	return c.request
}

// Method 返回请求方法
func (c *ChiContext) Method() string {
	return c.request.Method
}

// Path 返回请求路径
func (c *ChiContext) Path() string {
	return c.request.URL.Path
}

// QueryParam 获取查询参数
func (c *ChiContext) QueryParam(name string) string {
	return c.request.URL.Query().Get(name)
}

// Param 获取路径参数
func (c *ChiContext) Param(name string) string {
	return chi.URLParam(c.request, name)
}

// Status 设置状态码
func (c *ChiContext) Status(code int) {
	c.writer.WriteHeader(code)
}

// JSON 返回JSON响应
func (c *ChiContext) JSON(code int, obj interface{}) error {
	c.writer.Header().Set("Content-Type", "application/json")
	c.writer.WriteHeader(code)
	return writeJSON(c.writer, obj)
}

// Text 返回文本响应
func (c *ChiContext) Text(code int, text string) error {
	c.writer.Header().Set("Content-Type", "text/plain")
	c.writer.WriteHeader(code)
	_, err := c.writer.Write([]byte(text))
	return err
}

// HTML 返回HTML响应
func (c *ChiContext) HTML(code int, html string) error {
	c.writer.Header().Set("Content-Type", "text/html")
	c.writer.WriteHeader(code)
	_, err := c.writer.Write([]byte(html))
	return err
}

// Redirect 重定�?func (c *ChiContext) Redirect(code int, url string) error {
	http.Redirect(c.writer, c.request, url, code)
	return nil
}

// Set 设置�?func (c *ChiContext) Set(key string, value interface{}) {
	c.request = c.request.WithContext(context.WithValue(c.request.Context(), key, value))
}

// Get 获取�?func (c *ChiContext) Get(key string) interface{} {
	return c.request.Context().Value(key)
}

// Context 返回Go上下�?func (c *ChiContext) Context() context.Context {
	return c.request.Context()
}

// FormValue 获取表单字段�?func (c *ChiContext) FormValue(key string) string {
	return c.request.FormValue(key)
}

// PostForm 获取POST表单字段�?func (c *ChiContext) PostForm(key string) string {
	if c.request.Method == "POST" || c.request.Method == "PUT" || c.request.Method == "PATCH" {
		return c.request.PostFormValue(key)
	}
	return ""
}

// ParseForm 解析表单
func (c *ChiContext) ParseForm() error {
	return c.request.ParseForm()
}

// ParseMultipartForm 解析多部分表�?func (c *ChiContext) ParseMultipartForm(maxMemory int64) error {
	return c.request.ParseMultipartForm(maxMemory)
}

// BindJSON 绑定JSON请求�?func (c *ChiContext) BindJSON(obj interface{}) error {
	return json.NewDecoder(c.request.Body).Decode(obj)
}

// BindXML 绑定XML请求�?func (c *ChiContext) BindXML(obj interface{}) error {
	return xml.NewDecoder(c.request.Body).Decode(obj)
}

// BindQuery 绑定查询参数到结构体
func (c *ChiContext) BindQuery(obj interface{}) error {
	values := c.request.URL.Query()
	return mapForm(values, obj)
}

// FormFile 获取上传的文�?func (c *ChiContext) FormFile(key string) (multipart.File, *multipart.FileHeader, error) {
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

// MultipartForm 获取多部分表�?func (c *ChiContext) MultipartForm() (*multipart.Form, error) {
	err := c.ParseMultipartForm(32 << 20)
	return c.request.MultipartForm, err
}

// SaveUploadedFile 保存上传的文�?func (c *ChiContext) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
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

// Cookie 获取Cookie
func (c *ChiContext) Cookie(name string) (string, error) {
	cookie, err := c.request.Cookie(name)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// SetCookie 设置Cookie
func (c *ChiContext) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.writer, cookie)
}

// Logger 返回日志�?func (c *ChiContext) Logger() interfaces.Logger {
	if logger, ok := c.Get("logger").(interfaces.Logger); ok && logger != nil {
		return logger
	}
	return c.logger
}

// XML 返回XML响应
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

// mapForm 将表单值映射到结构体字�?func mapForm(values map[string][]string, obj interface{}) error {
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






