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

## 如何使用 Chi

通过代码构建使用 Chi 框架的 bufgot/web 项目，步骤如下：

### 1. 安装依赖

```bash
go get github.com/bufgot/web
go get github.com/go-chi/chi/v5
```

### 2. 创建适配器与路由

```go
package main

import (
    "context"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/bufgot/web"
    chiAdapter "github.com/bufgot/web/engine/chi"
)

func main() {
    adapter := chiAdapter.NewChiAdapter()
    router := adapter.NewRouter()

    // 注册中间件
    router.Use(requestLogger())

    // 注册路由
    router.GET("/hello", func(c web.Context) error {
        return c.JSON(http.StatusOK, map[string]string{"message": "Hello from Chi!"})
    })
    router.POST("/echo", func(c web.Context) error {
        return c.JSON(http.StatusOK, map[string]string{
            "method": c.Method(),
            "path":   c.Path(),
        })
    })

    // 启动服务
    if err := startServer(router); err != nil {
        panic(err)
    }
}

func requestLogger() web.Middleware {
    return func(next web.Handler) web.Handler {
        return func(ctx web.Context) error {
            println("[request]", ctx.Method(), ctx.Path())
            return next(ctx)
        }
    }
}

func startServer(router web.Router) error {
    type starter interface {
        Start(addr string) error
    }
    s, ok := router.(starter)
    if !ok {
        return nil
    }

    go func() {
        if err := s.Start(":8080"); err != nil {
            println("Server error:", err.Error())
        }
    }()

    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
    <-sigs

    println("Shutting down...")
    return nil
}
```

### 3. 运行

```bash
go run ./examples/chi_example.go
```

完整示例见 [examples/chi_example.go](examples/chi_example.go)。

## 如何使用 Echo

与 Chi 类似，通过 `engine/echo` 包创建适配器即可，API 完全一致：

```go
import echoAdapter "github.com/bufgot/web/engine/echo"

adapter := echoAdapter.NewEchoAdapter()
router := adapter.NewRouter()
```

完整示例见 [examples/echo_example.go](examples/echo_example.go)。

## 如何使用 FastHTTP

通过 `engine/fasthttp` 包创建适配器：

```go
import fasthttpAdapter "github.com/bufgot/web/engine/fasthttp"

adapter := fasthttpAdapter.NewFasthttpAdapter()
router := adapter.NewRouter()
```

完整示例见 [examples/fasthttp_example.go](examples/fasthttp_example.go)。

## 如何使用 Fiber

通过 `engine/fiber` 包创建适配器：

```go
import fiberAdapter "github.com/bufgot/web/engine/fiber"

adapter := fiberAdapter.NewFiberAdapter()
router := adapter.NewRouter()
```

完整示例见 [examples/fiber_example.go](examples/fiber_example.go)。

## 如何使用 Gin

通过 `engine/gin` 包创建适配器：

```go
import ginAdapter "github.com/bufgot/web/engine/gin"

adapter := ginAdapter.NewGinAdapter()
router := adapter.NewRouter()
```

完整示例见 [examples/gin_example.go](examples/gin_example.go)。

## 如何使用 Hertz

通过 `engine/hertz` 包创建适配器：

```go
import hertzAdapter "github.com/bufgot/web/engine/hertz"

adapter := hertzAdapter.NewHertzAdapter()
router := adapter.NewRouter()
```

完整示例见 [examples/hertz_example.go](examples/hertz_example.go)。

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