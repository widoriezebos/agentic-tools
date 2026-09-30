package main

// Idempotency rows for the goal, grant and decision objects (U-idem,
// R-129-ui). Each stateful witness runs the action twice through the router
// on an isolated ledger (stubbed Git, fixed clock) and asserts that the second
// run exits 0 and records nothing: no publication, no journal entry, no
// changed goal record.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// goalRepeatWitness builds a witness: prepare the bed, run args once (which
// must succeed or already be a no-op), then run them again and require an
// unchanged success that recorded nothing.
func goalRepeatWitness(amend func(*goal.GoalFile), prepare func(*intentBed), phrase string, args ...string) func(*testing.T) {
	return func(t *testing.T) {
		bed := newIntentBed(t, false, amend)
		if prepare != nil {
			prepare(bed)
		}
		code, first := bed.runJSON(bed.terminalOwners(), args...)
		if code != 0 || (first.Outcome != intentConfirmed && first.Outcome != intentUnchanged) {
			t.Fatalf("%v first run = %d %+v", args, code, first)
		}
		bed.expectRepeatRecordsNothing(phrase, args...)
	}
}

// expectRepeatRecordsNothing runs args again and requires exit 0, outcome
// unchanged, a summary naming what already holds, and no record: the
// publication count, the journal and the goal files stay as they were.
func (b *intentBed) expectRepeatRecordsNothing(phrase string, args ...string) {
	b.t.Helper()
	publications := b.publications()
	journal := goalJournalNames(b.t, b.root())
	files := goalLedgerBytes(b)
	code, second := b.runJSON(b.terminalOwners(), args...)
	if code != 0 || second.Outcome != intentUnchanged {
		b.t.Fatalf("%v repeat = exit %d %+v; want exit 0, unchanged", args, code, second)
	}
	if phrase != "" && !strings.Contains(second.Summary, phrase) {
		b.t.Fatalf("%v repeat does not say what already holds (%q): %q", args, phrase, second.Summary)
	}
	if b.publications() != publications {
		b.t.Fatalf("%v repeat published %d time(s)", args, b.publications()-publications)
	}
	if after := goalJournalNames(b.t, b.root()); strings.Join(after, ",") != strings.Join(journal, ",") {
		b.t.Fatalf("%v repeat changed the goal journal: %v to %v", args, journal, after)
	}
	if after := goalLedgerBytes(b); after != files {
		b.t.Fatalf("%v repeat changed the accepted ledger", args)
	}
}

// terminalOwners are the bed's owners at a proven enrolled terminal: the
// human proofs a person's act asks for are the fixture's.
func (b *intentBed) terminalOwners() intentOwners {
	owners := b.owners()
	proven := func(root string, pid int64, reader humanauthority.Reader, now time.Time) (humanauthority.Proof, error) {
		return fixedFixtureGoalAuthority(root, pid, reader, "", "", now)
	}
	owners.dependencies.proveHuman, owners.dependencies.proveTerminal = proven, proven
	return owners
}

func goalJournalNames(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "artifacts", "agents", "goal-transactions"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	return names
}

// goalLedgerBytes is the accepted commit's files, rendered as one string.
func goalLedgerBytes(b *intentBed) string {
	commit := b.repo.commit(b.repo.accepted)
	var all strings.Builder
	paths := make([]string, 0, len(commit.files))
	for path := range commit.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		all.WriteString(path + "\x00" + string(commit.files[path]) + "\x00")
	}
	return all.String()
}

func addOtherGoal(bed *intentBed) { bed.addGoal(queuedIntentGoal("other-goal", 1)) }

func runOrFail(args ...string) func(*intentBed) {
	return func(bed *intentBed) {
		if code, result := bed.runJSON(bed.owners(), args...); code != 0 {
			bed.t.Fatalf("%v = %d %+v", args, code, result)
		}
	}
}

