package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport/stopreporttest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

type stubStewardLandingRefGit struct{ stub *testgit.Stub }

func (g stubStewardLandingRefGit) Output(repo string, args ...string) ([]byte, error) {
	result := g.stub.Run(testgit.Call{Dir: repo, Args: append([]string{"Output"}, args...)})
	return result.Stdout, result.Err
}

func (g stubStewardLandingRefGit) CombinedOutput(repo string, args ...string) ([]byte, error) {
	result := g.stub.Run(testgit.Call{Dir: repo, Args: append([]string{"CombinedOutput"}, args...)})
	return result.Stdout, result.Err
}

func stewardSeedCall(repo, method string, result testgit.Result, args ...string) testgit.Expectation {
	return testgit.Expectation{
		Call:   testgit.Call{Dir: repo, Args: append([]string{method}, args...)},
		Result: result,
	}
}

func stewardSeedConfigRead(repo string, result testgit.Result) testgit.Expectation {
	return stewardSeedCall(repo, "Output", result, "config", "--local", "--no-includes", "--get", "metasystem.steward.landing-ref")
}

func stewardSeedBranchRead(repo string, result testgit.Result) testgit.Expectation {
	return stewardSeedCall(repo, "Output", result, "symbolic-ref", "--quiet", "--short", "HEAD")
}

func stewardSeedUpstreamRead(repo string, result testgit.Result) testgit.Expectation {
	return stewardSeedCall(repo, "CombinedOutput", result, "rev-parse", "--symbolic-full-name", "@{upstream}")
}

func stewardSeedConfigWrite(repo, ref string, result testgit.Result) testgit.Expectation {
	return stewardSeedCall(repo, "CombinedOutput", result, "config", "--local", "metasystem.steward.landing-ref", ref)
}

func stewardVerbGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func TestHumanStewardVerbSeedsTheCheckedOutUpstreamOnce(t *testing.T) {
	repo := t.TempDir()
	stub := testgit.New(t,
		stewardSeedConfigRead(repo, testgit.Result{Err: errors.New("exit status 1")}),
		stewardSeedBranchRead(repo, testgit.Result{Stdout: []byte(" release\n")}),
		stewardSeedUpstreamRead(repo, testgit.Result{Stdout: []byte("refs/remotes/origin/release\n")}),
		stewardSeedConfigWrite(repo, "refs/remotes/origin/release", testgit.Result{}),
		stewardSeedConfigRead(repo, testgit.Result{Stdout: []byte("refs/remotes/fork/operator-choice\n")}),
	)
	git := stubStewardLandingRefGit{stub}
	seeded, err := seedStewardLandingRefWithGit(repo, git)
	if err != nil || seeded != (stewardLandingRefSeed{Ref: "refs/remotes/origin/release"}) {
		t.Fatalf("checked-out upstream was not seeded: %+v %v", seeded, err)
	}
	seeded, err = seedStewardLandingRefWithGit(repo, git)
	if err != nil || seeded != (stewardLandingRefSeed{}) {
		t.Fatalf("an existing local value was reseeded: %+v %v", seeded, err)
	}
}

