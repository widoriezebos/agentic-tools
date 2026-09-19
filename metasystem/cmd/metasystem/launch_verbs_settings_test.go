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
	writeShippedSeatWindow(t, root, 200000, true)
	oldExecutable, oldLookup := launchExecutable, launchLookupEnv
	launchExecutable = func() (string, error) { return filepath.Join(root, "bin", "metasystem"), nil }
	launchLookupEnv = func(string) (string, bool) { return "", false }
	t.Cleanup(func() { launchExecutable, launchLookupEnv = oldExecutable, oldLookup })
	code, stdout, stderr := captureCommandOutput(t, true, true, func() int { return runLaunchSettings(nil) })
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 18 || lines[0] != "launch.seat.window.tokens=200000 source=conf" || lines[15] != "launch.seat.window.shipped=200000 source=scripts/enforcement/claude-code-hooks.json" || lines[16] != "context.ceiling.tokens=250000 source=conf" || lines[17] != "context.handoff.margin.tokens=145000 source=conf" {
		t.Fatalf("lines=%q", lines)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("launch.seat.window.tokens=210000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeShippedSeatWindow(t, root, 200000, true)
	code, stdout, stderr = captureCommandOutput(t, true, true, func() int { return runLaunchSettings(nil) })
	if code != 0 || stderr != "" || !strings.Contains(stdout, "launch.seat.window.shipped=200000 source=scripts/enforcement/claude-code-hooks.json shipped-differs-from-conf\n") {
		t.Fatalf("differing shipped setting: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, stdout, stderr = captureCommandOutput(t, true, true, func() int { return runLaunchSettings([]string{"--json"}) })
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	var values []launch.Setting
	if err := json.Unmarshal([]byte(stdout), &values); err != nil {
		t.Fatal(err)
	}
	shipped := values[15]
	if shipped.Key != launch.ShippedSeatWindowKey || shipped.Value != "200000" || shipped.Source != launch.ShippedClaudeSettingsSource || shipped.ShippedDiffersFromConf == nil || !*shipped.ShippedDiffersFromConf {
		t.Fatalf("shipped setting=%+v", shipped)
	}

	writeShippedSeatWindow(t, root, 0, false)
	code, stdout, stderr = captureCommandOutput(t, true, true, func() int { return runLaunchSettings(nil) })
	if code != 0 || stderr != "" || !strings.Contains(stdout, "launch.seat.window.shipped=0 source=absent shipped-differs-from-conf\n") {
		t.Fatalf("absent shipped setting: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

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
