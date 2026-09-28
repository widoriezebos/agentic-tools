package goal

import (
	"strings"
	"testing"
	"time"
)

// The goal CLI shell bed's carry scenarios (goal-cli-fixtures.sh carry-word
// and carried-record, with prepare_carried_record_fixture) ported to the
// owners over the fake goal repository, whose code history carries the
// Goal-Transaction and Carry trailers the shell bed pushed to its origin. The
// human acts carry the fixture-only proof, as --fixture-human-authority did.

// TestGoalCLICarryWord is the carry-word scenario at the carry owner: a
// format-1 ledger asks for the one-way raise, the raising word opens one carry
// on the seat with no debt and leaves the ledger at format 2, a second open
// word asks carry-cap-reached naming the standing word, and --supersede mints
// a distinct word. (The single-machine ask is TestGoalCLICarryAsks in
// cmd/metasystem, through the checkout's local sync mode.)
func TestGoalCLICarryWord(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC)
	endpoint, _, human := carryBedFor(t, base)
	root := endpoint.Root
	proof := goalHumanProof(t, root, human.Now)
	carry := func(ulid string, minutes int, args CarryArgs) (string, error) {
		request := carryVerb(human, ulid, minutes)
		result, err := Carry(request, args, proof)
		if err == nil && result.Outcome != OutcomeConfirmed {
			t.Fatalf("carry %s = %+v", ulid, result)
		}
		return request.opid(), err
	}
	word := CarryArgs{Goal: "g", Workspace: strings.Repeat("a", 40), Past: "missing-declaration",
		Why: "fixture format fence", Expires: human.Now.Add(2 * time.Hour)}

	_, err := carry("01J5X00000000000000000E001", 0, word)
	requireCarryAsk(t, err, "carry-format-required")
	if tree, _ := acceptedTreeForEndpoint(t, endpoint); tree.Root.FormatVersion != "1" {
		t.Fatalf("the format ask changed the ledger format to %s", tree.Root.FormatVersion)
	}

	raise := word
	raise.Why, raise.RaiseFormat = "fixture raises the carry format", true
	first, err := carry("01J5X00000000000000000E002", 1, raise)
	if err != nil {
		t.Fatalf("goal carry --raise-format: %v", err)
	}
	// The fake code origin answers the word's anchor query as the shell bed's
	// origin did: the word's transaction commit, and no carried commit yet.
	declareWordHistory(t, endpoint, first, "")
	// The counselor facts goal carry prints: open carries on the seat, the
	// carry debt and the ledger format, read as the command reads them.
	tree, tip := acceptedTreeForEndpoint(t, endpoint)
	now := human.Now.Add(time.Minute)
	open, err := openCarryWordsFor(endpoint, tree, tip, "mac-a", now)
	if err != nil || len(open) != 1 || open[0].History.Opid != first {
		t.Fatalf("open carries on seat mac-a = %+v, %v; want 1 (%s)", open, err, first)
	}
	counts, err := countCarriesFor(endpoint, tree, tip, now)
	if err != nil || counts.Debt != 0 || counts.Inflight != 0 {
		t.Fatalf("carry debt = %+v, %v; want obligations=0 inflight=0", counts, err)
	}
	if tree.Root.FormatVersion != "2" {
		t.Fatalf("ledger format: %s, want 2", tree.Root.FormatVersion)
	}

	capped := word
	capped.Why = "fixture proves the cap"
	_, err = carry("01J5X00000000000000000E003", 2, capped)
	requireCarryAsk(t, err, "carry-cap-reached")
	if !strings.Contains(err.Error(), first) {
		t.Fatalf("a second open carry did not ask with the existing word %s: %v", first, err)
	}

	successor := word
	successor.Why, successor.Supersede = "fixture supersedes the word", first
	replacement, err := carry("01J5X00000000000000000E004", 3, successor)
	if err != nil || replacement == first {
		t.Fatalf("supersede did not mint a distinct word: %q (was %q) %v", replacement, first, err)
	}
}

