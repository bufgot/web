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

// HertzAdapter 实现WebFramework接口
type HertzAdapter struct{}

// NewHertzAdapter 创建Hertz适配�?
func NewHertzAdapter() *HertzAdapter {
	return &HertzAdapter{}
}

// Name 返回框架名称
func (h *HertzAdapter) Name() string {
	return "hertz"
}

// NewRouter 创建新的路由�?
func (h *HertzAdapter) NewRouter() interfaces.Router {
	return New()
}

// HertzRouter 适配Hertz路由�?
type HertzRouter struct {
	routes      map[string]map[string]interfaces.Handler
	middlewares []interfaces.Middleware
	server      *server.Hertz
	prefix      string            // 路由组前缀
	staticPaths map[string]string // 静态文件路径映�?
	logger      interfaces.Logger
}

// New 创建新的路由器实�?
func New() *HertzRouter {
	return &HertzRouter{
		routes:      make(map[string]map[string]interfaces.Handler),
		staticPaths: make(map[string]string),
		logger:      interfaces.NewDefaultLogger(),
	}
}

// GET 注册GET路由
func (r *HertzRouter) addRoute(method string, path string, handler interfaces.Handler) {
	if r.routes[method] == nil {
		r.routes[method] = make(map[string]interfaces.Handler)
	}
	// 应用路由组前缀
	fullPath := r.prefix + path
	r.routes[method][fullPath] = handler
}

// GET 注册GET路由
func (r *HertzRouter) GET(path string, handler interfaces.Handler) {
	r.addRoute("GET", path, handler)
}

// POST 注册POST路由
func (r *HertzRouter) POST(path string, handler interfaces.Handler) {
	r.addRoute("POST", path, handler)
}

// PUT 注册PUT路由
func (r *HertzRouter) PUT(path string, handler interfaces.Handler) {
	r.addRoute("PUT", path, handler)
}

// DELETE 注册DELETE路由
func (r *HertzRouter) DELETE(path string, handler interfaces.Handler) {
	r.addRoute("DELETE", path, handler)
}

// PATCH 注册PATCH路由
func (r *HertzRouter) PATCH(path string, handler interfaces.Handler) {
	r.addRoute("PATCH", path, handler)
}

// HEAD 注册HEAD路由
func (r *HertzRouter) HEAD(path string, handler interfaces.Handler) {
	r.addRoute("HEAD", path, handler)
}

// OPTIONS 注册OPTIONS路由
func (r *HertzRouter) OPTIONS(path string, handler interfaces.Handler) {
	r.addRoute("OPTIONS", path, handler)
}

// Use 添加中间�?`n
func (r *HertzRouter) Use(middleware interfaces.Middleware) {
	r.middlewares = append(r.middlewares, middleware)
}

// Start 启动服务�?`n
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

// Group 创建路由�?`n
func (r *HertzRouter) Group(prefix string, middlewares ...interfaces.Middleware) interfaces.Router {
	group := &HertzRouter{
		routes:      r.routes, // 共享根路由器的路由表
		middlewares: append(r.middlewares, middlewares...),
		prefix:      r.prefix + prefix, // 连接前缀
		logger:      r.logger,
	}
	return group
}

// Static 服务静态文�?`n
func (r *HertzRouter) Static(prefix, root string) {
	r.staticPaths[prefix] = root
}

// SetLogger 设置日志�?`n
func (r *HertzRouter) SetLogger(logger interfaces.Logger) {
	r.logger = logger
}

// wrapHandler 将统一处理器包装为Hertz处理�?`n
func (r *HertzRouter) wrapHandler(h interfaces.Handler) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		shimCtx := &HertzContext{context: c, ctx: ctx, logger: r.logger}
		h(shimCtx)
	}
}

// wrapMiddleware 将统一中间件包装为Hertz中间�?`n
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

// HertzContext 适配Hertz上下�?
type HertzContext struct {
	context *app.RequestContext
	ctx     context.Context
	logger  interfaces.Logger
}

// Request 返回HTTP请求
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

// Method 返回请求方法
func (c *HertzContext) Method() string {
	return string(c.context.Method())
}

// Path 返回请求路径
func (c *HertzContext) Path() string {
	return string(c.context.Path())
}

// QueryParam 获取查询参数
func (c *HertzContext) QueryParam(name string) string {
	return string(c.context.Query(name))
}

// Param 获取路径参数
func (c *HertzContext) Param(name string) string {
	return string(c.context.Param(name))
}

// Status 设置状态码
func (c *HertzContext) Status(code int) {
	c.context.SetStatusCode(code)
}

// JSON 返回JSON响应
func (c *HertzContext) JSON(code int, obj interface{}) error {
	c.context.JSON(code, obj)
	return nil
}

// Text 返回文本响应
func (c *HertzContext) Text(code int, text string) error {
	c.context.String(code, text)
	return nil
}

// HTML 返回HTML响应
func (c *HertzContext) HTML(code int, html string) error {
	c.context.SetContentType("text/html")
	c.context.String(code, html)
	return nil
}

// Redirect 重定�?`n
func (c *HertzContext) Redirect(code int, url string) error {
	c.context.Redirect(code, []byte(url))
	return nil
}

// Set 设置�?`n
func (c *HertzContext) Set(key string, value interface{}) {
	c.context.Set(key, value)
}

// Logger 返回日志�?`n
func (c *HertzContext) Logger() interfaces.Logger {
	if logger, ok := c.Get("logger").(interfaces.Logger); ok && logger != nil {
		return logger
	}
	return c.logger
}

// Get 获取�?`n
func (c *HertzContext) Get(key string) interface{} {
	val, _ := c.context.Get(key)
	return val
}

// Context 返回Go上下�?`n
func (c *HertzContext) Context() context.Context {
	return c.ctx
}

// BindJSON 绑定JSON请求�?`n
func (c *HertzContext) BindJSON(obj interface{}) error {
	return c.context.BindJSON(obj)
}

// BindXML 绑定XML请求�?`n
func (c *HertzContext) BindXML(obj interface{}) error {
	// Hertz没有内置BindXML，这里简化实�?
	return nil
}

// BindQuery 绑定查询参数到结构体
func (c *HertzContext) BindQuery(obj interface{}) error {
	// Hertz没有内置BindQuery，这里简化实�?
	return nil
}

// Cookie 获取Cookie
func (c *HertzContext) Cookie(name string) (string, error) {
	value := c.context.Cookie(name)
	if len(value) == 0 {
		return "", http.ErrNoCookie
	}
	return string(value), nil
}

// SetCookie 设置Cookie
func (c *HertzContext) SetCookie(cookie *http.Cookie) {
	c.context.SetCookie(cookie.Name, cookie.Value, cookie.MaxAge, cookie.Path, cookie.Domain, 0, cookie.Secure, cookie.HttpOnly)
}

// XML 返回XML响应
func (c *HertzContext) XML(code int, obj interface{}) error {
	c.context.XML(code, obj)
	return nil
}

// FormValue 获取表单字段�?`n
func (c *HertzContext) FormValue(key string) string {
	return string(c.context.FormValue(key))
}

// PostForm 获取POST表单字段�?`n
func (c *HertzContext) PostForm(key string) string {
	return string(c.context.PostForm(key))
}

// ParseForm 解析表单
func (c *HertzContext) ParseForm() error {
	// Hertz自动解析表单，这里不需要额外操�?
	return nil
}

// ParseMultipartForm 解析多部分表�?`n
func (c *HertzContext) ParseMultipartForm(maxMemory int64) error {
	// Hertz自动处理多部分表单，这里不需要额外操�?
	return nil
}
