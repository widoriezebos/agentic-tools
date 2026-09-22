package act

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

var fixtureNow = time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC)

func box() goalbudget.Budget {
	return goalbudget.Budget{ElapsedLimit: "4h", AttemptLimit: 6, ReservedJobMinutesLimit: 720, ActiveJobLimit: 1, ReviewRoundLimit: 2}
}

func TestAnUnprovenServerWritesNothingAndSaysWhy(t *testing.T) {
	t.Parallel()
	authority := Unproven("the interface was started by an agent process (claude-code); " + Restart)

	testutil.Expect(t, "the status line", authority.Line(),
		"not proven: the interface was started by an agent process (claude-code); "+Restart)
	testutil.Expect(t, "nothing is proven", authority.Proven(), false)

	for name, err := range map[string]error{
		"approve":  authority.Approve("g-1", box()),
		"withdraw": authority.Withdraw("g-1", "changed my mind"),
	} {
		refusal, ok := err.(*Refusal)
		if !ok {
			t.Fatalf("%s refusal = %v, want an act.Refusal", name, err)
		}
		testutil.Expect(t, name+" refuses as unproven", refusal.Kind, KindUnproven)
		testutil.Expect(t, name+" carries the proof's own reason", refusal.Message, authority.Reason())
	}
}

func TestAProvenServerSaysWhoItActsAs(t *testing.T) {
	t.Parallel()
	root := ledger(t)
	authority, err := Fixture(root, "Wido", "terminal-ttys004-1", fixtureNow)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Expect(t, "the status line", authority.Line(), "acting as human:Wido — proven at the enrolled terminal")
	testutil.Expect(t, "the acting human", authority.Human(), "Wido")
}

// The whole point of the in-process path: the History line an approval from
// the browser writes is the line an approval at the terminal writes.
func TestApproveFromTheInterfaceWritesTheHumansOwnHistoryLine(t *testing.T) {
	t.Parallel()
	root := ledger(t)
	openGoal(t, root, "ui-one")
	authority := provenFor(t, root)

	if err := authority.Approve("ui-one", box()); err != nil {
		t.Fatalf("approve: %v", err)
	}

	file := readGoal(t, root, "ui-one")
	testutil.Expect(t, "the goal is approved", file.State, goal.StateApproved)
	testutil.Expect(t, "the approval's actor", file.Approved.By, "human:Wido")
	testutil.Expect(t, "the approval's authority", file.Approved.Authority, goal.ApprovalAuthorityProven)
	testutil.Expect(t, "the budget the human confirmed", *file.Budget, box())

	last := file.History[len(file.History)-1]
	testutil.Expect(t, "the last verb", last.Verb, "approve")
	// The actor is the human and nothing else, which is exactly what a
	// terminal approval writes: the seat's machine and lineage are the
	// operation's, not the act's.
	testutil.Expect(t, "the last actor", last.Actor, "human:Wido")
}

func TestWithdrawReturnsAnApprovedGoalToTheQueue(t *testing.T) {
	t.Parallel()
	root := ledger(t)
	openGoal(t, root, "ui-two")
	authority := provenFor(t, root)
	if err := authority.Approve("ui-two", box()); err != nil {
		t.Fatalf("approve: %v", err)
	}

	if err := authority.Withdraw("ui-two", "the design is not settled"); err != nil {
		t.Fatalf("withdraw: %v", err)
	}

	file := readGoal(t, root, "ui-two")
	testutil.Expect(t, "the goal is queued again", file.State, goal.StateQueued)
	testutil.Expect(t, "no approval stands", file.Approved == nil, true)
	last := file.History[len(file.History)-1]
	testutil.Expect(t, "the last verb", last.Verb, "unapprove")
	testutil.Expect(t, "the reason the human gave", last.Reason, "the design is not settled")
}

