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
	// The tracked conf imposes no seat window, so the shipped Claude settings
	// match it by carrying no autoCompactWindow at all, and no drift is reported.
	writeShippedSeatWindow(t, root, 0, false)
	oldExecutable, oldLookup := launchExecutable, launchLookupEnv
	launchExecutable = func() (string, error) { return filepath.Join(root, "bin", "metasystem"), nil }
	launchLookupEnv = func(string) (string, bool) { return "", false }
	t.Cleanup(func() { launchExecutable, launchLookupEnv = oldExecutable, oldLookup })
	code, stdout, stderr := captureCommandOutput(t, true, true, func() int { return runLaunchSettings(nil) })
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	// Read by key, not by position: the resolver grows new settings (the lane
	// runtimes did), and each one must still print once with its value and source.
	printed := settingLinesByKey(t, stdout)
	want := map[string]string{
		launch.SeatWindowKey:            "0 source=conf",
		launch.ShippedSeatWindowKey:     "0 source=absent",
		"context.ceiling.tokens":        "250000 source=conf",
		"context.handoff.margin.tokens": "145000 source=conf",
	}
	trackedValues := trackedConfValues(string(tracked))
	for _, setting := range launch.DefaultSettings().Values {
		if _, pinned := want[setting.Key]; pinned {
			continue
		}
		if value, ok := trackedValues[setting.Key]; ok {
			want[setting.Key] = value + " source=conf"
		} else {
			want[setting.Key] = setting.Value + " source=default"
		}
	}
	for _, key := range []string{launch.BuildRuntimeKey, launch.CritiqueRuntimeKey, launch.DesignRuntimeKey, launch.ReadRuntimeKey} {
		if !strings.HasSuffix(want[key], " source=conf") {
			t.Fatalf("tracked conf does not carry runtime setting %s: want=%q", key, want[key])
		}
	}
	if len(printed) != len(want) {
		t.Fatalf("printed %d settings, want %d: printed=%q want=%q", len(printed), len(want), printed, want)
	}
	for key, value := range want {
		if printed[key] != value {
			t.Fatalf("%s printed %q, want %q; stdout=%q", key, printed[key], value, stdout)
		}
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
	if len(values) != len(want) {
		t.Fatalf("json carries %d settings, want %d: %+v", len(values), len(want), values)
	}
	var shipped launch.Setting
	for _, value := range values {
		if _, ok := want[value.Key]; !ok {
			t.Fatalf("json carries unexpected setting %+v", value)
		}
		if value.Key == launch.ShippedSeatWindowKey {
			shipped = value
		}
	}
	if shipped.Key != launch.ShippedSeatWindowKey || shipped.Value != "200000" || shipped.Source != launch.ShippedClaudeSettingsSource || shipped.ShippedDiffersFromConf == nil || !*shipped.ShippedDiffersFromConf {
		t.Fatalf("shipped setting=%+v", shipped)
	}

	writeShippedSeatWindow(t, root, 0, false)
	code, stdout, stderr = captureCommandOutput(t, true, true, func() int { return runLaunchSettings(nil) })
	if code != 0 || stderr != "" || !strings.Contains(stdout, "launch.seat.window.shipped=0 source=absent shipped-differs-from-conf\n") {
		t.Fatalf("absent shipped setting: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

// settingLinesByKey maps each printed "key=value source=..." line to the text
// after its key, refusing a line without a key or a key printed twice.
func settingLinesByKey(t *testing.T, stdout string) map[string]string {
	t.Helper()
	printed := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		key, rest, ok := strings.Cut(line, "=")
		if !ok || key == "" {
			t.Fatalf("setting line without a key: %q", line)
		}
		if _, seen := printed[key]; seen {
			t.Fatalf("setting %s printed twice: %q", key, stdout)
		}
		printed[key] = rest
	}
	return printed
}

// trackedConfValues reads the uncommented key=value lines of a conf file.
func trackedConfValues(conf string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(conf, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok {
			values[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return values
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