func init() {
	read := func(name, why string) { registerIdempotency(name, idemRead, why, nil) }
	stateful := func(name, why string, witness func(*testing.T)) {
		registerIdempotency(name, idemStateful, why, witness)
	}

	read("goal list", "lists the goals")
	read("goal show", "shows one goal's record")
	read("decision list", "lists the recorded decisions")
	read("decision show", "shows one record")
	read("grant list", "lists the powers of attorney")

	stateful("goal approve", "the same approval that stands is success with no record",
		goalRepeatWitness(makeQueued, nil, "already has the same proven approval",
			"goal", "approve", bedGoal, "--fixture-human-authority", "--lineage", "m1"))
	stateful("goal budget", "the box the goal already has is success with no record; reading the budget is a read",
		goalRepeatWitness(nil, nil, "already has exactly this budget",
			"goal", "budget", bedGoal, "4h/4/240m/2/3", "--fixture-human-authority", "--lineage", "m1"))
	stateful("goal pause", "a paused goal paused again says since when and why",
		goalRepeatWitness(makeQueued, nil, "is already paused (since ",
			"goal", "pause", bedGoal, "--reason", "waits for the review", "--lineage", "m1"))
	stateful("goal resume", "a goal that is not paused is already resumed",
		goalRepeatWitness(makeParked, nil, "already not paused",
			"goal", "resume", bedGoal, "--lineage", "m1"))
	stateful("goal done", "a done goal concluded again keeps its conclusion",
		goalRepeatWitness(nil, nil, "is already done",
			"goal", "done", bedGoal, "--reason", "shipped and verified", "--lineage", "m1"))
	stateful("goal open", "an existing goal opened with the same fields is success; a new id is a creation",
		goalRepeatWitness(nil, func(bed *intentBed) { bed.lineage = "m1" }, "reads exactly this way",
			"goal", "open", "fresh-goal", "--intent", "Proof runs in half the time.", "--next", "Measure the slowest step.",
			"--blocks", bedGoal, "--risk", "severity=1,novelty=1,exposure=1,accumulation=1", "--basis", "local test tooling only"))
	stateful("goal edit", "an edit to what the goal already says is success with no record",
		goalRepeatWitness(makeQueued, nil, "already reads exactly this way",
			"goal", "edit", bedGoal, "--next", "Land slice 2.", "--lineage", "m1"))
	stateful("goal allow", "a goal already allowed that permission is success with no record",
		goalRepeatWitness(makeQueued, nil, "is already allowed stop-test changes",
			"goal", "allow", bedGoal, "stop-test-changes", "--reason", "the hook entry moved", "--fixture-human-authority"))
	stateful("goal disallow", "a goal not allowed that permission is already disallowed",
		goalRepeatWitness(makeQueued, runOrFail("goal", "allow", bedGoal, "stop-test-changes", "--reason", "the hook entry moved", "--fixture-human-authority"),
			"is already not allowed stop-test changes", "goal", "disallow", bedGoal, "stop-test-changes", "--lineage", "m1"))
	stateful("goal claim", "a goal this session already holds is claimed",
		goalRepeatWitness(nil, nil, "is already claimed by this session",
			"goal", "claim", bedGoal, "--lineage", "m1"))
	stateful("goal release", "a goal no session holds is already released",
		goalRepeatWitness(nil, nil, "is already released",
			"goal", "release", bedGoal, "--reason", "handing it back", "--lineage", "m1"))
	stateful("goal accept-risk", "the same person's acceptance of the same finding is success with no record",
		goalRepeatWitness(nil, func(bed *intentBed) { writeCriticChain(bed.t, bed.root()) }, "is already accepted by",
			"goal", "accept-risk", bedGoal, "--finding", "S-1", "--review", "critic-r2", "--reason", "bounded exposure"))
	stateful("goal pin", "the pin the goal already has is success with no record",
		goalRepeatWitness(nil, nil, "already reads exactly that",
			"goal", "pin", bedGoal, "mac-cli"))
	stateful("goal prioritize", "the rank the goal already holds is success with no record",
		goalRepeatWitness(nil, nil, "is already priority 1",
			"goal", "prioritize", bedGoal, "1"))
	stateful("goal reopen", "an open goal with that next step is already reopened",
		goalRepeatWitness(nil, runOrFail("goal", "done", bedGoal, "--reason", "shipped and verified", "--lineage", "m1"), "is already open",
			"goal", "reopen", bedGoal, "--next", "Cover the fleet commands.", "--lineage", "m1"))
	stateful("goal abandon", "an abandoned goal abandoned again says since when and why",
		goalRepeatWitness(makeQueued, nil, "is already abandoned",
			"goal", "abandon", bedGoal, "--reason", "superseded by the new design"))
	stateful("goal block", "a blocker already named is success with no record",
		goalRepeatWitness(makeQueued, addOtherGoal, "already waits for other-goal",
			"goal", "block", bedGoal, "--on", "other-goal"))
	stateful("goal unblock", "a blocker already absent is success with no record",
		goalRepeatWitness(makeQueued, addOtherGoal, "already does not wait for other-goal",
			"goal", "unblock", bedGoal, "--on", "other-goal"))
	stateful("goal unapprove", "a goal with no approval is already unapproved",
		goalRepeatWitness(nil, nil, "is already not approved",
			"goal", "unapprove", bedGoal, "--reason", "the design changes first"))
	stateful("goal split", "a parent already split into exactly these members is success with no record (witnessed at the owner: the router's split needs the MAIN checkout-lease holder)",
		witnessSplitAtOwner)
	stateful("goal group", "a goal already in that group is success with no record",
		goalRepeatWitness(makeQueued, addOtherGoal, "is already in group other-goal",
			"goal", "group", bedGoal, "other-goal"))
	stateful("goal ungroup", "a goal in no group is already ungrouped",
		goalRepeatWitness(makeQueued, runOrFail("goal", "group", bedGoal, "some-arc"), "is already in no group",
			"goal", "ungroup", bedGoal))
	stateful("goal notes", "a note already tracked, or the same close of a closed note, is success with no record; reading notes is a read",
		goalRepeatWitness(nil, nil, "already tracked",
			"goal", "notes", bedGoal, "--read", "r2", "--add", "help wraps at 80 columns"))
	stateful("goal sync", "a clean journal or an agreeing ledger is success with nothing to do; the preview is a read",
		goalRepeatWitness(nil, nil, "nothing was recovered",
			"goal", "sync", "--recover"))
	stateful("grant add", "the same live power of attorney from the same person is success with no second entry",
		goalRepeatWitness(nil, nil, "already grants exactly this",
			"grant", "add", "--tiers", "1", "--acts", "approve", "--until", "2026-09-05"))
	stateful("grant revoke", "a revoked power of attorney revoked again is success with no record",
		func(t *testing.T) {
			bed := newIntentBed(t, false, nil)
			code, granted := bed.runJSON(bed.owners(), "grant", "add", "--tiers", "1", "--acts", "approve", "--until", "2026-09-05")
			if code != 0 || len(granted.Targets) != 1 {
				t.Fatalf("grant = %d %+v", code, granted)
			}
			entry := granted.Targets[0].ID
			if code, revoked := bed.runJSON(bed.owners(), "grant", "revoke", entry); code != 0 {
				t.Fatalf("revoke = %d %+v", code, revoked)
			}
			bed.expectRepeatRecordsNothing("is already revoked", "grant", "revoke", entry)
		})
}

