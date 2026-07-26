package web

import (
	"os"
	"testing"

	"github.com/bufgot/log/factory"
)

func TestNewDefaultLogger(t *testing.T) {
	logger := NewDefaultLogger()
	if logger == nil {
		t.Fatal("NewDefaultLogger returned nil")
	}
	// Verify it works
	logger.Info("test")
}

func TestInit_DefaultLogBackend_Default(t *testing.T) {
	// Save and restore
	orig := DefaultLogBackend
	origEnv := os.Getenv("BUFGOT_LOG_BACKEND")
	os.Unsetenv("BUFGOT_LOG_BACKEND")
	defer func() {
		DefaultLogBackend = orig
		os.Setenv("BUFGOT_LOG_BACKEND", origEnv)
	}()

	// Reset to trigger init() behavior by re-setting
	DefaultLogBackend = orig

	if string(DefaultLogBackend) != "stdlib" {
		t.Fatalf("expected default backend 'stdlib', got '%s'", DefaultLogBackend)
	}
}

func TestNewDefaultLogger_AllBackends(t *testing.T) {
	backends := []string{"stdlib", "logrus", "zap"}
	for _, b := range backends {
		orig := DefaultLogBackend
		DefaultLogBackend = factory.Backend(b)
		logger := NewDefaultLogger()
		if logger == nil {
			t.Fatalf("NewDefaultLogger with %s returned nil", b)
		}
		logger.Info("test " + b)
		DefaultLogBackend = orig
	}
}

func TestNewDefaultLogger_Fallback(t *testing.T) {
	orig := DefaultLogBackend
	DefaultLogBackend = "nonexistent"
	defer func() { DefaultLogBackend = orig }()

	logger := NewDefaultLogger()
	if logger == nil {
		t.Fatal("NewDefaultLogger with invalid backend should fallback to stdlib")
	}
	logger.Info("fallback test")
}

func TestTypes(t *testing.T) {
	// Ensure interface types compile
	var _ Handler
	var _ Middleware
	var _ Context
	var _ Router
	var _ WebFramework
}
