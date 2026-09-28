package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func writeShippedSeatWindow(t *testing.T, root string, tokens int64, present bool) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(launch.ShippedClaudeSettingsSource))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	content := "{\"hooks\":{}}\n"
	if present {
		content = fmt.Sprintf("{\"autoCompactWindow\":%d,\"hooks\":{}}\n", tokens)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
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
	code, stdout, stderr := captureCommandOutput(t, true, true, func() int { return runLaunchReport([]string{"--id", record.ID}) })
	if code != 0 || stderr != "" || !strings.Contains(stdout, "declared-lines=42 read-mode=package") || !strings.Contains(stdout, "verdict-counts=false") || !strings.Contains(stdout, "rerun-split") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
