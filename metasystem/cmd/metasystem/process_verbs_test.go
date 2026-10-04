package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stoptransition"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
)

func separateProcessScopeFixture(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	var err error
	repo, err = canonicalPath(repo)
	if err != nil {
		t.Fatal(err)
	}
	// The named installation is a template installation outside the
	// checkout's default places, so it is its own state root and its process
	// records stay beside its engine.
	installation := filepath.Join(repo, "separate-engine", "metasystem")
	if err := os.MkdirAll(filepath.Join(installation, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(installation, "bin", "metasystem"), []byte("fixture engine\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.template=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo, installation
}

func declaredRepositoryTop(t *testing.T, top string, expected map[string]int) func(string) (string, error) {
	t.Helper()
	called := make(map[string]int)
	t.Cleanup(func() {
		for input, want := range expected {
			if called[input] != want {
				t.Errorf("repository top lookup for %q: got %d calls, want %d", input, called[input], want)
			}
		}
	})
	return func(input string) (string, error) {
		t.Helper()
		want, ok := expected[input]
		if !ok || called[input] >= want {
			t.Fatalf("unexpected repository top lookup for %q (call %d)", input, called[input]+1)
		}
		called[input]++
		return top, nil
	}
}

func runnableSeparateProcessScopeFixture(t *testing.T) (string, string) {
	t.Helper()
	repo, installation := separateProcessScopeFixture(t)
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.template=true\nmetasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := filepath.Join(installation, "scripts", "agents", "adapters", "fake.sh")
	if err := os.MkdirAll(filepath.Dir(adapter), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(adapter, []byte("#!/bin/sh\nprintf 'match fake-runtime-never-present\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	processes := filepath.Join(t.TempDir(), "processes.json")
	if err := os.WriteFile(processes, []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", processes)
	return repo, installation
}

func TestProcessVerbsShareTheNamedSeparateInstallation(t *testing.T) {
	repo, installation := runnableSeparateProcessScopeFixture(t)
	repositoryTop := declaredRepositoryTop(t, repo, map[string]int{repo: 3, installation: 2})
	var classifiedInstallations []string
	classify := func(_ string, gotInstallation string, _ int64) (lease.Classification, error) {
		classifiedInstallations = append(classifiedInstallations, gotInstallation)
		return lease.Classification{Class: lease.ClassDelegate}, nil
	}

	stdout, stderr, code := captureRelay(t, func(stdout, stderr io.Writer) int {
		return runProcessStatusWith([]string{"--repo", repo, "--installation", installation}, repositoryTop, stdout, stderr)
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "checkout "+repo+"\nnothing is running") || strings.Contains(stdout, "--metasystem-root") {
		t.Fatalf("separate-installation status = code %d stdout %q stderr %q", code, stdout, stderr)
	}

	for _, verb := range []struct {
		name string
		run  command
	}{
		{name: "stop", run: func(args []string, stdout, stderr io.Writer) int {
			return runProcessStopWith(args, repositoryTop, classify, stdout, stderr)
		}},
		{name: "arm", run: func(args []string, stdout, stderr io.Writer) int {
			return runProcessArmWith(args, repositoryTop, classify, stdout, stderr)
		}},
	} {
		t.Run(verb.name, func(t *testing.T) {
			stdout, stderr, code := captureRelay(t, func(stdout, stderr io.Writer) int {
				return verb.run([]string{"--repo", repo, "--installation", installation}, stdout, stderr)
			})
			public := publicProcessVerb(verb.name)
			want := "metasystem " + public + ": an agent started this shell.\n" +
				"in a terminal you opened yourself, run: metasystem " + public + " --repo " + repo + " --installation " + installation + "\n"
			if code != 1 || stdout != "" || stderr != want || strings.Contains(stderr, "--metasystem-root") {
				t.Fatalf("separate-installation %s = code %d stdout %q stderr %q, want stderr %q", verb.name, code, stdout, stderr, want)
			}
		})
	}
	if len(classifiedInstallations) != 2 || classifiedInstallations[0] != installation || classifiedInstallations[1] != installation {
		t.Fatalf("caller classification installations = %q, want the named installation twice", classifiedInstallations)
	}
}

func TestArmAcceptsASeparateInstallationScope(t *testing.T) {
	repo, installation := separateProcessScopeFixture(t)
	repositoryTop := declaredRepositoryTop(t, repo, map[string]int{repo: 1})
	scope, scale, code := parseProcessScopeWith(t.Output(), "arm", []string{"--repo", repo, "--installation", installation}, repositoryTop)
	if code != 0 || scope.Checkout != repo || scope.Installation.Path() != installation || scope.Root.Path() != installation || scale < 1 {
		t.Fatalf("arm scope = %#v scale=%d code=%d", scope, scale, code)
	}
	if err := stopfence.Write(scope.Installation.Path(), stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 8,
		ChangedAt: "2026-09-07T12:00:00Z", Checkout: scope.Checkout,
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 71, PidStartedAt: 70}},
	}); err != nil {
		t.Fatal(err)
	}
	transition := processTransition(scope, 1)
	transition.Self = func() (identity.Ref, error) { return identity.Ref{Pid: 72, StartedAtSec: 70}, nil }
	report, err := transition.Arm()
	if err != nil || report.ExitCode != 0 || !strings.Contains(strings.Join(report.Lines, "\n"), "armed "+repo+" generation 9") {
		t.Fatalf("separate-installation arm = %#v err=%v", report, err)
	}
}

// TestProcessScopeSeparatedRootsKeepsRunStateInTheInstallation finds a
// checkout's installation at <checkout>/metasystem that is not the template,
// so its state root is the checkout. The stop fence and the records a stop
// inventories are run state: the steward's arm checks the fence under the
// installation's artifacts/, the stop transition opens it there, the status
// reads it there, the stop finds jobs there, and the state root gains nothing.
func TestProcessScopeSeparatedRootsKeepsRunStateInTheInstallation(t *testing.T) {
	repo, err := canonicalPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	installation := filepath.Join(repo, "metasystem")
	if err := os.MkdirAll(filepath.Join(installation, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(installation, "bin", "metasystem"), []byte("fixture engine\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	repositoryTop := declaredRepositoryTop(t, repo, map[string]int{repo: 1, installation: 1})

	scope, err := resolveProcessScopeWith(repo, "", repositoryTop)
	if err != nil || scope.Checkout != repo || scope.Installation.Path() != installation || scope.Root.Path() != repo ||
		scope.Binary != filepath.Join(installation, "bin", "metasystem") {
		t.Fatalf("scope = %#v err=%v, want the installation %s and the state root %s", scope, err, installation, repo)
	}

	if err := stopfence.Write(installation, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 9,
		ChangedAt: "2026-10-03T09:00:00Z", Checkout: repo, NotStopped: []stopfence.Survivor{},
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 4321}},
	}); err != nil {
		t.Fatal(err)
	}
	var seeded string
	effects := defaultProcessArmEffects()
	effects.seed = func(root string) (stewardLandingRefSeed, error) { seeded = root; return stewardLandingRefSeed{}, nil }
	effects.up = func(up.Options) up.Result { t.Fatal("supervision started under a closed fence"); return up.Result{} }
	_, armErr := effects.steps(scope, 1, processArmAuthority{fixtureGranted: true})
	var stopped *steward.StoppedError
	if !errors.As(armErr, &stopped) || stopped.Checkout != installation || seeded != repo {
		t.Fatalf("arm = %v seeded at %q, want the installation's closed fence to refuse the steward", armErr, seeded)
	}
	generation, err := processTransition(scope, 1).OpenFence("arm")
	if err != nil || generation != 10 {
		t.Fatalf("opening the fence = generation %d err=%v, want 10", generation, err)
	}
	if record, fence := processFence(scope); record.Generation != 10 || fence != stopfence.StateOpen+"/"+stopfence.PhaseArmed {
		t.Fatalf("the status read the fence %q at generation %d, want the installation's open fence at 10", fence, record.Generation)
	}
	if _, err := os.Stat(stopfence.TransitionPath(installation)); err != nil {
		t.Fatalf("no fence under the installation's artifacts/: %v", err)
	}
	entries, err := os.ReadDir(repo)
	if err != nil || len(entries) != 1 || entries[0].Name() != "metasystem" {
		t.Fatalf("the state root holds %v (err %v); run state belongs to the installation", entries, err)
	}
	for root, id := range map[string]string{installation: "live", repo: "decoy"} {
		path := filepath.Join(root, "artifacts", "agents", "jobs", id+".json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"jobId":"`+id+`","status":"running"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, family := range processTransition(scope, 1).Families {
		if family.Name() != "job" {
			continue
		}
		items, err := family.Inventory()
		if err != nil || len(items) != 1 || items[0].Key != "job:live" {
			t.Fatalf("the stop's job inventory = %+v err=%v, want the installation's job only", items, err)
		}
		if outcome, err := family.Stop(items[0]); err != nil || !strings.Contains(outcome.Line, "job live") {
			t.Fatalf("the stop handled %+v err=%v, want the installation's job", outcome, err)
		}
	}
}

// TestProcessScopeClassifierRefusesAStateRootAsTheInstallation classifies a
// caller in a checkout whose installation carries no engine binary. The
// checkout is a state root only, so the classifier refuses rather than read
// runtime adapters there; once the checkout holds metasystem.conf, as a
// self-hosted installation does, it classifies against the checkout.
func TestProcessScopeClassifierRefusesAStateRootAsTheInstallation(t *testing.T) {
	repo, err := canonicalPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	installation := filepath.Join(repo, "engine")
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	table := filepath.Join(t.TempDir(), "identities.json")
	if err := os.WriteFile(table, []byte(fmt.Sprintf(`{"%d":{"terminal":true}}`, os.Getpid())), 0o644); err != nil {
		t.Fatal(err)
	}
	setOwnedGoGateProcessEnvironment(t, "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)
	repositoryTop := declaredRepositoryTop(t, repo, map[string]int{repo: 2})

	_, err = classifyVerbCallerWith(repo, int64(os.Getpid()), repositoryTop)
	if err == nil || !strings.Contains(err.Error(), "the installation that serves "+repo+" cannot be found") {
		t.Fatalf("classification in a state root = %v, want a refusal naming %s", err, repo)
	}

	if err := os.WriteFile(filepath.Join(repo, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := classifyVerbCallerWith(repo, int64(os.Getpid()), repositoryTop)
	if err != nil || result.Class != lease.ClassHuman {
		t.Fatalf("classification in a self-hosted installation = %+v err=%v, want the terminal's human", result, err)
	}
}

func TestArmAllUsesTheProcessVerbRefusal(t *testing.T) {
	repo, installation := separateProcessScopeFixture(t)
	repositoryTop := declaredRepositoryTop(t, repo, map[string]int{repo: 1})
	stderr, code := captureStderr(t, func(stdout, stderr io.Writer) int {
		return runProcessArmWith([]string{"--repo", repo, "--installation", installation, "--all"}, repositoryTop, lease.ClassifyAt, stdout, stderr)
	})
	want := "metasystem system start: the fleet form is not built yet.\nrun: metasystem system start --repo " + repo + " --installation " + installation + "\n"
	if code != 1 || stderr != want {
		t.Fatalf("arm --all = code %d stderr %q, want code 1 stderr %q", code, stderr, want)
	}
}

func TestArmTemporaryWordStillRequiresHumanCallerAndLeavesFenceUnchanged(t *testing.T) {
	repo, installation := separateProcessScopeFixture(t)
	repositoryTop := declaredRepositoryTop(t, repo, map[string]int{repo: 1, installation: 1})
	if err := stopfence.Write(repo, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 9,
		ChangedAt: "2026-09-08T09:00:00Z", Checkout: repo,
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 71, PidStartedAt: 70}},
	}); err != nil {
		t.Fatal(err)
	}
	fencePath := stopfence.TransitionPath(repo)
	before, err := os.ReadFile(fencePath)
	if err != nil {
		t.Fatal(err)
	}
	classify := func(string, string, int64) (lease.Classification, error) {
		return lease.Classification{Class: lease.ClassDelegate}, nil
	}

	stdout, stderr, code := captureRelay(t, func(stdout, stderr io.Writer) int {
		return runProcessArmWith([]string{
			"--repo", repo, "--installation", installation,
			"--temporary-human-word", "Wido authorizes this temporary arm", "--review-by", "2026-09-09",
		}, repositoryTop, classify, stdout, stderr)
	})
	want := "metasystem system start: an agent started this shell.\n" +
		"in a terminal you opened yourself, run: metasystem system start --repo " + repo + " --installation " + installation + "\n"
	if code != 1 || stdout != "" || stderr != want {
		t.Fatalf("temporary arm refusal = code %d stdout %q stderr %q, want code 1 stderr %q", code, stdout, stderr, want)
	}
	after, err := os.ReadFile(fencePath)
	if err != nil || string(after) != string(before) {
		t.Fatalf("temporary arm refusal changed the fence: %v", err)
	}
}

func TestProcessClassifierDataFailureRepairsThenRetriesTheRequestedVerb(t *testing.T) {
	repo, installation := runnableSeparateProcessScopeFixture(t)
	repositoryTop := declaredRepositoryTop(t, repo, map[string]int{installation: 3, repo: 1})
	jobPath := filepath.Join(installation, "artifacts", "agents", "jobs", "damaged.json")
	if err := os.MkdirAll(filepath.Dir(jobPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jobPath, []byte("{\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := stopfence.Write(installation, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopIncomplete, Generation: 8,
		ChangedAt: "2026-09-08T08:00:00Z", Checkout: repo,
		By:         stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 71, PidStartedAt: 70}},
		NotStopped: []stopfence.Survivor{{Component: "job", ID: "remote-one", MachineID: "fixture-remote", Reason: "terminal evidence is unavailable"}},
	}); err != nil {
		t.Fatal(err)
	}
	fencePath := stopfence.TransitionPath(installation)
	before, err := os.ReadFile(fencePath)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		verb string
	}{
		{verb: "stop"},
		{verb: "arm"},
	} {
		t.Run(test.verb, func(t *testing.T) {
			stderr, code := captureStderr(t, func(stdout, stderr io.Writer) int {
				_, authorized := requireHumanTerminalAtWith(stderr, installation, installation, "metasystem "+test.verb, repositoryTop, lease.ClassifyAt, processVerbRetryCommand(processScope{Checkout: repo, Installation: stateroottest.Installation(t, installation), InstallationExplicit: true}, test.verb))
				if authorized {
					return 0
				}
				return 1
			})
			want := "metasystem " + publicProcessVerb(test.verb) + ": who started this shell can't be told: job record " + jobPath + " is damaged (invalid JSON: unexpected end of JSON input).\n" +
				"repair " + jobPath + ", then in a terminal you opened yourself, run: metasystem " + publicProcessVerb(test.verb) + " --repo " + repo + " --installation " + installation + "\n"
			if code != 1 || stderr != want {
				t.Fatalf("%s classification refusal = code %d stderr %q, want code 1 stderr %q", test.verb, code, stderr, want)
			}
			after, readErr := os.ReadFile(fencePath)
			if readErr != nil || string(after) != string(before) {
				t.Fatalf("%s classification refusal changed the fence: %v", test.verb, readErr)
			}
		})
	}

	stdout, stderr, code := captureRelay(t, func(stdout, stderr io.Writer) int {
		return runProcessStatusWith([]string{"--repo", repo, "--installation", installation}, repositoryTop, stdout, stderr)
	})
	if code != 1 || stderr != "" || !strings.Contains(stdout, "inventory unreadable: job "+jobPath+":") || !strings.Contains(stdout, "unexpected EOF") ||
		!strings.Contains(stdout, "repair the named read failures, then run: metasystem system status --repo "+repo) || strings.Contains(stdout, "agent-free terminal") {
		t.Fatalf("ungated status = code %d stdout %q stderr %q", code, stdout, stderr)
	}

	if err := os.WriteFile(jobPath, []byte(`{"machineId":"fixture-remote","status":"running"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr, code = captureStderr(t, func(stdout, stderr io.Writer) int {
		_, authorized := requireHumanTerminalAtWith(stderr, installation, installation, "metasystem system start", repositoryTop, lease.ClassifyAt, processVerbRetryCommand(processScope{Checkout: repo, Installation: stateroottest.Installation(t, installation), InstallationExplicit: true}, "arm"))
		if authorized {
			return 0
		}
		return 1
	})
	want := "metasystem system start: who started this shell can't be told: job record " + jobPath + " is damaged (jobId is missing).\n" +
		"repair " + jobPath + ", then in a terminal you opened yourself, run: metasystem system start --repo " + repo + " --installation " + installation + "\n"
	if code != 1 || stderr != want || strings.Contains(stderr, "fixture-remote") {
		t.Fatalf("unidentifiable-job refusal = code %d stderr %q, want %q without guessed identity", code, stderr, want)
	}
}

func TestArmRefusalSecondLines(t *testing.T) {
	checkout := "/fixture/checkout"
	scope := processScope{Checkout: checkout}
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{
			name: "stop in progress",
			err:  &stoptransition.StopInProgressError{Pid: 41},
			want: "run: metasystem system status --repo /fixture/checkout",
		},
		{
			name: "local survivor",
			err: &stoptransition.LocalSurvivorError{Survivor: stopfence.Survivor{
				Component: "run", Pid: 42, PidStartedAt: 43,
			}},
			want: "run: metasystem system stop --repo /fixture/checkout; if it survives a second stop, end pid 42 yourself; it is listed with its start time",
		},
		{
			name: "remote job",
			err:  &stoptransition.RemoteJobSurvivorError{JobID: "job-one", MachineID: "machine-two"},
			want: "cancel it from machine-two with metasystem work stop j2:job-one; then run: metasystem system start --repo /fixture/checkout",
		},
		{
			name: "unknown creator names claim",
			err:  &stoptransition.CreatorClaimSurvivorError{Verb: "run-launch", Path: "/fixture/creating/run-launch.json", Pid: 44},
			want: "run: metasystem system stop --repo /fixture/checkout",
		},
		{
			name: "unprobeable local identity",
			err:  &stoptransition.UnprobeableLocalSurvivorError{Survivor: stopfence.Survivor{Component: "run", ID: "one", Pid: 45, PidStartedAt: 46}},
			want: "run: metasystem system stop --repo /fixture/checkout",
		},
		{
			name: "missing remote job evidence",
			err:  &stoptransition.RemoteJobEvidenceError{JobID: "job-one", MachineID: "machine-two", Path: "/fixture/jobs/job-one.json", Kind: "missing"},
			want: "run: metasystem system stop --repo /fixture/checkout",
		},
		{
			name: "unreadable remote job evidence",
			err:  &stoptransition.RemoteJobEvidenceError{JobID: "job-one", MachineID: "machine-two", Path: "/fixture/jobs/job-one.json", Kind: "unreadable", Reason: "bad JSON"},
			want: "run: metasystem system stop --repo /fixture/checkout",
		},
		{
			name: "unreadable family",
			err:  &stoptransition.UnreadableFamilySurvivorError{Family: "mission", Reason: "records unreadable"},
			want: "run: metasystem system stop --repo /fixture/checkout",
		},
		{
			name: "recorded non-process reason",
			err:  &stoptransition.RecordedReasonSurvivorError{Survivor: stopfence.Survivor{Component: "creator-claim", ID: "/fixture/creating/broken.json", Reason: "unreadable"}},
			want: "run: metasystem system stop --repo /fixture/checkout",
		},
		{
			name: "crashed stop re-inventory publication",
			err:  &stoptransition.FencePublicationError{Path: "/fixture/transition.json", Err: errors.New("read-only filesystem")},
			want: "run: metasystem system stop --repo /fixture/checkout",
		},
		{
			name: "other arm failure",
			err:  errors.New("arming failed"),
			want: "run: metasystem system start --repo /fixture/checkout",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := armRefusalSecondLine(scope, test.err); got != test.want {
				t.Fatalf("second line = %q, want %q", got, test.want)
			}
			if test.name == "unknown creator names claim" && test.err.Error() != "creator run-launch claim /fixture/creating/run-launch.json has unknown liveness" {
				t.Fatalf("creator refusal = %q", test.err.Error())
			}
		})
	}
}

func TestUnreadableFenceMakesStopPointAtArm(t *testing.T) {
	scope := processScope{Checkout: "/fixture/checkout"}
	err := &stopfence.RecordUnreadableError{Reason: "stop fence schema version 2 is unsupported", HighestGeneration: 7}
	if got := stopRefusalSecondLine(scope, err); got != "run: metasystem system start --repo /fixture/checkout" {
		t.Fatalf("unreadable stop second line = %q", got)
	}
	if got := stopRefusalSecondLine(scope, errors.New("lock failed")); got != "run: metasystem system status --repo /fixture/checkout" {
		t.Fatalf("ordinary stop second line = %q", got)
	}
}

func TestTransitionRefusalsPreserveTheNamedInstallation(t *testing.T) {
	scope := processScope{Checkout: "/fixture/checkout", Installation: "/fixture/engine", InstallationExplicit: true}
	unreadable := &stopfence.RecordUnreadableError{Reason: "invalid JSON", HighestGeneration: 7}
	if got := stopRefusalSecondLine(scope, unreadable); got != "run: metasystem system start --repo /fixture/checkout --installation /fixture/engine" {
		t.Fatalf("stop unreadable retry = %q", got)
	}
	if got := stopRefusalSecondLine(scope, errors.New("lock failed")); got != "run: metasystem system status --repo /fixture/checkout --installation /fixture/engine" {
		t.Fatalf("stop status retry = %q", got)
	}
	if got := armRefusalSecondLine(scope, &stoptransition.UnprobeableLocalSurvivorError{}); got != "run: metasystem system stop --repo /fixture/checkout --installation /fixture/engine" {
		t.Fatalf("arm stop retry = %q", got)
	}
	if got := armRefusalSecondLine(scope, errors.New("arming failed")); got != "run: metasystem system start --repo /fixture/checkout --installation /fixture/engine" {
		t.Fatalf("arm retry = %q", got)
	}
}

func TestScopeRefusalRetainsTheInstallationOptionWithAWorkingRepairPlaceholder(t *testing.T) {
	stderr, code := captureStderr(t, func(stdout, stderr io.Writer) int {
		return processScopeRefusal(stderr, "stop", "/fixture/checkout", "/broken/engine", errors.New("/broken/engine carries no engine"))
	})
	want := "metasystem system stop: /broken/engine carries no engine.\n" +
		"run: metasystem system stop --repo /fixture/checkout --installation <dir>, where <dir> holds this checkout's bin/metasystem\n"
	if code != 1 || stderr != want {
		t.Fatalf("scope refusal = code %d stderr %q, want %q", code, stderr, want)
	}
}

func TestCrashStepRefusalPreservesTheNamedInstallation(t *testing.T) {
	root := missionFenceFixture(t, false)
	repositoryTop := declaredRepositoryTop(t, root, map[string]int{root: 1})
	t.Setenv("METASYSTEM_STOP_CRASH_AFTER", "not-a-step")
	stderr, code := captureStderr(t, func(stdout, stderr io.Writer) int {
		return runProcessStopWith([]string{"--repo", root, "--installation", root}, repositoryTop, lease.ClassifyAt, stdout, stderr)
	})
	want := "metasystem system stop: a test setting that crashes the stop names no stop step (1 through 9), so nothing was stopped.\n" +
		"run: env -u METASYSTEM_STOP_CRASH_AFTER metasystem system stop --repo " + root + " --installation " + root + "\n"
	if code != 1 || stderr != want {
		t.Fatalf("crash-step refusal = code %d stderr %q, want %q", code, stderr, want)
	}
}

// The argument adapter of the retired internal stop, status and arm forms,
// kept for these tests: it parses a process scope and prints what the
// process owners the public system stop, status and start call return.

func parseProcessScopeWith(stderr io.Writer, verb string, args []string, repositoryTop func(string) (string, error), register ...func(*flag.FlagSet)) (processScope, int, int) {
	flags := flag.NewFlagSet("metasystem "+verb, flag.ContinueOnError)
	flags.SetOutput(stderr)
	repo := pathFlag(flags, "repo", ".", "repository or path inside it")
	installation := flags.String("installation", "", "metasystem installation for this checkout")
	all := flags.Bool("all", false, "every checkout on this host")
	for _, add := range register {
		add(flags)
	}
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return processScope{}, 0, 2
	}
	scope, err := resolveProcessScopeWith(*repo, *installation, repositoryTop)
	if err != nil {
		return processScope{}, 0, processScopeRefusal(stderr, verb, *repo, *installation, err)
	}
	if *all {
		return processScope{}, 0, refuseProcessVerbTo(stderr, verb, scope.Checkout, "the fleet form is not built yet", "run: "+processVerbRetryCommand(scope, verb))
	}
	scale := upWaitScale()
	if scale < 1 {
		fmt.Fprintf(stderr, "metasystem %s: METASYSTEM_FIXTURE_CAP_SCALE_MILLI must be a positive integer\n", verb)
		return processScope{}, 0, 2
	}
	return scope, scale, 0
}

func printProcessReport(stdout io.Writer, report stoptransition.Report) int {
	for _, line := range report.Lines {
		fmt.Fprintln(stdout, line)
	}
	return report.ExitCode
}

func processScopeRefusal(stderr io.Writer, verb, repo, installation string, err error) int {
	sentence := err.Error()
	checkout, named := "<a path inside the checkout>", installation
	if strings.Contains(sentence, "carries no metasystem installation") {
		checkout = strings.TrimSuffix(sentence, " carries no metasystem installation")
		named = "<dir>, where <dir> holds this checkout's bin/metasystem"
	} else if strings.Contains(sentence, "carries no engine") {
		checkout = repo
		named = "<dir>, where <dir> holds this checkout's bin/metasystem"
	}
	command := "metasystem " + publicProcessVerb(verb) + " --repo " + checkout
	if named != "" {
		command += " --installation " + named
	}
	return refuseProcessVerbTo(stderr, verb, repo, sentence, "run: "+command)
}

func runProcessStopWith(args []string, repositoryTop func(string) (string, error), classify processCallerClassifier, stdout, stderr io.Writer) int {
	scope, scale, code := parseProcessScopeWith(stderr, "stop", args, repositoryTop)
	if code != 0 {
		return code
	}
	owners := defaultProcessOwners()
	owners.repositoryTop, owners.classify = repositoryTop, classify
	report, refusal := owners.stop(scope, scale)
	if refusal != nil {
		return refusal.printTo(stderr)
	}
	return printProcessReport(stdout, report)
}

func runProcessStatusWith(args []string, repositoryTop func(string) (string, error), stdout, stderr io.Writer) int {
	scope, scale, code := parseProcessScopeWith(stderr, "status", args, repositoryTop)
	if code != 0 {
		return code
	}
	report, err := defaultProcessOwners().status(scope, scale)
	if err != nil {
		fmt.Fprintf(stderr, "metasystem status: %v.\n", err)
		return 1
	}
	return printProcessReport(stdout, report)
}

func runProcessArmWith(args []string, repositoryTop func(string) (string, error), classify processCallerClassifier, stdout, stderr io.Writer) int {
	var temporaryWord, reviewBy string
	scope, scale, code := parseProcessScopeWith(stderr, "arm", args, repositoryTop, func(flags *flag.FlagSet) {
		flags.StringVar(&temporaryWord, "temporary-human-word", "", "verbatim remote human authorization")
		flags.StringVar(&reviewBy, "review-by", "", "human re-approval date")
	})
	if code != 0 {
		return code
	}
	owners := defaultProcessOwners()
	owners.repositoryTop, owners.classify = repositoryTop, classify
	report, refusal := owners.arm(scope, scale, temporaryWord, reviewBy)
	if refusal != nil {
		for _, line := range report.Lines {
			fmt.Fprintln(stdout, line)
		}
		return refusal.printTo(stderr)
	}
	return printProcessReport(stdout, report)
}
