package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestLaunchSettingsPrintsEachValueAndSource(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	tracked, err := os.ReadFile(filepath.Join("..", "..", "metasystem.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), tracked, 0o600); err != nil {
		t.Fatal(err)
	}
	oldExecutable, oldLookup := launchExecutable, launchLookupEnv
	launchExecutable = func() (string, error) { return filepath.Join(root, "bin", "metasystem"), nil }
	launchLookupEnv = func(string) (string, bool) { return "", false }
	t.Cleanup(func() { launchExecutable, launchLookupEnv = oldExecutable, oldLookup })
	code, stdout, stderr := captureCommandOutput(t, true, true, func() int { return runLaunchSettings(nil) })
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 14 || lines[0] != "launch.seat.window.tokens=200000 source=conf" || !strings.HasPrefix(lines[13], "context.handoff.margin.tokens=") {
		t.Fatalf("lines=%q", lines)
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
