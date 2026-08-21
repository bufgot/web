package feign

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// HTTPError represents a non-2xx response returned by the upstream service.
type HTTPError struct {
	StatusCode int
	Status     string
	Body       []byte
	Message    string
}

// Error implements error.
func (e *HTTPError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = strings.TrimSpace(string(e.Body))
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
	}
	if msg == "" {
		msg = e.Status
	}
	return fmt.Sprintf("feign: unexpected status %d %s: %s", e.StatusCode, e.Status, msg)
}

// decodeErrorMessage best-effort extracts a human-readable message from an
// error response body. It tries {"message": "..."} / {"error": "..."} first,
// then falls back to the raw body.
func decodeErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err == nil {
		for _, key := range []string{"message", "error", "msg", "detail"} {
			if v, ok := m[key].(string); ok && v != "" {
				return v
			}
		}
	}
	return ""
}

// RespMeta carries HTTP response metadata for methods with a 3-value signature.
type RespMeta struct {
	StatusCode int
	Status     string
	Header     http.Header
}
