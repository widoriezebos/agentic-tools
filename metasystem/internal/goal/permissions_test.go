package goal

// Goal permissions (verbs-match-intent, Wido 2026-09-28): goal allow G NAME
// records a standing allowance on the goal and goal disallow G NAME
// withdraws it. stop-test-changes is stored as the sealed line
// "- StopSurface: moves". Allowing is a person's act under the proof a
// lowering takes; disallowing is anyone's; a repeat of the value the goal
// already holds is success with no second record (R-129-ui).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func stopTestChanges(allowed bool) *PermissionChange {
	return &PermissionChange{Name: PermissionStopTestChanges, Allowed: allowed}
}

func TestPermissionTableNamesStopTestChangesAndRefusesUnknownNames(t *testing.T) {
	t.Parallel()
	permission, err := LookupPermission("stop-test-changes")
	if err != nil || permission.Words != "stop-test changes" {
		t.Fatalf("stop-test-changes = %+v %v", permission, err)
	}
	file := &GoalFile{}
	if permission.Holds(file) || len(AllowedWords(file)) != 0 {
		t.Fatal("an empty record holds a permission")
	}
	permission.Set(file, true)
	if !file.StopSurfaceMoves || strings.Join(AllowedWords(file), ",") != "stop-test changes" {
		t.Fatalf("the permission is not stored as StopSurface: moves: %+v", file)
	}
	if _, err := LookupPermission("stop-surface"); err == nil || !strings.Contains(err.Error(), "the permissions are: stop-test-changes") {
		t.Fatalf("an unknown permission = %v; want the known names listed", err)
	}
}

