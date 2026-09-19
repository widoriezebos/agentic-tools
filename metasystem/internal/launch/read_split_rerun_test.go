package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestReadWithoutDiffRefusesBeforeRecord(t *testing.T) {
	t.Parallel()
	m, _, _, _ := manager(t)
	m.Supervisor = childStarter(m)
	_, err := m.Start(StartSpec{ID: "unsized-read", Kind: "read", Brief: writeLaunchFile(t, "brief.md", "read\n"), WorkingDirectory: t.TempDir()})
	if err == nil || !strings.HasPrefix(err.Error(), "LAUNCH_READ_UNSIZED") {
		t.Fatalf("error=%v", err)
	}
	if _, statErr := os.Stat(filepath.Join(m.Store.Root, "unsized-read")); !os.IsNotExist(statErr) {
		t.Fatalf("state directory exists: %v", statErr)
	}
	rows, readErr := m.Store.Refusals()
	if readErr != nil || len(rows) != 1 || rows[0].Code != "LAUNCH_READ_UNSIZED" {
		t.Fatalf("rows=%+v err=%v", rows, readErr)
	}
}

func TestMissingVerdictCountsOnReadDoesNotCount(t *testing.T) {
	t.Parallel()
	if (Record{Kind: "read"}).VerdictIsCounting() {
		t.Fatal("legacy read record without verdictCounts counted")
	}
	if !(Record{Kind: "critique"}).VerdictIsCounting() {
		t.Fatal("non-read legacy verdict changed")
	}
}

func TestFinishedReadRecordExplicitlyCounts(t *testing.T) {
	t.Parallel()
	m, _, _, _ := manager(t)
	m.Adapters["claude-headless"] = fakeAdapter{}
	record := Record{ID: "counting-read", Kind: "read", Adapter: "claude-headless", WorkingDirectory: t.TempDir(), State: Starting, StartedAt: m.Now().Format("2006-01-02T15:04:05Z07:00"), AdapterData: map[string]json.RawMessage{}}
	if err := m.Store.Create(record); err != nil {
		t.Fatal(err)
	}
	finished, err := m.Supervise(record.ID)
	if err != nil || finished.VerdictCounts == nil || !*finished.VerdictCounts {
		t.Fatalf("record=%+v err=%v", finished, err)
	}
}

func TestCompactedWholeReadRerunsPackages(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, sampleDiff())
	fixture.manager.Settings.ReadSplitLines = 100
	fixture.starter.readCounts = []bool{false, true, true}
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	round := result.Record.Rounds[0]
	if round.Outcome != "green" {
		t.Fatalf("outcome=%s steps=%+v", round.Outcome, round.Steps)
	}
	var reruns []string
	for _, step := range round.Steps {
		if step.Rerun {
			reruns = append(reruns, step.Package+":"+step.Mode)
		}
	}
	if !slices.Equal(reruns, []string{"pkg/a:package", "pkg/b:package"}) {
		t.Fatalf("reruns=%v", reruns)
	}
}

func TestCompactedPackageRerunEndsReadCompacted(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, sampleDiff())
	fixture.manager.Settings.ReadSplitLines = 100
	fixture.starter.readCounts = []bool{false, false, true}
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	round := result.Record.Rounds[0]
	if round.Outcome != "read-compacted" {
		t.Fatalf("outcome=%s steps=%+v", round.Outcome, round.Steps)
	}
	compacted := false
	for _, step := range round.Steps {
		if step.Rerun && step.Package == "pkg/a" && step.VerdictCounts != nil && !*step.VerdictCounts {
			compacted = true
		}
	}
	if !compacted {
		t.Fatalf("steps=%+v", round.Steps)
	}
}

func TestCompactedPackageReadRerunsWide(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, multiFilePackageDiff())
	fixture.manager.Settings.ReadSplitLines = 4
	fixture.starter.readCounts = []bool{false, true, true, true}
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	round := result.Record.Rounds[0]
	if round.Outcome != "green" {
		t.Fatalf("outcome=%s steps=%+v", round.Outcome, round.Steps)
	}
	var files []string
	for _, step := range round.Steps {
		if !step.Rerun {
			continue
		}
		launchRecord, readErr := fixture.manager.Store.Read(step.LaunchID)
		data, diffErr := os.ReadFile(filepath.Join(fixture.manager.Store.Root, step.LaunchID, "read.diff"))
		other := map[string]string{"pkg/a/a.go": "pkg/a/c.go", "pkg/a/c.go": "pkg/a/a.go"}[step.File]
		if readErr != nil || diffErr != nil || step.Mode != "file" || step.Package != "pkg/a" || launchRecord.ReadMode != "file" || launchRecord.ReadPackage != "pkg/a" || launchRecord.ReadFile != step.File || launchRecord.ChangedLines != 2 || !strings.Contains(string(data), step.File) || strings.Contains(string(data), other) {
			t.Fatalf("step=%+v launch=%+v diff=%q errors=%v/%v", step, launchRecord, data, readErr, diffErr)
		}
		files = append(files, step.File)
	}
	if !slices.Equal(files, []string{"pkg/a/a.go", "pkg/a/c.go"}) {
		t.Fatalf("file reruns=%v", files)
	}
}

