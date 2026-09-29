package main

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stoptransition"
)

// supBProber answers liveness from one per-test table; every live identity
// started at second 20.
type supBProber map[int64]identity.Liveness

func (p supBProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	state, ok := p[pid]
	if !ok {
		state = identity.Dead
	}
	return identity.Exact{Pid: pid, StartedAt: time.Unix(20, 0)}, state, nil
}

// supBEmptyFamily is a checkout whose own processes are all gone, so the
// transition acts only on the durable fence and its recorded survivors.
type supBEmptyFamily struct{}

func (supBEmptyFamily) Name() string                              { return "run" }
func (supBEmptyFamily) Inventory() ([]stoptransition.Item, error) { return nil, nil }
func (supBEmptyFamily) Stop(stoptransition.Item) (stoptransition.Outcome, error) {
	return stoptransition.Outcome{}, nil
}

func supBRepo(t *testing.T) string {
	t.Helper()
	repo, err := canonicalPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

// supBOwners are the process-verb owners of one human terminal: the caller is
// classified HUMAN, the transition runs over the given prober at a fixed
// clock as pid 10, and the arm sequence starts nothing.
func supBOwners(prober supBProber, armLines []string) processOwners {
	now := time.Date(2026, 9, 27, 13, 0, 0, 0, time.UTC)
	return processOwners{
		repositoryTop: func(path string) (string, error) { return path, nil },
		classify: func(string, string, int64) (lease.Classification, error) {
			return lease.Classification{Class: lease.ClassHuman}, nil
		},
		transition: func(scope processScope, scale int) *stoptransition.Transition {
			return &stoptransition.Transition{
				Root: scope.Root, Checkout: scope.Checkout, ScaleMilli: scale,
				Families: []stoptransition.Family{supBEmptyFamily{}},
				Prober:   prober, Now: func() time.Time { return now },
				Sleep: func(time.Duration) {},
				Self:  func() (identity.Ref, error) { return identity.Ref{Pid: 10, StartedAtSec: 20}, nil },
			}
		},
		armSteps: func(processScope, int, processArmAuthority) (processArmResult, error) {
			return processArmResult{lines: armLines}, nil
		},
	}
}

func supBPrintRefusal(t *testing.T, refusal *processRefusal) (string, int) {
	t.Helper()
	if refusal == nil {
		t.Fatal("the verb was not refused")
	}
	return captureStderr(t, func(_, stderr io.Writer) int { return refusal.printTo(stderr) })
}

func supBReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestSupBArmRefusesSurvivorsAndRemoteEvidenceUntilTerminal ports the
// arm-refuses-survivor and arm-again scenarios of supervision-fixtures part B
// at the process-verb owners: a listed local survivor, missing remote job
// evidence, a readable non-terminal remote record, and finally matching
// terminal evidence that lets stop complete and arm open the fence again.
// The corrupt and unidentifiable classifier inputs are
// TestProcessClassifierDataFailureRepairsThenRetriesTheRequestedVerb.
func TestSupBArmRefusesSurvivorsAndRemoteEvidenceUntilTerminal(t *testing.T) {
	repo := supBRepo(t)
	scope := processScope{Checkout: repo, Installation: repo, Root: repo, Binary: filepath.Join(repo, "bin", "metasystem")}
	prober := supBProber{10: identity.Alive, 33: identity.Alive}
	owners := supBOwners(prober, []string{"component=steward-runner outcome=started"})
	stopActor := stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 71, PidStartedAt: 70}}
	fencePath := stopfence.TransitionPath(repo)

	if err := stopfence.Write(repo, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopIncomplete, Generation: 4,
		ChangedAt: "2026-09-27T12:00:00Z", Checkout: repo, By: stopActor,
		NotStopped: []stopfence.Survivor{{Component: "run", Pid: 33, PidStartedAt: 20, Reason: "survived orderly stop"}},
	}); err != nil {
		t.Fatal(err)
	}
	before := supBReadFile(t, fencePath)
	_, refusal := owners.arm(scope, 1, "", "")
	stderr, code := supBPrintRefusal(t, refusal)
	want := "metasystem system start: run pid 33 started 20 survived the last stop.\n" +
		"run: metasystem system stop --repo " + repo + "; if it survives a second stop, end pid 33 yourself; it is listed with its start time\n"
	if code != 1 || stderr != want {
		t.Fatalf("local survivor refusal = code %d stderr %q, want %q", code, stderr, want)
	}
	if after := supBReadFile(t, fencePath); after != before {
		t.Fatalf("the refused arm changed the incomplete fence:\n%s", after)
	}
	if prober[33] != identity.Alive {
		t.Fatal("the refused arm ended the listed survivor")
	}

	// The survivor ends; a remote job whose record is missing is now the
	// only obligation.
	delete(prober, 33)
	remoteJob, remoteMachine := "arm-refuses-remote", "fixture-remote"
	remoteRecord := filepath.Join(repo, "artifacts", "agents", "jobs", remoteJob+".json")
	if err := stopfence.Write(repo, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopIncomplete, Generation: 4,
		ChangedAt: "2026-09-27T12:00:00Z", Checkout: repo, By: stopActor,
		NotStopped: []stopfence.Survivor{{Component: "job", ID: remoteJob, MachineID: remoteMachine, Reason: "remote terminal state remains unproven"}},
	}); err != nil {
		t.Fatal(err)
	}
	assertMissingRemoteArm := func(label string) {
		t.Helper()
		_, refusal := owners.arm(scope, 1, "", "")
		stderr, code := supBPrintRefusal(t, refusal)
		lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
		if code != 1 || len(lines) != 2 ||
			!strings.Contains(lines[0], "job "+remoteJob+" on machine "+remoteMachine) ||
			!strings.Contains(lines[0], "record "+remoteRecord+" is missing") ||
			!strings.Contains(lines[0], "restore that job's record from "+remoteMachine) ||
			lines[1] != "run: metasystem system stop --repo "+repo {
			t.Fatalf("%s missing-remote arm refusal = code %d stderr %q", label, code, stderr)
		}
	}
	assertMissingRemoteArm("first")

	report, refusal := owners.stop(scope, 1)
	joined := strings.Join(report.Lines, "\n")
	if refusal != nil || report.ExitCode != 1 || strings.Count(joined, "NOT STOPPED ") != 1 ||
		!strings.Contains(joined, "record "+remoteRecord+" is missing") {
		t.Fatalf("stop over missing remote evidence = %#v refusal=%+v", report, refusal)
	}
	retained, err := stopfence.Read(repo)
	if err != nil || retained.Phase != stopfence.PhaseStopIncomplete || len(retained.NotStopped) != 1 ||
		retained.NotStopped[0].ID != remoteJob || retained.NotStopped[0].MachineID != remoteMachine {
		t.Fatalf("stop did not retain the one missing remote obligation: %#v err=%v", retained, err)
	}
	assertMissingRemoteArm("repeated")

	if err := os.MkdirAll(filepath.Dir(remoteRecord), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(remoteRecord, []byte(`{"jobId":"`+remoteJob+`","machineId":"`+remoteMachine+`","status":"running"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, refusal = owners.arm(scope, 1, "", "")
	stderr, code = supBPrintRefusal(t, refusal)
	want = "metasystem system start: job " + remoteJob + " is owned by machine " + remoteMachine + " and is not terminal.\n" +
		"cancel it from " + remoteMachine + " with metasystem work stop j2:" + remoteJob + "; then run: metasystem system start --repo " + repo + "\n"
	if code != 1 || stderr != want {
		t.Fatalf("readable non-terminal remote refusal = code %d stderr %q, want %q", code, stderr, want)
	}

	if err := os.WriteFile(remoteRecord, []byte(`{"jobId":"`+remoteJob+`","machineId":"`+remoteMachine+`","status":"completed"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, refusal = owners.stop(scope, 1)
	if refusal != nil || report.ExitCode != 0 || report.Lines[len(report.Lines)-1] != "stopped "+repo+"; start again: metasystem system start --repo "+repo {
		t.Fatalf("stop with restored terminal evidence = %#v refusal=%+v", report, refusal)
	}
	stopped, err := stopfence.Read(repo)
	if err != nil || stopped.Phase != stopfence.PhaseStopped || len(stopped.NotStopped) != 0 {
		t.Fatalf("terminal evidence did not clear the remote obligation: %#v err=%v", stopped, err)
	}

	// arm-again: the next arm opens the fence at a new generation, runs the
	// arm sequence, and ends with its exact generation line.
	armed, refusal := owners.arm(scope, 1, "", "")
	opened, err := stopfence.Read(repo)
	if refusal != nil || err != nil || opened.State != stopfence.StateOpen || opened.Phase != stopfence.PhaseArmed ||
		opened.Generation != stopped.Generation+1 || opened.By.Verb != "arm" {
		t.Fatalf("arm after the obligation cleared = %#v refusal=%+v fence=%#v err=%v", armed, refusal, opened, err)
	}
	// A start says the evidence root first; this bed configures none, so it
	// is the default under the test process's HOME, named, never made.
	if len(armed.Lines) == 0 || !strings.HasPrefix(armed.Lines[0], "evidence root: ") || !strings.HasSuffix(armed.Lines[0], " (default; set "+config.EvidenceRootKey+" in metasystem.conf.local to change)") {
		t.Fatalf("arm page = %#v, want the evidence root line first", armed)
	}
	armed.Lines = armed.Lines[1:]
	wantArm := []string{"checkout " + repo, "component=steward-runner outcome=started", "armed " + repo + " generation " + strconv.FormatInt(opened.Generation, 10)}
	if armed.ExitCode != 0 || strings.Join(armed.Lines, "\n") != strings.Join(wantArm, "\n") {
		t.Fatalf("arm page = %#v, want %q", armed, wantArm)
	}
}

// TestSupBStewardCreationVerbsRefuseTheClosedFenceWithTheStartRemedy ports
// the stop-fence steward rows: steward arm and steward restart refuse a
// stopped checkout before any caller classification with the two stopped
// lines, and create no runner state.
func TestSupBStewardCreationVerbsRefuseTheClosedFenceWithTheStartRemedy(t *testing.T) {
	repo := supBRepo(t)
	if err := stopfence.Write(repo, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 6,
		ChangedAt: "2026-09-27T12:30:00Z", Checkout: repo,
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 72, PidStartedAt: 70}},
	}); err != nil {
		t.Fatal(err)
	}
	want := "the metasystem is stopped for " + repo + " since 2026-09-27T12:30:00Z, by stop pid 72\n" +
		"run: metasystem system start --repo " + repo + "\n"
	for name, verb := range map[string]command{"arm": runStewardArm} {
		stdout, stderr, code := captureRelay(t, func(stdout, stderr io.Writer) int { return verb([]string{"--repo", repo}, stdout, stderr) })
		if code != 1 || stdout != "" || stderr != want {
			t.Fatalf("steward %s under the closed fence = code %d stdout %q stderr %q, want %q", name, code, stdout, stderr, want)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, "artifacts", "agents", "steward")); !os.IsNotExist(err) {
		t.Fatalf("a refused steward verb created runner state: %v", err)
	}
}

