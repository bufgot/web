package gin

import (
	"context"
	"net/http"

	interfaces "github.com/bufgot/web"
	"github.com/gin-gonic/gin"
)

// GinAdapter 实现WebFramework接口
type GinAdapter struct{}

// NewGinAdapter 创建Gin适配�?
func NewGinAdapter() *GinAdapter {
	return &GinAdapter{}
}

// Name 返回框架名称
func (g *GinAdapter) Name() string {
	return "gin"
}

// NewRouter 创建新的路由�?
func (g *GinAdapter) NewRouter() interfaces.Router {
	return &GinRouter{
		gin:    gin.New(),
		group:  nil, // 初始时没有路由组
		logger: interfaces.NewDefaultLogger(),
	}
}

// GinRouter 适配Gin's路由�?
type GinRouter struct {
	gin    *gin.Engine
	group  *gin.RouterGroup // 当前路由组，如果为nil则使用gin.Engine
	logger interfaces.Logger
}

// currentRouter 返回当前使用的路由器
func (r *GinRouter) currentRouter() interface{} {
	if r.group != nil {
		return r.group
	}
	return r.gin
}

// addRoute 向当前路由器添加路由
func (r *GinRouter) addRoute(method, path string, handler interfaces.Handler) {
	if r.group != nil {
		r.group.Handle(method, path, r.wrapHandler(handler))
	} else {
		r.gin.Handle(method, path, r.wrapHandler(handler))
	}
}

// addUse 向当前路由器添加中间�?
func (r *GinRouter) addUse(middleware interfaces.Middleware) {
	if r.group != nil {
		r.group.Use(r.wrapMiddleware(middleware))
	} else {
		r.gin.Use(r.wrapMiddleware(middleware))
	}
}

// GET 注册GET路由
func (r *GinRouter) GET(path string, handler interfaces.Handler) {
	r.addRoute("GET", path, handler)
}

// POST 注册POST路由
func (r *GinRouter) POST(path string, handler interfaces.Handler) {
	r.addRoute("POST", path, handler)
}

// PUT 注册PUT路由
func (r *GinRouter) PUT(path string, handler interfaces.Handler) {
	r.addRoute("PUT", path, handler)
}

// DELETE 注册DELETE路由
func (r *GinRouter) DELETE(path string, handler interfaces.Handler) {
	r.addRoute("DELETE", path, handler)
}

// PATCH 注册PATCH路由
func (r *GinRouter) PATCH(path string, handler interfaces.Handler) {
	r.addRoute("PATCH", path, handler)
}

// HEAD 注册HEAD路由
func (r *GinRouter) HEAD(path string, handler interfaces.Handler) {
	r.addRoute("HEAD", path, handler)
}

// OPTIONS 注册OPTIONS路由
func (r *GinRouter) OPTIONS(path string, handler interfaces.Handler) {
	r.addRoute("OPTIONS", path, handler)
}

// Use 添加中间�?
func (r *GinRouter) Use(middleware interfaces.Middleware) {
	r.addUse(middleware)
}

// Start 启动服务�?
func (r *GinRouter) Start(addr string) error {
	return r.gin.Run(addr)
}

// Group 创建路由�
func (r *GinRouter) Group(prefix string, middlewares ...interfaces.Middleware) interfaces.Router {
	var newGroup *gin.RouterGroup
	if r.group != nil {
		newGroup = r.group.Group(prefix)
	} else {
		newGroup = r.gin.Group(prefix)
	}
	for _, middleware := range middlewares {
		newGroup.Use(r.wrapMiddleware(middleware))
	}
	return &GinRouter{
		gin:    r.gin,
		group:  newGroup,
		logger: r.logger,
	}
}

// Static 服务静态文�?
func (r *GinRouter) Static(prefix, root string) {
	if r.group != nil {
		r.group.Static(prefix, root)
	} else {
		r.gin.Static(prefix, root)
	}
}

// SetLogger 设置日志�?
func (r *GinRouter) SetLogger(logger interfaces.Logger) {
	r.logger = logger
}

// GetGinEngine 获取底层的gin.Engine
func (r *GinRouter) GetGinEngine() *gin.Engine {
	return r.gin
}

