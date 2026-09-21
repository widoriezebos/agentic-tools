package goal

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// validatedBed builds a converted single-machine checkout whose one commit
// carries exactly the given ledger bytes, committed at a fixed instant so a
// reported commit time is an assertion rather than a coincidence.
func validatedBed(t *testing.T, committedAt string, rootRecord *RootRecord, files map[string][]byte) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = testEnvironment(os.Environ(),
			"GIT_AUTHOR_DATE="+committedAt, "GIT_COMMITTER_DATE="+committedAt)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "validated-fixture")
	run("config", "user.email", "validated-fixture@example.invalid")
	run("config", "goal.sync-remote", "local")
	write := func(rel string, data []byte) {
		t.Helper()
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, data, 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", rel)
	}
	write(goalsPrefix+"backlog.md", RenderRoot(rootRecord))
	for name, data := range files {
		write(goalsPrefix+name, data)
	}
	run("commit", "-q", "-m", "validated bed")
	run("update-ref", AcceptedRef, "HEAD")
	return root
}

func bedRoot() *RootRecord {
	return &RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "2",
		SyncMode: SyncLocal, Revision: 1,
	}
}

// bedGoal is one lawful queued goal: the smallest record ParseFile accepts
// and ValidateTree leaves alone.
func bedGoal(id string) *GoalFile {
	return &GoalFile{
		Id: id, State: StateQueued, Intent: "Do " + id, Origin: "main",
		NextStep: "Start " + id + ".", OpenedAt: "2026-08-23T00:00:00Z", Revision: 1,
		History: []HistoryLine{{
			At: "2026-08-23T00:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAV-bed-00000000",
			Verb: "open", Actor: "bed-m1+coordinator", Targets: []string{id}, Keep: -1,
		}},
	}
}

// relayedApproved is an approved goal whose human word was relayed, so its
// admission depends on the review date the horizon is judged against.
func relayedApproved(id, reviewBy string) *GoalFile {
	f := bedGoal(id)
	f.Tier = 3
	f.Revision = 2
	f.Budget = &Budget{ElapsedLimit: "2h", AttemptLimit: 3, ReservedJobMinutesLimit: 60, ActiveJobLimit: 1, ReviewRoundLimit: 1}
	f.State = StateApproved
	event := HistoryLine{
		At: "2026-08-23T00:01:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAY-human-00000002",
		Verb: "approve", Actor: "human:wido", Targets: []string{id}, Keep: -1,
		AuthorityOutcome: AuthorityOutcomeTemporaryHumanWord, AuthorityReviewBy: reviewBy,
		AuthorityRuling: "01ARZ3NDEKTSV4RRFFQ69G5FB0-ruling-0000000a", TemporaryHumanWord: "go ahead",
	}
	f.History = append(f.History, event)
	f.Approved = &ApprovalRecord{
		By: event.Actor, At: event.At, Revision: 2, Opid: event.Opid,
		Authority: ApprovalAuthorityRelayed, ReviewBy: reviewBy,
		Digest: ApprovalDigest(f.Intent, f.Tier, *f.Budget),
	}
	return f
}

func bedTip(t *testing.T, root string) string {
	t.Helper()
	out, err := gitIn(root, "rev-parse", "--verify", AcceptedRef)
	if err != nil {
		t.Fatalf("the fixture's accepted ref does not resolve: %v", err)
	}
	return strings.TrimSpace(out)
}