func TestStewardLandingRefGitAdapterPersistsCheckedOutUpstream(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	stewardVerbGit(t, root, "init", "-q", "-b", "release")
	stewardVerbGit(t, root, "config", "user.name", "fixture")
	stewardVerbGit(t, root, "config", "user.email", "fixture@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "README"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stewardVerbGit(t, root, "add", "README")
	stewardVerbGit(t, root, "commit", "-qm", "fixture")
	remote := filepath.Join(t.TempDir(), "origin.git")
	if err := os.Mkdir(remote, 0o755); err != nil {
		t.Fatal(err)
	}
	stewardVerbGit(t, remote, "init", "--bare", "-q")
	stewardVerbGit(t, root, "remote", "add", "origin", remote)
	stewardVerbGit(t, root, "push", "-q", "-u", "origin", "release")
	seeded, err := seedStewardLandingRef(root)
	if err != nil || seeded.Ref != "refs/remotes/origin/release" || seeded.NotSeeded != "" {
		t.Fatalf("checked-out upstream was not seeded: %+v %v", seeded, err)
	}
	if got := stewardVerbGit(t, root, "config", "--local", "--get", "metasystem.steward.landing-ref"); got != seeded.Ref {
		t.Fatalf("local landing ref = %q, want %q", got, seeded.Ref)
	}
	stewardVerbGit(t, root, "config", "--local", "metasystem.steward.landing-ref", "refs/remotes/fork/operator-choice")
	seeded, err = seedStewardLandingRef(root)
	if err != nil || seeded.Ref != "" || seeded.NotSeeded != "" {
		t.Fatalf("an existing local value was reseeded: %+v %v", seeded, err)
	}
	if got := stewardVerbGit(t, root, "config", "--local", "--get", "metasystem.steward.landing-ref"); got != "refs/remotes/fork/operator-choice" {
		t.Fatalf("existing operator choice changed to %q", got)
	}
}

func TestHumanStewardVerbDoesNotSeedABranchWithoutAnUpstream(t *testing.T) {
	repo := t.TempDir()
	stub := testgit.New(t,
		stewardSeedConfigRead(repo, testgit.Result{Err: errors.New("exit status 1")}),
		stewardSeedBranchRead(repo, testgit.Result{Stdout: []byte("release\n")}),
		stewardSeedUpstreamRead(repo, testgit.Result{Err: errors.New("exit status 128"), Stdout: []byte("fatal: no upstream configured\n")}),
	)
	seeded, err := seedStewardLandingRefWithGit(repo, stubStewardLandingRefGit{stub})
	want := stewardLandingRefSeed{NotSeeded: "the checked-out branch release has no upstream; automatic machine re-arm remains disabled until the key is configured"}
	if err != nil || seeded != want {
		t.Fatalf("branch without an upstream did not preserve human arming: %+v %v", seeded, err)
	}
}

func TestHumanStewardVerbCannotInventABranchWhileDetached(t *testing.T) {
	repo := t.TempDir()
	stub := testgit.New(t,
		stewardSeedConfigRead(repo, testgit.Result{Err: errors.New("exit status 1")}),
		stewardSeedBranchRead(repo, testgit.Result{Err: errors.New("exit status 1")}),
	)
	seeded, err := seedStewardLandingRefWithGit(repo, stubStewardLandingRefGit{stub})
	want := stewardLandingRefSeed{NotSeeded: "the checkout is detached; automatic machine re-arm remains disabled until the key is configured"}
	if err != nil || seeded != want {
		t.Fatalf("detached checkout did not preserve human arming: %+v %v", seeded, err)
	}
}

func TestStewardLandingRefSeedRefusesInvalidUpstreamAndWriteFailure(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name          string
		upstream      testgit.Result
		write         *testgit.Result
		wantNotSeeded string
		wantError     string
	}{
		{
			name:          "malformed remote-tracking shape",
			upstream:      testgit.Result{Stdout: []byte(" refs/remotes/origin\n")},
			wantNotSeeded: "the checked-out branch release has upstream refs/remotes/origin, not a remote-tracking ref shaped refs/remotes/<remote>/<branch>; automatic machine re-arm remains disabled until the key is configured",
		},
		{
			name:          "nonremote ref",
			upstream:      testgit.Result{Stdout: []byte("refs/heads/release\n")},
			wantNotSeeded: "the checked-out branch release has upstream refs/heads/release, not a remote-tracking ref shaped refs/remotes/<remote>/<branch>; automatic machine re-arm remains disabled until the key is configured",
		},
		{
			name:          "blank upstream output",
			upstream:      testgit.Result{Stdout: []byte(" \n\t")},
			wantNotSeeded: "the checked-out branch release has no upstream; automatic machine re-arm remains disabled until the key is configured",
		},
		{
			name:          "upstream command error",
			upstream:      testgit.Result{Stdout: []byte("refs/remotes/origin/release\n"), Err: errors.New("exit status 128")},
			wantNotSeeded: "the checked-out branch release has no upstream; automatic machine re-arm remains disabled until the key is configured",
		},
		{
			name:      "config-write failure",
			upstream:  testgit.Result{Stdout: []byte(" refs/remotes/origin/release\n")},
			write:     &testgit.Result{Stdout: []byte(" permission denied \n"), Err: errors.New("exit status 1")},
			wantError: "seed metasystem.steward.landing-ref: exit status 1 (permission denied)",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			repo := t.TempDir()
			expected := []testgit.Expectation{
				stewardSeedConfigRead(repo, testgit.Result{Err: errors.New("exit status 1")}),
				stewardSeedBranchRead(repo, testgit.Result{Stdout: []byte("release\n")}),
				stewardSeedUpstreamRead(repo, scenario.upstream),
			}
			if scenario.write != nil {
				expected = append(expected, stewardSeedConfigWrite(repo, "refs/remotes/origin/release", *scenario.write))
			}
			stub := testgit.New(t, expected...)
			seeded, err := seedStewardLandingRefWithGit(repo, stubStewardLandingRefGit{stub})
			if seeded != (stewardLandingRefSeed{NotSeeded: scenario.wantNotSeeded}) {
				t.Fatalf("seed result = %+v, want refusal %q", seeded, scenario.wantNotSeeded)
			}
			if scenario.wantError == "" && err != nil || scenario.wantError != "" && (err == nil || err.Error() != scenario.wantError) {
				t.Fatalf("seed error = %v, want %q", err, scenario.wantError)
			}
		})
	}
}

