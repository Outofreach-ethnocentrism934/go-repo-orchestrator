package app

import (
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewRuntimeUsesStateDirForPlaywrightDriver(t *testing.T) {
	t.Parallel()

	runtime := NewRuntime("/var/lib/gbc-state", "/var/lib/gbc-state/workspace", time.Second, "", nil, zap.NewNop())
	if runtime == nil || runtime.Playwright == nil {
		t.Fatal("runtime playwright is required")
	}

	driverDirectory := runtime.Playwright.DriverDirectory()
	if !strings.Contains(driverDirectory, "/var/lib/gbc-state/playwright/driver/") {
		t.Fatalf("unexpected driver directory: %q", driverDirectory)
	}
}
