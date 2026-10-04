package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func captureMissionStderr(t *testing.T, run func(stdout, stderr io.Writer) int) (string, int) {
	t.Helper()
	return captureStderr(t, run)
}

func missionFenceFixture(t *testing.T, terminal bool) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(root, "bin", "metasystem"), []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := canonicalPath(root)
	if err != nil {
		t.Fatal(err)
	}
	table := filepath.Join(t.TempDir(), "identities.json")
	body := fmt.Sprintf(`{"%d":{"terminal":%t}}`, os.Getpid(), terminal)
	if err := os.WriteFile(table, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	setOwnedGoGateProcessEnvironment(t, "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)
	return root
}

func TestMissionLaunchHumanOpensClosedFenceBeforeArming(t *testing.T) {
	for _, mode := range []string{"start", "resume"} {
		t.Run(mode, func(t *testing.T) {
			root := missionFenceFixture(t, true)
			repositoryTop := declaredRepositoryTop(t, root, map[string]int{root: 1})
			record := stopfence.Record{
				State: stopfence.StateClosed, Phase: stopfence.PhaseStopped,
				Generation: 9, ChangedAt: "2026-09-07T09:30:00Z", Checkout: root,
				NotStopped: []stopfence.Survivor{},
				By:         stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 4321}},
			}
			if err := stopfence.Write(root, record); err != nil {
				t.Fatal(err)
			}
			generation, code := missionFenceBeforeArmWith(t.Output(), root, mode, repositoryTop, lease.ClassifyAt)
			if code != 0 || generation != 10 {
				t.Fatalf("human handover = generation %d code %d", generation, code)
			}
			opened, err := stopfence.Read(root)
			if err != nil || opened.State != stopfence.StateOpen || opened.Phase != stopfence.PhaseArmed || opened.By.Verb != "mission-"+mode {
				t.Fatalf("opened fence = %#v err=%v", opened, err)
			}
		})
	}
}

func TestMissionLaunchNonHumanKeepsStoppedRefusalAheadOfArming(t *testing.T) {
	root := missionFenceFixture(t, false)
	repositoryTop := declaredRepositoryTop(t, root, map[string]int{root: 1})
	if err := stopfence.Write(root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped,
		Generation: 9, ChangedAt: "2026-09-07T09:30:00Z", Checkout: root,
		NotStopped: []stopfence.Survivor{},
		By:         stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 4321}},
	}); err != nil {
		t.Fatal(err)
	}
	output, code := captureMissionStderr(t, func(stdout, stderr io.Writer) int {
		_, code := missionFenceBeforeArmWith(stderr, root, "resume", repositoryTop, lease.ClassifyAt)
		return code
	})
	expected := "the metasystem is stopped for " + root + " since 2026-09-07T09:30:00Z, by stop pid 4321\n" +
		"at an agent-free terminal, run: metasystem mission resume --root " + root + " --mission <id>\n"
	if code == 0 || output != expected {
		t.Fatalf("non-human stopped refusal = code %d output %q, want %q", code, output, expected)
	}
	record, err := stopfence.Read(root)
	if err != nil || record.State != stopfence.StateClosed {
		t.Fatalf("non-human changed fence = %#v err=%v", record, err)
	}
}

