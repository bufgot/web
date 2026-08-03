package web

import "time"

// WebConfig holds all configurable parameters for the web server.
type WebConfig struct {
	// Port is the TCP port the server listens on.
	Port int `mapstructure:"port" json:"port" yaml:"port"`

	// ReadTimeout is the maximum duration for reading the entire request.
	ReadTimeout time.Duration `mapstructure:"read_timeout" json:"read_timeout" yaml:"read_timeout"`

	// WriteTimeout is the maximum duration before timing out writes of the response.
	WriteTimeout time.Duration `mapstructure:"write_timeout" json:"write_timeout" yaml:"write_timeout"`

	// IdleTimeout is the maximum amount of time to wait for the next request.
	IdleTimeout time.Duration `mapstructure:"idle_timeout" json:"idle_timeout" yaml:"idle_timeout"`

	// MaxHeaderBytes is the maximum size of request headers.
	MaxHeaderBytes int `mapstructure:"max_header_bytes" json:"max_header_bytes" yaml:"max_header_bytes"`
}

// DefaultWebConfig returns a WebConfig with sensible defaults:
// port 8080, 30s read/write timeout, 60s idle timeout, 1MB max headers.
func DefaultWebConfig() *WebConfig {
	return &WebConfig{
		Port:           8080,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}
}
