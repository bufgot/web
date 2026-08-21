// Package swagger provides a swagger documentation middleware for bufgot/web.
//
// It serves a swagger-ui page (loaded from a CDN) plus a raw OpenAPI JSON spec
// document. When disabled, handlers return 404.
package swagger

import (
	"encoding/json"
	"net/http"

	web "github.com/bufgot/web"
)

// Config controls the swagger middleware.
type Config struct {
	// Enable turns the swagger handlers on.
	Enable bool `mapstructure:"enable" json:"enable" yaml:"enable"`
	// Title is the documentation title shown in the swagger-ui page.
	Title string `mapstructure:"title" json:"title" yaml:"title"`
	// Version is the API version shown in the swagger-ui page.
	Version string `mapstructure:"version" json:"version" yaml:"version"`
	// Spec is an optional OpenAPI JSON document. When nil, a minimal
	// auto-generated spec (info only) is served.
	Spec json.RawMessage
}

// DefaultConfig returns a Config with sensible defaults (disabled).
func DefaultConfig() Config {
	return Config{Title: "API Documentation", Version: "1.0.0"}
}

// Swagger serves the swagger documentation UI at the configured route.
type Swagger struct {
	cfg Config
}

// New creates a Swagger instance with the given config.
func New(cfg Config) *Swagger {
	if cfg.Title == "" {
		cfg.Title = "API Documentation"
	}
	if cfg.Version == "" {
		cfg.Version = "1.0.0"
	}
	return &Swagger{cfg: cfg}
}

// Handler returns a web.Handler that serves the swagger-ui page at /swagger.
// When disabled it returns 404.
func (s *Swagger) Handler() web.Handler {
	if !s.cfg.Enable {
		return func(ctx web.Context) error {
			return ctx.Text(http.StatusNotFound, "swagger disabled")
		}
	}
	return func(ctx web.Context) error {
		return ctx.HTML(http.StatusOK, s.pageHTML())
	}
}

// SpecHandler returns a web.Handler that serves the raw OpenAPI JSON spec at
// /swagger/spec. When disabled it returns 404.
func (s *Swagger) SpecHandler() web.Handler {
	if !s.cfg.Enable {
		return func(ctx web.Context) error {
			return ctx.Text(http.StatusNotFound, "swagger disabled")
		}
	}
	spec := s.specJSON()
	return func(ctx web.Context) error {
		ctx.Set("Content-Type", "application/json")
		return ctx.JSON(http.StatusOK, spec)
	}
}

// specJSON returns the OpenAPI spec to serve (user-supplied or auto-generated).
func (s *Swagger) specJSON() interface{} {
	if len(s.cfg.Spec) > 0 {
		var spec interface{}
		if err := json.Unmarshal(s.cfg.Spec, &spec); err == nil {
			return spec
		}
	}
	return map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":   s.cfg.Title,
			"version": s.cfg.Version,
		},
		"paths": map[string]interface{}{},
	}
}

// pageHTML renders the swagger-ui page pointing at /swagger/spec.
func (s *Swagger) pageHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>` + htmlEscape(s.cfg.Title) + `</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css"/>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function () {
      window.ui = SwaggerUIBundle({ url: '/swagger/spec', dom_id: '#swagger-ui' });
    };
  </script>
</body>
</html>`
}

func htmlEscape(s string) string {
	var b []byte
	for _, r := range s {
		switch r {
		case '&':
			b = append(b, "&amp;"...)
		case '<':
			b = append(b, "&lt;"...)
		case '>':
			b = append(b, "&gt;"...)
		case '"':
			b = append(b, "&quot;"...)
		case '\'':
			b = append(b, "&#39;"...)
		default:
			b = append(b, string(r)...)
		}
	}
	return string(b)
}