// TestSupBProofRunLaunchRefusesTheClosedFenceWithAFailedProofResult ports the
// stop-fence proof-run row: a proof launch under the closed fence prints the
// two stopped lines, reports one failed PROOF-RESULT, exits 1, and publishes
// no proof-run record.
func TestSupBProofRunLaunchRefusesTheClosedFenceWithAFailedProofResult(t *testing.T) {
	repo := supBRepo(t)
	conf := filepath.Join(repo, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := stopfence.Write(repo, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 6,
		ChangedAt: "2026-09-27T12:30:00Z", Checkout: repo,
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 72, PidStartedAt: 70}},
	}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"METASYSTEM_PROOF_CONTROL_ROOT", "METASYSTEM_PROOF_ATTEMPT", "METASYSTEM_PROOF_RUN_ROOT",
		"METASYSTEM_PROOF_RUN_ID", "METASYSTEM_PROOF_RECORD_KEY", "METASYSTEM_PROOF_CREATION_CLAIM", "METASYSTEM_PROOF_AUTH_BIN",
		"METASYSTEM_HOOK_DELEGATE_STATE_ROOT", "METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT", "METASYSTEM_HOOK_DELEGATE_JOB"} {
		t.Setenv(name, "")
	}
	// An unusable admission directory proves the refusal never reaches host
	// capacity.
	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", "/")
	logPath := filepath.Join(repo, "stop-fence-proof.log")
	stdout, stderr, code := captureRelay(t, func(stdout, stderr io.Writer) int {
		return runProofRunLaunch([]string{"--suite", "stop-fence-proof", "--root", repo, "--control-root", repo,
			"--conf", conf, "--progress", filepath.Join(repo, "stop-fence-progress.jsonl"), "--log", logPath,
			"--banner", "stop-fence", "--", "/bin/true"}, stdout, stderr)
	})
	want := "the metasystem is stopped for " + repo + " since 2026-09-27T12:30:00Z, by stop pid 72\n" +
		"at an agent-free terminal, run: metasystem system start --repo " + repo + "\n" +
		`PROOF-RESULT {"schemaVersion":1,"disposition":"failed","exitStatus":1}` + "\n"
	if code != 1 || stdout != "" || stderr != want {
		t.Fatalf("closed-fence proof launch = code %d stdout %q stderr %q, want %q", code, stdout, stderr, want)
	}
	for _, path := range []string{logPath, filepath.Join(repo, "artifacts", "agents", "proof-runs")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("the refused proof launch created %s: %v", path, err)
		}
	}
	if claims, err := stopfence.Claims(repo, 6); err != nil || len(claims) != 0 {
		t.Fatalf("the refused proof launch left creation claims %+v: %v", claims, err)
	}
}