// wrapHandler 将shim.Handler包装为gin.HandlerFunc
func (r *GinRouter) wrapHandler(h interfaces.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := &GinContext{context: c, logger: r.logger}
		h(ctx)
	}
}

// wrapMiddleware 将shim.Middleware包装为gin.HandlerFunc
func (r *GinRouter) wrapMiddleware(m interfaces.Middleware) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := &GinContext{context: c}
		handler := m(func(ctx interfaces.Context) error {
			c.Next()
			return nil
		})
		handler(ctx)
	}
}

// GinContext 适配Gin's上下�?
type GinContext struct {
	context *gin.Context
	logger  interfaces.Logger
}

// Request 返回HTTP请求
func (c *GinContext) Request() *http.Request {
	return c.context.Request
}

// Method 返回请求方法
func (c *GinContext) Method() string {
	return c.context.Request.Method
}

// Path 返回请求路径
func (c *GinContext) Path() string {
	return c.context.Request.URL.Path
}

// QueryParam 获取查询参数
func (c *GinContext) QueryParam(name string) string {
	return c.context.Query(name)
}

// Param 获取路径参数
func (c *GinContext) Param(name string) string {
	return c.context.Param(name)
}

// Status 设置状态码
func (c *GinContext) Status(code int) {
	c.context.Status(code)
}

// JSON 返回JSON响应
func (c *GinContext) JSON(code int, obj interface{}) error {
	c.context.JSON(code, obj)
	return nil
}

// Text 返回文本响应
func (c *GinContext) Text(code int, text string) error {
	c.context.String(code, text)
	return nil
}

// HTML 返回HTML响应
func (c *GinContext) HTML(code int, html string) error {
	c.context.HTML(code, html, nil)
	return nil
}

// Redirect 重定�?
func (c *GinContext) Redirect(code int, url string) error {
	c.context.Redirect(code, url)
	return nil
}

// Set 设置�
func (c *GinContext) Set(key string, value interface{}) {
	c.context.Set(key, value)
}

// Get 获取�?
func (c *GinContext) Get(key string) interface{} {
	val, _ := c.context.Get(key)
	return val
}

// Context 返回Go上下�?
func (c *GinContext) Context() context.Context {
	return c.context.Request.Context()
}

// BindJSON 绑定JSON请求�?
func (c *GinContext) BindJSON(obj interface{}) error {
	return c.context.ShouldBindJSON(obj)
}

// BindXML 绑定XML请求�?
func (c *GinContext) BindXML(obj interface{}) error {
	return c.context.ShouldBindXML(obj)
}

// BindQuery 绑定查询参数到结构体
func (c *GinContext) BindQuery(obj interface{}) error {
	return c.context.ShouldBindQuery(obj)
}

// Cookie 获取Cookie
func (c *GinContext) Cookie(name string) (string, error) {
	return c.context.Cookie(name)
}

// SetCookie 设置Cookie
func (c *GinContext) SetCookie(cookie *http.Cookie) {
	c.context.SetCookie(cookie.Name, cookie.Value, cookie.MaxAge, cookie.Path, cookie.Domain, cookie.Secure, cookie.HttpOnly)
}

// Logger 返回日志�?
func (c *GinContext) Logger() interfaces.Logger {
	if logger, ok := c.Get("logger").(interfaces.Logger); ok && logger != nil {
		return logger
	}
	return c.logger
}

// XML 返回XML响应
func (c *GinContext) XML(code int, obj interface{}) error {
	c.context.XML(code, obj)
	return nil
}

// FormValue 获取表单字段�?
func (c *GinContext) FormValue(key string) string {
	return c.context.Request.FormValue(key)
}

// PostForm 获取POST表单字段�?
func (c *GinContext) PostForm(key string) string {
	return c.context.PostForm(key)
}

// ParseForm 解析表单
func (c *GinContext) ParseForm() error {
	return c.context.Request.ParseForm()
}

// ParseMultipartForm 解析多部分表�?
func (c *GinContext) ParseMultipartForm(maxMemory int64) error {
	return c.context.Request.ParseMultipartForm(maxMemory)
}
