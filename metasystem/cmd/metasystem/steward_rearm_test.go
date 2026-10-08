package main

import (
	"bytes"
	"context"
	"errors"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestStewardRunPublicVerbRefreshRefusalStillRevivesAndDelivers(t *testing.T) {
	t.Parallel()
	report := os.Getenv("HOUSEKEEPING_REARM_REPORT")
	if report == "" {
		runIsolatedRearm(t, nil)
		return
	}
	registryPath, err := registry.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	endedStore, err := diskstore.CreateTempStore(diskstore.UnitReadFindingsName("ended-rearm"), unitReadFindingsClass,
		diskstore.Owner{Kind: diskstore.OwnerUnit, Ref: "ended-rearm"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(endedStore, "payload"), []byte("ended"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(report, "home"), []byte(filepath.Dir(registryPath)), 0o600); err != nil {
		t.Fatal(err)
	}
	ready := os.NewFile(3, "rearm-ready")
	if _, err := ready.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := ready.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := steward.QueueNotification(root, steward.PendingNotification{Nonce: "pending", Message: "pending before refresh"}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
	clock := &steward.HandoffClock{Now: func() time.Time { return now }, Sleep: func(d time.Duration) { now = now.Add(d) }}
	refreshes := 0
	refresh := func() (bool, error) {
		refreshes++
		if err := steward.MintIntent(root, steward.Intent{Nonce: "prepared", RepoIdentity: "fixture", InstallGen: 1, Goal: "fix-it", Runtime: "fake", Model: "fixture"}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "steward", "stop"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		return false, errors.New("this checkout's engine is behind the landing branch, and its source has local edits")
	}
	registered := families()
	for i := range registered {
		if registered[i].name == "steward" {
			for j := range registered[i].verbs {
				if registered[i].verbs[j].name == "run" {
					registered[i].verbs[j].run = func(args []string, stdout, stderr io.Writer) int {
						return runStewardRunWithDependencies(args, stdout, stderr, steward.RunLoop, probeStewardRuntime, clock, func(string) int { return 1 }, refresh)
					}
				}
			}
		}
	}
	var stdout, stderr bytes.Buffer
	code := dispatchWithFamiliesAndRepositoryTop([]string{"steward", "run", "--repo", root}, &stdout, &stderr, registered, func(string) (string, error) {
		t.Fatal("runner verb called Git")
		return "", nil
	})
	log, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "steward", "notifications.log"))
	if code != 0 || refreshes != 1 || err != nil || !strings.Contains(string(log), "pending before refresh") {
		t.Fatalf("exit=%d refreshes=%d deliveries=%q error=%v stderr=%s", code, refreshes, log, err, stderr.String())
	}
	ended, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "steward", "cancelled", "prepared.json"))
	if err != nil || !strings.Contains(string(ended), "cancelled:") {
		t.Fatalf("revival did not re-arbitrate the prepared intent: %q %v", ended, err)
	}
	if pending, err := steward.PendingNotifications(root); err != nil || len(pending) != 0 {
		t.Fatalf("runner left notifications pending: %+v %v", pending, err)
	}
	if _, err := os.Stat(endedStore); !os.IsNotExist(err) {
		t.Fatalf("apply-mode runner left its ended findings store: %v", err)
	}
	rows, bad := diskstore.MachineRegistry(filepath.Dir(registryPath)).Inventory()
	released := false
	for _, row := range rows {
		if row.Path == endedStore && row.State == diskstore.StateReleased {
			released = true
		}
	}
	if !released || len(bad) != 0 {
		t.Fatalf("ended store release record missing: released=%t unreadable=%v", released, bad)
	}

}

// runIsolatedRearm keeps the actual runner in a TestMain-owned child home.
// The readiness pipe holds the child before its apply pass until live work
// in the parent has established its findings and named owner record.
func runIsolatedRearm(t *testing.T, overlap func(*testutil.HeldProcess)) {
	t.Helper()
	report := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestStewardRunPublicVerbRefreshRefusalStillRevivesAndDelivers$", "-test.count=1", "-test.timeout=30m")
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if name != "METASYSTEM_SUPERVISION_REGISTRY_HOME" && name != "METASYSTEM_TESTENV_REGISTRY_NONCE" {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "HOUSEKEEPING_REARM_REPORT="+report)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	pid := 0
	// The child runs a steward pass; its process group is reaped like every fixture engine child.
	testenv.ReapFixtureProcessGroups(t, []testenv.FixtureProcessGroup{{Verb: "isolated steward rearm", Resolve: func() (int, bool, error) {
		return pid, pid != 0, nil
	}}})
	held := testutil.StartHeldProcess(t, command)
	pid = command.Process.Pid
	if overlap != nil {
		overlap(held)
	}
	if err := held.Release(); err != nil {
		t.Fatalf("rearm child: %v\n%s", err, output.String())
	}
	childHome, err := os.ReadFile(filepath.Join(report, "home"))
	parentRegistry, parentErr := registry.DefaultPath()
	if err != nil || parentErr != nil || string(childHome) == filepath.Dir(parentRegistry) {
		t.Fatalf("rearm registry isolation: child=%q parent=%q errors=%v %v", childHome, parentRegistry, err, parentErr)
	}
	t.Logf("apply sweep ran in %s while parent stores belong to %s", childHome, filepath.Dir(parentRegistry))
}

func TestHousekeepingFixtureOwnership(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	bed.starter.hold = "read"
	brief := bed.brief("ownership.md", "Read each round: yes\nBuild the unit.\n")
	args := append([]string{"work", "build", bed.id, "owned", "--brief", brief, "--lines", "5"}, workCheck...)
	code, result, _ := bed.work(args...)
	if code != 3 || result.Outcome != intentInProgress {
		t.Fatalf("held read: exit=%d %+v", code, result)
	}
	plan, err := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
	if err != nil || len(plan.Read.Outputs) != 1 {
		t.Fatalf("held read plan: %+v %v", plan, err)
	}
	findings := plan.Read.Outputs[0]
	ready, release, joined := make(chan struct{}), make(chan struct{}), make(chan struct{})
	sleep := bed.manager.Sleep
	var once sync.Once
	bed.manager.Sleep = func(d time.Duration) { once.Do(func() { close(ready); <-release }); sleep(d) }
	var continued intentResult
	var continuedCode int
	go func() { defer close(joined); continuedCode, continued, _ = bed.work(args...) }()
	steps := resultData(t, result)["steps"].([]any)
	readID := steps[len(steps)-1].(map[string]any)["launchId"].(string)
	var unhold sync.Once
	releaseWork := func() {
		unhold.Do(func() {
			_, err := bed.manager.Store.Update(readID, func(record *launch.Record) error {
				code, yes := 0, true
				record.State, record.ExitCode, record.VerdictCounts, record.Measurement.Verdict = launch.Completed, &code, &yes, "pass"
				record.FinishedAt = bed.manager.Now().UTC().Format(time.RFC3339Nano)
				dead := workProcessRef(99)
				record.Supervisor, record.Child, record.ProcessGroup = &dead, &dead, &dead
				return nil
			})
			if err != nil {
				t.Errorf("complete owned read before joining work: %v", err)
			}
			close(release)
		})
		<-joined
	}
	t.Cleanup(releaseWork)
	<-ready
	if err := os.WriteFile(findings, []byte("retained findings"), 0o600); err != nil {
		t.Fatal(err)
	}
	runIsolatedRearm(t, func(child *testutil.HeldProcess) {
		if data, err := os.ReadFile(findings); err != nil || string(data) != "retained findings" {
			t.Fatalf("live findings before sweep: %q %v", data, err)
		}
		if err := child.Release(); err != nil {
			t.Fatalf("apply sweep child: %v", err)
		}
		if data, err := os.ReadFile(findings); err != nil || string(data) != "retained findings" {
			t.Fatalf("apply sweep removed barrier-held live findings: %q %v", data, err)
		}
		path, err := registry.DefaultPath()
		if err != nil {
			t.Fatal(err)
		}
		rows, bad := diskstore.MachineRegistry(filepath.Dir(path)).Inventory()
		accepted := false
		for _, row := range rows {
			if row.Path == filepath.Dir(findings) && row.State == diskstore.StateAccepted {
				accepted = true
				proof := diskstore.UnitFindingsProof{UnitRoot: bed.unitRoot}
				if verdict := proof.Observe(context.Background(), row); verdict.Decision != diskstore.Keep {
					t.Fatalf("live work lost its named owner: %+v", verdict)
				}
			}
		}
		if !accepted || len(bad) != 0 {
			t.Fatalf("live findings lost registration: accepted=%t unreadable=%v", accepted, bad)
		}
	})
	releaseWork()
	if continuedCode != 0 || continued.Outcome != intentConfirmed {
		t.Fatalf("joined work: exit=%d %+v", continuedCode, continued)
	}
	// The owner releases the store while the unit records are still readable.
	bed.removeReadDirs()
	if _, err := os.Stat(filepath.Dir(findings)); !os.IsNotExist(err) {
		t.Fatalf("owned findings cleanup: %v", err)
	}
	bed.readDirsMu.Lock()
	clear(bed.readDirs)
	bed.readDirsMu.Unlock()
}

func TestStewardStatusPublicVerbShowsDeferredRearm(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatal(stderr)
	}
	if err := steward.NoteDeferredRearm(bed.landingA, "main", "old", laneTestNow); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := bed.run(t, "landing", "status")
	if code != 0 || !strings.Contains(stdout, "engine main, checkout old, re-arm deferred") {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}