func TestEditAllowsAndDisallowsAGoalPermission(t *testing.T) {
	t.Parallel()
	endpoint, _ := fakeGoalEndpoint(t)
	root := endpoint.Root
	if result, err := Open(verbReqFor(endpoint, "01J5X00000000000000000SS00", "mac-a"),
		"stop-moves", "Move one Stop assertion.", OriginMain, "Move it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	read := func(tip string) *GoalFile {
		t.Helper()
		tree, err := loadTreeFor(endpoint, tip)
		if err != nil {
			t.Fatal(err)
		}
		return tree.Live["stop-moves"]
	}

	// An agent's allow is refused, and the refusal names the command a person runs.
	agent, err := Edit(verbReqFor(endpoint, "01J5X00000000000000000SS10", "mac-a"), "stop-moves",
		EditFields{Permission: stopTestChanges(true), Why: "the hook entry moved"})
	if err != nil || agent.Outcome != OutcomeRejected || !strings.Contains(agent.Detail, "person's act") ||
		!strings.Contains(agent.Detail, "metasystem goal allow stop-moves stop-test-changes --reason TEXT") {
		t.Fatalf("an agent's allow = %+v %v; want a refusal naming the person's command", agent, err)
	}
	// A person's name without a proof is refused the same way.
	unproven := verbReqFor(endpoint, "01J5X00000000000000000SS15", "mac-a")
	unproven.Actor.Human = "Wido"
	if result, err := Edit(unproven, "stop-moves", EditFields{Permission: stopTestChanges(true), Why: "x"}); err != nil || result.Outcome != OutcomeRejected {
		t.Fatalf("an unproven allow = %+v %v", result, err)
	}
	// An unknown permission is refused before any transaction.
	if _, err := Edit(unproven, "stop-moves", EditFields{Permission: &PermissionChange{Name: "anything", Allowed: true}}); err == nil ||
		!strings.Contains(err.Error(), "stop-test-changes") {
		t.Fatalf("an unknown permission = %v", err)
	}

	human := verbReqFor(endpoint, "01J5X00000000000000000SS20", "mac-a")
	human.Actor.Human = "Wido"
	proof := sessionProofForTest(t, root, human.Now)
	allowed, err := Edit(human, "stop-moves", EditFields{Permission: stopTestChanges(true), Why: "the hook entry moved", Proof: proof})
	if err != nil || allowed.Outcome != OutcomeConfirmed {
		t.Fatalf("the person's allow: %+v %v", allowed, err)
	}
	file := read(allowed.Tip)
	if !file.StopSurfaceMoves || !strings.Contains(string(RenderFile(file)), "\n- StopSurface: moves\n") {
		t.Fatalf("the allow did not record StopSurface: moves:\n%s", RenderFile(file))
	}
	last := file.History[len(file.History)-1]
	if last.Verb != "edit" || last.Actor != "human:Wido" || last.Reason != "Allowed: stop-test-changes why=the hook entry moved" {
		t.Fatalf("the allow's history line = %+v", last)
	}
	if parsed, problems := ParseFile(RenderFile(file)); len(problems) != 0 || !parsed.StopSurfaceMoves {
		t.Fatalf("the sealed record does not parse back: %v", problems)
	}
	if record, problems := StopSurfaceGoalReader("stop-moves", RenderFile(file)); len(problems) != 0 || !record.StopSurfaceMoves {
		t.Fatalf("the Stop-surface reader does not see the allowance: %+v %v", record, problems)
	}
	revision, lines := file.Revision, len(file.History)

	// The same allow again holds already: no second record, from either hand.
	humanRepeat := human
	humanRepeat.Ulid = "01J5X00000000000000000SS31"
	for _, repeat := range []VerbRequest{humanRepeat, verbReqFor(endpoint, "01J5X00000000000000000SS30", "mac-a")} {
		again, err := Edit(repeat, "stop-moves", EditFields{Permission: stopTestChanges(true), Why: "said twice", Proof: proof})
		if err != nil || again.Outcome != OutcomeAbandoned || !strings.Contains(again.Detail, "goal stop-moves is already allowed stop-test changes") {
			t.Fatalf("the repeated allow = %+v %v; want an explicit no-op", again, err)
		}
		if after := read(again.Tip); after.Revision != revision || len(after.History) != lines {
			t.Fatalf("the repeated allow wrote a record: revision %d, %d lines", after.Revision, len(after.History))
		}
	}

	// Disallowing is anyone's act.
	removed, err := Edit(verbReqFor(endpoint, "01J5X00000000000000000SS40", "mac-a"), "stop-moves",
		EditFields{Permission: stopTestChanges(false)})
	if err != nil || removed.Outcome != OutcomeConfirmed {
		t.Fatalf("an agent's disallow: %+v %v", removed, err)
	}
	file = read(removed.Tip)
	if file.StopSurfaceMoves || strings.Contains(string(RenderFile(file)), "- StopSurface: moves") ||
		file.History[len(file.History)-1].Reason != "Disallowed: stop-test-changes" {
		t.Fatalf("the disallow left the permission or no reason:\n%s", RenderFile(file))
	}
	again, err := Edit(verbReqFor(endpoint, "01J5X00000000000000000SS50", "mac-a"), "stop-moves",
		EditFields{Permission: stopTestChanges(false)})
	if err != nil || again.Outcome != OutcomeAbandoned || !strings.Contains(again.Detail, "goal stop-moves is already not allowed stop-test changes") {
		t.Fatalf("the repeated disallow = %+v %v; want an explicit no-op", again, err)
	}

	// A reason is one line.
	multiline := verbReqFor(endpoint, "01J5X00000000000000000SS60", "mac-a")
	multiline.Actor.Human = "Wido"
	if _, err := Edit(multiline, "stop-moves", EditFields{Permission: stopTestChanges(true), Why: "one\ntwo", Proof: proof}); err == nil {
		t.Fatal("a multi-line reason was accepted")
	}
}

