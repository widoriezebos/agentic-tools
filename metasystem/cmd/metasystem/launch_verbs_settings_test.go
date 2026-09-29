package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// writeShippedSeatWindow stands in for the Claude settings the engine ships:
// tokens as their autoCompactWindow, or none when present is false.
func writeShippedSeatWindow(t *testing.T, tokens int64, present bool) {
	t.Helper()
	content := "{\"hooks\":{}}\n"
	if present {
		content = fmt.Sprintf("{\"autoCompactWindow\":%d,\"hooks\":{}}\n", tokens)
	}
	original := shippedClaudeSettings
	t.Cleanup(func() { shippedClaudeSettings = original })
	shippedClaudeSettings = func() ([]byte, error) { return []byte(content), nil }
}

func TestReportForOneLaunch(t *testing.T) {
	manager := &launch.Manager{Store: launch.Store{Root: t.TempDir()}, Now: func() time.Time { return time.Unix(1, 0) }}
	counts := false
	record := launch.Record{ID: "one-report", Kind: "read", Adapter: "claude-headless", State: launch.Completed, DeclaredLines: 42, ReadMode: "package", ReadPackage: "pkg/a", ChangedLines: 10, VerdictCounts: &counts, AdapterData: map[string]json.RawMessage{}}
	if err := manager.Store.Create(record); err != nil {
		t.Fatal(err)
	}
	old := launchManager
	launchManager = func() *launch.Manager { return manager }
	t.Cleanup(func() { launchManager = old })
	code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return runLaunchReport([]string{"--id", record.ID}, stdout, stderr)
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "declared-lines=42 read-mode=package") || !strings.Contains(stdout, "verdict-counts=false") || !strings.Contains(stdout, "rerun-split") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