func TestStewardHookCompleteRejectsNegativeElapsedBeforeRepositoryAccess(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "must-not-exist")
	var stderr bytes.Buffer
	code := completeHookAttempt(hooks.HookCompletion{
		Repo: repo, Generation: "1", Attempt: "1", Result: "ERROR", Outcome: "EMISSION_FAILED",
		ElapsedSec: -1, HasElapsed: true,
	}, &stderr)
	if problem := stderr.String(); code != 2 || !strings.Contains(problem, "--elapsed-sec must be non-negative") {
		t.Fatalf("negative Stop elapsed seconds returned code %d and stderr %q, want a usage refusal", code, problem)
	}
	if _, err := os.Stat(repo); !os.IsNotExist(err) {
		t.Fatalf("negative elapsed validation touched the repository before refusing: %v", err)
	}
}

func TestStewardHookCompleteResolvesTheReportFromTheResponseRecord(t *testing.T) {
	forStopResponseCases(t, func(t *testing.T, runtime string, blocked bool) {
		root, published, attempt, health, payloadPath := stewardHookCompleteFixture(t, runtime, blocked)
		var stderr bytes.Buffer
		if code := completeHookAttempt(stewardHookCompletion(root, attempt, health, payloadPath), &stderr); code != 0 || stderr.Len() != 0 {
			t.Fatalf("response-record settlement returned code=%d stderr=%q", code, stderr.String())
		}

		data, err := os.ReadFile(steward.ComponentEvidencePath(root, "supervision-hook"))
		if err != nil {
			t.Fatal(err)
		}
		var settled steward.ComponentEvidence
		if err := json.Unmarshal(data, &settled); err != nil {
			t.Fatal(err)
		}
		if settled.Result != steward.ComponentOK || settled.Outcome != "EMITTED" {
			t.Fatalf("response-record settlement was not durable: %+v", settled)
		}

		resolved, err := stopreport.ResolveResponse(root, published.Payload, runtime, "command-settlement")
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Response.Report != published.Response.Report {
			t.Fatalf("resolved report reference = %+v, want %+v", resolved.Response.Report, published.Response.Report)
		}
	})
}

