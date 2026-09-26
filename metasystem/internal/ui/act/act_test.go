package act

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
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
		"approve":      authority.Approve("g-1", box()),
		"withdraw":     authority.Withdraw("g-1", "changed my mind"),
		"set-priority": authority.SetPriority("g-1", 2, nil),
		"open":         authority.Open(opening("g-2")),
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
	bed := ledger(t)
	authority, err := Fixture(bed.root, "Wido", "terminal-ttys004-1", fixtureNow)
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
	bed := ledger(t)
	openGoal(t, bed, "ui-one")
	authority := provenFor(t, bed)

	if err := authority.Approve("ui-one", box()); err != nil {
		t.Fatalf("approve: %v", err)
	}

	file := readGoal(t, bed, "ui-one")
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
	bed := ledger(t)
	openGoal(t, bed, "ui-two")
	authority := provenFor(t, bed)
	if err := authority.Approve("ui-two", box()); err != nil {
		t.Fatalf("approve: %v", err)
	}

	if err := authority.Withdraw("ui-two", "the design is not settled"); err != nil {
		t.Fatalf("withdraw: %v", err)
	}

	file := readGoal(t, bed, "ui-two")
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
	bed := ledger(t)
	openGoal(t, bed, "ui-three")
	authority := provenFor(t, bed)
	if err := authority.Approve("ui-three", box()); err != nil {
		t.Fatalf("approve: %v", err)
	}
	claim(t, bed, "ui-three")

	if err := authority.Withdraw("ui-three", "second thoughts"); err != nil {
		t.Fatalf("withdraw on claimed work: %v", err)
	}

	file := readGoal(t, bed, "ui-three")
	testutil.Expect(t, "the work is parked, not queued", file.State, goal.StateParked)
	testutil.Expect(t, "the park carries the human's reason", file.Parked.Because,
		"approval revoked: second thoughts")
}

// A goal that carries no approval has none to withdraw. The engine's own
// sentence is what the browser shows, under the engine's own code.
func TestWithdrawingUnapprovedWorkIsTheEnginesRefusal(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-six")
	authority := provenFor(t, bed)

	err := authority.Withdraw("ui-six", "never mind")

	refusal, ok := err.(*Refusal)
	if !ok {
		t.Fatalf("withdraw without an approval = %v, want an act.Refusal", err)
	}
	testutil.Expect(t, "the refusal is the engine's", refusal.Kind, KindEngine)
	if refusal.Message == "" {
		t.Fatal("the engine's refusal reached the browser with no words")
	}
	testutil.Expect(t, "the goal was not touched", readGoal(t, bed, "ui-six").State, goal.StateQueued)
}

func TestApproveRefusesAnIncompleteBudgetBeforeItReachesTheLedger(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-four")
	authority := provenFor(t, bed)

	err := authority.Approve("ui-four", goalbudget.Budget{ElapsedLimit: "4h"})

	refusal, ok := err.(*Refusal)
	if !ok {
		t.Fatalf("approve with half a budget = %v, want an act.Refusal", err)
	}
	testutil.Expect(t, "the request is at fault", refusal.Kind, KindRequest)
	testutil.Expect(t, "the goal was not touched", readGoal(t, bed, "ui-four").State, goal.StateQueued)
}

