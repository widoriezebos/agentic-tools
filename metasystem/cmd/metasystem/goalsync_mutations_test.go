package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func completeSetObligationArgs(root string) []string {
	return []string{
		"--root", root,
		"--id", "standing-validation",
		"--by", "Wido",
		"--lineage", "m1",
		"--state", "DRAFT",
		"--owner", "Wido",
		"--recurrence", "standing-shared-process",
		"--platform", "darwin/arm64",
		"--toolchain-identity", "go-fixture",
		"--surface-digest", "fixture-surface",
		"--max-active-jobs", "1",
		"--timing-envelope-sec", "60",
		"--effect", "discharge-obligation",
		"--effect", "authorize-spend",
		"--value-judgment", "unknown",
		"--reversibility", "reversible",
		"--severe-harm", "no",
		"--unfamiliar-approach", "no",
		"--test-discrimination", "strong",
		"--correlated-assumption-risk", "no",
		"--authority-scope-change", "no",
		"--destructive-reach", "none",
	}
}

func TestGoalSetObligationTemporaryWordFlagsTravelTogether(t *testing.T) {
	base := completeSetObligationArgs(t.TempDir())
	for _, loneFlag := range []string{"--temporary-human-word", "--review-by"} {
		args := append(append([]string(nil), base...), loneFlag, "present")
		stderr, code := captureStderr(t, func() int { return runGoalSetObligation(args) })
		if code != 2 || !strings.Contains(stderr, "--temporary-human-word and --review-by travel together") {
			t.Fatalf("single temporary flag did not refuse as a pair error: flag=%s code=%d stderr=%q", loneFlag, code, stderr)
		}
	}

	stderr, code := captureStderr(t, func() int {
		return runGoalSetObligation([]string{"--temporary-human-word", "word", "--review-by", "2026-09-06"})
	})
	if code != 2 || !strings.Contains(stderr, "requires identity, recurrence") {
		t.Fatalf("the temporary pair bypassed ordinary obligation requirements: code=%d stderr=%q", code, stderr)
	}

	for _, test := range []struct {
		name     string
		word     string
		reviewBy string
		want     string
	}{
		{name: "whitespace word", word: " \t ", reviewBy: "2026-09-06", want: "non-whitespace"},
		{name: "non-date review", word: "word", reviewBy: "whenever", want: "real date"},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := append(append([]string(nil), base...),
				"--temporary-human-word", test.word, "--review-by", test.reviewBy)
			stderr, code := captureStderr(t, func() int { return runGoalSetObligation(args) })
			if code != 2 || !strings.Contains(stderr, test.want) {
				t.Fatalf("invalid temporary authority was not refused before mutation: code=%d stderr=%q", code, stderr)
			}
		})
	}
}

func TestGoalSetObligationWithoutTemporaryWordStillProvesAncestry(t *testing.T) {
	root := convertedGoalFixture(t)
	stderr, code := captureStderr(t, func() int { return runGoalSetObligation(completeSetObligationArgs(root)) })
	if code != 1 || !strings.Contains(stderr, "could not prove enrolled human ancestry") {
		t.Fatalf("ordinary set-obligation no longer failed closed on missing ancestry: code=%d stderr=%q", code, stderr)
	}
}

func TestGoalSetObligationAnnouncesTemporaryAuthority(t *testing.T) {
	root := convertedGoalFixture(t)
	args := append(completeSetObligationArgs(root),
		"--temporary-human-word", "Wido authorizes this obligation", "--review-by", "2026-09-06")
	stderr, code := captureStderr(t, func() int { return runGoalSetObligation(args) })
	if code != 1 || !strings.Contains(stderr, "TEMPORARY authority under a recorded remote human word") ||
		!strings.Contains(stderr, "re-approval due 2026-09-06 at an agent-free terminal") {
		t.Fatalf("temporary set-obligation did not announce its status: code=%d stderr=%q", code, stderr)
	}
	if strings.Contains(stderr, "could not prove enrolled human ancestry") {
		t.Fatalf("temporary set-obligation still attempted enrolled ancestry: %q", stderr)
	}
}

func TestGoalSetObligationTemporaryPathConfirmsAndRecords(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-08-31T10:00:00Z")
	args := append(completeSetObligationArgs(root),
		"--temporary-human-word", "Wido authorizes this obligation",
		"--review-by", "2026-09-06")
	stdout, stderr, code := captureSetObligationOutput(t, args)
	if code != 0 || !strings.Contains(stdout, `"outcome":"confirmed"`) ||
		!strings.Contains(stderr, "TEMPORARY authority under a recorded remote human word") {
		t.Fatalf("temporary set-obligation did not complete through the CLI: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	tip := goalSyncMutationGit(t, root, "rev-parse", "--verify", goal.AcceptedRef)
	rendered := goalSyncMutationGit(t, root, "cat-file", "-p", tip+":plans/goals/standing-validation.md")
	if !strings.Contains(rendered, "authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06") {
		t.Fatalf("the landed goal record did not enumerate the temporary authority:\n%s", rendered)
	}
	matches, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "authority", "proofs", "*.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("temporary CLI mutation did not record exactly one local proof: matches=%v err=%v", matches, err)
	}
	proofRecord, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(proofRecord), `"temporaryHumanWord": "Wido authorizes this obligation"`) ||
		!strings.Contains(string(proofRecord), `"reviewBy": "2026-09-06"`) {
		t.Fatalf("the local proof record lost the verbatim word or review date: %s", proofRecord)
	}
}