// witnessSplitAtOwner splits the bed goal twice through the split owner, as
// the lease holder's router call does, and requires the second split to be an
// unchanged success that published and journaled nothing.
func witnessSplitAtOwner(t *testing.T) {
	bed := newIntentBed(t, false, makeQueued)
	endpoint, err := bed.dependencies().endpoint(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	members := []goal.MemberDraft{
		{ID: bedGoal + "-one", Intent: "Deliver the first part.", NextStep: "Build part one."},
		{ID: bedGoal + "-two", Intent: "Deliver the second part.", NextStep: "Build part two."},
	}
	ratification := goal.SplitRatification{Tier: goal.RatifierMain, MainID: "main-1", ClaimEpoch: 1, DraftSHA256: goal.SplitDraftSHA256(bedGoal, members)}
	request := func(ulid string) goal.VerbRequest {
		return goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Machine: "mac-cli", Lineage: "m1"}, Ulid: ulid,
			Now: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), ClaimEpoch: 1,
			ParkBranchCheck: func(string, string) (string, error) { return "", nil }}
	}
	first, err := goal.Split(request("01J5X0000000000000000QDS01"), bedGoal, members, ratification, nil)
	if err != nil || first.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("the first split = %+v %v", first, err)
	}
	publications, journal, files := bed.publications(), goalJournalNames(t, bed.root()), goalLedgerBytes(bed)
	second, err := goal.Split(request("01J5X0000000000000000QDS02"), bedGoal, members, ratification, nil)
	if err != nil || second.Outcome != goal.OutcomeAbandoned || !second.Unchanged || !strings.Contains(second.Detail, "is already split into") {
		t.Fatalf("the repeated split = %+v %v; want an unchanged success", second, err)
	}
	if bed.publications() != publications || strings.Join(goalJournalNames(t, bed.root()), ",") != strings.Join(journal, ",") || goalLedgerBytes(bed) != files {
		t.Fatal("the repeated split recorded something")
	}
}

