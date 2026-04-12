package client

import (
	"strings"
	"testing"
)

// TestSend_daemonNotRunning verifies Send returns an error containing "daemon not running"
// when no socket exists at the resolved path. HOME is set to a temp dir so sockAddr()
// points to a path that is guaranteed not to exist.
func TestSend_daemonNotRunning(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_, err := Send(Request{Action: "ping"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "daemon not running") {
		t.Fatalf("expected 'daemon not running' in error, got: %v", err)
	}
}
