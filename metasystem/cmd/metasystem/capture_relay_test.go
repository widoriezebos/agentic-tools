package main

import (
	"io"
	"testing"
)

// captureRelay runs a verb function on streams of its own and returns its
// stdout, stderr and exit code.
func captureRelay(t *testing.T, fn func(stdout, stderr io.Writer) int) (stdout, stderr string, code int) {
	t.Helper()
	code, stdout, stderr = runOnOwnStreams(fn)
	return stdout, stderr, code
}