func bedCommitterSeconds(t *testing.T, root, tip string) int64 {
	t.Helper()
	out, err := gitIn(root, "log", "-1", "--format=%ct", tip)
	if err != nil {
		t.Fatalf("the fixture's commit time does not read: %v", err)
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil {
		t.Fatalf("the fixture's commit time is not a number: %v", err)
	}
	return seconds
}

func TestReadValidatedTreeKeepsTheTreeAndTheCommitTime(t *testing.T) {
	t.Parallel()
	root := validatedBed(t, "2026-08-23T09:00:00+00:00", bedRoot(), map[string][]byte{
		"alpha.md": RenderFile(bedGoal("alpha")),
		"beta.md":  RenderFile(bedGoal("beta")),
	})
	tip := bedTip(t, root)

	read, err := ReadValidatedTree(root, tip)
	if err != nil {
		t.Fatalf("a lawful ledger was refused: %v", err)
	}
	if read.Tip != tip {
		t.Fatalf("the read names tip %q, not %q", read.Tip, tip)
	}
	if read.Tree == nil || read.Tree.Live["alpha"] == nil || read.Tree.Live["beta"] == nil {
		t.Fatalf("the validated tree does not carry both live goals: %+v", read.Tree)
	}
	if read.Tree.Root == nil || read.Tree.Root.SyncMode != SyncLocal {
		t.Fatalf("the validated tree does not carry the root record")
	}
	if want := bedCommitterSeconds(t, root, tip); read.CommittedAt.Unix() != want {
		t.Fatalf("committer time %d is not the commit's %d", read.CommittedAt.Unix(), want)
	}
	if read.CommittedAt.Location() != time.UTC {
		t.Fatalf("the committer time is not reported in UTC: %s", read.CommittedAt)
	}
}

func TestReadValidatedTreeRefusesAParseProblemWithItsProblems(t *testing.T) {
	t.Parallel()
	root := validatedBed(t, "2026-08-23T09:00:00+00:00", bedRoot(), map[string][]byte{
		"alpha.md": RenderFile(bedGoal("alpha")),
		"torn.md":  []byte("# torn\n- State: queued\n"),
	})
	tip := bedTip(t, root)

	_, err := ReadValidatedTree(root, tip)
	var refusal *TreeReadError
	if !errors.As(err, &refusal) {
		t.Fatalf("a torn file was not refused with its typed problems: %v", err)
	}
	if len(refusal.Problems) == 0 {
		t.Fatalf("the refusal carries no problems")
	}
	if !problemsName(refusal.Problems, goalsPrefix+"torn.md") {
		t.Fatalf("no problem names the torn file: %v", refusal.Problems)
	}
}

func TestReadValidatedTreeRefusesATreeRuleWithItsProblems(t *testing.T) {
	t.Parallel()
	first, second := bedGoal("alpha"), bedGoal("beta")
	// Two open goals claiming the same rank: every file parses and the
	// tree-level rank rule is the only thing that refuses them.
	first.Priority, first.Sequence = 1, 1
	second.Priority, second.Sequence = 1, 1
	root := validatedBed(t, "2026-08-23T09:00:00+00:00", bedRoot(), map[string][]byte{
		"alpha.md": RenderFile(first),
		"beta.md":  RenderFile(second),
	})
	tip := bedTip(t, root)

	_, err := ReadValidatedTree(root, tip)
	var refusal *TreeReadError
	if !errors.As(err, &refusal) {
		t.Fatalf("a duplicate rank was not refused with its typed problems: %v", err)
	}
	files, readErr := ReadCommitGoals(root, tip)
	if readErr != nil {
		t.Fatalf("the fixture tree does not read: %v", readErr)
	}
	parsed, parseProblems := ParseTreeFiles(files)
	if len(parseProblems) != 0 {
		t.Fatalf("the fixture was meant to parse cleanly: %v", parseProblems)
	}
	want := ValidateTree(parsed)
	if len(want) == 0 {
		t.Fatalf("the fixture does not break a tree rule")
	}
	if len(refusal.Problems) != len(want) {
		t.Fatalf("the refusal carries %d problems, the tree rules give %d", len(refusal.Problems), len(want))
	}
	for index, problem := range want {
		if refusal.Problems[index] != problem {
			t.Fatalf("problem %d is %q, not the tree rule's %q", index, refusal.Problems[index], problem)
		}
	}
}

// TestReadValidatedTreeAgreesWithValidateCommit holds the two readers to one
// law: a commit is lawful for both or for neither, and the refusal a caller
// reads names everything the engine's own refusal names.
func TestReadValidatedTreeAgreesWithValidateCommit(t *testing.T) {
	t.Parallel()
	ranked, alsoRanked := bedGoal("alpha"), bedGoal("beta")
	ranked.Priority, ranked.Sequence = 1, 1
	alsoRanked.Priority, alsoRanked.Sequence = 1, 1
	beds := map[string]map[string][]byte{
		"lawful": {
			"alpha.md": RenderFile(bedGoal("alpha")),
		},
		"a file that does not parse": {
			"alpha.md": RenderFile(bedGoal("alpha")),
			"torn.md":  []byte("# torn\n- State: queued\n"),
		},
		"a tree rule that refuses": {
			"alpha.md": RenderFile(ranked),
			"beta.md":  RenderFile(alsoRanked),
		},
	}
	for name, files := range beds {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := validatedBed(t, "2026-08-23T09:00:00+00:00", bedRoot(), files)
			tip := bedTip(t, root)

			_, readErr := ReadValidatedTree(root, tip)
			commitErr := ValidateCommit(root, tip)
			if (readErr == nil) != (commitErr == nil) {
				t.Fatalf("the two readers disagree: read %v, validate %v", readErr, commitErr)
			}
			if readErr == nil {
				return
			}
			var refusal *TreeReadError
			if !errors.As(readErr, &refusal) {
				t.Fatalf("the refusal is not typed: %v", readErr)
			}
			for _, problem := range refusal.Problems {
				if !strings.Contains(commitErr.Error(), string(problem)) {
					t.Fatalf("the engine's refusal omits %q: %v", problem, commitErr)
				}
			}
		})
	}
}