// goal budget --id G BOX names the goal by --id and the box by the one word
// left, as the help's "--id is an alternative to naming it first" says.
func TestIntentBudgetTakesTheGoalByID(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	before := bed.publications()
	code, result := bed.runJSON(bed.terminalOwners(), "goal", "budget", "--id", bedGoal, "4h/4/240m/2/3", "--fixture-human-authority", "--lineage", "m1")
	if code != 0 || result.Outcome != intentUnchanged || !strings.Contains(result.Summary, "already has exactly this budget") || bed.publications() != before {
		t.Fatalf("goal budget --id G BOX = %d %+v", code, result)
	}
}

// A budget request that completes to the box the goal already carries is a
// repeat whose effect holds: success with nothing recorded, not a refusal
// with no way forward.
func TestIntentBudgetCompletingToTheCarriedBoxIsUnchanged(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	before := bed.publications()
	code, result := bed.runJSON(bed.terminalOwners(), "goal", "budget", bedGoal, "4h/4/240m/2/3/9", "--fixture-human-authority", "--lineage", "m1")
	if code != 0 || result.Outcome != intentUnchanged || !strings.Contains(result.Summary, "already carries the box 4h/4/240m/2/3") || bed.publications() != before {
		t.Fatalf("a box completing to the carried box = %d %+v", code, result)
	}
}

// Rule H1: a person at a terminal that is not enrolled releases a claim by
// naming themself; the stopping act's own terminal-grade proof attributes it.
func TestIntentReleaseAtAnUnenrolledTerminal(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	reader := goalSyncTerminalReader(t, bed.root(), "ttys:fixture_release")
	bed.facts.reader = &reader
	owners := bed.owners()
	owners.prove = unprovable
	code, result := bed.runJSON(owners, "goal", "release", bedGoal, "--reason", "the seat is gone")
	if code == 0 || result.Next == nil || !strings.HasSuffix(shellCommand(result.Next.Argv), "--by NAME") {
		t.Fatalf("an unnamed release at an unenrolled terminal must guide to --by: %d %+v", code, result)
	}
	code, result = bed.runJSON(owners, "goal", "release", bedGoal, "--reason", "the seat is gone", "--by", "Wido")
	if code != 0 || result.Outcome != intentConfirmed || bed.goalFile(bedGoal).State == goal.StateClaimed {
		t.Fatalf("a named release at an unenrolled terminal = %d %+v", code, result)
	}
}
