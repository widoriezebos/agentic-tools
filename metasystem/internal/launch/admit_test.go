package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeLaunchFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStartStoresModelEffortAndWindow(t *testing.T) {
	m, _, _, _ := manager(t)
	m.Supervisor = childStarter(m)
	m.Settings = DefaultSettings()
	record, err := m.Start(StartSpec{ID: "configured", Kind: "build", Brief: writeLaunchFile(t, "brief", "| Unit | Lines |\n|---|---|\n| configured | 20 |\n"), WorkingDirectory: t.TempDir(), Model: "override", Effort: "high"})
	if err != nil {
		t.Fatal(err)
	}
	if readString(record.AdapterData, "model") != "override" || readString(record.AdapterData, "effort") != "high" || readInt64(record.AdapterData, "window") != 0 {
		t.Fatalf("adapter data=%s", record.AdapterData)
	}
}

// The recorded window is 0 by default, which the adapters read as "impose no
// cap". A lane that needs a window different from its runtime's own puts a
// positive number in metasystem.conf and it travels through this same field.
func TestReadStartRecordsNoImposedWindow(t *testing.T) {
	t.Parallel()
	m, _, _, _ := manager(t)
	m.Supervisor = childStarter(m)
	m.Adapters["claude-headless"] = fakeAdapter{}
	record, err := m.Start(StartSpec{ID: "read-window", Kind: "read", Brief: writeLaunchFile(t, "brief.md", "read\n"), WorkingDirectory: t.TempDir(), DiffFile: writeLaunchFile(t, "change.diff", "")})
	if err != nil || readInt64(record.AdapterData, "window") != 0 {
		t.Fatalf("window=%d record=%+v err=%v", readInt64(record.AdapterData, "window"), record, err)
	}
}

// A configured read window is recorded for the selected adapter even though
// the shipped default imposes no cap.
func TestReadStartRecordsFourHundredThousandTokenWindow(t *testing.T) {
	t.Parallel()
	m, _, _, _ := manager(t)
	m.Supervisor = childStarter(m)
	m.Adapters["claude-headless"] = fakeAdapter{}
	m.Settings = DefaultSettings()
	m.Settings.ReadWindow = 400000
	record, err := m.Start(StartSpec{ID: "configured-read-window", Kind: "read", Brief: writeLaunchFile(t, "brief.md", "read\n"), WorkingDirectory: t.TempDir(), DiffFile: writeLaunchFile(t, "change.diff", "")})
	if err != nil || readInt64(record.AdapterData, "window") != 400000 {
		t.Fatalf("window=%d record=%+v err=%v", readInt64(record.AdapterData, "window"), record, err)
	}
}

