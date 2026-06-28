package echo

import (
	"context"
	"net/http"

	"github.com/bufgot/log/stdlib"
	interfaces "github.com/bufgot/web"
	"github.com/labstack/echo/v4"
)

// NewEchoAdapter 实现WebFramework接口
type EchoAdapter struct{}

// NewEchoAdapter 创建Echo适配�?
func NewEchoAdapter() *EchoAdapter {
	return &EchoAdapter{}
}

// Name 返回框架名称
func (e *EchoAdapter) Name() string {
	return "echo"
}

// NewRouter 创建新的路由�?
func (e *EchoAdapter) NewRouter() interfaces.Router {
	return &EchoRouter{
		echo:   echo.New(),
		group:  nil, // 初始时没有路由组
		logger: stdlib.NewLogger(nil),
	}
}

// EchoRouter 适配Echo的路由器
type EchoRouter struct {
	echo   *echo.Echo
	group  *echo.Group // 当前路由组，如果为nil则使用echo.Echo
	logger interfaces.Logger
}

// currentRouter 返回当前使用的路由器
func (r *EchoRouter) currentRouter() interface{} {
	if r.group != nil {
		return r.group
	}
	return r.echo
}

// addRoute 向当前路由器添加路由
func (r *EchoRouter) addRoute(method, path string, handler interfaces.Handler) {
	if r.group != nil {
		r.group.Add(method, path, r.wrapHandler(handler))
	} else {
		r.echo.Add(method, path, r.wrapHandler(handler))
	}
}

// addUse 向当前路由器添加中间�?
func (r *EchoRouter) addUse(middleware interfaces.Middleware) {
	if r.group != nil {
		r.group.Use(r.wrapMiddleware(middleware))
	} else {
		r.echo.Use(r.wrapMiddleware(middleware))
	}
}

// GET 注册GET路由
func (r *EchoRouter) GET(path string, handler interfaces.Handler) {
	r.addRoute("GET", path, handler)
}

// POST 注册POST路由
func (r *EchoRouter) POST(path string, handler interfaces.Handler) {
	r.addRoute("POST", path, handler)
}

// PUT 注册PUT路由
func (r *EchoRouter) PUT(path string, handler interfaces.Handler) {
	r.addRoute("PUT", path, handler)
}

// DELETE 注册DELETE路由
func (r *EchoRouter) DELETE(path string, handler interfaces.Handler) {
	r.addRoute("DELETE", path, handler)
}

// PATCH 注册PATCH路由
func (r *EchoRouter) PATCH(path string, handler interfaces.Handler) {
	r.addRoute("PATCH", path, handler)
}

// HEAD 注册HEAD路由
func (r *EchoRouter) HEAD(path string, handler interfaces.Handler) {
	r.addRoute("HEAD", path, handler)
}

// OPTIONS 注册OPTIONS路由
func (r *EchoRouter) OPTIONS(path string, handler interfaces.Handler) {
	r.addRoute("OPTIONS", path, handler)
}

// Use 添加中间�?
func (r *EchoRouter) Use(middleware interfaces.Middleware) {
	r.addUse(middleware)
}

// Start 启动服务�?
func (r *EchoRouter) Start(addr string) error {
	return r.echo.Start(addr)
}

// Group 创建路由�?
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

// Static 服务静态文�?
func (r *EchoRouter) Static(prefix, root string) {
	if r.group != nil {
		r.group.Static(prefix, root)
	} else {
		r.echo.Static(prefix, root)
	}
}

// SetLogger 设置日志�?
func (r *EchoRouter) SetLogger(logger interfaces.Logger) {
	r.logger = logger
}

// wrapHandler 将shim.Handler包装为echo.HandlerFunc
func (r *EchoRouter) wrapHandler(h interfaces.Handler) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := &EchoContext{context: c, logger: r.logger}
		return h(ctx)
	}
}

// wrapMiddleware 将shim.Middleware包装为echo.MiddlewareFunc
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

// EchoContext 适配Echo的上下文
type EchoContext struct {
	context echo.Context
	logger  interfaces.Logger
}

// Request 返回HTTP请求
func (c *EchoContext) Request() *http.Request {
	return c.context.Request()
}

// Method 返回请求方法
func (c *EchoContext) Method() string {
	return c.context.Request().Method
}

// Path 返回请求路径
func (c *EchoContext) Path() string {
	return c.context.Request().URL.Path
}

// QueryParam 获取查询参数
func (c *EchoContext) QueryParam(name string) string {
	return c.context.QueryParam(name)
}

// Param 获取路径参数
func (c *EchoContext) Param(name string) string {
	return c.context.Param(name)
}

// Status 设置状态码
func (c *EchoContext) Status(code int) {
	c.context.Response().Status = code
}

// JSON 返回JSON响应
func (c *EchoContext) JSON(code int, obj interface{}) error {
	return c.context.JSON(code, obj)
}

// Text 返回文本响应
func (c *EchoContext) Text(code int, text string) error {
	return c.context.String(code, text)
}

// HTML 返回HTML响应
func (c *EchoContext) HTML(code int, html string) error {
	return c.context.HTML(code, html)
}

// Redirect 重定�?`n
func (c *EchoContext) Redirect(code int, url string) error {
	return c.context.Redirect(code, url)
}

// Set 设置�?`n
func (c *EchoContext) Set(key string, value interface{}) {
	c.context.Set(key, value)
}

// Get 获取�?`n
func (c *EchoContext) Get(key string) interface{} {
	return c.context.Get(key)
}

// Context 返回Go上下�?`n
func (c *EchoContext) Context() context.Context {
	return c.context.Request().Context()
}

// BindJSON 绑定JSON请求�?`
func (c *EchoContext) BindJSON(obj interface{}) error {
	return c.context.Bind(obj)
}

// BindXML 绑定XML请求�?`n
func (c *EchoContext) BindXML(obj interface{}) error {
	return c.context.Bind(obj)
}

// BindQuery 绑定查询参数到结构体
func (c *EchoContext) BindQuery(obj interface{}) error {
	// Echo没有内置BindQuery，这里简化实�?
	return nil
}

// Cookie 获取Cookie
func (c *EchoContext) Cookie(name string) (string, error) {
	cookie, err := c.context.Cookie(name)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// SetCookie 设置Cookie
func (c *EchoContext) SetCookie(cookie *http.Cookie) {
	c.context.SetCookie(cookie)
}

// Logger 返回日志�?`n
func (c *EchoContext) Logger() interfaces.Logger {
	if logger, ok := c.Get("logger").(interfaces.Logger); ok && logger != nil {
		return logger
	}
	return c.logger
}

// XML 返回XML响应
func (c *EchoContext) XML(code int, obj interface{}) error {
	return c.context.XML(code, obj)
}

// FormValue 获取表单字段�?`n
func (c *EchoContext) FormValue(key string) string {
	return c.context.FormValue(key)
}

// PostForm 获取POST表单字段�?`n
func (c *EchoContext) PostForm(key string) string {
	return c.context.FormValue(key) // Echo的FormValue处理POST和GET
}

// ParseForm 解析表单
func (c *EchoContext) ParseForm() error {
	return c.context.Request().ParseForm()
}

// ParseMultipartForm 解析多部分表�?`n
func (c *EchoContext) ParseMultipartForm(maxMemory int64) error {
	return c.context.Request().ParseMultipartForm(maxMemory)
}
