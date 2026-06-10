package main

import (
	"net/http"

	ginAdapter "github.com/bufgot/web/engine/gin" // 只导入gin，避免编译其他框�?
	"github.com/bufgot/web/loggers"
	""
	"github.com/gin-gonic/gin"
)

func main() {
	// 创建gin适配�?
	adapter := ginAdapter.NewGinAdapter()

	// 创建路由�?
	router := adapter.NewRouter()

	// 设置日志�?
	logger := stdlib.NewDefaultLogger()
	router.SetLogger(logger)

	// 添加中间�?- 使用正确的参数类�?
	router.Use(loggers.StructuredLoggerMiddleware(logger, map[string]interface{}{"service": "my-gin-service"}))

	// 添加路由 - 使用gin原生handler
	ginRouter := router.(*ginAdapter.GinRouter)
	ginRouter.GetGinEngine().GET("/hello", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Hello from Gin!"})
	})

	// 启动服务�?
	ginRouter.Start(":8080")
}






