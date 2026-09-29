package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Port of conformance-fixtures.sh's seat-receipt leg: the conformance review
// refuses a delegate's receipt (internal/validate), while the ordinary seat
// entrypoint stays free to append its own landing receipt.
func TestConformanceBedSeatReceiptEntrypointAppends(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return runReceipt([]string{"add", "--type", "implement", "--outcome", "shipped", "--verify", "clean",
			"--goal", "seat-owned", "--built-by", "coordinator", "--note", "fixture seat landing", "--root", root, "--file", filepath.Join(root, "memory", "receipts.log")}, stdout, stderr)
	})
	if code != 0 {
		t.Fatalf("receipt add = %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	data, err := os.ReadFile(filepath.Join(root, "memory", "receipts.log"))
	if err != nil || !strings.Contains(string(data), "|goal=seat-owned|built_by=coordinator|") {
		t.Fatalf("the seat receipt entrypoint did not append its own landing receipt: %q %v", data, err)
	}
}
