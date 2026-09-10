package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stoptransition"
)

func separateProcessScopeFixture(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	if output, err := fixtureGitCommand("init", "--quiet", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	var err error
	repo, err = canonicalPath(repo)
	if err != nil {
		t.Fatal(err)
	}
	installation := filepath.Join(repo, "separate-engine")
	if err := os.MkdirAll(filepath.Join(installation, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "bin", "metasystem"), []byte("fixture engine\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if output, err := fixtureGitCommand("-C", installation, "rev-parse", "--show-toplevel").CombinedOutput(); err != nil {
		t.Fatalf("separate engine Git scope: %v: %s (repo %s, installation %s)", err, output, repo, installation)
	}
	return repo, installation
}

func fixtureGitCommand(args ...string) *exec.Cmd {
	command := exec.Command("git", args...)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			command.Env = append(command.Env, entry)
		}
	}
	return command
}

func runnableSeparateProcessScopeFixture(t *testing.T) (string, string) {
	t.Helper()
	repo, installation := separateProcessScopeFixture(t)
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := filepath.Join(installation, "scripts", "agents", "adapters", "fake.sh")
	if err := os.MkdirAll(filepath.Dir(adapter), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(adapter, []byte("#!/bin/sh\nprintf 'match fake-runtime-never-present\\n'\n"), 0o755); err != nil {
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
	previous := classifyProcessVerbCaller
	var classifiedInstallations []string
	classifyProcessVerbCaller = func(_ string, gotInstallation string, _ int64) (lease.Classification, error) {
		classifiedInstallations = append(classifiedInstallations, gotInstallation)
		return lease.Classification{Class: lease.ClassDelegate}, nil
	}
	t.Cleanup(func() { classifyProcessVerbCaller = previous })

	stdout, stderr, code := captureRelay(t, func() int {
		return runProcessStatus([]string{"--repo", repo, "--installation", installation})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "checkout "+repo+"\nnothing is running") || strings.Contains(stdout, "--metasystem-root") {
		t.Fatalf("separate-installation status = code %d stdout %q stderr %q", code, stdout, stderr)
	}

	for _, verb := range []struct {
		name string
		run  func([]string) int
	}{
		{name: "stop", run: runProcessStop},
		{name: "arm", run: runProcessArm},
	} {
		t.Run(verb.name, func(t *testing.T) {
			stdout, stderr, code := captureRelay(t, func() int {
				return verb.run([]string{"--repo", repo, "--installation", installation})
			})
			want := "metasystem " + verb.name + ": " + verb.name + " is a human act at a terminal; this caller is DELEGATE.\n" +
				"at an agent-free terminal, run: metasystem " + verb.name + " --repo " + repo + " --installation " + installation + "\n"
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
	scope, scale, code := parseProcessScope("arm", []string{"--repo", repo, "--installation", installation})
	if code != 0 || scope.Checkout != repo || scope.Installation != installation || scope.Root != installation || scale < 1 {
		t.Fatalf("arm scope = %#v scale=%d code=%d", scope, scale, code)
	}
	if err := stopfence.Write(scope.Root, stopfence.Record{
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

func TestArmAllUsesTheProcessVerbRefusal(t *testing.T) {
	repo, installation := separateProcessScopeFixture(t)
	stderr, code := captureStderr(t, func() int {
		return runProcessArm([]string{"--repo", repo, "--installation", installation, "--all"})
	})
	want := "metasystem arm: the fleet form is not built yet.\nrun: metasystem arm --repo " + repo + " --installation " + installation + "\n"
	if code != 1 || stderr != want {
		t.Fatalf("arm --all = code %d stderr %q, want code 1 stderr %q", code, stderr, want)
	}
}

func TestArmTemporaryWordStillRequiresHumanCallerAndLeavesFenceUnchanged(t *testing.T) {
	repo, installation := separateProcessScopeFixture(t)
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
	previous := classifyProcessVerbCaller
	classifyProcessVerbCaller = func(string, string, int64) (lease.Classification, error) {
		return lease.Classification{Class: lease.ClassDelegate}, nil
	}
	t.Cleanup(func() { classifyProcessVerbCaller = previous })

	stdout, stderr, code := captureRelay(t, func() int {
		return runProcessArm([]string{
			"--repo", repo, "--installation", installation,
			"--temporary-human-word", "Wido authorizes this temporary arm", "--review-by", "2026-09-09",
		})
	})
	want := "metasystem arm: arm is a human act at a terminal; this caller is DELEGATE.\n" +
		"at an agent-free terminal, run: metasystem arm --repo " + repo + " --installation " + installation + "\n"
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
			stderr, code := captureStderr(t, func() int {
				_, authorized := requireHumanTerminalAt(installation, installation, "metasystem "+test.verb, processVerbRetryCommand(processScope{Checkout: repo, Installation: installation, InstallationExplicit: true}, test.verb))
				if authorized {
					return 0
				}
				return 1
			})
			want := "metasystem " + test.verb + ": caller classification is blocked by job record " + jobPath + ": invalid JSON: unexpected end of JSON input.\n" +
				"repair " + jobPath + ", then at an agent-free terminal, run: metasystem " + test.verb + " --repo " + repo + " --installation " + installation + "\n"
			if code != 1 || stderr != want {
				t.Fatalf("%s classification refusal = code %d stderr %q, want code 1 stderr %q", test.verb, code, stderr, want)
			}
			after, readErr := os.ReadFile(fencePath)
			if readErr != nil || string(after) != string(before) {
				t.Fatalf("%s classification refusal changed the fence: %v", test.verb, readErr)
			}
		})
	}

	stdout, stderr, code := captureRelay(t, func() int {
		return runProcessStatus([]string{"--repo", repo, "--installation", installation})
	})
	if code != 1 || stderr != "" || !strings.Contains(stdout, "inventory unreadable: job "+jobPath+":") || !strings.Contains(stdout, "unexpected EOF") ||
		!strings.Contains(stdout, "repair the named read failures, then run: metasystem status --repo "+repo) || strings.Contains(stdout, "agent-free terminal") {
		t.Fatalf("ungated status = code %d stdout %q stderr %q", code, stdout, stderr)
	}

	if err := os.WriteFile(jobPath, []byte(`{"machineId":"fixture-remote","status":"running"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr, code = captureStderr(t, func() int {
		_, authorized := requireHumanTerminalAt(installation, installation, "metasystem arm", processVerbRetryCommand(processScope{Checkout: repo, Installation: installation, InstallationExplicit: true}, "arm"))
		if authorized {
			return 0
		}
		return 1
	})
	want := "metasystem arm: caller classification is blocked by job record " + jobPath + ": jobId is missing.\n" +
		"repair " + jobPath + ", then at an agent-free terminal, run: metasystem arm --repo " + repo + " --installation " + installation + "\n"
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
			want: "run: metasystem status --repo /fixture/checkout",
		},
		{
			name: "local survivor",
			err: &stoptransition.LocalSurvivorError{Survivor: stopfence.Survivor{
				Component: "run", Pid: 42, PidStartedAt: 43,
			}},
			want: "run: metasystem stop --repo /fixture/checkout; if it survives a second stop, end pid 42 yourself; it is listed with its start time",
		},
		{
			name: "remote job",
			err:  &stoptransition.RemoteJobSurvivorError{JobID: "job-one", MachineID: "machine-two"},
			want: "cancel it from machine-two with metasystem delegate --cancel job-one; then run: metasystem arm --repo /fixture/checkout",
		},
		{
			name: "unknown creator names claim",
			err:  &stoptransition.CreatorClaimSurvivorError{Verb: "run-launch", Path: "/fixture/creating/run-launch.json", Pid: 44},
			want: "run: metasystem stop --repo /fixture/checkout",
		},
		{
			name: "unprobeable local identity",
			err:  &stoptransition.UnprobeableLocalSurvivorError{Survivor: stopfence.Survivor{Component: "run", ID: "one", Pid: 45, PidStartedAt: 46}},
			want: "run: metasystem stop --repo /fixture/checkout",
		},
		{
			name: "missing remote job evidence",
			err:  &stoptransition.RemoteJobEvidenceError{JobID: "job-one", MachineID: "machine-two", Path: "/fixture/jobs/job-one.json", Kind: "missing"},
			want: "run: metasystem stop --repo /fixture/checkout",
		},
		{
			name: "unreadable remote job evidence",
			err:  &stoptransition.RemoteJobEvidenceError{JobID: "job-one", MachineID: "machine-two", Path: "/fixture/jobs/job-one.json", Kind: "unreadable", Reason: "bad JSON"},
			want: "run: metasystem stop --repo /fixture/checkout",
		},
		{
			name: "unreadable family",
			err:  &stoptransition.UnreadableFamilySurvivorError{Family: "mission", Reason: "records unreadable"},
			want: "run: metasystem stop --repo /fixture/checkout",
		},
		{
			name: "recorded non-process reason",
			err:  &stoptransition.RecordedReasonSurvivorError{Survivor: stopfence.Survivor{Component: "creator-claim", ID: "/fixture/creating/broken.json", Reason: "unreadable"}},
			want: "run: metasystem stop --repo /fixture/checkout",
		},
		{
			name: "crashed stop re-inventory publication",
			err:  &stoptransition.FencePublicationError{Path: "/fixture/transition.json", Err: errors.New("read-only filesystem")},
			want: "run: metasystem stop --repo /fixture/checkout",
		},
		{
			name: "other arm failure",
			err:  errors.New("arming failed"),
			want: "run: metasystem arm --repo /fixture/checkout",
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

func TestRunLaunchReadsClosedFenceBeforeCallerGate(t *testing.T) {
	root := t.TempDir()
	if err := stopfence.Write(root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 3,
		ChangedAt: "2026-09-07T12:00:00Z", Checkout: root,
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 71, PidStartedAt: 70}},
	}); err != nil {
		t.Fatal(err)
	}
	stderr, code := captureStderr(t, func() int {
		return runRunLaunch([]string{"--root", root, "--id", "fence-first", "--caller-pid", "-1", "--", "/bin/true"})
	})
	if code != 1 || !strings.Contains(stderr, "the metasystem is stopped for "+root+" since 2026-09-07T12:00:00Z, by stop pid 71") || strings.Contains(stderr, "caller classification failed") {
		t.Fatalf("run launch fence precedence: code=%d stderr=%q", code, stderr)
	}
}

func TestUnreadableFenceMakesStopPointAtArm(t *testing.T) {
	scope := processScope{Checkout: "/fixture/checkout"}
	err := &stopfence.RecordUnreadableError{Reason: "stop fence schema version 2 is unsupported", HighestGeneration: 7}
	if got := stopRefusalSecondLine(scope, err); got != "run: metasystem arm --repo /fixture/checkout" {
		t.Fatalf("unreadable stop second line = %q", got)
	}
	if got := stopRefusalSecondLine(scope, errors.New("lock failed")); got != "run: metasystem status --repo /fixture/checkout" {
		t.Fatalf("ordinary stop second line = %q", got)
	}
}

func TestTransitionRefusalsPreserveTheNamedInstallation(t *testing.T) {
	scope := processScope{Checkout: "/fixture/checkout", Installation: "/fixture/engine", InstallationExplicit: true}
	unreadable := &stopfence.RecordUnreadableError{Reason: "invalid JSON", HighestGeneration: 7}
	if got := stopRefusalSecondLine(scope, unreadable); got != "run: metasystem arm --repo /fixture/checkout --installation /fixture/engine" {
		t.Fatalf("stop unreadable retry = %q", got)
	}
	if got := stopRefusalSecondLine(scope, errors.New("lock failed")); got != "run: metasystem status --repo /fixture/checkout --installation /fixture/engine" {
		t.Fatalf("stop status retry = %q", got)
	}
	if got := armRefusalSecondLine(scope, &stoptransition.UnprobeableLocalSurvivorError{}); got != "run: metasystem stop --repo /fixture/checkout --installation /fixture/engine" {
		t.Fatalf("arm stop retry = %q", got)
	}
	if got := armRefusalSecondLine(scope, errors.New("arming failed")); got != "run: metasystem arm --repo /fixture/checkout --installation /fixture/engine" {
		t.Fatalf("arm retry = %q", got)
	}
}

func TestScopeRefusalRetainsTheInstallationOptionWithAWorkingRepairPlaceholder(t *testing.T) {
	stderr, code := captureStderr(t, func() int {
		return processScopeRefusal("stop", "/fixture/checkout", "/broken/engine", errors.New("/broken/engine carries no engine"))
	})
	want := "metasystem stop: /broken/engine carries no engine.\n" +
		"run: metasystem stop --repo /fixture/checkout --installation <dir>, where <dir> holds this checkout's bin/metasystem\n"
	if code != 1 || stderr != want {
		t.Fatalf("scope refusal = code %d stderr %q, want %q", code, stderr, want)
	}
}

func TestCrashStepRefusalPreservesTheNamedInstallation(t *testing.T) {
	root := missionFenceFixture(t, false)
	t.Setenv("METASYSTEM_STOP_CRASH_AFTER", "not-a-step")
	stderr, code := captureStderr(t, func() int {
		return runProcessStop([]string{"--repo", root, "--installation", root})
	})
	want := "metasystem stop: METASYSTEM_STOP_CRASH_AFTER must name a numbered section-4 step from 1 through 9.\n" +
		"run: metasystem stop --repo " + root + " --installation " + root + "\n"
	if code != 1 || stderr != want {
		t.Fatalf("crash-step refusal = code %d stderr %q, want %q", code, stderr, want)
	}
}
