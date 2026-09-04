package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestGoalClassifySweepEmptyListingInstallsTierLawAndClosesDispatch(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "m1")
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-01T10:00:00Z")
	draft := filepath.Join(t.TempDir(), "classification.txt")
	if err := os.WriteFile(draft, []byte("standing-validation 2 stale draft row\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	preview, previewCode := captureStdout(t, func() int {
		return runGoalClassifySweep([]string{"--root", root, "--draft", draft, "--preview"})
	})
	markerIndex := strings.LastIndex(preview, "listing-digest ")
	if previewCode != 0 || markerIndex < 0 || strings.TrimSpace(preview[:markerIndex]) != "" {
		t.Fatalf("already-tiered ledger did not preview as an empty listing: code=%d output=%q", previewCode, preview)
	}
	digest := strings.TrimSpace(strings.TrimPrefix(preview[markerIndex:], "listing-digest "))
	confirmed, confirmCode := captureStdout(t, func() int {
		return runGoalClassifySweep([]string{"--root", root, "--draft", draft, "--confirm", digest, "--by", "Wido"})
	})
	if confirmCode != 0 || !strings.Contains(confirmed, `"outcome":"confirmed"`) || !strings.Contains(confirmed, `"classified":0`) {
		t.Fatalf("empty classification confirmation failed: code=%d output=%q", confirmCode, confirmed)
	}

	tip := goalSyncMutationGit(t, root, "rev-parse", "--verify", goal.AcceptedRef)
	rootBytes := goalSyncMutationGit(t, root, "cat-file", "-p", tip+":plans/goals/backlog.md")
	rootRecord, problems := goal.ParseRoot([]byte(rootBytes))
	if len(problems) != 0 || rootRecord.TierLaw == "" {
		t.Fatalf("empty confirmation did not install TierLaw: marker=%q problems=%v", rootRecord.TierLaw, problems)
	}

	// A post-law hand edit cannot smuggle a tierless goal into a delegate.
	goalSyncMutationGit(t, root, "reset", "-q", "--hard", goal.AcceptedRef)
	amendSyncedGoalFixture(t, root, "hand-written tierless goal", func(file *goal.GoalFile) {
		file.Tier = 0
		file.Approved = nil
		file.StopCapability = &goal.StopCapability{
			Generation: 1, Revision: file.Claimed.Revision, Machine: file.Claimed.Machine, ClaimEpoch: 1,
		}
	})
	if _, err := dispatchcore.ResolveGoalBinding(root, "standing-validation", time.Date(2026, 9, 1, 10, 1, 0, 0, time.UTC)); err == nil || !strings.Contains(err.Error(), "classify the goal first") {
		t.Fatalf("delegate accepted a hand-written tierless goal after TierLaw: %v", err)
	}
}

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

func completeResumeArgs(root string) []string {
	return []string{
		"--root", root,
		"--id", "standing-validation",
		"--by", "Wido",
		"--lineage", "m1",
		"--elapsed-limit", "4h",
		"--attempt-limit", "4",
		"--reserved-job-minutes-limit", "240",
		"--active-job-limit", "2",
		"--review-round-limit", "3",
	}
}

func TestGoalSetObligationTemporaryWordFlagsTravelTogether(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-01T10:00:00Z")
	base := completeSetObligationArgs(root)
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
		{name: "short word", word: "Wido authorizes", reviewBy: "2026-09-06", want: "at least three words"},
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

func TestGoalTemporaryAuthorityRefusesPastAndBeyondHorizon(t *testing.T) {
	for _, test := range []struct {
		name     string
		reviewBy string
		want     string
	}{
		{name: "past", reviewBy: "2026-08-31", want: "--review-by 2026-08-31 is in the past"},
		{name: "beyond horizon", reviewBy: "2026-09-07", want: "--review-by 2026-09-07 exceeds temporary goal authority horizon 2026-09-06"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := syncedClaimedGoalFixture(t)
			t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-01T10:00:00Z")
			args := append(completeSetObligationArgs(root),
				"--temporary-human-word", "Wido authorizes this obligation", "--review-by", test.reviewBy)
			stderr, code := captureStderr(t, func() int { return runGoalSetObligation(args) })
			if code != 2 || !strings.Contains(stderr, test.want) {
				t.Fatalf("temporary authority date was not refused exactly: code=%d stderr=%q", code, stderr)
			}
		})
	}
}