// Recovery rebuilds a stranded disallow through the real verb and refuses to
// rebuild an allow, which is proof-bearing.
func TestRecoveryRebuildsADisallowAndRefusesAnAllow(t *testing.T) {
	t.Parallel()
	endpoint, _ := fakeGoalEndpoint(t)
	if res, err := Open(verbReqFor(endpoint, "01J5X00000000000000000SR00", "mac-a"), "target", "Work.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	allowOpid := Opid("01J5X00000000000000000SR10", "mac-a", "lin-1")
	stored := func(value string) Intent {
		return Intent{Verb: "edit", Targets: []string{"target"}, Deltas: []FieldDelta{{Target: "target", Field: "permission", New: value}}}
	}
	strandEntry(t, endpoint.Root, allowOpid, PhaseCreated, stored("stop-test-changes=allowed"))
	if _, err := Recover(endpoint); err != nil {
		t.Fatal(err)
	}
	p, err := Project(endpoint, true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if p.Tree.Live["target"].StopSurfaceMoves {
		t.Fatal("recovery replayed an allow from journal text")
	}
	entry := func(value string) Entry {
		return Entry{Opid: allowOpid, Machine: "mac-a", Lineage: "lin-1", Intent: stored(value)}
	}
	if _, err := requestForEntry(endpoint, entry("stop-test-changes=allowed")); err == nil || !strings.Contains(err.Error(), "proof-bearing") ||
		!strings.Contains(err.Error(), "metasystem goal allow target stop-test-changes") {
		t.Fatalf("a stored allow rebuilt: %v", err)
	}
	for _, invalid := range []string{"stop-test-changes=sometimes", "anything=allowed", "stop-test-changes"} {
		if _, err := requestForEntry(endpoint, entry(invalid)); err == nil {
			t.Fatalf("a stored invalid permission change %q rebuilt", invalid)
		}
	}
	if _, err := requestForEntry(endpoint, entry("stop-test-changes=disallowed")); err != nil {
		t.Fatalf("a stored disallow did not rebuild: %v", err)
	}
}

// A person's hand edit of the StopSurface line comes back through goal sync
// as the allow or disallow it is: an allow under a proof the approval gate
// admits, a disallow under any name. A named hand without a proof is refused
// by the permission's name and the command that grants it.
func TestReconcileMapsAHandEditedPermissionToAllowAndDisallow(t *testing.T) {
	t.Parallel()
	a, _, endpoint := newFakeReconcileBed(t)
	path := filepath.Join(a, "plans", "goals", "editable.md")
	handEdit := func(from, to string) {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		edited := strings.Replace(string(raw), from, to, 1)
		if edited == string(raw) {
			t.Fatalf("the record does not carry %q:\n%s", from, raw)
		}
		if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	handEdit("\n- Intent: ", "\n- StopSurface: moves\n- Intent: ")

	before := acceptedTipForEndpoint(t, endpoint)
	unproven, err := reconcileForTest(t, humanReconcileReqForEndpoint(endpoint, "01J5X00000000000000000PH10"))
	if err != nil {
		t.Fatal(err)
	}
	if len(unproven.Rows) != 1 || unproven.Rows[0].Verb != "allow" {
		t.Fatalf("the hand edit did not map to allow: %+v", unproven.Rows)
	}
	if unproven.Publish.Outcome == OutcomeConfirmed || !strings.Contains(unproven.Publish.Detail, "allowing stop-test changes is a person's act") ||
		!strings.Contains(unproven.Publish.Detail, "metasystem goal allow editable stop-test-changes --reason TEXT") {
		t.Fatalf("a hand allow without a proof = %+v", unproven.Publish)
	}
	if acceptedTipForEndpoint(t, endpoint) != before {
		t.Fatal("the refused reconcile moved the ledger")
	}

	proven := humanReconcileReqForEndpoint(endpoint, "01J5X00000000000000000PH11")
	proven.Actor.Human = "Wido"
	proven.Authority = sessionProofForTest(t, a, proven.Now)
	reconciled, err := reconcileForTest(t, proven)
	if err != nil || reconciled.Publish.Outcome != OutcomeConfirmed {
		t.Fatalf("a proven hand allow was refused: %+v %v", reconciled, err)
	}
	tree, err := loadTreeFor(endpoint, reconciled.Publish.Tip)
	if err != nil {
		t.Fatal(err)
	}
	held := tree.Live["editable"]
	if !held.StopSurfaceMoves {
		t.Fatalf("the hand allow did not land:\n%s", RenderFile(held))
	}
	if last := held.History[len(held.History)-1]; last.Verb != "edit" || last.Actor != "human:Wido" || last.Reason != "Allowed: stop-test-changes" {
		t.Fatalf("the hand allow's history line = %+v", last)
	}

	// The checkout now carries the published record; a hand disallow needs no proof.
	handEdit("\n- StopSurface: moves\n", "\n")
	disallowed, err := reconcileForTest(t, humanReconcileReqForEndpoint(endpoint, "01J5X00000000000000000PH20"))
	if err != nil || disallowed.Publish.Outcome != OutcomeConfirmed || len(disallowed.Rows) != 1 || disallowed.Rows[0].Verb != "disallow" {
		t.Fatalf("a hand disallow = %+v %v", disallowed, err)
	}
	tree, err = loadTreeFor(endpoint, disallowed.Publish.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live["editable"].StopSurfaceMoves {
		t.Fatal("the hand disallow did not land")
	}
}
