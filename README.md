# web

一个为多个 Go web 框架提供统一接口的库，从 go-web-shim 项目迁移而来。

## 特性

- 统一的 Handler、Middleware、Context 和 Router 接口
- 支持多种流行的 Go web 框架
- 简化的跨框架开发体验

## 支持的框架

- Gin
- Echo
- Chi
- Fiber
- Hertz
- FastHTTP

## 安装

```bash
go get github.com/bufgot/web
```

## 使用示例

```go
package main

import (
    "github.com/bufgot/web"
    "github.com/bufgot/web/engine/gin"
)

func main() {
    adapter := gin.NewGinAdapter()
    router := adapter.NewRouter()

    router.GET("/hello", func(c web.Context) error {
        return c.Text(200, "Hello, World!")
    })

    // 启动服务器（具体实现取决于框架）
}
```

## 接口定义

### Handler

```go
type Handler func(Context) error
```

### Middleware

```go
type Middleware func(Handler) Handler
```

### Context

提供统一的请求和响应接口，包括 JSON、Text、HTML 等方法。

### Router

提供路由注册和中间件支持。

## 贡献

欢迎提交 Issue 和 Pull Request。

## 许可证

MIT License