// gcliCarryLanded prepares the shell bed's carried record: a raised carry
// word on g, the seat's carrying reservation, the carried commit on the code
// origin under the word, and the record goal carried rebuilds from that
// commit's trailers. It returns the endpoint, the seat request, the word, the
// commit and the rebuilt record.
func gcliCarryLanded(t *testing.T, base time.Time) (Endpoint, VerbRequest, string, string, CarriedArgs) {
	t.Helper()
	endpoint, other, human := carryBedFor(t, base)
	word := CarryArgs{Goal: "g", Workspace: strings.Repeat("a", 40), Past: "missing-declaration",
		Why: "fixture carries one named refusal", Expires: human.Now.Add(2 * time.Hour), RaiseFormat: true}
	carry := carryVerb(human, "01J5X00000000000000000E101", 0)
	if result, err := Carry(carry, word, goalHumanProof(t, endpoint.Root, carry.Now)); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("goal carry: %+v %v", result, err)
	}
	ref := carry.opid()
	declareWordHistory(t, endpoint, ref, "")
	seat := carryVerb(human, "01J5X00000000000000000E102", 1)
	seat.Actor.Human = ""
	project := strings.Repeat("b", 40)
	result, reservation, err := Carrying(seat, CarryingArgs{Goal: "g", ApprovedRef: ref, Workspace: word.Workspace, Project: project})
	if err != nil || result.Outcome != OutcomeConfirmed || reservation == "" {
		t.Fatalf("goal carrying: %+v %q %v", result, reservation, err)
	}
	_, ledger := acceptedTreeForEndpoint(t, endpoint)
	commit := fakeCodeCarryCommit(t, other, ref)
	declareWordHistory(t, endpoint, ref, commit)
	record := CarriedArgs{
		Goal: "g", ApprovedRef: ref, Commit: commit, Project: project, Workspace: word.Workspace,
		Past: word.Past, Battery: "green", Judge: "live", JudgeDigest: strings.Repeat("d", 64), Ledger: ledger, By: "human:Wido",
		Outcome: "landed",
	}
	carried := carryVerb(seat, "01J5X00000000000000000E103", 2)
	carried.Endpoint.ConfigureCarriedCounselorAppend(func(string, string, HistoryLine, time.Time) error { return nil })
	if result, err := CarriedFromCommit(carried, record); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("goal carried --rebuild-from-commit: %+v %v", result, err)
	}
	return endpoint, carried, ref, commit, record
}

// TestGoalCLICarryRecord is the carried-record scenario: goal carried writes
// the approvedRef row for the carry word, the exact open review obligation on
// the carried commit and one budget exception, and a replay of the same
// record does not advance the accepted ledger.
func TestGoalCLICarryRecord(t *testing.T) {
	t.Parallel()
	endpoint, carried, ref, commit, record := gcliCarryLanded(t, time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC))
	tree, tip := acceptedTreeForEndpoint(t, endpoint)
	rendered := string(RenderFile(tree.Live["g"]))
	if !strings.Contains(rendered, "approvedRef="+ref) {
		t.Fatalf("goal carried wrote no row for the carry word %s:\n%s", ref, rendered)
	}
	obligation := "finding=carried:" + commit + " chain=human-carried artifact=\"commit:" + commit + "\" test=\"pending\" state=open"
	if !strings.Contains(rendered, obligation) {
		t.Fatalf("goal carried wrote no exact review obligation %q:\n%s", obligation, rendered)
	}
	if !strings.Contains(rendered, "\n- BudgetExceptions: 1\n") {
		t.Fatalf("goal carried did not count its budget exception:\n%s", rendered)
	}

	replay := carryVerb(carried, "01J5X00000000000000000E104", 1)
	replay.Endpoint.ConfigureCarriedCounselorAppend(func(string, string, HistoryLine, time.Time) error { return nil })
	result, err := CarriedFromCommit(replay, record)
	if err != nil || result.Outcome != OutcomeConfirmed || result.Detail != "idempotent" {
		t.Fatalf("goal carried replay = %+v %v; want confirmed idempotent", result, err)
	}
	if after := acceptedTipForEndpoint(t, endpoint); after != tip {
		t.Fatalf("goal carried replay advanced the accepted ledger: %s -> %s", tip, after)
	}
}