func TestCompactedSingleFilePackageReadEndsWithoutRerun(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, sampleDiff())
	fixture.manager.Settings.ReadSplitLines = 2
	fixture.starter.readCounts = []bool{false, true}
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	round := result.Record.Rounds[0]
	for _, step := range round.Steps {
		if step.Rerun {
			t.Fatalf("single-file share reran: %+v", round.Steps)
		}
	}
	if round.Outcome != "read-compacted" {
		t.Fatalf("outcome=%s steps=%+v", round.Outcome, round.Steps)
	}
}

func TestCompactedPerFileRerunEndsReadCompacted(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, multiFilePackageDiff())
	fixture.manager.Settings.ReadSplitLines = 4
	fixture.starter.readCounts = []bool{false, true, false, true}
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	round := result.Record.Rounds[0]
	reruns := 0
	for _, step := range round.Steps {
		if step.Rerun {
			reruns++
		}
	}
	if round.Outcome != "read-compacted" || reruns != 2 {
		t.Fatalf("outcome=%s reruns=%d steps=%+v", round.Outcome, reruns, round.Steps)
	}
}

func TestCompactedWideReadRerunsPackages(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, multiFilePackageDiff())
	fixture.manager.Settings.ReadSplitLines = 2
	fixture.starter.readCounts = []bool{false, true, true}
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	var reruns []string
	for _, step := range result.Record.Rounds[0].Steps {
		if step.Rerun {
			reruns = append(reruns, step.Package+":"+step.Mode)
		}
	}
	if result.Record.Rounds[0].Outcome != "green" || !slices.Equal(reruns, []string{"pkg/a:package", "pkg/b:package"}) {
		t.Fatalf("outcome=%s reruns=%v", result.Record.Rounds[0].Outcome, reruns)
	}
}

func TestLargeSingleDirectoryDiffStartsOneWideRead(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "diff --git a/pkg/a/a.go b/pkg/a/a.go\n--- a/pkg/a/a.go\n+++ b/pkg/a/a.go\n-old\n+new\ndiff --git a/pkg/a/c.go b/pkg/a/c.go\n--- a/pkg/a/c.go\n+++ b/pkg/a/c.go\n-old\n+new\n")
	fixture.manager.Settings.ReadSplitLines = 2
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	var reads []UnitStep
	for _, step := range result.Record.Rounds[0].Steps {
		if strings.HasPrefix(step.Name, "read") {
			reads = append(reads, step)
		}
	}
	if len(reads) != 1 || reads[0].Mode != "wide" || reads[0].Package != "" {
		t.Fatalf("reads=%+v", reads)
	}
}

func multiFilePackageDiff() string {
	return "diff --git a/pkg/a/a.go b/pkg/a/a.go\n--- a/pkg/a/a.go\n+++ b/pkg/a/a.go\n-old\n+new\ndiff --git a/pkg/a/c.go b/pkg/a/c.go\n--- a/pkg/a/c.go\n+++ b/pkg/a/c.go\n-old\n+new\ndiff --git a/pkg/b/b.go b/pkg/b/b.go\n--- a/pkg/b/b.go\n+++ b/pkg/b/b.go\n-old\n+new\n"
}

func TestIndependentReadStartsThroughLaunchVerbs(t *testing.T) {
	t.Parallel()
	const sentence = "An independent read of a unit diff starts through `metasystem launch start --kind read --diff-file`, or through `metasystem unit run`, never as an in-process agent."
	root := moduleRoot(t)
	for _, path := range []string{
		filepath.Join(root, "docs", "project-rules.md"),
		filepath.Join(root, "scripts", "agents", "templates", "review-brief.md"),
	} {
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), sentence) {
			t.Errorf("%s does not route independent reads through launch verbs: %v", path, err)
		}
	}
}
