package launch

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

// readGit is one test's Git for standalone reads: it answers the capture
// and context calls the read owner makes and fails any other call.
type readGit struct {
	t                 *testing.T
	mu                sync.Mutex
	top, head, index  string
	diff              []byte
	applyErr          error
	calls             []testgit.Call
	checkoutFileNames []string
	excluded          []string
}

func (git *readGit) Run(directory string, environment []string, args ...string) ([]byte, error) {
	git.mu.Lock()
	defer git.mu.Unlock()
	git.calls = append(git.calls, testgit.Call{Dir: directory, Env: slices.Clone(environment), Args: slices.Clone(args)})
	joined := strings.Join(args, " ")
	switch {
	case joined == "rev-parse --show-toplevel":
		return []byte(git.top + "\n"), nil
	case directory == git.top && joined == "rev-parse --verify HEAD^{commit}":
		return []byte(git.head + "\n"), nil
	case directory == git.top && joined == "rev-parse --path-format=absolute --git-path index":
		return []byte(git.index + "\n"), nil
	case directory == git.top && joined == "rev-parse --path-format=absolute --git-path objects":
		return []byte(filepath.Join(filepath.Dir(git.index), "objects") + "\n"), nil
	case directory == git.top && joined == "add -A --sparse -- .":
		return nil, nil
	case directory == git.top && strings.HasPrefix(joined, "diff --cached --binary "+git.head+" -- ."):
		git.excluded = append(git.excluded, args[6:]...)
		return git.diff, nil
	case directory == git.top && joined == "read-tree "+git.head:
		return nil, nil
	case directory == git.top && args[0] == "checkout-index":
		prefix := strings.TrimPrefix(args[2], "--prefix=")
		for _, name := range git.checkoutFileNames {
			os.MkdirAll(filepath.Dir(filepath.Join(prefix, name)), 0o700)
			os.WriteFile(filepath.Join(prefix, name), []byte("base\n"), 0o600)
		}
		return nil, nil
	case args[0] == "apply":
		return nil, git.applyErr
	}
	git.t.Errorf("unexpected Git call dir=%q args=%q", directory, args)
	return nil, errors.New("unexpected Git call")
}

func (git *readGit) count(subcommand string) int {
	git.mu.Lock()
	defer git.mu.Unlock()
	count := 0
	for _, call := range git.calls {
		if call.Args[0] == subcommand {
			count++
		}
	}
	return count
}

type standaloneFixture struct {
	unitFixture
	git     *readGit
	source  string
	request ReadRequest
}

func newStandaloneFixture(t *testing.T, diff string) standaloneFixture {
	t.Helper()
	fixture := baseUnitFixture(t)
	root := filepath.Dir(fixture.worktree)
	source := filepath.Join(root, "source")
	nested := filepath.Join(source, "metasystem")
	os.MkdirAll(nested, 0o700)
	os.WriteFile(filepath.Join(source, "tracked.go"), []byte("source\n"), 0o600)
	index := filepath.Join(root, "git", "index")
	os.MkdirAll(filepath.Dir(index), 0o700)
	os.WriteFile(index, []byte("index"), 0o600)
	git := &readGit{t: t, top: source, head: "0123456789abcdef0123456789abcdef01234567", index: index, diff: []byte(diff)}
	fixture.runner.Git = git
	brief := filepath.Join(root, "read-brief.md")
	os.WriteFile(brief, []byte("read the change\n"), 0o600)
	return standaloneFixture{unitFixture: fixture, git: git, source: source, request: ReadRequest{Directory: nested, Brief: brief}}
}

func readSteps(steps []UnitStep) []string {
	var names []string
	for _, step := range steps {
		if strings.HasPrefix(step.Name, "read") {
			names = append(names, step.Name+"|"+step.Mode+"|"+step.Package+"|"+step.File)
		}
	}
	return names
}

