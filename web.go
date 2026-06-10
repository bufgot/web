package web

import (
	"context"
	"net/http"
)

// Handler 定义统一的处理器接口
type Handler func(Context) error

// Middleware 定义统一的中间件接口
type Middleware func(Handler) Handler

// Context 定义统一的上下文接口
type Context interface {
	// 请求相关
	Request() *http.Request
	Method() string
	Path() string
	QueryParam(name string) string
	Param(name string) string

	// 表单相关
	FormValue(key string) string
	PostForm(key string) string
	ParseForm() error
	ParseMultipartForm(maxMemory int64) error

	// 响应相关
	Status(code int)
	JSON(code int, obj interface{}) error
	Text(code int, text string) error
	HTML(code int, html string) error
	Redirect(code int, url string) error

	// Cookie相关
	Cookie(name string) (string, error)
	SetCookie(cookie *http.Cookie)

	// 日志
	Logger() Logger

	// 其他
	Set(key string, value interface{})
	Get(key string) interface{}
	Context() context.Context
}

// Router 定义统一的路由器接口
type Router interface {
	GET(path string, handler Handler)
	POST(path string, handler Handler)
	PUT(path string, handler Handler)
	DELETE(path string, handler Handler)
	PATCH(path string, handler Handler)
	HEAD(path string, handler Handler)
	OPTIONS(path string, handler Handler)

	Use(middleware Middleware)
	Group(prefix string, middlewares ...Middleware) Router
	Static(prefix, root string)
	SetLogger(logger Logger)
}

// WebFramework 定义Web框架接口
type WebFramework interface {
	Name() string
	NewRouter() Router
}