func TestGoalTemporaryAuthorityIgnoresFixtureClock(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	wallDate := time.Now().UTC().AddDate(0, 0, -1)
	horizon, err := time.Parse("2006-01-02", "2026-09-06")
	if err != nil {
		t.Fatal(err)
	}
	if wallDate.After(horizon) {
		wallDate = horizon
	}
	reviewBy := wallDate.Format("2006-01-02")
	t.Setenv("METASYSTEM_GOAL_NOW", wallDate.AddDate(0, 0, -1).Format(time.RFC3339))
	args := append(completeSetObligationArgs(root),
		"--temporary-human-word", "Wido authorizes stale obligation", "--review-by", reviewBy)
	stderr, code := captureStderr(t, func() int { return runGoalSetObligation(args) })
	want := "--review-by " + reviewBy + " is in the past"
	if code != 2 || !strings.Contains(stderr, want) {
		t.Fatalf("fixture clock moved a temporary authority decision: code=%d stderr=%q want=%q", code, stderr, want)
	}
}

func TestGoalResumeTemporaryPathConfirmsAndRecordsWords(t *testing.T) {
	root := syncedStoppedGoalFixture(t)
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-01T10:00:00Z")
	word := "Wido authorizes this goal resume"
	args := append(completeResumeArgs(root),
		"--temporary-human-word", word, "--review-by", "2026-09-06")
	stdout, stderr, code := captureResumeOutputWithAuthority(t, args, fixedTemporaryGoalAuthority)
	if code != 0 || !strings.Contains(stdout, `"outcome":"confirmed"`) ||
		!strings.Contains(stdout, "goal resume: TEMPORARY authority under a recorded relayed word (human provenance not verified); re-approval due 2026-09-06 at an agent-free terminal") {
		t.Fatalf("temporary resume did not confirm and announce: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	tip := goalSyncMutationGit(t, root, "rev-parse", "--verify", goal.AcceptedRef)
	rendered := goalSyncMutationGit(t, root, "cat-file", "-p", tip+":plans/goals/standing-validation.md")
	want := `resume actor=human:Wido targets=standing-validation authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="Wido authorizes this goal resume"`
	if !strings.Contains(rendered, want) {
		t.Fatalf("the resume history did not retain its complete temporary authority:\n%s", rendered)
	}
	matches, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "authority", "proofs", "*.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("temporary resume did not record exactly one local proof: matches=%v err=%v", matches, err)
	}
	proofRecord, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(proofRecord), `"action": "goal resume"`) ||
		!strings.Contains(string(proofRecord), `"temporaryHumanWord": "Wido authorizes this goal resume"`) ||
		!strings.Contains(string(proofRecord), `"departure": "R-32-m1"`) {
		t.Fatalf("the resume proof lost its action, words, or ruling: %s", proofRecord)
	}
}

