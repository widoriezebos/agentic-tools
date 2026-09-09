package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCapContractPublicAdapterAndRetiredVerb is the public edge of the
// Go-owned cap-contract group. Its bounded, protocol, and severe matrices are
// selected beside it from the existing dispatch owners in testing.json.
func TestCapContractPublicAdapterAndRetiredVerb(t *testing.T) {
	record := filepath.Join(t.TempDir(), "critic.json")
	if err := os.WriteFile(record, []byte("{\"role\":\"code-critic\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"runtime", []string{"--stage", "initial", "--record", record, "--cli-status", "7", "--handshake-done"}, "finish failed protocol_error runtime"},
		{"delivery", []string{"--stage", "empty-reply", "--record", record, "--handshake-done"}, "finish failed protocol_error delivery"},
	} {
		t.Run(test.name, func(t *testing.T) {
			output, code := captureStdout(t, func() int { return runAdapterAdjudicateTurn(test.args) })
			if code != 0 || strings.TrimSpace(output) != test.want {
				t.Fatalf("adapter cap outcome: code=%d output=%q want=%q", code, output, test.want)
			}
		})
	}
	stderr, code := captureStderr(t, func() int {
		return dispatch([]string{"job", "exhaustion-patches"})
	})
	if code != 2 || stderr != "metasystem job: unknown verb \"exhaustion-patches\"\n" {
		t.Fatalf("retired public cap verb: code=%d stderr=%q", code, stderr)
	}
}