// TestReadValidatedTreeIgnoresASteeredGitEnvironment proves the read answers
// for the root it was given: an inherited GIT_DIR, work tree, or injected
// config points git at a repository of the caller's choosing, and -C alone
// does not defeat them.
func TestReadValidatedTreeIgnoresASteeredGitEnvironment(t *testing.T) {
	t.Parallel()
	root := validatedBed(t, "2026-08-23T09:00:00+00:00", bedRoot(), map[string][]byte{
		"alpha.md": RenderFile(bedGoal("alpha")),
	})
	other := validatedBed(t, "2001-02-03T04:05:06+00:00", bedRoot(), map[string][]byte{
		"stranger.md": RenderFile(bedGoal("stranger")),
	})
	tip := bedTip(t, root)
	steered := testEnvironment(os.Environ(),
		"GIT_DIR="+filepath.Join(other, ".git"),
		"GIT_WORK_TREE="+other,
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=core.bare",
		"GIT_CONFIG_VALUE_0=true",
	)

	read, err := ReadValidatedTree(root, tip, steered)
	if err != nil {
		t.Fatalf("a steered environment refused a lawful read: %v", err)
	}
	if read.Tree.Live["alpha"] == nil || read.Tree.Live["stranger"] != nil {
		t.Fatalf("the read answered for the steered repository: %v", sortedGoalIds(read.Tree.Live))
	}
	if want := bedCommitterSeconds(t, root, tip); read.CommittedAt.Unix() != want {
		t.Fatalf("the commit time %d came from the steered repository, not %d", read.CommittedAt.Unix(), want)
	}
}

func TestNewApprovalHorizonCarriesTheFleetEnrollment(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	plain := &TreeGoals{Root: bedRoot()}
	if horizon := NewApprovalHorizon(plain, now); !horizon.Now.Equal(now) || !horizon.EnrolledAt.IsZero() {
		t.Fatalf("an unenrolled fleet gave %+v", horizon)
	}
	enrolled := &TreeGoals{Root: bedRoot()}
	enrolled.Root.FleetEnrollment = &FleetEnrollmentRecord{At: "2026-08-01T00:00:00Z", Machine: "m1", Generation: 1, Opid: "op"}
	horizon := NewApprovalHorizon(enrolled, now)
	if horizon.EnrolledAt.UTC().Format(time.RFC3339) != "2026-08-01T00:00:00Z" {
		t.Fatalf("the enrolled terminal did not reach the horizon: %+v", horizon)
	}
}

// TestNewApprovalHorizonDecidesExpiryAsClaimAdmissionDoes ties the exported
// constructor to the frontier: the same tree and the same instant put the
// same goal in the same bucket, whichever reader asks.
func TestNewApprovalHorizonDecidesExpiryAsClaimAdmissionDoes(t *testing.T) {
	t.Parallel()
	const reviewBy = "2026-09-02"
	root := validatedBed(t, "2026-08-23T09:00:00+00:00", bedRoot(), map[string][]byte{
		"relayed.md": RenderFile(relayedApproved("relayed", reviewBy)),
	})
	tip := bedTip(t, root)
	read, err := ReadValidatedTree(root, tip)
	if err != nil {
		t.Fatalf("the relayed fixture was refused: %v", err)
	}
	file := read.Tree.Live["relayed"]
	if file == nil || file.State != StateApproved {
		t.Fatalf("the relayed fixture is not an approved live goal: %+v", file)
	}

	within := time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)
	past := within.AddDate(0, 0, 2)
	for _, observation := range []struct {
		name    string
		now     time.Time
		expired bool
	}{
		{"on the review date", within, false},
		{"two days past it", past, true},
	} {
		t.Run(observation.name, func(t *testing.T) {
			t.Parallel()
			horizon := NewApprovalHorizon(read.Tree, observation.now)
			expired, why := file.ApprovalExpired(horizon)
			if expired != observation.expired {
				t.Fatalf("expiry answered %v (%s), wanted %v", expired, why, observation.expired)
			}
			frontier, err := Next(Projection{Root: root, Tip: tip, Tree: read.Tree, Horizon: horizon}, "")
			if err != nil {
				t.Fatalf("the frontier could not be answered: %v", err)
			}
			inAwaiting := containsID(frontier.Awaiting, "relayed")
			if inAwaiting != observation.expired {
				t.Fatalf("the frontier put the goal in Awaiting=%v while expiry said %v (ready=%v refused=%v)",
					inAwaiting, observation.expired, frontier.Ready, frontier.Refused)
			}
			if !observation.expired && !containsID(frontier.Ready, "relayed") {
				t.Fatalf("an unexpired relayed approval was not ready: %+v", frontier)
			}
		})
	}
}

func problemsName(problems []Problem, path string) bool {
	for _, problem := range problems {
		if strings.Contains(string(problem), path) {
			return true
		}
	}
	return false
}

func containsID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}
