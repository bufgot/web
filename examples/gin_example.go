package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bufgot/web"
	ginAdapter "github.com/bufgot/web/engine/gin"
)

func main() {
	// Create Gin adapter and router via unified interface
	adapter := ginAdapter.NewGinAdapter()
	router := adapter.NewRouter()

	// Register middleware
	router.Use(requestLogger())

	// Register routes
	router.GET("/hello", helloHandler)
	router.POST("/echo", echoHandler)

	// Start server with graceful shutdown
	if err := startServer(router); err != nil {
		panic(err)
	}
}

func helloHandler(ctx web.Context) error {
	return ctx.JSON(http.StatusOK, map[string]string{"message": "Hello from Gin!"})
}

func echoHandler(ctx web.Context) error {
	body := ctx.Request().Body
	if body != nil {
		defer body.Close()
	}
	return ctx.JSON(http.StatusOK, map[string]string{
		"method": ctx.Method(),
		"path":   ctx.Path(),
	})
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
	type shutdowner interface {
		Shutdown(ctx context.Context) error
	}

	s, ok := router.(starter)
	if !ok {
		return nil
	}

	go func() {
		println("Gin server starting on :8080")
		if err := s.Start(":8080"); err != nil {
			println("Server error:", err.Error())
		}
	}()

	// Wait for signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	println("Shutting down...")
	if sd, ok := router.(shutdowner); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return sd.Shutdown(ctx)
	}
	return nil
}