func TestTheProofIsRecordedBesideTheActItAuthorized(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-five")
	authority := provenFor(t, bed)
	if err := authority.Approve("ui-five", box()); err != nil {
		t.Fatalf("approve: %v", err)
	}

	file := readGoal(t, bed, "ui-five")
	proofPath := filepath.Join(bed.root, "artifacts", "agents", "authority", "proofs", file.Approved.Opid+".json")
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

// opening is a complete intake statement: the engine derives the rigor tier
// from the four risk answers and refuses an open without them and their basis.
func opening(id string) Opened {
	return Opened{
		ID: id, Intent: "Make " + id + " work end to end.",
		NextStep: "Take " + id + " to a working end state; the approach is yours.",
		Risk: goal.RiskRecord{
			Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1,
			Basis: "one well-understood change behind an existing seam",
		},
	}
}

// A re-rank is never about one record: the engine inserts the goal at the
// requested one-based position and renumbers the whole band behind it.
func TestSetPriorityPlacesAGoalAndRenumbersTheBandAsTheHuman(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	for _, id := range []string{"ui-one", "ui-two", "ui-three"} {
		openGoal(t, bed, id)
	}
	authority := provenFor(t, bed)
	// A goal the fixture opened carries no rank at all, so the band is built
	// here: the first two land in order, and the third is the one that moves.
	const band = uint8(2)
	for at, id := range []string{"ui-one", "ui-two"} {
		if err := authority.SetPriority(id, band, sequence(uint64(at+1))); err != nil {
			t.Fatalf("building the band at %s: %v", id, err)
		}
	}

	if err := authority.SetPriority("ui-three", band, sequence(1)); err != nil {
		t.Fatalf("set-priority: %v", err)
	}

	testutil.Expect(t, "the goal is first in the band", rank(t, bed, "ui-three"), ranked{band, 1})
	testutil.Expect(t, "the goal behind it moved down", rank(t, bed, "ui-one"), ranked{band, 2})
	testutil.Expect(t, "and so did the one behind that", rank(t, bed, "ui-two"), ranked{band, 3})

	file := readGoal(t, bed, "ui-three")
	last := file.History[len(file.History)-1]
	testutil.Expect(t, "the re-rank is the human's own line", last.Actor, "human:Wido")
	testutil.Expect(t, "and it is the verb the terminal writes", last.Verb, "set-priority")
	if !strings.Contains(last.Reason, "requested-sequence=1") {
		t.Fatalf("the reason does not carry the requested position: %q", last.Reason)
	}
}

// The engine refuses a position outside the destination band rather than
// clamping it, so nothing in the interface clamps one either.
func TestSetPriorityCarriesTheEnginesRefusalOfAnImpossiblePosition(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-one")
	authority := provenFor(t, bed)
	const band = uint8(2)
	if err := authority.SetPriority("ui-one", band, sequence(1)); err != nil {
		t.Fatalf("building the band: %v", err)
	}

	err := authority.SetPriority("ui-one", band, sequence(9))

	refusal, ok := err.(*Refusal)
	if !ok {
		t.Fatalf("refusal = %v, want an act.Refusal", err)
	}
	testutil.Expect(t, "the ledger refused it", refusal.Kind, KindEngine)
	if !strings.Contains(refusal.Message, "outside the current destination range") {
		t.Fatalf("the refusal is not the engine's own: %q", refusal.Message)
	}
	testutil.Expect(t, "and nothing moved", rank(t, bed, "ui-one"), ranked{band, 1})
}

func TestSetPriorityRefusesARankThatIsNotOneBeforeItReachesTheLedger(t *testing.T) {
	t.Parallel()
	authority := provenFor(t, ledger(t))

	for name, err := range map[string]error{
		"no goal":       authority.SetPriority("  ", 2, nil),
		"band 0":        authority.SetPriority("ui-one", 0, nil),
		"band 4":        authority.SetPriority("ui-one", 4, nil),
		"position zero": authority.SetPriority("ui-one", 2, sequence(0)),
	} {
		refusal, ok := err.(*Refusal)
		if !ok {
			t.Fatalf("%s refusal = %v, want an act.Refusal", name, err)
		}
		testutil.Expect(t, name+" is the request's own fault", refusal.Kind, KindRequest)
	}
}

// The intake act, written as the human: origin human, the tier the risk
// answers derive, and the History line a terminal open writes.
func TestOpenCreatesAQueuedGoalAsTheHuman(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	authority := provenFor(t, bed)

	if err := authority.Open(opening("ui-new")); err != nil {
		t.Fatalf("open: %v", err)
	}

	file := readGoal(t, bed, "ui-new")
	testutil.Expect(t, "the goal is queued", file.State, goal.StateQueued)
	testutil.Expect(t, "the origin is the human's intake", file.Origin, goal.OriginHuman)
	testutil.Expect(t, "the intent is the human's own", file.Intent, "Make ui-new work end to end.")
	testutil.Expect(t, "the tier is the one the risk answers derive", file.Tier, uint8(1))
	if file.Risk == nil {
		t.Fatal("the goal carries no risk record")
	}
	testutil.Expect(t, "the basis is recorded", file.Risk.Basis, "one well-understood change behind an existing seam")
	testutil.Expect(t, "the open is the human's own line", file.History[0].Actor, "human:Wido")
	testutil.Expect(t, "and it is the verb the terminal writes", file.History[0].Verb, "open")
}

func TestOpenRefusesAnIncompleteIntakeBeforeItReachesTheLedger(t *testing.T) {
	t.Parallel()
	authority := provenFor(t, ledger(t))

	blank := func(change func(*Opened)) error {
		asked := opening("ui-new")
		change(&asked)
		return authority.Open(asked)
	}
	for name, err := range map[string]error{
		"no id":        blank(func(o *Opened) { o.ID = " " }),
		"no intent":    blank(func(o *Opened) { o.Intent = "" }),
		"no next step": blank(func(o *Opened) { o.NextStep = "" }),
		"no basis":     blank(func(o *Opened) { o.Risk.Basis = "" }),
		"no severity":  blank(func(o *Opened) { o.Risk.Severity = 0 }),
		"tier 4":       blank(func(o *Opened) { o.Tier = 4 }),
	} {
		refusal, ok := err.(*Refusal)
		if !ok {
			t.Fatalf("%s refusal = %v, want an act.Refusal", name, err)
		}
		testutil.Expect(t, name+" is the request's own fault", refusal.Kind, KindRequest)
	}
}

// The engine has the last word on an id: opening the same goal twice is the
// ledger's refusal, not this package's.
func TestOpenCarriesTheEnginesRefusalOfAnIdItAlreadyHas(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-one")
	authority := provenFor(t, bed)

	err := authority.Open(opening("ui-one"))

	refusal, ok := err.(*Refusal)
	if !ok {
		t.Fatalf("refusal = %v, want an act.Refusal", err)
	}
	testutil.Expect(t, "the ledger refused it", refusal.Kind, KindEngine)
}

type ranked struct {
	priority uint8
	sequence uint64
}

func rank(t *testing.T, bed *ledgerBed, id string) ranked {
	t.Helper()
	file := readGoal(t, bed, id)
	return ranked{file.Priority, file.Sequence}
}

func sequence(at uint64) *uint64 { return &at }

// The readings a bed hands in are a test's and nothing else's: an authority
// nobody handed them to asks the checkout itself, so a directory that is no
// checkout refuses rather than answering. This is what keeps the fixture from
// standing in for the server's own reading of a real one.
func TestTheDefaultReadingsAreTheCheckoutsOwn(t *testing.T) {
	t.Parallel()
	plain := reads{}
	root := t.TempDir()

	if err := plain.ensureFence(root); err == nil {
		t.Error("a directory shipping no executable guard passed the ledger fence")
	}
	// The endpoint it answers is the checkout's own, and it owns no committed
	// state of its own: a reading that came back carrying a repository would be
	// a fixture's, and this one is the server's.
	switch at, err := plain.resolveEndpoint(root); {
	case err != nil:
		// A directory that is no checkout may refuse instead, which is an answer.
	case at.Root != root || at.Repository != nil:
		t.Errorf("the default endpoint reading answered %+v, want this checkout with no repository", at)
	}
	if _, err := plain.resolveMachine(root); err == nil {
		t.Error("a directory that is no checkout resolved a machine name")
	}
	if plain.parkBranchCheck(root, goal.Endpoint{Root: root}) == nil {
		t.Error("no park branch check was carried, so every park would fail closed")
	}
}

func provenFor(t *testing.T, bed *ledgerBed) Authority {
	t.Helper()
	authority, err := Fixture(bed.root, "Wido", "terminal-ttys004-1", fixtureNow)
	if err != nil {
		t.Fatal(err)
	}
	return bed.acting(t, authority)
}

// ledgerBed is one private accepted ledger and the checkout beside it.
//
// Nothing here runs Git, and nothing here is shared between tests. The
// committed state is goal.Endpoint's own Repository seam, held in memory by
// internal/testgoal, whose first commit carries a lawful root record; the
// checkout is a directory with the one file a mutation reads from disk, the
// configuration that declares the fake runtime. The readings an act would
// otherwise ask Git for — the ledger fence's enrollment, this checkout's
// endpoint and machine name, and a goal branch's safety — are handed to the
// authority as its own, so the whole publication path runs with Git absent,
// which is what this repository's rules ask of a behaviour test.
type ledgerBed struct {
	root       string
	repository *testgoal.Repository
}

// The bed's own seed commit: forty characters, because a commit id is what the
// engine compares and carries.
const bedSeedCommit = "0000000000000000000000000000000000000001"

func ledger(t *testing.T) *ledgerBed {
	t.Helper()
	root := filepath.Join(t.TempDir(), "clone")
	write(t, filepath.Join(root, "README.md"), "seed\n", 0o644)
	write(t, filepath.Join(root, "metasystem.conf"),
		"metasystem.runtimes=fake\nmetasystem.budget.review-round-max=3\n", 0o644)

	record := &goal.RootRecord{
		Identity: "01J5X000000000000000000000", FormatVersion: "1",
		SyncMode: goal.SyncRemote, MigrationEpoch: "2026-09-01T00:00:00Z",
		ManifestDigest: strings.Repeat("ab", 32), MigrationMode: "manifest", Revision: 1,
		History: []goal.HistoryLine{{
			At: "2026-09-01T09:00:00Z", Opid: "01J5X0000000000000000000A0-mac-ui-1a2b3c4d",
			Verb: "migrate", Actor: "mac-ui+terminal-ttys004-1", Keep: -1,
		}},
	}
	files := map[string][]byte{"plans/goals/backlog.md": []byte(goal.RenderRoot(record))}
	return &ledgerBed{root: root, repository: testgoal.New(files, fixtureNow, bedSeedCommit)}
}

// acting is one authority with this bed's own readings on it, which is what
// keeps an act off Git. Each reading answers for this bed's root and refuses
// any other, so a reading reached for the wrong checkout fails here rather
// than answering quietly.
func (bed *ledgerBed) acting(t *testing.T, authority Authority) Authority {
	t.Helper()
	authority.reads = reads{
		// That the fence refuses a checkout shipping no executable guard is
		// internal/ledgerfence's own test; its probe is a Git command.
		fence: func(root string) error { return bed.mine(root) },
		endpoint: func(root string) (goal.Endpoint, error) {
			if err := bed.mine(root); err != nil {
				return goal.Endpoint{}, err
			}
			return bed.endpoint(), nil
		},
		machine: func(root string) (string, error) {
			if err := bed.mine(root); err != nil {
				return "", err
			}
			return "mac-ui", nil
		},
		// The branch package's own park check, over its three readings rather
		// than a repository: this checkout has no goal branch, which is the
		// case that answers without reading a remote at all, and a remote
		// reading that IS reached fails the test rather than answering.
		parkCheck: func(root string, at goal.Endpoint) func(goalID, next string) (string, error) {
			return goalbranch.ParkCheckWithReaders(root, at,
				func(string, string) (string, bool, error) { return "", false, nil },
				func(string, goal.Endpoint) (string, error) {
					t.Errorf("the park check read the endpoint's main on a remote")
					return "", fmt.Errorf("this bed has no remote")
				},
				func(string, goal.Endpoint, string) (string, bool, error) {
					t.Errorf("the park check read a goal branch on a remote")
					return "", false, fmt.Errorf("this bed has no remote")
				})
		},
	}
	return authority
}

func (bed *ledgerBed) mine(root string) error {
	if root != bed.root {
		return fmt.Errorf("undeclared checkout root %q", root)
	}
	return nil
}

// endpoint is the bed's ledger: the endpoint the command edge would resolve,
// with the in-memory repository owning the committed state. The branch is main
// because that is the endpoint a park's branch check accepts.
func (bed *ledgerBed) endpoint() goal.Endpoint {
	return goal.Endpoint{Root: bed.root, Remote: "origin", Branch: "refs/heads/main", Repository: bed.repository}
}

// request is the engine request the bed's own setup verbs ride, which is the
// seat's request rather than the human's: no proof, no human name.
func request(t *testing.T, bed *ledgerBed) goal.VerbRequest {
	t.Helper()
	ulid, err := goal.NewOperationULID()
	if err != nil {
		t.Fatal(err)
	}
	return goal.VerbRequest{
		Endpoint: bed.endpoint(), Actor: goal.Actor{Machine: "mac-ui", Lineage: "terminal-ttys004-1"},
		Ulid: ulid, Now: fixtureNow, ClaimEpoch: 1,
	}
}

func openGoal(t *testing.T, bed *ledgerBed, id string) {
	t.Helper()
	result, err := goal.Open(request(t, bed), id, "Make "+id+" work end to end.", "main", "Start "+id+".")
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("open %s: %+v %v", id, result, err)
	}
}

func claim(t *testing.T, bed *ledgerBed, id string) {
	t.Helper()
	result, err := goal.Claim(request(t, bed), id)
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("claim %s: %+v %v", id, result, err)
	}
}

func readGoal(t *testing.T, bed *ledgerBed, id string) *goal.GoalFile {
	t.Helper()
	projection, err := goal.Project(bed.endpoint(), false, fixtureNow)
	if err != nil {
		t.Fatalf("projecting the ledger: %v", err)
	}
	file := projection.Tree.Live[id]
	if file == nil {
		t.Fatalf("goal %s is not live at the accepted tip", id)
	}
	return file
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