func TestGoalSetObligationDoesNotRecordProofForRejectedMutation(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-08-31T10:00:00Z")
	args := replaceFlagValue(completeSetObligationArgs(root), "--state", "UNKNOWN")
	args = append(args, "--temporary-human-word", "Wido authorizes this obligation", "--review-by", "2026-09-06")
	_, stderr, code := captureSetObligationOutput(t, args)
	if code != 1 || !strings.Contains(stderr, `unknown obligation state "UNKNOWN"`) {
		t.Fatalf("invalid set-obligation did not refuse by state: code=%d stderr=%q", code, stderr)
	}
	matches, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "authority", "proofs", "*.json"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("a rejected set-obligation left an authority proof record: matches=%v err=%v", matches, err)
	}
}

func TestStewardArmTemporaryWordRequiresContentAndDate(t *testing.T) {
	for _, test := range []struct {
		name     string
		word     string
		reviewBy string
		want     string
	}{
		{name: "whitespace word", word: " \t ", reviewBy: "2026-09-06", want: "non-whitespace"},
		{name: "non-date review", word: "Wido authorizes this enrollment", reviewBy: "whenever", want: "real date"},
	} {
		t.Run(test.name, func(t *testing.T) {
			stderr, code := captureStderr(t, func() int {
				return runStewardArm([]string{"--repo", t.TempDir(), "--temporary-human-word", test.word, "--review-by", test.reviewBy})
			})
			if code != 2 || !strings.Contains(stderr, test.want) {
				t.Fatalf("steward arm did not mirror temporary validation: code=%d stderr=%q", code, stderr)
			}
		})
	}
}

func convertedGoalFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plans", "goals"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "backlog.md"), []byte("# Backlog\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func syncedClaimedGoalFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	goalSyncMutationGit(t, root, "init", "-q", "-b", "main")
	goalSyncMutationGit(t, root, "config", "user.name", "goalsync-fixture")
	goalSyncMutationGit(t, root, "config", "user.email", "goalsync-fixture@example.invalid")
	goalSyncMutationGit(t, root, "config", "metasystem.goal.machine", "mac-cli")
	goalSyncMutationGit(t, root, "config", "goal.sync-remote", "local")
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\nmetasystem.governance.correlation-policy=A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	guard := filepath.Join(root, "scripts", "agents", "pre-commit-guard.sh")
	if err := os.MkdirAll(filepath.Dir(guard), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(guard, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	rootRecord := &goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1,
	}
	openedAt := "2026-08-30T08:00:00Z"
	claimAt := "2026-08-30T08:05:00Z"
	file := &goal.GoalFile{
		Id: "standing-validation", State: goal.StateClaimed, Intent: "Govern validation.", Origin: goal.OriginMain,
		NextStep: "Run it.", OpenedAt: openedAt, Revision: 2,
		Budget:  &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 2},
		Claimed: &goal.ClaimRecord{Machine: "mac-cli", Lineage: "m1", At: claimAt, Revision: 2},
		History: []goal.HistoryLine{
			{At: openedAt, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAA", "mac-cli", "m1"), Verb: "open", Actor: "mac-cli+m1", Targets: []string{"standing-validation"}, Keep: -1},
			{At: claimAt, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAB", "mac-cli", "m1"), Verb: "claim", Actor: "mac-cli+m1", Targets: []string{"standing-validation"}, Keep: -1},
		},
	}
	for path, data := range map[string][]byte{
		filepath.Join(root, "plans", "goals", "backlog.md"):             goal.RenderRoot(rootRecord),
		filepath.Join(root, "plans", "goals", "standing-validation.md"): goal.RenderFile(file),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	goalSyncMutationGit(t, root, "add", "metasystem.conf", "plans/goals", "scripts/agents/pre-commit-guard.sh")
	goalSyncMutationGit(t, root, "commit", "-q", "-m", "synced set-obligation fixture")
	goalSyncMutationGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	return root
}

func captureSetObligationOutput(t *testing.T, args []string) (string, string, int) {
	t.Helper()
	var stdout string
	stderr, code := captureStderr(t, func() int {
		var innerCode int
		stdout, innerCode = captureStdout(t, func() int { return runGoalSetObligation(args) })
		return innerCode
	})
	return stdout, stderr, code
}

func replaceFlagValue(args []string, flagName, value string) []string {
	out := append([]string(nil), args...)
	for i := 0; i < len(out)-1; i++ {
		if out[i] == flagName {
			out[i+1] = value
			return out
		}
	}
	return out
}

func goalSyncMutationGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = environWithoutGitSteeringCLI()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
