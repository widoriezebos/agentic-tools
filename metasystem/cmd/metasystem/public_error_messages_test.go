package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// These tests hold the public refusal texts the 2026-09-28 message audit
// found false, misleading or jargon-laden (the EM rows outside U9b). Each
// runs the public command in process on the existing per-test fakes; no Git
// process runs.

type unfetchedIntentRepository struct{ goal.Repository }

func (unfetchedIntentRepository) Accepted() (string, bool, error) { return "", false, nil }

// EM-25: a checkout that has not fetched the ledger was told it "cannot
// project the accepted goal ledger: no accepted tree; the first fetch or
// the migration bootstraps it", and not which command fetches it.
func TestUnfetchedLedgerNamesTheFetch(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	owners := bed.owners()
	endpoint := owners.dependencies.endpoint
	owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
		e, err := endpoint(root)
		e.Repository = unfetchedIntentRepository{e.Repository}
		return e, err
	}
	code, result := bed.runJSON(owners, "goal", "list")
	if code != 1 || result.Summary != "this checkout has not fetched the goal ledger yet; nothing was read" {
		t.Fatalf("goal list = %d %q", code, result.Summary)
	}
	if result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "goal", "list", "--fetch"}) {
		t.Fatalf("goal list next = %+v", result.Next)
	}
}

// EM-28 and EM-29: grant revoke with no grant sent the reader to
// scrollback; goal open with nothing gave no example and listed G last.
func TestBareGrantRevokeAndGoalOpenNameTheWayForward(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	owners := bed.owners()
	code, result := bed.runJSON(owners, "grant", "revoke")
	if code != 2 || result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "grant", "list"}) {
		t.Fatalf("grant revoke = %d %+v", code, result)
	}
	code, result = bed.runJSON(owners, "goal", "open")
	if code != 2 || result.Summary != "a new goal needs a goal id, --basis, --intent, --next, --risk; nothing was done" {
		t.Fatalf("goal open = %d %q", code, result.Summary)
	}
	want := []string{"metasystem", "goal", "open", "GOAL", "--json", "--basis", "TEXT", "--intent", "TEXT", "--next", "TEXT", "--risk", "severity=N,novelty=N,exposure=N,accumulation=N"}
	if result.Next == nil || !slices.Equal(result.Next.Argv, want) || result.Decision != "" {
		t.Fatalf("goal open next = %+v, decision %q", result.Next, result.Decision)
	}
}

// EM-46 and EM-47: the bare goal show said "goals it could take" and then
// repeated itself with "needed first: name the goal"; an unknown goal
// permission repeated the permission list as its needed-first line.
func TestGoalRefusalsDoNotRepeatThemselves(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	owners := bed.owners()
	code, _, stderr := bed.run(owners, "goal", "show")
	if code != 2 || !strings.Contains(stderr, "goals you can name: ") || strings.Contains(stderr, "could take") || strings.Contains(stderr, "needed first") {
		t.Fatalf("goal show = %d %q", code, stderr)
	}
	code, _, stderr = bed.run(owners, "goal", "allow", "standing-validation", "something", "--reason", "x")
	if code != 2 || !strings.Contains(stderr, `"something" is not a goal permission; the permissions are: stop-test-changes; nothing was done`) || strings.Contains(stderr, "needed first") {
		t.Fatalf("goal allow = %d %q", code, stderr)
	}
}

// EM-18: test wait on an unknown proof attempt pointed at work status
// --all, which lists launches and dispatch jobs and no proof attempts.
func TestUnknownProofAttemptSaysWhereItsIdComesFrom(t *testing.T) {
	t.Parallel()
	b := newReferenceBed(t)
	inv := b.invocation("test wait")
	_, problem := inv.resolveWorkRef("nosuch", inv.command.accepts)
	if problem == nil || problem.Summary != "no proof attempt names nosuch; nothing was done" {
		t.Fatalf("test wait nosuch = %+v", problem)
	}
	if len(problem.next) != 0 || problem.Decision != "nothing to wait on; metasystem test run prints the reference to wait on" {
		t.Fatalf("test wait nosuch points to %v (%q)", problem.next, problem.Decision)
	}
	// A search that includes launches or jobs still points to work status.
	work := b.invocation("work status")
	if _, problem := work.resolveWorkRef("j1:nosuch", work.command.accepts); problem == nil || !slices.Equal(problem.next, []string{"metasystem", "work", "status", "--all"}) {
		t.Fatalf("work status nosuch = %+v", problem)
	}
}

// EM-38: helm take from a shell no person opened leaked the refusal code
// and doubled its prefix ("helm take: helm take refused: ...").
func TestHelmTakeRefusalSaysWhyInPlainTerms(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		invoker int64
		reason  string
	}{
		{80, "an agent started this shell"},
		{99, "this shell isn't attached to a terminal"},
	} {
		bed := newHelmBed(t, test.invoker, true)
		code, out := bed.run("helm", "take", "--reason", "by hand")
		want := "✗ " + test.reason + ", so the helm wasn't taken\n  → metasystem helm take --reason 'by hand'  in a terminal you opened yourself\n"
		if code != 3 || !strings.Contains(out, want) || strings.Contains(out, "helm take refused") {
			t.Errorf("invoker %d: exit %d, output:\n%s\nwant %q", test.invoker, code, out, want)
		}
	}
}