func TestStewardHookCompleteRefusesAContradictoryLegacyReportFlag(t *testing.T) {
	forStopResponseCases(t, func(t *testing.T, runtime string, blocked bool) {
		root, _, attempt, health, payloadPath := stewardHookCompleteFixture(t, runtime, blocked)
		request := stewardHookCompletion(root, attempt, health, payloadPath)
		request.ReportAlias = "contradicts-response-record"
		var stderr bytes.Buffer
		if code := completeHookAttempt(request, &stderr); code != 1 || strings.Count(stderr.String(), "\n") != 1 ||
			!strings.Contains(stderr.String(), "alias flag does not match the response record") {
			t.Fatalf("contradictory legacy flag returned code=%d stderr=%q", code, stderr.String())
		}

		data, err := os.ReadFile(steward.ComponentEvidencePath(root, "supervision-hook"))
		if err != nil {
			t.Fatal(err)
		}
		var unsettled steward.ComponentEvidence
		if err := json.Unmarshal(data, &unsettled); err != nil {
			t.Fatal(err)
		}
		if unsettled.Result != steward.ComponentIndeterminate || unsettled.Outcome != "ATTEMPTING" {
			t.Fatalf("contradictory legacy flag changed the attempt: %+v", unsettled)
		}
	})
}

func stewardHookCompleteFixture(t *testing.T, runtime string, blocked bool) (string, stopreporttest.Published, steward.ComponentEvidence, string, string) {
	t.Helper()
	root := stopResponseCommandRoot(t, runtime)
	health := "HEALTH healthy — runner=alive"
	published := stopreporttest.Publish(t, stopreporttest.Options{
		Root: root, Runtime: runtime, Session: "command-settlement", Attempt: strings.Repeat("e", 32),
		ShouldBlock: blocked, HealthLine: health, HumanLine: stopreporttest.ChangedHumanLine,
	})
	now := time.Unix(1, 0).UTC()
	attempt, err := steward.BeginHookAttempt(root, identity.Ref{Pid: 81, StartedAtSec: 101}, "command-settlement", now)
	if err != nil {
		t.Fatal(err)
	}
	payloadPath := filepath.Join(t.TempDir(), "payload.json")
	if err := os.WriteFile(payloadPath, published.Payload, 0o600); err != nil {
		t.Fatal(err)
	}
	return root, published, attempt, health, payloadPath
}

// stewardHookCompletion is the completion the Stop hook hands its owner
// (completeHookAttempt) in process.
func stewardHookCompletion(root string, attempt steward.ComponentEvidence, health, payloadPath string) hooks.HookCompletion {
	return hooks.HookCompletion{
		Repo: root, Generation: strconv.Itoa(attempt.Generation), Attempt: strconv.FormatInt(attempt.AttemptSeq, 10),
		Result: "OK", Outcome: "EMITTED", HealthLine: health, PayloadFile: payloadPath,
	}
}

func TestStewardStatusSurfacesSeatIdleAlertEpisode(t *testing.T) {
	root := t.TempDir()
	incident := steward.SeatIdleIncident{
		SessionID: "seat-session", BacklogDigest: strings.Repeat("a", 64), Refusal: 3,
		StopHookActive: true, ClaimDetail: "claimed", IntentDetail: "prepared",
	}
	if _, err := steward.RecordSeatIdleIncident(root, incident, time.Now()); err != nil {
		t.Fatal(err)
	}
	out, code := captureStdout(t, func(stdout, stderr io.Writer) int { return runStewardStatus([]string{"--repo", root}, stdout, stderr) })
	if code != 0 {
		t.Fatalf("steward status returned %d: %s", code, out)
	}
	var report struct {
		AlertEpisodes []steward.AlertEpisode `json:"alertEpisodes"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.AlertEpisodes) != 1 || report.AlertEpisodes[0].SeatIdle == nil ||
		!report.AlertEpisodes[0].SeatIdle.StopHookActive {
		t.Fatalf("steward status omitted the seat-idle incident: %s", out)
	}
}
