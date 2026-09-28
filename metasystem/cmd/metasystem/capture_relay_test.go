package main

import "testing"

// captureRelay runs a verb function and returns its stdout, stderr and exit
// code.
func captureRelay(t *testing.T, fn func() int) (stdout, stderr string, code int) {
	t.Helper()
	code, stdout, stderr = captureCommandOutput(t, true, true, fn)
	return stdout, stderr, code
}