func TestMissionLaunchClassificationDataFailureNamesRepairBeforeRetry(t *testing.T) {
	root := missionFenceFixture(t, true)
	repositoryTop := declaredRepositoryTop(t, root, map[string]int{root: 1})
	if err := stopfence.Write(root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped,
		Generation: 9, ChangedAt: "2026-09-07T09:30:00Z", Checkout: root,
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 4321}},
	}); err != nil {
		t.Fatal(err)
	}
	jobPath := filepath.Join(root, "artifacts", "agents", "jobs", "damaged.json")
	if err := os.MkdirAll(filepath.Dir(jobPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jobPath, []byte("{\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	printedJobPath, err := canonicalPath(jobPath)
	if err != nil {
		t.Fatal(err)
	}
	fencePath := stopfence.TransitionPath(root)
	before, err := os.ReadFile(fencePath)
	if err != nil {
		t.Fatal(err)
	}
	output, code := captureMissionStderr(t, func(stdout, stderr io.Writer) int {
		_, code := missionFenceBeforeArmWith(stderr, root, "resume", repositoryTop, lease.ClassifyAt)
		return code
	})
	want := "metasystem mission resume: who started this shell can't be told: job record " + printedJobPath + " is damaged (invalid JSON: unexpected end of JSON input).\n" +
		"repair " + printedJobPath + ", then in a terminal you opened yourself, run: metasystem mission resume --root " + root + " --mission <id>\n"
	if code != 1 || output != want {
		t.Fatalf("mission classification refusal = code %d output %q, want %q", code, output, want)
	}
	after, err := os.ReadFile(fencePath)
	if err != nil || string(after) != string(before) {
		t.Fatalf("mission classification refusal changed the fence: %v", err)
	}
}

// The checkout is the state root and its nested installation keeps the stop
// fence: a mission launch reads the fence a stop closed there and refuses.
func TestMissionFenceSeparatedRootsClassifiesAtTheNestedInstallationAndKeepsItsFenceClosed(t *testing.T) {
	root := t.TempDir()
	root, err := canonicalPath(root)
	if err != nil {
		t.Fatal(err)
	}
	installation := filepath.Join(root, "metasystem")
	repositoryTop := declaredRepositoryTop(t, root, map[string]int{root: 1, installation: 1})
	if err := os.MkdirAll(filepath.Join(installation, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(installation, "bin", "metasystem"), []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The nested installation is fixture-mode (metasystem.runtimes=fake):
	// its classifier, not the checkout's, must recognize the fake runtime's
	// agent as the caller.
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	caller := agentChildPid(t)
	table := filepath.Join(t.TempDir(), "identities.json")
	if err := os.WriteFile(table, []byte(fmt.Sprintf(`{"%d":{"terminal":false}}`, caller)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)
	if err := stopfence.Write(installation, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 4,
		ChangedAt: "2026-09-08T04:00:00Z", Checkout: root,
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 72}},
	}); err != nil {
		t.Fatal(err)
	}
	var gotRoot, gotInstallation string
	classify := func(checkout, installed string, _ int64) (lease.Classification, error) {
		gotRoot, gotInstallation = checkout, installed
		return lease.ClassifyAt(checkout, installed, caller)
	}
	_, code := missionFenceBeforeArmWith(t.Output(), root, "start", repositoryTop, classify)
	if code != 1 || gotRoot != root || gotInstallation != installation {
		t.Fatalf("mission classifier root=%q installation=%q code=%d, want %q and %q", gotRoot, gotInstallation, code, root, installation)
	}
	classification, err := lease.ClassifyAt(root, installation, caller)
	if err != nil || classification.Class != lease.ClassDelegate {
		t.Fatalf("nested installation did not classify the mission caller as an agent: %+v, %v", classification, err)
	}
	record, err := stopfence.Read(installation)
	if err != nil || record.State != stopfence.StateClosed || record.Generation != 4 {
		t.Fatalf("agent-classified mission changed the fence: %#v, %v", record, err)
	}
	if _, err := os.Stat(stopfence.TransitionPath(root)); !os.IsNotExist(err) {
		t.Fatalf("a fence appeared under the state root: %v", err)
	}
}

// A stop closed the installation's fence and its engine binary is gone: the
// state root holds no fence, so neither the launch's fence check nor the
// engine may read one there. Both refuse; nothing is classified or written.
func TestMissionLaunchSeparatedRootsRefusesWhenTheInstallationHasNoEngine(t *testing.T) {
	root := t.TempDir()
	installation := filepath.Join(root, "metasystem")
	installationShapeAt(t, installation)
	if err := os.Remove(filepath.Join(installation, "bin", "metasystem")); err != nil {
		t.Fatal(err)
	}
	if err := stopfence.Write(installation, stopfence.Record{State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 6,
		ChangedAt: "2026-10-03T00:00:00Z", Checkout: root, By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 73}}}); err != nil {
		t.Fatal(err)
	}
	output, code := captureMissionStderr(t, func(_, stderr io.Writer) int {
		_, code := missionFenceBeforeArmWith(stderr, root, "start", declaredRepositoryTop(t, root, map[string]int{root: 1}), nil)
		return code
	})
	if code != 1 || !strings.HasPrefix(output, "mission start: "+noEngineRefusal+"\n") {
		t.Fatalf("launch without an engine = code %d %q, want the refusal", code, output)
	}
	if engine, err := missionRunnerCommandEngineWith(root, "m-1", stateroot.RepositoryTop); engine != nil || err == nil || !strings.HasPrefix(err.Error(), noEngineRefusal) {
		t.Fatalf("mission engine without an installation = %v, %v", engine, err)
	}
	if record, err := stopfence.Read(installation); err != nil || record.State != stopfence.StateClosed || record.Generation != 6 {
		t.Fatalf("the installation's fence changed: %+v %v", record, err)
	}
	if _, err := os.Stat(filepath.Join(root, "artifacts")); !os.IsNotExist(err) {
		t.Fatalf("a refused launch wrote under the state root: %v", err)
	}
}

// missionFenceBeforeArmWith is the fence check as the launching command runs
// it: this process is the caller, refusals go to this process's stderr.
func missionFenceBeforeArmWith(stderr io.Writer, root, mode string, repositoryTop func(string) (string, error), classify processCallerClassifier) (int64, int) {
	return missionFenceBeforeArmFor(ownercall.Process{Pid: int64(os.Getpid())}, stderr, root, mode, repositoryTop, classify)
}
