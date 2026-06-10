package fasthttp

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/bufgot/web"
	""
	"github.com/valyala/fasthttp"
)

// FasthttpAdapter 实现WebFramework接口
type FasthttpAdapter struct{}

// NewFasthttpAdapter 创建Fasthttp适配�?func NewFasthttpAdapter() *FasthttpAdapter {
	return &FasthttpAdapter{}
}

// Name 返回框架名称
func (f *FasthttpAdapter) Name() string {
	return "fasthttp"
}

// NewRouter 创建新的路由�?func (f *FasthttpAdapter) NewRouter() interfaces.Router {
	return New()
}

// FasthttpRouter 适配Fasthttp的路由器
type FasthttpRouter struct {
	router      *fasthttp.Server
	routes      map[string]map[string]interfaces.Handler
	middlewares []interfaces.Middleware
	prefix      string            // 路由组前缀
	staticPaths map[string]string // 静态文件路径映�?	logger      interfaces.Logger
}

// New 创建新的路由器实�?func New() *FasthttpRouter {
	return &FasthttpRouter{
		routes:      make(map[string]map[string]interfaces.Handler),
		staticPaths: make(map[string]string),
		logger:      default.NewDefaultLogger(),
	}
}

// addRoute 注册路由
func (r *FasthttpRouter) addRoute(method string, path string, handler interfaces.Handler) {
	if r.routes[method] == nil {
		r.routes[method] = make(map[string]interfaces.Handler)
	}
	// 应用路由组前缀
	fullPath := r.prefix + path
	r.routes[method][fullPath] = handler
}

// GET 注册GET路由
func (r *FasthttpRouter) GET(path string, handler interfaces.Handler) {
	r.addRoute("GET", path, handler)
}

// POST 注册POST路由
func (r *FasthttpRouter) POST(path string, handler interfaces.Handler) {
	r.addRoute("POST", path, handler)
}

// PUT 注册PUT路由
func (r *FasthttpRouter) PUT(path string, handler interfaces.Handler) {
	r.addRoute("PUT", path, handler)
}

// DELETE 注册DELETE路由
func (r *FasthttpRouter) DELETE(path string, handler interfaces.Handler) {
	r.addRoute("DELETE", path, handler)
}

// PATCH 注册PATCH路由
func (r *FasthttpRouter) PATCH(path string, handler interfaces.Handler) {
	r.addRoute("PATCH", path, handler)
}

// HEAD 注册HEAD路由
func (r *FasthttpRouter) HEAD(path string, handler interfaces.Handler) {
	r.addRoute("HEAD", path, handler)
}

// OPTIONS 注册OPTIONS路由
func (r *FasthttpRouter) OPTIONS(path string, handler interfaces.Handler) {
	r.addRoute("OPTIONS", path, handler)
}

// Use 添加中间�?func (r *FasthttpRouter) Use(middleware interfaces.Middleware) {
	r.middlewares = append(r.middlewares, middleware)
}

// Start 启动服务�?func (r *FasthttpRouter) Start(addr string) error {
	server := &fasthttp.Server{
		Handler: r.handleRequest,
	}
	return server.ListenAndServe(addr)
}

// Group 创建路由�?func (r *FasthttpRouter) Group(prefix string, middlewares ...interfaces.Middleware) interfaces.Router {
	group := &FasthttpRouter{
		routes:      r.routes, // 共享根路由器的路由表
		middlewares: append(r.middlewares, middlewares...),
		prefix:      r.prefix + prefix, // 连接前缀
		logger:      r.logger,
	}
	return group
}

// Static 服务静态文�?func (r *FasthttpRouter) Static(prefix, root string) {
	r.staticPaths[prefix] = root
}

// SetLogger 设置日志�?func (r *FasthttpRouter) SetLogger(logger interfaces.Logger) {
	r.logger = logger
}

// handleRequest 处理请求
func (r *FasthttpRouter) handleRequest(ctx *fasthttp.RequestCtx) {
	method := string(ctx.Method())
	path := string(ctx.Path())

	// 检查静态文�?	for prefix, root := range r.staticPaths {
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

	// 应用中间�?	for _, middleware := range r.middlewares {
		handler = middleware(handler)
	}

	handler(shimCtx)
}

