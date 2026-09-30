package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
)

// TestCapContractPublicAdapterAndRetiredVerb is the adapter edge of the
// Go-owned cap-contract group. Its bounded, protocol, and severe matrices are
// selected beside it from the existing dispatch owners in testing.json.
func TestCapContractPublicAdapterAndRetiredVerb(t *testing.T) {
	record := filepath.Join(t.TempDir(), "critic.json")
	if err := os.WriteFile(record, []byte("{\"role\":\"code-critic\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		params adapter.AdjudicateParams
		want   string
	}{
		{"runtime", adapter.AdjudicateParams{Stage: "initial", RecordPath: record, CLIStatus: 7, HandshakeDone: true}, "finish failed protocol_error runtime"},
		{"delivery", adapter.AdjudicateParams{Stage: "empty-reply", RecordPath: record, HandshakeDone: true}, "finish failed protocol_error delivery"},
	} {
		t.Run(test.name, func(t *testing.T) {
			output, err := adapter.AdjudicateTurn(test.params)
			if err != nil || output != test.want {
				t.Fatalf("adapter cap outcome: err=%v output=%q want=%q", err, output, test.want)
			}
		})
	}
	stderr, code := captureStderr(t, func(stdout, stderr io.Writer) int {
		return dispatchOn([]string{"internal", "job", "exhaustion-patches"}, stdout, stderr)
	})
	// The internal job family itself is gone (U9b): the retired cap verb
	// routes nowhere and is refused as an entrypoint that does not exist.
	if code != 2 ||
		!strings.HasPrefix(stderr, "no internal entrypoint is named \"job\"; nothing was done\n") ||
		strings.Contains(stderr, "exhaustion-patches") {
		t.Fatalf("retired public cap verb: code=%d stderr=%q", code, stderr)
	}
}
