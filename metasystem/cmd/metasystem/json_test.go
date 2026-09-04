package main

import (
	"strings"
	"testing"
)

func TestJSONGetAbsentFieldHasDistinctStatus(t *testing.T) {
	_, code := captureStdout(t, func() int {
		return runJSONGet([]string{"--value", `{"present":null}`, "--field", "absent"})
	})
	if code != 3 {
		t.Fatalf("absent field status = %d, want 3", code)
	}

	out, code := captureStdout(t, func() int {
		return runJSONGet([]string{"--value", `{"present":null}`, "--field", "present"})
	})
	if code != 0 || strings.TrimSpace(out) != "null" {
		t.Fatalf("present null = status %d output %q, want status 0 output null", code, out)
	}

	_, code = captureStdout(t, func() int {
		return runJSONGet([]string{"--value", `{`, "--field", "absent"})
	})
	if code != 1 {
		t.Fatalf("malformed JSON status = %d, want 1", code)
	}
}