// TestStandaloneReadPartition proves a standalone read follows the same
// partition and compaction-rerun policy as a build's reads, needs no mode
// choice for a larger diff, runs only reads, and is never complete when a
// read failed or did not count.
func TestStandaloneReadPartition(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		counts   []bool
		fail     bool
		outcome  string
		complete bool
	}{
		{"compacted package reruns per file", []bool{false, true, true, true}, false, "green", true},
		{"compacted rerun stays incomplete", []bool{false, true, false, true}, false, "read-compacted", false},
		{"failed read stays incomplete", nil, true, "read-failed", false},
		// pkg/a's per-file reruns count, but pkg/b's single-file read has
		// no rerun of its own, so it never counted.
		{"mixed compaction leaves an uncovered package incomplete", []bool{false, false, true, true}, false, "read-compacted", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newStandaloneFixture(t, multiFilePackageDiff())
			fixture.manager.Settings.ReadSplitLines = 4
			fixture.starter.readCounts = slices.Clone(test.counts)
			if test.fail {
				fixture.starter.failKind = "read"
			}
			result, err := fixture.runner.StartRead(fixture.request)
			if err != nil {
				t.Fatal(err)
			}
			if result.Attempt.State != readAttemptFinished || result.Attempt.Outcome != test.outcome || result.Complete != test.complete {
				t.Fatalf("attempt=%+v complete=%t", result.Attempt, result.Complete)
			}
			for _, kind := range fixture.starter.order {
				if kind != "read" {
					t.Fatalf("a standalone read launched %q: %v", kind, fixture.starter.order)
				}
			}
			if len(result.Reports) != len(fixture.starter.ids) || len(result.Reports) == 0 {
				t.Fatalf("reports=%+v launches=%v", result.Reports, fixture.starter.ids)
			}
			for index, report := range result.Reports {
				if report.Launch != fixture.starter.ids[index] || !strings.HasPrefix(report.Launch, result.Ref+"-a1-s") || !strings.HasPrefix(report.Path, fixture.runner.readAttemptDir(result.Ref, 1)) {
					t.Fatalf("report %d=%+v launches=%v", index, report, fixture.starter.ids)
				}
				launchRecord, readErr := fixture.manager.Store.Read(report.Launch)
				if readErr != nil || launchRecord.Kind != "read" || launchRecord.Tag != result.Ref || !strings.Contains(string(launchRecord.AdapterData["unitReadPacket"]), "Diagnostic feedback only") {
					t.Fatalf("launch=%+v err=%v", launchRecord, readErr)
				}
			}
			if !slices.Equal(result.Request.Files, []string{"pkg/a/a.go", "pkg/a/c.go", "pkg/b/b.go"}) || result.Request.Base != fixture.git.head || result.Request.TopLevel != fixture.source {
				t.Fatalf("request=%+v", result.Request)
			}
			// The same diff through a unit run's reads takes the same steps.
			unit := newUnitFixture(t, multiFilePackageDiff())
			unit.manager.Settings.ReadSplitLines = 4
			unit.starter.readCounts = slices.Clone(test.counts)
			if test.fail {
				unit.starter.failKind = "read"
			}
			unitResult, err := unit.runner.Advance(UnitRequest{Plan: unit.plan})
			if err != nil {
				t.Fatal(err)
			}
			if got, want := readSteps(result.Attempt.Round.Steps), readSteps(unitResult.Record.Rounds[0].Steps); !slices.Equal(got, want) || unitResult.Record.Rounds[0].Outcome != test.outcome {
				t.Fatalf("standalone steps=%v unit steps=%v unit outcome=%s", got, want, unitResult.Record.Rounds[0].Outcome)
			}
			if unitResult.Record.Rounds[0].Outcome == "green" != test.complete {
				t.Fatalf("unit outcome=%s complete=%t", unitResult.Record.Rounds[0].Outcome, test.complete)
			}
			for _, step := range result.Attempt.Round.Steps {
				if step.Rerun && step.Package == "pkg/b" {
					t.Fatalf("single-file package reran: %+v", result.Attempt.Round.Steps)
				}
			}
		})
	}
	t.Run("similar package names keep separate reports", func(t *testing.T) {
		t.Parallel()
		diff := "diff --git a/pkg/a_b/x.go b/pkg/a_b/x.go\n--- a/pkg/a_b/x.go\n+++ b/pkg/a_b/x.go\n-old\n+new\n" +
			"diff --git a/pkg/a/b/y.go b/pkg/a/b/y.go\n--- a/pkg/a/b/y.go\n+++ b/pkg/a/b/y.go\n-old\n+new\n"
		fixture := newStandaloneFixture(t, diff)
		fixture.manager.Settings.ReadSplitLines = 2
		fixture.starter.onStart = func(record Record) error {
			var outputs []string
			if err := json.Unmarshal(record.AdapterData["declaredOutputs"], &outputs); err != nil || len(outputs) != 1 {
				return errors.New("one declared report expected")
			}
			return os.WriteFile(outputs[0], []byte("feedback for "+record.ReadPackage+"\n"), 0o600)
		}
		first, err := fixture.runner.StartRead(fixture.request)
		if err != nil || !first.Complete {
			t.Fatalf("first=%+v err=%v", first.Attempt, err)
		}
		launches := len(fixture.starter.ids)
		repeat, err := fixture.runner.StartRead(fixture.request)
		if err != nil || len(fixture.starter.ids) != launches {
			t.Fatalf("repeat=%+v err=%v", repeat.Attempt, err)
		}
		shown, err := fixture.runner.InspectRead(first.Ref)
		if err != nil {
			t.Fatal(err)
		}
		for _, result := range []ReadResult{first, repeat, shown} {
			byPackage := map[string]string{}
			for _, report := range result.Reports {
				data, readErr := os.ReadFile(report.Path)
				launchRecord, _ := fixture.manager.Store.Read(report.Launch)
				if readErr != nil || !report.Retained || string(data) != "feedback for "+launchRecord.ReadPackage+"\n" {
					t.Fatalf("report=%+v data=%q err=%v", report, data, readErr)
				}
				byPackage[launchRecord.ReadPackage] = report.Path
			}
			if len(result.Reports) != 2 || byPackage["pkg/a_b"] == "" || byPackage["pkg/a/b"] == "" || byPackage["pkg/a_b"] == byPackage["pkg/a/b"] {
				t.Fatalf("reports=%+v", result.Reports)
			}
		}
		// A retained launch's own declared output names its report, so an
		// attempt written under another naming is still shown truthfully.
		launch := shown.Reports[0].Launch
		legacy := filepath.Join(filepath.Dir(shown.Reports[0].Path), "legacy.md")
		os.WriteFile(legacy, []byte("legacy feedback\n"), 0o600)
		if _, err := fixture.manager.Store.Update(launch, func(record *Record) error {
			setStrings(record.AdapterData, "declaredOutputs", []string{legacy})
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if again, err := fixture.runner.InspectRead(first.Ref); err != nil || again.Reports[0].Path != legacy || again.Reports[0].Bytes != int64(len("legacy feedback\n")) {
			t.Fatalf("legacy report=%+v err=%v", again.Reports, err)
		}
	})
	t.Run("empty changes start nothing", func(t *testing.T) {
		t.Parallel()
		fixture := newStandaloneFixture(t, "")
		result, err := fixture.runner.StartRead(fixture.request)
		if err != nil || !result.Empty || result.Ref != "" || len(fixture.starter.ids) != 0 {
			t.Fatalf("result=%+v launches=%v err=%v", result, fixture.starter.ids, err)
		}
		if _, statErr := os.Stat(filepath.Join(fixture.runner.root(), ".reads")); !os.IsNotExist(statErr) {
			t.Fatalf("empty request reserved: %v", statErr)
		}
	})
}

// TestStandaloneReadReplayAndRecovery proves an identical request rejoins
// its reads whatever their state, that retry creates exactly one new attempt
// only after the displayed attempt is proved stopped and replays after that,
// and that changed source is a new request whose frozen bytes do not move.
func TestStandaloneReadReplayAndRecovery(t *testing.T) {
	t.Parallel()
	fixture := newStandaloneFixture(t, sampleDiff())
	fixture.starter.failKind = "read"
	first, err := fixture.runner.StartRead(fixture.request)
	if err != nil || first.Attempt.Outcome != "read-failed" || first.Complete {
		t.Fatalf("first=%+v err=%v", first.Attempt, err)
	}
	launches := len(fixture.starter.ids)
	again, err := fixture.runner.StartRead(fixture.request)
	if err != nil || again.Ref != first.Ref || again.Attempt.Number != 1 || len(fixture.starter.ids) != launches {
		t.Fatalf("rejoin=%+v launches=%d err=%v", again.Attempt, len(fixture.starter.ids), err)
	}
	for _, retry := range []int{2, 7} {
		request := fixture.request
		request.Retry = retry
		if _, err := fixture.runner.StartRead(request); err == nil || !strings.HasPrefix(err.Error(), "READ_RETRY_NOT_LATEST") {
			t.Fatalf("retry %d err=%v", retry, err)
		}
	}
	// A start is not proved stopped by its state: one a supervisor has
	// claimed, or one still inside the start cap, refuses the retry. Only
	// the launch startup recovery, past the cap, settles it, and the late
	// supervisor then refuses to start a child.
	fixture.manager.StartCap = time.Minute
	stranded := first.Reports[0].Launch
	strandedRecord, _ := fixture.manager.Store.Read(stranded)
	os.RemoveAll(filepath.Join(fixture.manager.Store.Root, stranded))
	claimed := ref(10)
	strandedRecord.State, strandedRecord.FinishedAt, strandedRecord.ExitCode, strandedRecord.Supervisor = Starting, "", nil, &claimed
	strandedRecord.StartedAt = fixture.manager.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	if err := fixture.manager.Store.Create(strandedRecord); err != nil {
		t.Fatal(err)
	}
	retry := fixture.request
	retry.Retry = 1
	if _, err := fixture.runner.StartRead(retry); err == nil || !strings.HasPrefix(err.Error(), "READ_RETRY_UNPROVEN") || !strings.Contains(err.Error(), stranded) {
		t.Fatalf("claimed start retry err=%v", err)
	}
	if _, err := fixture.manager.Store.Update(stranded, func(record *Record) error {
		record.Supervisor, record.StartedAt = nil, fixture.manager.Now().UTC().Format(time.RFC3339Nano)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.runner.StartRead(retry); err == nil || !strings.HasPrefix(err.Error(), "READ_RETRY_UNPROVEN") {
		t.Fatalf("start inside the cap retry err=%v", err)
	}
	if shown, err := fixture.runner.InspectRead(first.Ref); err != nil || shown.Attempt.Number != 1 {
		t.Fatalf("show=%+v err=%v", shown.Attempt, err)
	}
	if launchRecord, _ := fixture.manager.Store.Read(stranded); launchRecord.State != Starting {
		t.Fatalf("a refused retry or show changed the launch: %+v", launchRecord)
	}
	fixture.manager.Sleep(2 * time.Minute)
	fixture.starter.failKind = ""
	second, err := fixture.runner.StartRead(retry)
	if err != nil || second.Ref != first.Ref || second.Attempt.Number != 2 || second.Attempt.RetryOf != 1 || !second.Complete {
		t.Fatalf("retry=%+v err=%v", second.Attempt, err)
	}
	recovered, _ := fixture.manager.Store.Read(stranded)
	if recovered.State != Failed || recovered.Reason != "supervisor-start-unrecorded" {
		t.Fatalf("stranded start not recovered: %+v", recovered)
	}
	processes := fixture.manager.Processes.(*fakeProcesses)
	if _, err := fixture.manager.Supervise(stranded); err == nil || !strings.HasPrefix(err.Error(), "LAUNCH_ALREADY_SUPERVISED") || processes.command.Program != "" || processes.command.LogPath != "" {
		t.Fatalf("late supervisor err=%v command=%+v", err, processes.command)
	}
	launches = len(fixture.starter.ids)
	for _, request := range []ReadRequest{retry, fixture.request} {
		replay, err := fixture.runner.StartRead(request)
		if err != nil || replay.Attempt.Number != 2 || !replay.Complete || len(fixture.starter.ids) != launches {
			t.Fatalf("replay=%+v launches=%d err=%v", replay.Attempt, len(fixture.starter.ids), err)
		}
	}
	complete := retry
	complete.Retry = 2
	if _, err := fixture.runner.StartRead(complete); err == nil || !strings.HasPrefix(err.Error(), "READ_RETRY_COMPLETE") {
		t.Fatalf("retry of a complete attempt err=%v", err)
	}
	// Changed source is a genuinely new request; the old frozen bytes stay.
	frozenBrief, _ := os.ReadFile(first.Request.Brief)
	os.WriteFile(fixture.request.Brief, []byte("a different brief\n"), 0o600)
	fixture.git.diff = []byte(multiFilePackageDiff())
	changed, err := fixture.runner.StartRead(fixture.request)
	if err != nil || changed.Ref == first.Ref || changed.Attempt.Number != 1 {
		t.Fatalf("changed=%+v err=%v", changed.Attempt, err)
	}
	if data, _ := os.ReadFile(first.Request.Brief); string(data) != string(frozenBrief) {
		t.Fatalf("frozen brief moved: %q", data)
	}
	if data, _ := os.ReadFile(first.Request.Diff); string(data) != sampleDiff() {
		t.Fatalf("frozen diff moved: %q", data)
	}
	shown, err := fixture.runner.InspectRead(first.Ref)
	if err != nil || shown.Attempt.Number != 2 || !shown.Complete {
		t.Fatalf("show=%+v err=%v", shown.Attempt, err)
	}
	if _, err := fixture.runner.InspectRead("read-000000000000000000000000"); err == nil || !strings.HasPrefix(err.Error(), "READ_REF_UNKNOWN") {
		t.Fatalf("unknown ref err=%v", err)
	}
}

// TestStandaloneReadStopEndsTheSequence proves stop cancels the sequence's
// live read, starts no further partition, reports uncertain custody
// truthfully until the processes are proved dead, and repeats safely.
func TestStandaloneReadStopEndsTheSequence(t *testing.T) {
	t.Parallel()
	fixture := newStandaloneFixture(t, multiFilePackageDiff())
	fixture.manager.Settings.ReadSplitLines = 4
	fixture.starter.holdKind = "read"
	processes := fixture.manager.Processes.(*fakeProcesses)
	probe := fixture.manager.Prober.(*fakeProber)
	processes.ignoreKill = true
	started, err := fixture.runner.StartRead(fixture.request)
	if err != nil || !started.Capped || len(fixture.starter.ids) != 1 || started.Attempt.State != readAttemptRunning {
		t.Fatalf("start=%+v launches=%v err=%v", started, fixture.starter.ids, err)
	}
	stopped, err := fixture.runner.StopRead(started.Ref)
	if err != nil || !stopped.Stopping || !slices.Equal(stopped.Uncertain, fixture.starter.ids) {
		t.Fatalf("uncertain stop=%+v err=%v", stopped, err)
	}
	retry := fixture.request
	retry.Retry = 1
	if _, err := fixture.runner.StartRead(retry); err == nil || !strings.HasPrefix(err.Error(), "READ_RETRY_RUNNING") {
		t.Fatalf("retry before proof err=%v", err)
	}
	waited, err := fixture.runner.AdvanceRead(started.Ref, 0)
	if err != nil || waited.Attempt.State != readAttemptRunning || !waited.Stopping || len(fixture.starter.ids) != 1 {
		t.Fatalf("wait while unproven=%+v launches=%v err=%v", waited.Attempt, fixture.starter.ids, err)
	}
	processes.ignoreKill = false
	probe.states[10], probe.states[20] = identity.Dead, identity.Dead
	processes.group = false
	for range 2 {
		stopped, err = fixture.runner.StopRead(started.Ref)
		if err != nil || len(stopped.Uncertain) != 0 {
			t.Fatalf("proved stop=%+v err=%v", stopped, err)
		}
	}
	final, err := fixture.runner.AdvanceRead(started.Ref, 0)
	if err != nil || final.Attempt.State != readAttemptStopped || final.Complete || len(fixture.starter.ids) != 1 {
		t.Fatalf("final=%+v launches=%v err=%v", final.Attempt, fixture.starter.ids, err)
	}
	skipped := 0
	for _, step := range final.Attempt.Round.Steps[1:] {
		if step.State == StepSkipped && step.Reason == "stopped" && step.LaunchID == "" {
			skipped++
		}
	}
	if skipped != len(final.Attempt.Round.Steps)-1 || skipped == 0 {
		t.Fatalf("steps=%+v", final.Attempt.Round.Steps)
	}
	if _, statErr := os.Stat(final.Attempt.Context); !os.IsNotExist(statErr) || !final.Attempt.ContextRemoved {
		t.Fatalf("context retained after proved stop: %v", statErr)
	}
	// A start racing the stop sees the marker under the start lock.
	attempt := final.Attempt
	sequence := fixture.runner.standaloneSequence(final.Request, &attempt)
	if _, err := sequence.driver.start(StartSpec{}); !errors.Is(err, errReadStopped) {
		t.Fatalf("start after stop err=%v", err)
	}
	fixture.starter.holdKind = ""
	next, err := fixture.runner.StartRead(retry)
	if err != nil || next.Attempt.Number != 2 || !next.Complete {
		t.Fatalf("retry after proved stop=%+v err=%v", next.Attempt, err)
	}
}

// TestStandaloneReadReaderCannotTouchSource proves a reader writing its
// working directory writes only the disposable context, whose report is
// retained in launch custody and whose directory is removed afterwards.
func TestStandaloneReadReaderCannotTouchSource(t *testing.T) {
	t.Parallel()
	fixture := newStandaloneFixture(t, sampleDiff())
	fixture.git.checkoutFileNames = []string{"pkg/a/a.go"}
	var cwd string
	fixture.starter.onStart = func(record Record) error {
		cwd = record.WorkingDirectory
		if err := os.WriteFile(filepath.Join(record.WorkingDirectory, "tracked.go"), []byte("reader wrote\n"), 0o600); err != nil {
			return err
		}
		var outputs []string
		if err := json.Unmarshal(record.AdapterData["declaredOutputs"], &outputs); err != nil || len(outputs) != 1 {
			return errors.New("one declared report expected")
		}
		return os.WriteFile(outputs[0], []byte("findings\n"), 0o600)
	}
	before := directoryDigest(t, fixture.source)
	result, err := fixture.runner.StartRead(fixture.request)
	if err != nil || !result.Complete {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if after := directoryDigest(t, fixture.source); after != before {
		t.Fatalf("source changed:\n%s\n%s", before, after)
	}
	if cwd == "" || strings.HasPrefix(cwd, fixture.source+string(os.PathSeparator)) || cwd != result.Attempt.Checkout {
		t.Fatalf("reader cwd=%q checkout=%q", cwd, result.Attempt.Checkout)
	}
	if _, statErr := os.Stat(result.Attempt.Context); !os.IsNotExist(statErr) {
		t.Fatalf("context not removed: %v", statErr)
	}
	for _, report := range result.Reports {
		data, readErr := os.ReadFile(report.Path)
		if readErr != nil || string(data) != "findings\n" || !report.Retained || report.Bytes != int64(len(data)) {
			t.Fatalf("report=%+v data=%q err=%v", report, data, readErr)
		}
	}
	if fixture.git.count("apply") != 1 || fixture.git.count("add") != 1 {
		t.Fatalf("calls=%+v", fixture.git.calls)
	}
}

// TestStandaloneReadCheckoutGitAdapter is the narrow Git adapter check:
// real capture from a nested directory includes changes outside it and
// untracked files, the reader's checkout holds the captured tree without
// ignored files or a Git directory, a non-applying patch is a stated
// limitation, and the source's HEAD, index and files are unchanged even when
// the reader writes its working directory.
func TestStandaloneReadCheckoutGitAdapter(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, "app"), 0o700)
	os.MkdirAll(filepath.Join(repo, "metasystem"), 0o700)
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test")
	writeFile(t, filepath.Join(repo, ".gitignore"), "*.local\n")
	writeFile(t, filepath.Join(repo, "app", "main.go"), "old\n")
	writeFile(t, filepath.Join(repo, "metasystem", "tool.go"), "tool\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "base")
	writeFile(t, filepath.Join(repo, "app", "main.go"), "new\n")
	writeFile(t, filepath.Join(repo, "app", "fresh.go"), "fresh\n")
	writeFile(t, filepath.Join(repo, "metasystem", "metasystem.conf.local"), "secret\n")
	runGit(t, repo, "add", "app/main.go")
	head := runGit(t, repo, "rev-parse", "HEAD")
	indexPath := strings.TrimSpace(runGit(t, repo, "rev-parse", "--absolute-git-dir")) + "/index"
	indexBefore, _ := os.ReadFile(indexPath)
	sourceBefore := directoryDigest(t, repo)
	fixture := baseUnitFixture(t)
	fixture.runner.Git = OSGitRunner{}
	brief := filepath.Join(t.TempDir(), "brief.md")
	writeFile(t, brief, "read\n")
	var seen []string
	var gitDirectory bool
	fixture.starter.onStart = func(record Record) error {
		filepath.WalkDir(record.WorkingDirectory, func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				relative, _ := filepath.Rel(record.WorkingDirectory, path)
				data, _ := os.ReadFile(path)
				seen = append(seen, relative+"="+strings.TrimSpace(string(data)))
			}
			return nil
		})
		_, statErr := os.Stat(filepath.Join(record.WorkingDirectory, ".git"))
		gitDirectory = statErr == nil
		return os.WriteFile(filepath.Join(record.WorkingDirectory, "app", "main.go"), []byte("reader wrote\n"), 0o600)
	}
	result, err := fixture.runner.StartRead(ReadRequest{Directory: filepath.Join(repo, "metasystem"), Brief: brief})
	if err != nil || !result.Complete {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	slices.Sort(seen)
	if !slices.Equal(seen, []string{".gitignore=*.local", "app/fresh.go=fresh", "app/main.go=new", "metasystem/tool.go=tool"}) || gitDirectory {
		t.Fatalf("reader checkout=%v git directory=%t", seen, gitDirectory)
	}
	if !slices.Equal(result.Request.Files, []string{"app/fresh.go", "app/main.go"}) || result.Request.Base != strings.TrimSpace(head) {
		t.Fatalf("request=%+v", result.Request)
	}
	patch := filepath.Join(t.TempDir(), "other.diff")
	writeFile(t, patch, "diff --git a/app/main.go b/app/main.go\n--- a/app/main.go\n+++ b/app/main.go\n@@ -1 +1 @@\n-missing\n+other\n")
	seen = nil
	limited, err := fixture.runner.StartRead(ReadRequest{Directory: filepath.Join(repo, "metasystem"), Brief: brief, Patch: patch})
	if err != nil || limited.Attempt.Limitation == "" || !slices.Contains(seen, "app/main.go=old") {
		t.Fatalf("limited=%+v seen=%v err=%v", limited.Attempt, seen, err)
	}
	indexAfter, _ := os.ReadFile(indexPath)
	if runGit(t, repo, "rev-parse", "HEAD") != head || string(indexAfter) != string(indexBefore) || directoryDigest(t, repo) != sourceBefore {
		t.Fatal("source HEAD, index or files changed")
	}
}

func directoryDigest(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		if entry.IsDir() {
			if relative == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		data, _ := os.ReadFile(path)
		lines = append(lines, relative+"="+digestBytes(data))
		return nil
	})
	return strings.Join(lines, "\n")
}

// TestStandaloneReadExcludesDeclaredBrief proves the caller's own brief
// inside the checkout is left out of the captured changes by its exact path,
// while it is still frozen as the brief, and that a supplied patch is kept
// exactly as supplied even when it touches that path.
func TestStandaloneReadExcludesDeclaredBrief(t *testing.T) {
	t.Parallel()
	fixture := newStandaloneFixture(t, sampleDiff())
	brief := filepath.Join(fixture.source, "metasystem", "plans", "foo-brief.md")
	os.MkdirAll(filepath.Dir(brief), 0o700)
	os.WriteFile(brief, []byte("in-repo brief\n"), 0o600)
	fixture.request.Brief = brief
	first, err := fixture.runner.StartRead(fixture.request)
	if err != nil || !first.Complete {
		t.Fatalf("first=%+v err=%v", first.Attempt, err)
	}
	if !slices.Equal(fixture.git.excluded, []string{":(exclude,literal)metasystem/plans/foo-brief.md"}) {
		t.Fatalf("excluded=%q", fixture.git.excluded)
	}
	if data, _ := os.ReadFile(first.Request.Brief); string(data) != "in-repo brief\n" || first.Request.BriefSHA256 != digestBytes(data) {
		t.Fatalf("frozen brief=%q", data)
	}
	launches := len(fixture.starter.ids)
	if again, err := fixture.runner.StartRead(fixture.request); err != nil || again.Ref != first.Ref || len(fixture.starter.ids) != launches {
		t.Fatalf("repeat=%+v err=%v", again.Attempt, err)
	}
	patch := filepath.Join(t.TempDir(), "supplied.diff")
	supplied := sampleDiff() + "diff --git a/metasystem/plans/foo-brief.md b/metasystem/plans/foo-brief.md\n--- a/metasystem/plans/foo-brief.md\n+++ b/metasystem/plans/foo-brief.md\n-old\n+new\n"
	os.WriteFile(patch, []byte(supplied), 0o600)
	fixture.request.Patch = patch
	calls := fixture.git.count("diff")
	patched, err := fixture.runner.StartRead(fixture.request)
	if err != nil || fixture.git.count("diff") != calls || !slices.Contains(patched.Request.Files, "metasystem/plans/foo-brief.md") {
		t.Fatalf("patched=%+v diff calls=%d err=%v", patched.Request, fixture.git.count("diff"), err)
	}
	if data, _ := os.ReadFile(patched.Request.Diff); string(data) != supplied {
		t.Fatalf("supplied patch changed: %q", data)
	}
}

// TestStandaloneReadDeclaredBriefGitAdapter is the narrow Git adapter check
// for brief exclusion: real capture from a nested directory leaves out the
// declared in-repo brief, keeps the other tracked and untracked changes,
// changes nothing in the source, and a brief-only change starts no reader.
func TestStandaloneReadDeclaredBriefGitAdapter(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	for _, directory := range []string{"app", "metasystem/plans"} {
		os.MkdirAll(filepath.Join(repo, directory), 0o700)
	}
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test")
	writeFile(t, filepath.Join(repo, "app", "main.go"), "old\n")
	writeFile(t, filepath.Join(repo, "metasystem", "plans", "tracked-brief.md"), "old brief\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "base")
	writeFile(t, filepath.Join(repo, "app", "main.go"), "new\n")
	writeFile(t, filepath.Join(repo, "app", "fresh.go"), "fresh\n")
	brief := filepath.Join(repo, "metasystem", "plans", "foo-brief.md")
	writeFile(t, brief, "read this\n")
	head := runGit(t, repo, "rev-parse", "HEAD")
	indexPath := strings.TrimSpace(runGit(t, repo, "rev-parse", "--absolute-git-dir")) + "/index"
	indexBefore, _ := os.ReadFile(indexPath)
	sourceBefore := directoryDigest(t, repo)
	fixture := baseUnitFixture(t)
	fixture.runner.Git = OSGitRunner{}
	nested := filepath.Join(repo, "metasystem")
	result, err := fixture.runner.StartRead(ReadRequest{Directory: nested, Brief: brief})
	if err != nil || !result.Complete {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	diff, _ := os.ReadFile(result.Request.Diff)
	if !slices.Equal(result.Request.Files, []string{"app/fresh.go", "app/main.go"}) || strings.Contains(string(diff), "foo-brief") {
		t.Fatalf("files=%v diff=%s", result.Request.Files, diff)
	}
	if data, _ := os.ReadFile(result.Request.Brief); string(data) != "read this\n" {
		t.Fatalf("frozen brief=%q", data)
	}
	launches := len(fixture.starter.ids)
	if again, err := fixture.runner.StartRead(ReadRequest{Directory: nested, Brief: brief}); err != nil || again.Ref != result.Ref || len(fixture.starter.ids) != launches {
		t.Fatalf("repeat=%+v err=%v", again.Attempt, err)
	}
	indexAfter, _ := os.ReadFile(indexPath)
	if runGit(t, repo, "rev-parse", "HEAD") != head || string(indexAfter) != string(indexBefore) || directoryDigest(t, repo) != sourceBefore {
		t.Fatal("source HEAD, index or files changed")
	}
	// Only the tracked brief changed: nothing is left to read.
	writeFile(t, filepath.Join(repo, "app", "main.go"), "old\n")
	os.Remove(filepath.Join(repo, "app", "fresh.go"))
	os.Remove(brief)
	tracked := filepath.Join(repo, "metasystem", "plans", "tracked-brief.md")
	writeFile(t, tracked, "new brief\n")
	empty, err := fixture.runner.StartRead(ReadRequest{Directory: nested, Brief: tracked})
	if err != nil || !empty.Empty || len(fixture.starter.ids) != launches {
		t.Fatalf("brief-only=%+v launches=%d err=%v", empty, len(fixture.starter.ids), err)
	}
}

// TestWorktreeDiffRacyCleanIndexGitAdapter is the narrow Git adapter check
// that the private index copy keeps the source index's time. Git populates
// the entry's cached stat fields from the edited file; the fixture then
// points that entry back at the committed blob and dates the index to the
// file's own time, so the entry is racily clean: its stat data matches an
// edited file, and only the index time tells Git to read the contents. The
// production capture must still see the same-size edit, and must change
// nothing in the source.
func TestWorktreeDiffRacyCleanIndexGitAdapter(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	runGit(t, repo, "init", "--object-format=sha1")
	for _, setting := range [][]string{{"user.email", "test@example.invalid"}, {"user.name", "Test"}, {"index.skipHash", "false"}, {"feature.manyFiles", "false"}, {"index.version", "2"}} {
		runGit(t, repo, append([]string{"config"}, setting...)...)
	}
	file := filepath.Join(repo, "main.go")
	writeFile(t, file, "old\n")
	runGit(t, repo, "add", "main.go")
	runGit(t, repo, "commit", "-m", "base")
	oldBlob := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD:main.go"))
	writeFile(t, file, "new\n")
	past := time.Now().Add(-10 * time.Second).Truncate(time.Second)
	if err := os.Chtimes(file, past, past); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "main.go")
	runGit(t, repo, "update-index", "--index-version", "2")
	newBlob := strings.TrimSpace(runGit(t, repo, "rev-parse", ":main.go"))
	indexPath := filepath.Join(strings.TrimSpace(runGit(t, repo, "rev-parse", "--absolute-git-dir")), "index")
	index, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	// Index version 2, one entry: 40 bytes of stat data, the 20-byte blob
	// id, 2 bytes of flags, then the path; a SHA-1 of everything before it
	// ends the file.
	if string(index[:4]) != "DIRC" || binary.BigEndian.Uint32(index[4:8]) != 2 || binary.BigEndian.Uint32(index[8:12]) != 1 || !strings.HasPrefix(string(index[74:]), "main.go\x00") {
		t.Fatalf("unexpected index layout: %x", index[:min(len(index), 80)])
	}
	if fmt.Sprintf("%x", index[52:72]) != newBlob {
		t.Fatalf("entry blob=%x want %s", index[52:72], newBlob)
	}
	oldID, err := hex.DecodeString(oldBlob)
	if err != nil || len(oldID) != 20 {
		t.Fatalf("old blob=%q err=%v", oldBlob, err)
	}
	copy(index[52:72], oldID)
	sum := sha1.Sum(index[:len(index)-20])
	copy(index[len(index)-20:], sum[:])
	if err := os.WriteFile(indexPath, index, 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(indexPath, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	head := runGit(t, repo, "rev-parse", "HEAD")
	indexInfo, _ := os.Stat(indexPath)
	runner := UnitRunner{Git: OSGitRunner{}}
	diff, err := runner.WorktreeDiff(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(diff), "-old\n+new\n") {
		t.Fatalf("racily clean edit lost from capture: %q", diff)
	}
	after, _ := os.ReadFile(indexPath)
	afterInfo, _ := os.Stat(indexPath)
	data, _ := os.ReadFile(file)
	if runGit(t, repo, "rev-parse", "HEAD") != head || string(after) != string(index) || !afterInfo.ModTime().Equal(indexInfo.ModTime()) || string(data) != "new\n" {
		t.Fatal("source HEAD, index or file changed")
	}
}
