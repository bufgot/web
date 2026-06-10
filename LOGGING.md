# Go Web Shim - 结构化日志系�?

这个项目提供了一个完整的结构化日志系统，支持业务字段统一注入、request-id、trace-id和多种日志后端�?

## 特�?

- **统一接口**: 所有web框架使用相同的日志接�?
- **结构化日�?*: 支持带字段的日志记录
- **请求跟踪**: 自动生成request-id和trace-id
- **业务字段注入**: 支持全局业务字段注入
- **多后端支�?*: 支持标准库log、Zap等多种日志后�?
- **中间件集�?*: 自动为每个请求创建带上下文的logger

## 快速开�?

### 1. 基本使用

```go
import (
    "github.com/bufgot/web/adapters/gin"
    "github.com/bufgot/web/loggers"
    "go.uber.org/zap"
)

// 创建Zap logger
zapLogger, _ := zap.NewProduction()
logger := loggers.NewZapLogger(zapLogger)

// 创建路由�?
adapter := gin.NewGinAdapter()
router := adapter.NewRouter()
router.SetLogger(logger)

// 使用请求日志中间�?
router.Use(loggers.RequestLoggerMiddleware(logger))

router.GET("/hello", func(ctx interfaces.Context) error {
    // 在handler中使用logger
    ctx.Logger().Info("Hello World")
    return ctx.Text(200, "Hello World")
})

router.Start(":8080")
```

### 2. 结构化日志和业务字段

```go
// 业务字段
businessFields := map[string]interface{}{
    "service": "user-service",
    "version": "1.0.0",
    "env": "prod",
}

// 使用结构化日志中间件
router.Use(loggers.StructuredLoggerMiddleware(logger, businessFields))

router.GET("/users/:id", func(ctx interfaces.Context) error {
    userID := ctx.Param("id")

    // 使用便捷日志函数
    loggers.LogInfo(ctx, "fetching user", "user_id", userID)

    // 记录业务事件
    loggers.LogBusinessEvent(ctx, "user_view", "user_id", userID)

    return ctx.JSON(200, map[string]string{"user_id": userID})
})
```

### 3. 日志输出示例

```
[INFO] [request_id=abc123 trace_id=def456 method=GET path=/users/123 user_agent=Mozilla/5.0 service=user-service version=1.0.0 env=prod] fetching user user_id=123
[INFO] [request_id=abc123 trace_id=def456 method=GET path=/users/123 user_agent=Mozilla/5.0 service=user-service version=1.0.0 env=prod] business_event: user_view user_id=123
[INFO] [request_id=abc123 trace_id=def456 method=GET path=/users/123 user_agent=Mozilla/5.0 service=user-service version=1.0.0 env=prod] request completed duration=10.5ms status=200
```

## API 参�?

### Logger 接口

```go
type Logger interface {
    // 基础日志方法
    Info(msg string, args ...interface{})
    Warn(msg string, args ...interface{})
    Error(msg string, args ...interface{})
    Debug(msg string, args ...interface{})

    // 结构化日志方�?
    Infof(msg string, args ...interface{})
    Warnf(msg string, args ...interface{})
    Errorf(msg string, args ...interface{})
    Debugf(msg string, args ...interface{})

    // 带字段的日志方法
    Infow(msg string, keysAndValues ...interface{})
    Warnw(msg string, keysAndValues ...interface{})
    Errorw(msg string, keysAndValues ...interface{})
    Debugw(msg string, keysAndValues ...interface{})

    // 创建带附加字段的logger
    With(args ...interface{}) Logger
}
```

### 中间�?

- `RequestLoggerMiddleware(logger)`: 基础请求日志中间�?
- `StructuredLoggerMiddleware(logger, businessFields)`: 结构化日志中间件，支持业务字�?

### 工具函数

- `GetRequestID(ctx)`: 获取请求ID
- `GetTraceID(ctx)`: 获取trace ID
- `GetRequestLogger(ctx)`: 获取请求级logger
- `LogBusinessEvent(ctx, event, fields...)`: 记录业务事件
- `LogError(ctx, err, msg, fields...)`: 记录错误
- `LogWarn(ctx, msg, fields...)`: 记录警告
- `LogInfo(ctx, msg, fields...)`: 记录信息
- `LogDebug(ctx, msg, fields...)`: 记录调试信息

## 支持的日志后�?

### Zap Logger

```go
logger, _ := zap.NewProduction()
shimLogger := zap.NewZapLogger(logger)
```

### 标准�?Logger

```go
import "github.com/bufgot/web/loggers/default"
shimLogger := default.NewDefaultLogger()
```

## 扩展自定义Logger

实现 `interfaces.Logger` 接口即可�?

```go
type MyLogger struct {
    // 实现所有Logger接口方法
}

func (l *MyLogger) Info(msg string, args ...interface{}) { /* 实现 */ }
func (l *MyLogger) With(args ...interface{}) interfaces.Logger { /* 实现 */ }
// ... 其他方法
```

## 请求跟踪

系统自动处理以下HTTP头：

- `X-Trace-Id`: 优先使用此头作为trace ID
- `X-Request-Id`: 次优先使用此�?
- 如果都没有，会自动生成新的trace ID

每个请求都会自动生成唯一的request ID�
