package fiber

import (
	"bytes"
	"context"
	"net/http"

	interfaces "github.com/bufgot/web"
	"github.com/gofiber/fiber/v2"
)

// NewFiberAdapter 实现WebFramework接口
type FiberAdapter struct{}

// NewFiberAdapter 创建Fiber适配器
func NewFiberAdapter() *FiberAdapter {
	return &FiberAdapter{}
}

// Name 返回框架名称
func (f *FiberAdapter) Name() string {
	return "fiber"
}

// NewRouter 创建新的路由
func (f *FiberAdapter) NewRouter() interfaces.Router {
	return &FiberRouter{
		app:    fiber.New(),
		group:  nil, // 初始时没有路由组
		logger: interfaces.NewDefaultLogger(),
	}
}

// FiberRouter 适配Fiber的路由器
type FiberRouter struct {
	app    *fiber.App
	group  fiber.Router // 当前路由组，如果为nil则使用app
	logger interfaces.Logger
}

// currentRouter 返回当前使用的路由器
func (r *FiberRouter) currentRouter() fiber.Router {
	if r.group != nil {
		return r.group
	}
	return r.app
}

// addRoute 向当前路由器添加路由
func (r *FiberRouter) addRoute(method, path string, handler interfaces.Handler) {
	r.currentRouter().Add(method, path, r.wrapHandler(handler))
}

// addUse 向当前路由器添加中间件
func (r *FiberRouter) addUse(middleware interfaces.Middleware) {
	r.currentRouter().Use(r.wrapMiddleware(middleware))
}

// GET 注册GET路由
func (r *FiberRouter) GET(path string, handler interfaces.Handler) {
	r.addRoute("GET", path, handler)
}

// POST 注册POST路由
func (r *FiberRouter) POST(path string, handler interfaces.Handler) {
	r.addRoute("POST", path, handler)
}

// PUT 注册PUT路由
func (r *FiberRouter) PUT(path string, handler interfaces.Handler) {
	r.addRoute("PUT", path, handler)
}

// DELETE 注册DELETE路由
func (r *FiberRouter) DELETE(path string, handler interfaces.Handler) {
	r.addRoute("DELETE", path, handler)
}

// PATCH 注册PATCH路由
func (r *FiberRouter) PATCH(path string, handler interfaces.Handler) {
	r.addRoute("PATCH", path, handler)
}

// HEAD 注册HEAD路由
func (r *FiberRouter) HEAD(path string, handler interfaces.Handler) {
	r.addRoute("HEAD", path, handler)
}

// OPTIONS 注册OPTIONS路由
func (r *FiberRouter) OPTIONS(path string, handler interfaces.Handler) {
	r.addRoute("OPTIONS", path, handler)
}

// Use 添加中间件
func (r *FiberRouter) Use(middleware interfaces.Middleware) {
	r.addUse(middleware)
}

// Start 启动服务
func (r *FiberRouter) Start(addr string) error {
	return r.app.Listen(addr)
}

// Group 创建路由
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

// Static 服务静态文件
func (r *FiberRouter) Static(prefix, root string) {
	r.currentRouter().Static(prefix, root)
}

// SetLogger 设置日志
func (r *FiberRouter) SetLogger(logger interfaces.Logger) {
	r.logger = logger
}

// wrapHandler 将shim.Handler包装为fiber.Handler
func (r *FiberRouter) wrapHandler(h interfaces.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := &FiberContext{context: c, logger: r.logger}
		return h(ctx)
	}
}

// wrapMiddleware 将shim.Middleware包装为fiber.Handler
func (r *FiberRouter) wrapMiddleware(m interfaces.Middleware) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := &FiberContext{context: c, logger: r.logger}
		handler := m(func(ctx interfaces.Context) error {
			return c.Next()
		})
		return handler(ctx)
	}
}

// FiberContext 适配Fiber的上下文
type FiberContext struct {
	context *fiber.Ctx
	logger  interfaces.Logger
}

// Request 返回HTTP请求
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

// Method 返回请求方法
func (c *FiberContext) Method() string {
	return c.context.Method()
}

// Path 返回请求路径
func (c *FiberContext) Path() string {
	return c.context.Path()
}

// QueryParam 获取查询参数
func (c *FiberContext) QueryParam(name string) string {
	return c.context.Query(name)
}

// Param 获取路径参数
func (c *FiberContext) Param(name string) string {
	return c.context.Params(name)
}

// Status 设置状态码
func (c *FiberContext) Status(code int) {
	c.context.Status(code)
}

// JSON 返回JSON响应
func (c *FiberContext) JSON(code int, obj interface{}) error {
	return c.context.Status(code).JSON(obj)
}

// Text 返回文本响应
func (c *FiberContext) Text(code int, text string) error {
	return c.context.Status(code).SendString(text)
}

// HTML 返回HTML响应
func (c *FiberContext) HTML(code int, html string) error {
	return c.context.Status(code).SendString(html)
}

// Redirect 重定向
func (c *FiberContext) Redirect(code int, url string) error {
	return c.context.Status(code).Redirect(url)
}

// Set 设置
func (c *FiberContext) Set(key string, value interface{}) {
	c.context.Locals(key, value)
}

// Get 获取
func (c *FiberContext) Get(key string) interface{} {
	return c.context.Locals(key)
}

// Context 返回Go上下文
func (c *FiberContext) Context() context.Context {
	return c.context.Context()
}

// BindJSON 绑定JSON请求体
func (c *FiberContext) BindJSON(obj interface{}) error {
	return c.context.BodyParser(obj)
}

// BindXML 绑定XML请求体
func (c *FiberContext) BindXML(obj interface{}) error {
	return c.context.BodyParser(obj)
}

// BindQuery 绑定查询参数到结构体
func (c *FiberContext) BindQuery(obj interface{}) error {
	// Fiber没有内置BindQuery，这里简化实现
	return nil
}

// Cookie 获取Cookie
func (c *FiberContext) Cookie(name string) (string, error) {
	return c.context.Cookies(name), nil
}

// SetCookie 设置Cookie
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

// Logger 返回日志
func (c *FiberContext) Logger() interfaces.Logger {
	if logger, ok := c.Get("logger").(interfaces.Logger); ok && logger != nil {
		return logger
	}
	return c.logger
}

// XML 返回XML响应
func (c *FiberContext) XML(code int, obj interface{}) error {
	return c.context.Status(code).XML(obj)
}

// FormValue 获取表单字段值
func (c *FiberContext) FormValue(key string) string {
	return c.context.FormValue(key)
}

// PostForm 获取POST表单字段值
func (c *FiberContext) PostForm(key string) string {
	return c.context.FormValue(key) // Fiber的FormValue处理POST数据
}

// ParseForm 解析表单
func (c *FiberContext) ParseForm() error {
	// Fiber自动解析，这里不需要额外操作
	return nil
}

// ParseMultipartForm 解析多部分表单
func (c *FiberContext) ParseMultipartForm(maxMemory int64) error {
	// Fiber自动处理多部分表单，这里不需要额外操作
	return nil
}