func TestWindowReachesTheChildFromTheRecord(t *testing.T) {
	brief := writeLaunchFile(t, "brief", "hello")
	data := map[string]json.RawMessage{}
	setString(data, "brief", brief)
	setInt64(data, "window", 345678)
	codex, err := (CodexExec{}).Command(Record{Kind: "build", WorkingDirectory: t.TempDir(), AdapterData: data}, t.TempDir())
	if err != nil || len(codex.Args) < 3 || codex.Args[len(codex.Args)-2] != "model_auto_compact_token_limit=345678" || containsString(codex.Environment, "CODEX_CONTEXT_WINDOW=345678") {
		t.Fatalf("codex=%+v err=%v", codex, err)
	}
	claude, err := (ClaudeHeadless{}).Command(Record{Kind: "read", WorkingDirectory: t.TempDir(), AdapterData: data}, t.TempDir())
	if err != nil || !containsString(claude.Environment, "CLAUDE_CODE_AUTO_COMPACT_WINDOW=345678") {
		t.Fatalf("claude=%+v err=%v", claude, err)
	}
	delete(data, "window")
	uncapped, err := (ClaudeHeadless{}).Command(Record{Kind: "read", WorkingDirectory: t.TempDir(), AdapterData: data}, t.TempDir())
	if err != nil || len(uncapped.Environment) != 0 {
		t.Fatalf("missing window: environment=%q err=%v", uncapped.Environment, err)
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func TestWaitCapComesFromTheSetting(t *testing.T) {
	m, _, _, now := manager(t)
	m.Settings = DefaultSettings()
	m.Settings.WaitCapSeconds = 2
	seed(t, m, "capped-wait", Running)
	_, terminal, err := m.Wait("capped-wait", time.Hour)
	if err != nil || terminal || now.Sub(time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)) != 2*time.Second {
		t.Fatalf("terminal=%v now=%s err=%v", terminal, now, err)
	}
}

func TestRefusedLaunchLeavesNoRecordAndOneRefusalRow(t *testing.T) {
	m, _, _, _ := manager(t)
	m.Supervisor = childStarter(m)
	_, err := m.Start(StartSpec{ID: "unsized", Kind: "build", Brief: writeLaunchFile(t, "brief", "no declaration\n"), WorkingDirectory: t.TempDir()})
	if err == nil || !strings.HasPrefix(err.Error(), "LAUNCH_BUILD_UNSIZED") {
		t.Fatalf("error=%v", err)
	}
	if _, statErr := os.Stat(filepath.Join(m.Store.Root, "unsized")); !os.IsNotExist(statErr) {
		t.Fatalf("state directory exists: %v", statErr)
	}
	rows, readErr := m.Store.Refusals()
	if readErr != nil || len(rows) != 1 || rows[0].Code != "LAUNCH_BUILD_UNSIZED" {
		t.Fatalf("rows=%+v err=%v", rows, readErr)
	}
}

func TestOversizeBriefRefusesAndListsInputsLargestFirst(t *testing.T) {
	m, _, _, _ := manager(t)
	m.Settings = DefaultSettings()
	m.Settings.BriefCap = 2
	brief := writeLaunchFile(t, "brief", "Declared size: 1 changed lines\n")
	input := writeLaunchFile(t, "large", strings.Repeat("x", 80))
	err := m.Admit(StartSpec{Kind: "design", Brief: brief, Inputs: []string{input}})
	if err == nil || !strings.HasPrefix(err.Error(), "LAUNCH_BRIEF_OVERSIZE total=") {
		t.Fatalf("error=%v", err)
	}
	lines := strings.Split(err.Error(), "\n")
	if len(lines) < 3 || !strings.Contains(lines[1], input) || !strings.Contains(lines[2], brief) {
		t.Fatalf("lines=%q", lines)
	}
}

func TestOversizeUnitsPageRefuses(t *testing.T) {
	m, _, _, _ := manager(t)
	m.Settings = DefaultSettings()
	m.Settings.BriefCap = 8
	brief := writeLaunchFile(t, "brief", "x\n")
	page := writeLaunchFile(t, "units", "| Unit | Size |\n|---|---|\n| a | 1 |\n"+strings.Repeat("x", 80))
	err := m.Admit(StartSpec{Kind: "build", Brief: brief, UnitsPage: page, Units: []string{"a"}})
	if err == nil || !strings.HasPrefix(err.Error(), "LAUNCH_BRIEF_OVERSIZE") || !strings.Contains(err.Error(), "input="+page) {
		t.Fatalf("error=%v", err)
	}
}

func TestBuildSizeComesFromTheUnitRowsOrTheBriefLine(t *testing.T) {
	page := writeLaunchFile(t, "page", "| Unit | Changed lines |\n|---|---|\n| alpha. first | 400 plus witness 20 |\n| beta | 30 |\n")
	units, size, err := buildSize(StartSpec{UnitsPage: page, Units: []string{"beta", "alpha"}})
	if err != nil || size != 450 || len(units) != 2 || units[0].Name != "alpha" {
		t.Fatalf("units=%+v size=%d err=%v", units, size, err)
	}
	allUnits, allSize, allErr := buildSize(StartSpec{UnitsPage: page})
	if allErr != nil || allSize != 450 || len(allUnits) != 2 {
		t.Fatalf("all units=%+v size=%d err=%v", allUnits, allSize, allErr)
	}
	units, size, err = buildSize(StartSpec{Brief: writeLaunchFile(t, "brief-table", "| Unit | Lines |\n|---|---|\n| one | 40 |\n| two | 31 |\n")})
	if err != nil || size != 71 || len(units) != 2 {
		t.Fatalf("brief units=%+v size=%d err=%v", units, size, err)
	}
	_, size, err = buildSize(StartSpec{Brief: writeLaunchFile(t, "brief", "Declared size: 71 changed lines\n")})
	if err == nil || !strings.HasPrefix(err.Error(), "LAUNCH_BUILD_UNSIZED") {
		t.Fatalf("size=%d err=%v", size, err)
	}
}

func TestSizesFromTableStopsAtTheFirstNonTableLine(t *testing.T) {
	t.Parallel()
	page := strings.Join([]string{
		"| Unit | Lines |",
		"| --- | --- |",
		"| first | 40 |",
		"Count the output with command | wc -l.",
		"```sh",
		"printf '%s\\n' value | sed 's/value/other/'",
		"```",
		"| Unit | Lines |",
		"| --- | --- |",
		"| later | 900 |",
	}, "\n")

	units, size, err := sizesFromTable(page, nil)
	if err != nil || size != 40 || len(units) != 1 || units[0] != (UnitSize{Name: "first", Lines: 40}) {
		t.Fatalf("units=%+v size=%d err=%v", units, size, err)
	}
	units, size, err = sizesFromTable("| Unit | Lines |\n| --- | --- |\n| first | 40 |\n\n| later | 900 |\n", nil)
	if err != nil || size != 40 || len(units) != 1 || units[0].Name != "first" {
		t.Fatalf("blank boundary: units=%+v size=%d err=%v", units, size, err)
	}
}

func TestUnsizedBuildRefuses(t *testing.T) {
	_, _, err := buildSize(StartSpec{Brief: writeLaunchFile(t, "brief", "none\n")})
	if err == nil || !strings.Contains(err.Error(), "LAUNCH_BUILD_UNSIZED missing=declared-size") {
		t.Fatalf("error=%v", err)
	}
}

func TestOversizeBuildRefusesAndPrintsTheSerialSplit(t *testing.T) {
	m, _, _, _ := manager(t)
	m.Settings = DefaultSettings()
	m.Settings.BuildLinesCap = 100
	page := writeLaunchFile(t, "page", "| Unit | Size |\n|---|---|\n| a | 70 |\n| b | 60 |\n| c | 140 |\n")
	err := m.Admit(StartSpec{Kind: "build", Brief: writeLaunchFile(t, "brief", "brief\n"), UnitsPage: page, Units: []string{"a", "b", "c"}})
	if err == nil || !strings.Contains(err.Error(), "LAUNCH_BUILD_OVERSIZE size=270 cap=100") || !strings.Contains(err.Error(), "units=c size=140 over-cap") {
		t.Fatalf("error=%v", err)
	}
}

func sampleDiff() string {
	return "diff --git a/pkg/a/a.go b/pkg/a/a.go\n--- a/pkg/a/a.go\n+++ b/pkg/a/a.go\n-old\n+new\ndiff --git a/pkg/b/b.go b/pkg/b/b.go\n--- a/pkg/b/b.go\n+++ b/pkg/b/b.go\n-old\n+new\n"
}

func TestReadAboveTheSplitLineRefusesUnsplit(t *testing.T) {
	m, _, _, _ := manager(t)
	m.Settings = DefaultSettings()
	m.Settings.ReadSplitLines = 2
	diff := writeLaunchFile(t, "change.diff", sampleDiff())
	err := m.Admit(StartSpec{Kind: "read", Brief: writeLaunchFile(t, "brief", "read\n"), DiffFile: diff})
	if err == nil || !strings.Contains(err.Error(), "LAUNCH_READ_UNSPLIT choice=package") || !strings.Contains(err.Error(), "directory=pkg/a lines=2") {
		t.Fatalf("error=%v", err)
	}
}

func TestPackageReadGetsOnlyItsShareOfTheDiff(t *testing.T) {
	m, _, _, _ := manager(t)
	m.Settings = DefaultSettings()
	m.Settings.ReadSplitLines = 2
	m.Supervisor = childStarter(m)
	record, err := m.Start(StartSpec{ID: "package-read", Kind: "read", Brief: writeLaunchFile(t, "brief", "inspect\n"), WorkingDirectory: t.TempDir(), DiffFile: writeLaunchFile(t, "change.diff", sampleDiff()), Package: "pkg/a"})
	if err != nil {
		t.Fatal(err)
	}
	data, readErr := os.ReadFile(filepath.Join(m.Store.Root, record.ID, "read.diff"))
	if readErr != nil || !strings.Contains(string(data), "pkg/a") || strings.Contains(string(data), "pkg/b") || record.ReadMode != "package" || record.ChangedLines != 2 {
		t.Fatalf("record=%+v diff=%q err=%v", record, data, readErr)
	}
	command, err := (ClaudeHeadless{}).Command(record, filepath.Join(m.Store.Root, record.ID))
	if err != nil || !strings.Contains(command.Stdin, "Diff: "+filepath.Join(m.Store.Root, record.ID, "read.diff")) {
		t.Fatalf("stdin=%q err=%v", command.Stdin, err)
	}
}

type compactingAdapter struct{ fakeAdapter }

func (compactingAdapter) Measure(Record, string) (Measurement, []Output, map[string]json.RawMessage, error) {
	return Measurement{Compactions: 1}, nil, nil, nil
}

func TestCompactedReadDoesNotCount(t *testing.T) {
	m, _, _, _ := manager(t)
	m.Adapters["claude-headless"] = compactingAdapter{}
	seedRecord := Record{ID: "compacted-read", Kind: "read", Adapter: "claude-headless", WorkingDirectory: t.TempDir(), State: Starting, StartedAt: m.Now().Format(time.RFC3339Nano), AdapterData: map[string]json.RawMessage{}}
	if err := m.Store.Create(seedRecord); err != nil {
		t.Fatal(err)
	}
	record, err := m.Supervise(seedRecord.ID)
	if err != nil || record.VerdictCounts == nil || *record.VerdictCounts {
		t.Fatalf("record=%+v err=%v", record, err)
	}
}
