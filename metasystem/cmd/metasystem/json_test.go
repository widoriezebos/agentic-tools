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

func TestJSONGetShellSafePrintsExactStringWithoutNewline(t *testing.T) {
	out, code := captureStdout(t, func() int {
		return runJSONGet([]string{"--value", `{"text":"line one\nline two"}`, "--field", "text", "--shell-safe"})
	})
	if code != 0 || out != "line one\nline two" {
		t.Fatalf("shell-safe string = status %d output %q", code, out)
	}

	for _, input := range []string{`{"text":"\u0000"}`, `{"text":7}`, string([]byte{'{', '"', 't', 'e', 'x', 't', '"', ':', '"', 0xff, '"', '}'})} {
		_, code = captureStdout(t, func() int {
			return runJSONGet([]string{"--value", input, "--field", "text", "--shell-safe"})
		})
		if code != 1 {
			t.Fatalf("unsafe shell value %q status = %d, want 1", input, code)
		}
	}
	_, code = captureStdout(t, func() int {
		return runJSONGet([]string{"--value", `{"outer":7}`, "--field", "outer.text", "--default", "fallback", "--shell-safe"})
	})
	if code != 1 {
		t.Fatalf("shell-safe scalar traversal status = %d, want 1", code)
	}

	out, code = captureStdout(t, func() int {
		return runJSONGet([]string{"--value", `{"text":null}`, "--field", "text", "--default", "", "--shell-safe"})
	})
	if code != 0 || out != "" {
		t.Fatalf("shell-safe default = status %d output %q", code, out)
	}
}