// FasthttpContext 适配Fasthttp的上下文
type FasthttpContext struct {
	context *fasthttp.RequestCtx
	store   map[string]interface{} // 存储中间件数�?	logger  interfaces.Logger
}

// Request 返回HTTP请求 (构�?
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

// Method 返回请求方法
func (c *FasthttpContext) Method() string {
	return string(c.context.Method())
}

// Path 返回请求路径
func (c *FasthttpContext) Path() string {
	return string(c.context.Path())
}

// QueryParam 获取查询参数
func (c *FasthttpContext) QueryParam(name string) string {
	return string(c.context.QueryArgs().Peek(name))
}

// Param 获取路径参数 (fasthttp不支持路径参数，需要简单实�?
func (c *FasthttpContext) Param(name string) string {
	// 简单实现，实际项目中可能需要路由匹�?	return ""
}

// Status 设置状态码
func (c *FasthttpContext) Status(code int) {
	c.context.SetStatusCode(code)
}

// JSON 返回JSON响应
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

// Text 返回文本响应
func (c *FasthttpContext) Text(code int, text string) error {
	c.context.SetStatusCode(code)
	c.context.SetContentType("text/plain")
	c.context.WriteString(text)
	return nil
}

// HTML 返回HTML响应
func (c *FasthttpContext) HTML(code int, html string) error {
	c.context.SetStatusCode(code)
	c.context.SetContentType("text/html")
	c.context.WriteString(html)
	return nil
}

// Redirect 重定�?func (c *FasthttpContext) Redirect(code int, url string) error {
	c.context.SetStatusCode(code)
	c.context.Redirect(url, code)
	return nil
}

// Get 获取�?func (c *FasthttpContext) Get(key string) interface{} {
	return c.store[key]
}

// Set 设置�?func (c *FasthttpContext) Set(key string, value interface{}) {
	c.store[key] = value
}

// Context 返回Go上下�?func (c *FasthttpContext) Context() context.Context {
	return context.Background()
}

// BindJSON 绑定JSON请求�?func (c *FasthttpContext) BindJSON(obj interface{}) error {
	return json.Unmarshal(c.context.Request.Body(), obj)
}

// BindXML 绑定XML请求�?func (c *FasthttpContext) BindXML(obj interface{}) error {
	return xml.Unmarshal(c.context.Request.Body(), obj)
}

// BindQuery 绑定查询参数到结构体
func (c *FasthttpContext) BindQuery(obj interface{}) error {
	// 简化实�?	return nil
}

// FormFile 获取上传的文�?func (c *FasthttpContext) FormFile(key string) (multipart.File, *multipart.FileHeader, error) {
	// Fasthttp的文件上传处理比较复杂，这里简�?	return nil, nil, http.ErrMissingFile
}

// MultipartForm 获取多部分表�?func (c *FasthttpContext) MultipartForm() (*multipart.Form, error) {
	// 简化实�?	return nil, nil
}

// SaveUploadedFile 保存上传的文�?func (c *FasthttpContext) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	// 简化实�?	return nil
}

// Cookie 获取Cookie
func (c *FasthttpContext) Cookie(name string) (string, error) {
	return string(c.context.Request.Header.Cookie(name)), nil
}

// SetCookie 设置Cookie
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

// Logger 返回日志�?func (c *FasthttpContext) Logger() interfaces.Logger {
	if logger, ok := c.Get("logger").(interfaces.Logger); ok && logger != nil {
		return logger
	}
	return c.logger
}

// XML 返回XML响应
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

// FormValue 获取表单字段�?func (c *FasthttpContext) FormValue(key string) string {
	return string(c.context.FormValue(key))
}

// PostForm 获取POST表单字段�?func (c *FasthttpContext) PostForm(key string) string {
	return string(c.context.PostArgs().Peek(key))
}

// ParseForm 解析表单
func (c *FasthttpContext) ParseForm() error {
	// Fasthttp自动解析，这里不需要额外操�?	return nil
}

// ParseMultipartForm 解析多部分表�?func (c *FasthttpContext) ParseMultipartForm(maxMemory int64) error {
	// Fasthttp自动处理多部分表单，这里不需要额外操�?	return nil
}






