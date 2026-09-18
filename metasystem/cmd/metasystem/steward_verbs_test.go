package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport/stopreporttest"
)

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
	root := t.TempDir()
	stewardVerbGit(t, root, "init", "-q", "-b", "release")
	seeded, err := seedStewardLandingRef(root)
	if err != nil || seeded.Ref != "" || !strings.Contains(seeded.NotSeeded, "branch release has no upstream") {
		t.Fatalf("branch without an upstream did not preserve human arming: %+v %v", seeded, err)
	}
	if _, err := exec.Command("git", "-C", root, "config", "--local", "--get", "metasystem.steward.landing-ref").Output(); err == nil {
		t.Fatal("branch without an upstream invented a landing ref")
	}
}

func TestHumanStewardVerbCannotInventABranchWhileDetached(t *testing.T) {
	root := t.TempDir()
	stewardVerbGit(t, root, "init", "-q", "-b", "release")
	stewardVerbGit(t, root, "config", "user.name", "fixture")
	stewardVerbGit(t, root, "config", "user.email", "fixture@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "README"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stewardVerbGit(t, root, "add", "README")
	stewardVerbGit(t, root, "commit", "-qm", "fixture")
	stewardVerbGit(t, root, "checkout", "--detach", "-q")
	seeded, err := seedStewardLandingRef(root)
	if err != nil || seeded.Ref != "" || !strings.Contains(seeded.NotSeeded, "checkout is detached") {
		t.Fatalf("detached checkout did not preserve human arming: %+v %v", seeded, err)
	}
	if _, err := exec.Command("git", "-C", root, "config", "--local", "--get", "metasystem.steward.landing-ref").Output(); err == nil {
		t.Fatal("detached checkout invented a landing ref")
	}
}

func TestStewardHookCompleteRejectsNegativeElapsedBeforeRepositoryAccess(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "must-not-exist")
	_, problem, code := captureRelay(t, func() int {
		return runStewardHookComplete([]string{
			"--repo", repo,
			"--generation", "1",
			"--attempt", "1",
			"--result", "ERROR",
			"--outcome", "EMISSION_FAILED",
			"--elapsed-sec", "-1",
		})
	})
	if code != 2 || !strings.Contains(problem, "--elapsed-sec must be non-negative") {
		t.Fatalf("negative Stop elapsed seconds returned code %d and stderr %q, want a usage refusal", code, problem)
	}
	if _, err := os.Stat(repo); !os.IsNotExist(err) {
		t.Fatalf("negative elapsed validation touched the repository before refusing: %v", err)
	}
}

func TestStewardHookCompleteResolvesTheReportFromTheResponseRecord(t *testing.T) {
	forStopResponseCases(t, func(t *testing.T, runtime string, blocked bool) {
		root, published, attempt, health, payloadPath := stewardHookCompleteFixture(t, runtime, blocked)
		code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
			return runStewardHookComplete(stewardHookCompleteArgs(root, attempt, health, payloadPath))
		})
		if code != 0 || stdout != "" || stderr != "" {
			t.Fatalf("response-record settlement returned code=%d stdout=%q stderr=%q", code, stdout, stderr)
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
		args := append(stewardHookCompleteArgs(root, attempt, health, payloadPath), "--report-alias", "contradicts-response-record")
		code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
			return runStewardHookComplete(args)
		})
		if code != 1 || stdout != "" || strings.Count(stderr, "\n") != 1 ||
			!strings.Contains(stderr, "alias flag does not match the response record") {
			t.Fatalf("contradictory legacy flag returned code=%d stdout=%q stderr=%q", code, stdout, stderr)
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

func stewardHookCompleteArgs(root string, attempt steward.ComponentEvidence, health, payloadPath string) []string {
	return []string{
		"--repo", root,
		"--generation", strconv.Itoa(attempt.Generation),
		"--attempt", strconv.FormatInt(attempt.AttemptSeq, 10),
		"--result", "OK",
		"--outcome", "EMITTED",
		"--health-line", health,
		"--payload-file", payloadPath,
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
	out, code := captureStdout(t, func() int { return runStewardStatus([]string{"--repo", root}) })
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