func TestGoalApproveAndUnapproveTemporarySurfacesRecordProofs(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-01T10:00:00Z")
	approveArgs := []string{
		"--root", root, "--id", "standing-validation", "--by", "Wido", "--lineage", "m1",
		"--temporary-human-word", "Wido authorizes this goal approval", "--review-by", "2026-09-06",
	}
	var approveOut string
	approveErr, approveCode := captureStderr(t, func() int {
		var code int
		approveOut, code = captureStdout(t, func() int { return runGoalApproveWithAuthority(approveArgs, fixedTemporaryGoalAuthority) })
		return code
	})
	if approveCode != 0 || !strings.Contains(approveOut, `"outcome":"confirmed"`) || !strings.Contains(approveOut, "TEMPORARY authority") {
		t.Fatalf("goal approve temporary surface failed: code=%d stdout=%q stderr=%q", approveCode, approveOut, approveErr)
	}

	tip := goalSyncMutationGit(t, root, "rev-parse", "--verify", goal.AcceptedRef)
	rendered := goalSyncMutationGit(t, root, "cat-file", "-p", tip+":plans/goals/standing-validation.md")
	if !strings.Contains(rendered, "authority=relayed") || !strings.Contains(rendered, "reviewBy=2026-09-06") {
		t.Fatalf("goal approve did not publish its relayed record:\n%s", rendered)
	}

	unapproveArgs := []string{
		"--root", root, "--id", "standing-validation", "--by", "Wido", "--lineage", "m1", "--because", "human withdrew execution",
		"--temporary-human-word", "Wido withdraws this goal approval", "--review-by", "2026-09-06",
	}
	var unapproveOut string
	unapproveErr, unapproveCode := captureStderr(t, func() int {
		var code int
		unapproveOut, code = captureStdout(t, func() int { return runGoalUnapproveWithAuthority(unapproveArgs, fixedTemporaryGoalAuthority) })
		return code
	})
	if unapproveCode != 0 || !strings.Contains(unapproveOut, `"outcome":"confirmed"`) {
		t.Fatalf("goal unapprove temporary surface failed: code=%d stdout=%q stderr=%q", unapproveCode, unapproveOut, unapproveErr)
	}
	tip = goalSyncMutationGit(t, root, "rev-parse", "--verify", goal.AcceptedRef)
	rendered = goalSyncMutationGit(t, root, "cat-file", "-p", tip+":plans/goals/standing-validation.md")
	if !strings.Contains(rendered, "State: parked") || strings.Contains(rendered, "- Approved:") || strings.Contains(rendered, "- Budget:") {
		t.Fatalf("goal unapprove did not park the claim and clear execution authority:\n%s", rendered)
	}

	matches, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "authority", "proofs", "*.json"))
	if err != nil || len(matches) != 2 {
		t.Fatalf("approve and unapprove did not record two proofs: matches=%v err=%v", matches, err)
	}
	var actions string
	for _, match := range matches {
		data, readErr := os.ReadFile(match)
		if readErr != nil {
			t.Fatal(readErr)
		}
		actions += string(data)
	}
	if !strings.Contains(actions, `"action": "goal approve"`) || !strings.Contains(actions, `"action": "goal unapprove"`) {
		t.Fatalf("authority proofs were not action-bound: %s", actions)
	}
}