// A claim is a seat's fact, and the board never offers this move for claimed
// work. A seat that claims between the read and the drop is the race, and the
// engine does not pretend the work never began: it parks it with the reason
// and displaces the claim. The sheet's note says so rather than promising a
// refusal the engine does not make.
func TestWithdrawingClaimedWorkParksItRatherThanUnwindingIt(t *testing.T) {
	t.Parallel()
	root := ledger(t)
	openGoal(t, root, "ui-three")
	authority := provenFor(t, root)
	if err := authority.Approve("ui-three", box()); err != nil {
		t.Fatalf("approve: %v", err)
	}
	claim(t, root, "ui-three")

	if err := authority.Withdraw("ui-three", "second thoughts"); err != nil {
		t.Fatalf("withdraw on claimed work: %v", err)
	}

	file := readGoal(t, root, "ui-three")
	testutil.Expect(t, "the work is parked, not queued", file.State, goal.StateParked)
	testutil.Expect(t, "the park carries the human's reason", file.Parked.Because,
		"approval revoked: second thoughts")
}

// A goal that carries no approval has none to withdraw. The engine's own
// sentence is what the browser shows, under the engine's own code.
func TestWithdrawingUnapprovedWorkIsTheEnginesRefusal(t *testing.T) {
	t.Parallel()
	root := ledger(t)
	openGoal(t, root, "ui-six")
	authority := provenFor(t, root)

	err := authority.Withdraw("ui-six", "never mind")

	refusal, ok := err.(*Refusal)
	if !ok {
		t.Fatalf("withdraw without an approval = %v, want an act.Refusal", err)
	}
	testutil.Expect(t, "the refusal is the engine's", refusal.Kind, KindEngine)
	if refusal.Message == "" {
		t.Fatal("the engine's refusal reached the browser with no words")
	}
	testutil.Expect(t, "the goal was not touched", readGoal(t, root, "ui-six").State, goal.StateQueued)
}

func TestApproveRefusesAnIncompleteBudgetBeforeItReachesTheLedger(t *testing.T) {
	t.Parallel()
	root := ledger(t)
	openGoal(t, root, "ui-four")
	authority := provenFor(t, root)

	err := authority.Approve("ui-four", goalbudget.Budget{ElapsedLimit: "4h"})

	refusal, ok := err.(*Refusal)
	if !ok {
		t.Fatalf("approve with half a budget = %v, want an act.Refusal", err)
	}
	testutil.Expect(t, "the request is at fault", refusal.Kind, KindRequest)
	testutil.Expect(t, "the goal was not touched", readGoal(t, root, "ui-four").State, goal.StateQueued)
}

func TestTheProofIsRecordedBesideTheActItAuthorized(t *testing.T) {
	t.Parallel()
	root := ledger(t)
	openGoal(t, root, "ui-five")
	authority := provenFor(t, root)
	if err := authority.Approve("ui-five", box()); err != nil {
		t.Fatalf("approve: %v", err)
	}

	file := readGoal(t, root, "ui-five")
	proofPath := filepath.Join(root, "artifacts", "agents", "authority", "proofs", file.Approved.Opid+".json")
	recorded, err := os.ReadFile(proofPath)
	if err != nil {
		t.Fatalf("the act's proof was not recorded at %s: %v", proofPath, err)
	}
	if !strings.Contains(string(recorded), `"action": "goal approve"`) {
		t.Fatalf("the recorded proof does not name the act:\n%s", recorded)
	}
	if !strings.Contains(string(recorded), humanauthority.OutcomeProven) {
		t.Fatalf("the recorded proof does not carry its outcome:\n%s", recorded)
	}
}

/* ------------------------------------------------------ the fixture bed -- */

func provenFor(t *testing.T, root string) Authority {
	t.Helper()
	authority, err := Fixture(root, "Wido", "terminal-ttys004-1", fixtureNow)
	if err != nil {
		t.Fatal(err)
	}
	return authority
}

