package main

import (
	"strings"
	"testing"
)

func TestHealthIsRegisteredAtTheTopLevel(t *testing.T) {
	stderr, code := captureStderr(t, func() int {
		return dispatch([]string{"health"})
	})
	if code != 2 || !strings.Contains(stderr, "health: --repo is required") || strings.Contains(stderr, "unknown family") {
		t.Fatalf("top-level health must route to the steward health implementation: code=%d stderr=%q", code, stderr)
	}
}

func TestHealthAcknowledgmentIsRegisteredAtTheTopLevel(t *testing.T) {
	stderr, code := captureStderr(t, func() int {
		return dispatch([]string{"health", "acknowledge-alert"})
	})
	if code != 2 || !strings.Contains(stderr, "--episode is required") || strings.Contains(stderr, "unknown family") {
		t.Fatalf("acknowledge-alert must route through health: code=%d stderr=%q", code, stderr)
	}
}