func TestGoalSecondRelayedResumeRefusesWithFirstAct(t *testing.T) {
	root := syncedStoppedGoalFixture(t)
	amendSyncedGoalFixture(t, root, "land first relayed resume marker", func(file *goal.GoalFile) {
		file.Revision++
		file.History = append(file.History, goal.HistoryLine{
			At: "2026-09-01T09:30:00Z", Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAD", "mac-cli", "m1"),
			Verb: "resume", Actor: "human:Wido", Targets: []string{"standing-validation"}, Keep: -1,
			AuthorityOutcome: goal.AuthorityOutcomeTemporaryHumanWord, AuthorityReviewBy: "2026-09-06",
			AuthorityRuling: goal.TemporaryGoalAuthorityRuling, TemporaryHumanWord: "Wido authorizes first resume",
		})
	})
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-01T10:00:00Z")
	args := append(completeResumeArgs(root),
		"--temporary-human-word", "Wido authorizes second resume", "--review-by", "2026-09-06")
	stdout, stderr, code := captureResumeOutputWithAuthority(t, args, fixedTemporaryGoalAuthority)
	want := `goal standing-validation already used relayed resume authority on 2026-09-01T09:30:00Z with recorded word \"Wido authorizes first resume\"; a further resume needs freshly observed enrolled-terminal authority`
	if code != 1 || !strings.Contains(stdout, want) || strings.Contains(stdout, "TEMPORARY authority") {
		t.Fatalf("second relayed resume refusal mismatch: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestGoalResumeTemporaryWordFlagsTravelTogether(t *testing.T) {
	for _, loneFlag := range []string{"--temporary-human-word", "--review-by"} {
		root := syncedStoppedGoalFixture(t)
		t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-01T10:00:00Z")
		args := append(completeResumeArgs(root), loneFlag, "present")
		stderr, code := captureStderr(t, func() int { return runGoalResume(args) })
		if code != 2 || !strings.Contains(stderr, "--temporary-human-word and --review-by travel together") {
			t.Fatalf("resume accepted one temporary flag: flag=%s code=%d stderr=%q", loneFlag, code, stderr)
		}
	}
}

func TestGoalMigrateStillRefusesTemporaryHumanWord(t *testing.T) {
	stderr, code := captureStderr(t, func() int {
		return runGoalMigrate([]string{"--temporary-human-word", "Wido authorizes this migration"})
	})
	if code != 2 || !strings.Contains(stderr, "flag provided but not defined: -temporary-human-word") {
		t.Fatalf("goal migrate unexpectedly accepted the relay flag: code=%d stderr=%q", code, stderr)
	}
}

func TestGoalSetObligationWithoutTemporaryWordStillProvesAncestry(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-01T10:00:00Z")
	stderr, code := captureStderr(t, func() int { return runGoalSetObligation(completeSetObligationArgs(root)) })
	if code != 1 || !strings.Contains(stderr, "could not prove enrolled human ancestry") {
		t.Fatalf("ordinary set-obligation no longer failed closed on missing ancestry: code=%d stderr=%q", code, stderr)
	}
}

func TestGoalSetObligationAnnouncesTemporaryAuthority(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-01T10:00:00Z")
	args := append(completeSetObligationArgs(root),
		"--temporary-human-word", "Wido authorizes this obligation", "--review-by", "2026-09-06")
	stdout, stderr, code := captureSetObligationOutputWithAuthority(t, args, fixedTemporaryGoalAuthority)
	if code != 0 || !strings.Contains(stdout, "TEMPORARY authority under a recorded relayed word (human provenance not verified)") ||
		!strings.Contains(stdout, "re-approval due 2026-09-06 at an agent-free terminal") {
		t.Fatalf("temporary set-obligation did not announce its status: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if strings.Contains(stderr, "could not prove enrolled human ancestry") {
		t.Fatalf("temporary set-obligation still attempted enrolled ancestry: %q", stderr)
	}
}

func TestGoalSetObligationTemporaryPathConfirmsAndRecords(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-01T10:00:00Z")
	args := append(completeSetObligationArgs(root),
		"--temporary-human-word", "Wido authorizes this obligation",
		"--review-by", "2026-09-06")
	stdout, stderr, code := captureSetObligationOutputWithAuthority(t, args, fixedTemporaryGoalAuthority)
	if code != 0 || !strings.Contains(stdout, `"outcome":"confirmed"`) ||
		!strings.Contains(stdout, "TEMPORARY authority under a recorded relayed word (human provenance not verified)") {
		t.Fatalf("temporary set-obligation did not complete through the CLI: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	tip := goalSyncMutationGit(t, root, "rev-parse", "--verify", goal.AcceptedRef)
	rendered := goalSyncMutationGit(t, root, "cat-file", "-p", tip+":plans/goals/standing-validation.md")
	if !strings.Contains(rendered, `authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="Wido authorizes this obligation"`) {
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
		!strings.Contains(string(proofRecord), `"reviewBy": "2026-09-06"`) ||
		!strings.Contains(string(proofRecord), `"departure": "R-32-m1"`) {
		t.Fatalf("the local proof record lost the verbatim word or review date: %s", proofRecord)
	}
}

func TestGoalSecondRelayedSetObligationRefusesWithFirstAct(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-01T10:00:00Z")
	firstArgs := append(completeSetObligationArgs(root),
		"--temporary-human-word", "Wido authorizes first obligation", "--review-by", "2026-09-06")
	if stdout, stderr, code := captureSetObligationOutputWithAuthority(t, firstArgs, fixedTemporaryGoalAuthority); code != 0 {
		t.Fatalf("first relayed set-obligation did not confirm: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	secondArgs := append(completeSetObligationArgs(root),
		"--temporary-human-word", "Wido authorizes second obligation", "--review-by", "2026-09-06")
	stdout, stderr, code := captureSetObligationOutputWithAuthority(t, secondArgs, fixedTemporaryGoalAuthority)
	want := `goal standing-validation already used relayed set-obligation authority on 2026-09-01T10:00:00Z with recorded word \"Wido authorizes first obligation\"; a further set-obligation needs freshly observed enrolled-terminal authority`
	if code != 1 || !strings.Contains(stdout, want) || strings.Contains(stdout, "TEMPORARY authority") {
		t.Fatalf("second relayed set-obligation refusal mismatch: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestForeignLandedAuthorityKeepsGoalTreeUsable(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-01T10:00:00Z")
	amendSyncedGoalFixture(t, root, "land renewed authority marker", func(file *goal.GoalFile) {
		file.Revision++
		file.History = append(file.History, goal.HistoryLine{
			At: "2026-09-07T10:00:00Z", Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAE", "mac-cli", "m1"),
			Verb: "set-obligation", Actor: "human:Wido", Targets: []string{"standing-validation"}, Keep: -1,
			AuthorityOutcome: goal.AuthorityOutcomeTemporaryHumanWord, AuthorityReviewBy: "2026-09-07",
			AuthorityRuling: "R-33-m1", TemporaryHumanWord: "Wido renews this obligation",
		})
		file.Obligation = &goal.GovernedObligation{
			Revision: file.Revision, BudgetRevision: file.Claimed.Revision, State: goal.ObligationDraft, Owner: "Wido",
			AuthorityOutcome: goal.AuthorityOutcomeTemporaryHumanWord, AuthorityReviewBy: "2026-09-07",
			AuthorityRuling: "R-33-m1", TemporaryHumanWord: "Wido renews this obligation",
			Effects: []goal.GoverningEffect{goal.EffectAuthorizeSpend},
			Assumptions: goal.ObligationAssumptions{
				Recurrence: goal.SingleExperiment, Platform: "darwin/arm64", ToolchainIdentity: "go-fixture",
				SurfaceDigest: "fixture-surface", MaxActiveJobs: 1, TimingEnvelopeSeconds: 60,
				ObservationSource: "run-terminal-record",
			},
			Triggers: goal.HumanReviewTriggers{
				ValueJudgment: "unknown", Reversibility: "reversible", SevereHarm: "no", UnfamiliarApproach: "no",
				TestDiscrimination: "strong", CorrelatedAssumptionRisk: "no", AuthorityScopeChange: "no", DestructiveReach: "none",
			},
		}
	})

	for name, run := range map[string]func() int{
		"list":  func() int { return runGoalList([]string{"--root", root}) },
		"show":  func() int { return runGoalShow([]string{"--root", root, "--id", "standing-validation"}) },
		"next":  func() int { return runGoalNext([]string{"--root", root}) },
		"fetch": func() int { return runGoalFetch([]string{"--root", root}) },
	} {
		t.Run(name, func(t *testing.T) {
			stderr, code := captureStderr(t, run)
			if code != 0 {
				t.Fatalf("goal %s refused the renewed landed authority fact: code=%d stderr=%q", name, code, stderr)
			}
		})
	}

	mutationArgs := append(completeSetObligationArgs(root),
		"--temporary-human-word", "Wido authorizes current obligation", "--review-by", "2026-09-06")
	if stdout, stderr, code := captureSetObligationOutputWithAuthority(t, mutationArgs, fixedTemporaryGoalAuthority); code != 0 {
		t.Fatalf("a goal mutation could not use the tree after renewal: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestGoalSetObligationDoesNotRecordProofForRejectedMutation(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-01T10:00:00Z")
	args := replaceFlagValue(completeSetObligationArgs(root), "--state", "UNKNOWN")
	args = append(args, "--temporary-human-word", "Wido authorizes this obligation", "--review-by", "2026-09-06")
	_, stderr, code := captureSetObligationOutputWithAuthority(t, args, fixedTemporaryGoalAuthority)
	if code != 1 || !strings.Contains(stderr, `unknown obligation state "UNKNOWN"`) {
		t.Fatalf("invalid set-obligation did not refuse by state: code=%d stderr=%q", code, stderr)
	}
	matches, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "authority", "proofs", "*.json"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("a rejected set-obligation left an authority proof record: matches=%v err=%v", matches, err)
	}
}

func twoMachineEnrollmentFixture(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	goalSyncMutationGit(t, base, "init", "-q", "--bare", "-b", "main", origin)
	seed := filepath.Join(base, "seed")
	goalSyncMutationGit(t, base, "clone", "-q", origin, seed)
	goalSyncMutationGit(t, seed, "config", "user.name", "enrollment-fixture")
	goalSyncMutationGit(t, seed, "config", "user.email", "enrollment-fixture@example.invalid")

	approvalAt := "2026-09-01T08:00:00Z"
	approvalOpid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAX", "mac-a", "enrollment-fixture")
	budget := goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 2, ReviewRoundLimit: 3}
	rootRecord := &goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncRemote,
		MigrationEpoch: "2026-09-01T07:00:00Z", ManifestDigest: strings.Repeat("ab", 32), MigrationMode: "manifest", Revision: 2,
		ApprovalGate: &goal.ApprovalGateRecord{Since: approvalAt, Opid: approvalOpid},
		History: []goal.HistoryLine{{
			At: "2026-09-01T07:00:00Z", Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAW", "mac-a", "enrollment-fixture"),
			Verb: "migrate", Actor: "mac-a+enrollment-fixture", Keep: -1,
		}},
	}
	file := &goal.GoalFile{
		Id: "relayed-waiting", State: goal.StateApproved, Tier: 3, Intent: "Run only while relayed authority remains valid.",
		Origin: goal.OriginMain, NextStep: "Claim it.", OpenedAt: "2026-09-01T07:30:00Z", Revision: 2, Budget: &budget,
		Approved: &goal.ApprovalRecord{
			By: "human:Wido", At: approvalAt, Revision: 2, Opid: approvalOpid,
			Authority: goal.ApprovalAuthorityRelayed, Digest: goal.ApprovalDigest("Run only while relayed authority remains valid.", 3, budget), ReviewBy: "2026-09-06",
		},
		History: []goal.HistoryLine{
			{At: "2026-09-01T07:30:00Z", Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAY", "mac-a", "enrollment-fixture"), Verb: "open", Actor: "mac-a+enrollment-fixture", Targets: []string{"relayed-waiting"}, Keep: -1},
			{At: approvalAt, Opid: approvalOpid, Verb: "approve", Actor: "human:Wido", Targets: []string{"relayed-waiting"}, Keep: -1,
				AuthorityOutcome: goal.AuthorityOutcomeTemporaryHumanWord, AuthorityReviewBy: "2026-09-06", AuthorityRuling: goal.TemporaryGoalAuthorityRuling,
				TemporaryHumanWord: "Wido authorizes this relayed goal"},
		},
	}
	for path, data := range map[string][]byte{
		filepath.Join(seed, "metasystem.conf"):                          []byte("metasystem.runtimes=fake\n"),
		filepath.Join(seed, "plans", "goals", "backlog.md"):             goal.RenderRoot(rootRecord),
		filepath.Join(seed, "plans", "goals", "relayed-waiting.md"):     goal.RenderFile(file),
		filepath.Join(seed, "scripts", "agents", "pre-commit-guard.sh"): []byte("#!/bin/sh\nexit 0\n"),
		filepath.Join(seed, "scripts", "agents", "adapters", "fake.sh"): []byte("#!/bin/sh\n[ \"$1\" = signature ] && printf '%s\\n' 'match never-an-attended-human-shell'\n"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(path, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(path, data, mode); err != nil {
			t.Fatal(err)
		}
	}
	goalSyncMutationGit(t, seed, "add", "metasystem.conf", "plans/goals", "scripts/agents")
	goalSyncMutationGit(t, seed, "commit", "-q", "-m", "seed fleet enrollment fixture")
	goalSyncMutationGit(t, seed, "push", "-q", "origin", "main")

	clones := []string{filepath.Join(base, "machine-a"), filepath.Join(base, "machine-b")}
	for index, clone := range clones {
		goalSyncMutationGit(t, base, "clone", "-q", origin, clone)
		goalSyncMutationGit(t, clone, "config", "user.name", "enrollment-fixture")
		goalSyncMutationGit(t, clone, "config", "user.email", "enrollment-fixture@example.invalid")
		goalSyncMutationGit(t, clone, "config", "metasystem.goal.machine", []string{"mac-a", "mac-b"}[index])
		goalSyncMutationGit(t, clone, "config", "goal.sync-remote", "origin")
		goalSyncMutationGit(t, clone, "config", "goal.sync-branch", "refs/heads/main")
		goalSyncMutationGit(t, clone, "update-ref", goal.AcceptedRef, "origin/main")
	}
	return clones[0], clones[1]
}

func captureEnrollTerminalOutput(t *testing.T, args []string, enroll goalTerminalEnroller) (string, string, int) {
	t.Helper()
	var stdout string
	stderr, code := captureStderr(t, func() int {
		var innerCode int
		stdout, innerCode = captureStdout(t, func() int { return runGoalEnrollTerminalWith(args, enroll) })
		return innerCode
	})
	return stdout, stderr, code
}

func TestGoalEnrollTerminalSucceedsOnEveryMachineAndFirstEndsRelay(t *testing.T) {
	machineA, machineB := twoMachineEnrollmentFixture(t)
	firstAt := time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)
	secondAt := firstAt.Add(time.Hour)
	times := map[string]time.Time{machineA: firstAt, machineB: secondAt}
	enroll := func(root string, _ int64, _ humanauthority.Reader, _ time.Time) (humanauthority.Enrollment, error) {
		exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
		if err != nil || state != identity.Alive {
			return humanauthority.Enrollment{}, fmt.Errorf("probe fixture process: %s: %w", state, err)
		}
		reader := sessionStopCommandAuthorityReader{pid: exact.Pid, exact: exact}
		return humanauthority.Enroll(root, exact.Pid, reader, times[root])
	}

	stdoutA, stderrA, codeA := captureEnrollTerminalOutput(t, []string{"--root", machineA, "--lineage", "enrollment-fixture"}, enroll)
	if codeA != 0 || stderrA != "" {
		t.Fatalf("first machine enrollment failed: code=%d stdout=%q stderr=%q", codeA, stdoutA, stderrA)
	}
	var printedA humanauthority.Enrollment
	if err := json.Unmarshal([]byte(stdoutA), &printedA); err != nil || printedA.EnrolledAt != firstAt || printedA.Generation != 1 {
		t.Fatalf("first machine did not print its local enrollment: enrollment=%+v err=%v stdout=%q", printedA, err, stdoutA)
	}

	endpointB, err := goal.ResolveEndpoint(machineB)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := goal.FetchAdvance(endpointB); err != nil {
		t.Fatalf("second machine did not observe the first fleet cutoff: %v", err)
	}
	claim, err := goal.Claim(goal.VerbRequest{
		Endpoint: endpointB, Actor: goal.Actor{Machine: "mac-b", Lineage: "enrollment-fixture"},
		Ulid: "01ARZ3NDEKTSV4RRFFQ69G5FAZ", Now: firstAt.Add(time.Minute), ClaimEpoch: 1,
	}, "relayed-waiting")
	if err != nil || claim.Outcome != goal.OutcomeRejected || !strings.Contains(claim.Detail, "APPROVAL_EXPIRED") || !strings.Contains(claim.Detail, "fleet's first terminal") {
		t.Fatalf("the first machine's enrollment did not end relayed approval fleet-wide: result=%+v err=%v", claim, err)
	}

	stdoutB, stderrB, codeB := captureEnrollTerminalOutput(t, []string{"--root", machineB, "--lineage", "enrollment-fixture"}, enroll)
	if codeB != 0 || stderrB != "" {
		t.Fatalf("second machine enrollment failed: code=%d stdout=%q stderr=%q", codeB, stdoutB, stderrB)
	}
	var printedB humanauthority.Enrollment
	if err := json.Unmarshal([]byte(stdoutB), &printedB); err != nil || printedB.EnrolledAt != secondAt || printedB.Generation != 1 {
		t.Fatalf("second machine did not print its local enrollment: enrollment=%+v err=%v stdout=%q", printedB, err, stdoutB)
	}
	for root, want := range map[string]time.Time{machineA: firstAt, machineB: secondAt} {
		local, err := humanauthority.ReadEnrollment(root)
		if err != nil || local.EnrolledAt != want || local.Generation != 1 {
			t.Fatalf("machine %s local enrollment=%+v, want at=%s generation=1: %v", root, local, want, err)
		}
	}
	projection, err := goal.Project(endpointB, false, secondAt)
	if err != nil {
		t.Fatal(err)
	}
	cutoff := projection.Tree.Root.FleetEnrollment
	if cutoff == nil || cutoff.Machine != "mac-a" || cutoff.At != firstAt.Format(time.RFC3339) {
		t.Fatalf("the later local enrollment replaced the fleet's first cutoff: %+v", cutoff)
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
	approvedAt := "2026-08-30T08:06:00Z"
	budget := &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 2, ReviewRoundLimit: 3}
	approvalOpid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAZ", "mac-cli", "m1")
	file := &goal.GoalFile{
		Id: "standing-validation", State: goal.StateClaimed, Tier: 3, Intent: "Govern validation.", Origin: goal.OriginMain,
		NextStep: "Run it.", OpenedAt: openedAt, Revision: 3,
		Budget:  budget,
		Claimed: &goal.ClaimRecord{Machine: "mac-cli", Lineage: "m1", At: claimAt, Revision: 2},
		Approved: &goal.ApprovalRecord{
			By: "human:Wido", At: approvedAt, Revision: 3, Opid: approvalOpid,
			Authority: goal.ApprovalAuthorityProven, Digest: goal.ApprovalDigest("Govern validation.", 3, *budget),
		},
		History: []goal.HistoryLine{
			{At: openedAt, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAA", "mac-cli", "m1"), Verb: "open", Actor: "mac-cli+m1", Targets: []string{"standing-validation"}, Keep: -1},
			{At: claimAt, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAB", "mac-cli", "m1"), Verb: "claim", Actor: "mac-cli+m1", Targets: []string{"standing-validation"}, Keep: -1},
			{At: approvedAt, Opid: approvalOpid, Verb: "approve", Actor: "human:Wido", Targets: []string{"standing-validation"}, Keep: -1},
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

func syncedStoppedGoalFixture(t *testing.T) string {
	t.Helper()
	root := syncedClaimedGoalFixture(t)
	path := filepath.Join(root, "plans", "goals", "standing-validation.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("claimed fixture did not parse before stop setup: %v", problems)
	}
	closedAt := "2026-09-01T09:00:00Z"
	stopID := "stop-standing-validation-r2-f1"
	file.Revision++
	file.StopCapability = &goal.StopCapability{
		Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1, FenceEpoch: 1,
	}
	file.StopFence = &goal.StopFence{
		StopID: stopID, Revision: 2, Epoch: 1, CapabilityGeneration: 2,
		ClosedAt: closedAt, Reason: goal.StopReasonElapsedLimit,
	}
	file.History = append(file.History, goal.HistoryLine{
		At: closedAt, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAC", "mac-cli", "m1"),
		Verb: "breach-stop", Actor: "mac-cli+m1", Targets: []string{"standing-validation"}, Keep: -1,
	})
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, root, "add", "plans/goals/standing-validation.md")
	goalSyncMutationGit(t, root, "commit", "-q", "-m", "breach-stopped resume fixture")
	goalSyncMutationGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	batch := goal.StopBatch{
		StopID: stopID, GoalID: "standing-validation", GoalRevision: 2,
		FenceEpoch: 1, CapabilityGeneration: 2, Machine: "mac-cli", ClaimEpoch: 1,
		Reason: goal.StopReasonElapsedLimit, State: goal.StopBatchComplete,
		OpenedAt: closedAt, UpdatedAt: closedAt, CompletedAt: closedAt, Pass: 1,
	}
	if err := goal.WriteStopBatch(root, batch); err != nil {
		t.Fatal(err)
	}
	return root
}

func amendSyncedGoalFixture(t *testing.T, root, message string, mutate func(*goal.GoalFile)) {
	t.Helper()
	path := filepath.Join(root, "plans", "goals", "standing-validation.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("goal fixture did not parse before amendment: %v", problems)
	}
	mutate(file)
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, root, "add", "plans/goals/standing-validation.md")
	goalSyncMutationGit(t, root, "commit", "-q", "-m", message)
	goalSyncMutationGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
}

func captureSetObligationOutputWithAuthority(t *testing.T, args []string, prove goalAuthorityProver) (string, string, int) {
	t.Helper()
	var stdout string
	stderr, code := captureStderr(t, func() int {
		var innerCode int
		stdout, innerCode = captureStdout(t, func() int { return runGoalSetObligationWithAuthority(args, prove) })
		return innerCode
	})
	return stdout, stderr, code
}

func captureResumeOutputWithAuthority(t *testing.T, args []string, prove goalAuthorityProver) (string, string, int) {
	t.Helper()
	var stdout string
	stderr, code := captureStderr(t, func() int {
		var innerCode int
		stdout, innerCode = captureStdout(t, func() int { return runGoalResumeWithAuthority(args, prove) })
		return innerCode
	})
	return stdout, stderr, code
}

// fixedTemporaryGoalAuthority is compiled only into this package's test
// binary. It supplies an already-granted proof to the direct worker seam so
// historical/transaction tests remain meaningful after R-32-m1 expires;
// shipped CLI entrypoints cannot select it.
func fixedTemporaryGoalAuthority(root string, _ int64, _ humanauthority.Reader, word, reviewBy string, _ time.Time) (humanauthority.Proof, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return humanauthority.Proof{}, err
	}
	proof := humanauthority.Proof{
		Schema: 1, CheckedAt: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
		Outcome: humanauthority.OutcomeTemporary, TemporaryHumanWord: word,
		ReviewBy: reviewBy, Departure: humanauthority.TemporaryWordRuling,
	}
	value := reflect.ValueOf(&proof).Elem()
	setPrivateAuthorityTestField(value.FieldByName("observedRoot"), reflect.ValueOf(filepath.Clean(abs)))
	setPrivateAuthorityTestField(value.FieldByName("observed"), reflect.ValueOf(true))
	return proof, nil
}

func setPrivateAuthorityTestField(field, value reflect.Value) {
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(value)
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
