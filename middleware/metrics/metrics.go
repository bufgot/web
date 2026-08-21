// Package metrics provides a Prometheus metrics middleware for bufgot/web.
//
// It records request count and duration per (method, path, status) tuple and
// exposes the collected metrics via a /metrics handler. The status code is
// captured by wrapping the ResponseWriter via Context.SetResponseWriter on
// net/http based engines (chi, gin, echo). Engines without an
// http.ResponseWriter (hertz, fiber, fasthttp) still record count and
// duration with status "unknown".
package metrics

import (
	"net/http"
	"strconv"
	"time"

	web "github.com/bufgot/web"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Config controls the metrics middleware.
type Config struct {
	// Enable turns the metrics middleware and handler on.
	Enable bool `mapstructure:"enable" json:"enable" yaml:"enable"`
	// Prefix is the metric name prefix; defaults to "http".
	Prefix string `mapstructure:"prefix" json:"prefix" yaml:"prefix"`
}

// DefaultConfig returns a Config with sensible defaults (disabled).
func DefaultConfig() Config {
	return Config{Prefix: "http"}
}

// writerSetter is implemented by engine contexts that allow replacing the
// underlying http.ResponseWriter (chi, gin, echo).
type writerSetter interface {
	SetResponseWriter(http.ResponseWriter)
}

// Metrics bundles the registry and collectors shared between the middleware
// and the /metrics handler.
type Metrics struct {
	cfg     Config
	enabled bool
	reg     *prometheus.Registry
	reqCnt  *prometheus.CounterVec
	reqDur  *prometheus.HistogramVec
}

// New creates a Metrics instance. When cfg.Enable is false, Middleware() and
// Handler() become no-ops.
func New(cfg Config) *Metrics {
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "http"
	}
	m := &Metrics{cfg: cfg}
	if !cfg.Enable {
		return m
	}
	m.enabled = true
	m.reg = prometheus.NewRegistry()
	m.reqCnt = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prefix + "_requests_total",
		Help: "Total number of HTTP requests processed.",
	}, []string{"method", "path", "status"})
	m.reqDur = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    prefix + "_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path", "status"})
	m.reg.MustRegister(m.reqCnt, m.reqDur)
	return m
}

// Middleware returns a web.Middleware that records request metrics.
func (m *Metrics) Middleware() web.Middleware {
	if !m.enabled {
		return func(next web.Handler) web.Handler { return next }
	}
	return func(next web.Handler) web.Handler {
		return func(ctx web.Context) error {
			start := time.Now()

			// Wrap the ResponseWriter to capture the status code written by
			// downstream handlers. Restored right after next() returns.
			w := ctx.ResponseWriter()
			rec := &responseRecorder{ResponseWriter: w}
			if w != nil {
				if ws, ok := ctx.(writerSetter); ok {
					ws.SetResponseWriter(rec)
				}
			}

			err := next(ctx)

			if w != nil {
				if ws, ok := ctx.(writerSetter); ok {
					ws.SetResponseWriter(w)
				}
			}

			status := rec.statusCode
			if status == 0 {
				status = http.StatusOK
			}
			path := ctx.Path()
			if path == "" {
				path = "/"
			}
			labels := prometheus.Labels{
				"method": ctx.Method(),
				"path":   path,
				"status": strconv.Itoa(status),
			}
			m.reqCnt.With(labels).Inc()
			m.reqDur.With(labels).Observe(time.Since(start).Seconds())
			return err
		}
	}
}

// Handler returns a web.Handler exposing the Prometheus metrics at /metrics.
// When metrics are disabled it returns 404.
func (m *Metrics) Handler() web.Handler {
	if !m.enabled {
		return func(ctx web.Context) error {
			return ctx.Text(http.StatusNotFound, "metrics disabled")
		}
	}
	return func(ctx web.Context) error {
		w := ctx.ResponseWriter()
		if w == nil {
			return ctx.Text(http.StatusServiceUnavailable, "metrics endpoint not supported on this engine")
		}
		promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{}).ServeHTTP(w, ctx.Request())
		return nil
	}
}

// responseRecorder wraps http.ResponseWriter to capture the status code.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// Unwrap returns the underlying ResponseWriter.
func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