// ledger builds one clone of one bare origin whose accepted tip carries a
// lawful root record, declares the fake runtime, and ships the executable
// pre-commit guard every goal mutation stands on.
func ledger(t *testing.T) string {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "origin.git")
	git(t, t.TempDir(), "init", "-q", "--bare", "-b", "main", origin)
	seed := filepath.Join(t.TempDir(), "seed")
	git(t, t.TempDir(), "clone", "-q", origin, seed)
	write(t, filepath.Join(seed, "README.md"), "seed\n", 0o644)
	write(t, filepath.Join(seed, "metasystem.conf"),
		"metasystem.runtimes=fake\nmetasystem.budget.review-round-max=3\n", 0o644)
	git(t, seed, "add", "README.md", "metasystem.conf")
	git(t, seed, "commit", "-qm", "seed")
	git(t, seed, "push", "-q", "origin", "main")

	root := filepath.Join(t.TempDir(), "clone")
	git(t, t.TempDir(), "clone", "-q", origin, root)
	git(t, root, "config", "metasystem.goal.machine", "mac-ui")
	// The guard is what ledgerfence.Ensure requires before any mutation. It
	// is never executed here: a publication builds its tree with
	// commit-tree, which runs no hook.
	write(t, filepath.Join(root, "scripts", "agents", "pre-commit-guard.sh"), "#!/usr/bin/env bash\nexit 0\n", 0o755)

	record := &goal.RootRecord{
		Identity: "01J5X000000000000000000000", FormatVersion: "1",
		SyncMode: goal.SyncRemote, MigrationEpoch: "2026-09-01T00:00:00Z",
		ManifestDigest: strings.Repeat("ab", 32), MigrationMode: "manifest", Revision: 1,
		History: []goal.HistoryLine{{
			At: "2026-09-01T09:00:00Z", Opid: "01J5X0000000000000000000A0-mac-ui-1a2b3c4d",
			Verb: "migrate", Actor: "mac-ui+terminal-ttys004-1", Keep: -1,
		}},
	}
	result, err := goal.Publish(endpoint(root), goal.PublishRequest{
		Opid: "01J5X0000000000000000000A1-mac-ui-1a2b3c4d", Machine: "mac-ui", Lineage: "terminal-ttys004-1",
		Intent:  goal.Intent{Verb: "migrate", Targets: []string{"backlog"}},
		Message: "seed the ledger root record",
		Mutate: func(string) ([]goal.Change, error) {
			return []goal.Change{{Path: "plans/goals/backlog.md", Content: goal.RenderRoot(record)}}, nil
		},
	})
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("seeding the ledger: %+v %v", result, err)
	}
	return root
}

func endpoint(root string) goal.Endpoint {
	return goal.Endpoint{Root: root, Remote: "origin", Branch: "refs/heads/main"}
}

// request is the engine request the bed's own setup verbs ride, which is the
// seat's request rather than the human's: no proof, no human name.
func request(t *testing.T, root string) goal.VerbRequest {
	t.Helper()
	ulid, err := goal.NewOperationULID()
	if err != nil {
		t.Fatal(err)
	}
	return goal.VerbRequest{
		Endpoint: endpoint(root), Actor: goal.Actor{Machine: "mac-ui", Lineage: "terminal-ttys004-1"},
		Ulid: ulid, Now: fixtureNow, ClaimEpoch: 1,
	}
}

func openGoal(t *testing.T, root, id string) {
	t.Helper()
	result, err := goal.Open(request(t, root), id, "Make "+id+" work end to end.", "main", "Start "+id+".")
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("open %s: %+v %v", id, result, err)
	}
}

func claim(t *testing.T, root, id string) {
	t.Helper()
	result, err := goal.Claim(request(t, root), id)
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("claim %s: %+v %v", id, result, err)
	}
}

func readGoal(t *testing.T, root, id string) *goal.GoalFile {
	t.Helper()
	projection, err := goal.Project(endpoint(root), false, fixtureNow)
	if err != nil {
		t.Fatalf("projecting the ledger: %v", err)
	}
	file := projection.Tree.Live[id]
	if file == nil {
		t.Fatalf("goal %s is not live at the accepted tip", id)
	}
	return file
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir,
		"-c", "user.name=t", "-c", "user.email=t@t",
		"-c", "protocol.file.allow=always"}, args...)...)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}
