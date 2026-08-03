package context

import "time"

// EngineType represents the web engine backend.
type EngineType string

const (
	EngineHertz    EngineType = "hertz"
	EngineChi      EngineType = "chi"
	EngineGin      EngineType = "gin"
	EngineEcho     EngineType = "echo"
	EngineFiber    EngineType = "fiber"
	EngineFasthttp EngineType = "fasthttp"
)

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port           int           `mapstructure:"port"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	IdleTimeout    time.Duration `mapstructure:"idle_timeout"`
	MaxHeaderBytes int           `mapstructure:"max_header_bytes"`
}

// DefaultServerConfig returns a ServerConfig with sensible defaults.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Port:           8080,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
}

// LogConfig holds logger configuration.
type LogConfig struct {
	Backend  string `mapstructure:"backend"`
	FileDir  string `mapstructure:"file_dir"`
	Format   string `mapstructure:"format"`
	Level    string `mapstructure:"level"`
	Stdout   bool   `mapstructure:"stdout"`
	MaxAge   int    `mapstructure:"max_age"`
	Rotation bool   `mapstructure:"rotation"`
}

// DefaultLogConfig returns a LogConfig with sensible defaults.
// FileDir is left empty; set via app.yml (e.g. /app/log for Docker volume mapping).
func DefaultLogConfig() LogConfig {
	return LogConfig{
		Backend: "zap",
		Format:  "text",
		Level:   "info",
		Stdout:  true,
	}
}

// RouteGroupConfig defines a route group with its handler.
type RouteGroupConfig struct {
	Prefix  string `mapstructure:"prefix"`
	Handler string `mapstructure:"handler"` // handler class name
}
