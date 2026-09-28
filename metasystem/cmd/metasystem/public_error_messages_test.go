package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// These tests hold the public refusal texts the 2026-09-28 message audit
// found false, misleading or jargon-laden (the EM rows outside U9b). Each
// runs the public command in process on the existing per-test fakes; no Git
// process runs.

func failingGoalAuthority(outcome string) goalAuthorityProver {
	return func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New(outcome)
	}
}

// EM-21: a person's act from a shell that is not the enrolled terminal
// leaked TERMINAL_NOT_REACHED and "ancestry", named the internal verb
// (goal set-pin, goal revoke) and offered --lineage, which cannot make an
// agent a person.
func TestPersonActRefusalUsesThePublicVerbAndPlainReason(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	owners := bed.owners()
	owners.prove = failingGoalAuthority(humanauthority.OutcomeTerminalMissing)
	for _, test := range []struct {
		args []string
		verb string
	}{
		{[]string{"goal", "pin", "standing-validation", "m1e"}, "goal pin"},
		{[]string{"grant", "revoke", "g-1"}, "grant revoke"},
	} {
		code, result := bed.runJSON(owners, test.args...)
		want := test.verb + " is a person's act at the enrolled terminal, and no enrolled person was proven here (no terminal was found above this shell); nothing was done"
		if code != 1 || result.Outcome != intentRefused || result.Summary != want {
			t.Errorf("%v = %d %q, want %q", test.args, code, result.Summary, want)
		}
		if result.Decision != "run it at the enrolled terminal; a person enrolls a terminal with metasystem system enroll --name NAME" {
			t.Errorf("%v decision = %q", test.args, result.Decision)
		}
	}
}

// EM-21: an act either an agent session or a person may perform, run from
// a shell that is neither, said "the enrolled-terminal proof failed:
// AGENT_IN_AUTHORITY_CHAIN".
func TestEitherActorRefusalSaysWhoMayActInPlainTerms(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	owners := bed.owners()
	owners.dependencies.ownerLineage = func() string { return "" }
	owners.dependencies.proveHuman = func(string, int64, humanauthority.Reader, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New(humanauthority.OutcomeAgent)
	}
	code, result := bed.runJSON(owners, "goal", "pause", "standing-validation", "--reason", "x")
	want := "cannot tell who runs goal pause: no agent session is named and no enrolled person was proven here (an agent started this shell); nothing was done"
	if code != 1 || result.Outcome != intentRefused || result.Summary != want {
		t.Fatalf("goal pause = %d %q, want %q", code, result.Summary, want)
	}
	if result.Decision != "a person runs it at the enrolled terminal (metasystem system enroll --name NAME enrolls one); an agent session names itself with --lineage LINEAGE or METASYSTEM_OWNER_LINEAGE" {
		t.Fatalf("goal pause decision = %q", result.Decision)
	}
}

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
	if code != 2 || result.Summary != "a new goal needs G, --basis, --intent, --next, --risk; nothing was done" {
		t.Fatalf("goal open = %d %q", code, result.Summary)
	}
	if result.Decision != "metasystem goal open G --intent TEXT --next TEXT --risk severity=N,novelty=N,exposure=N,accumulation=N --basis TEXT; the four risk answers and their basis are a judgement about this goal, not a default" {
		t.Fatalf("goal open decision = %q", result.Decision)
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

// EM-49: system adopt with no target did not say nothing was done.
func TestBareSystemAdoptSaysNothingWasDone(t *testing.T) {
	t.Parallel()
	command := mustIntentCommand(t, "system adopt")
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, []string{"--json"}, &stdout, &stderr, t.TempDir(), defaultIntentOwners())
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("not one JSON result: %v\n%s%s", err, stdout.String(), stderr.String())
	}
	if code != 2 || result.Summary != "name the repository to adopt into; nothing was done" {
		t.Fatalf("system adopt = %d %q", code, result.Summary)
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
	if len(problem.next) != 0 || problem.Decision != "a proof attempt's id is the proof:ID that metasystem test run printed when it started" {
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
		{99, "no terminal was found above this shell"},
	} {
		bed := newHelmBed(t, test.invoker, true)
		code, out := bed.run("helm", "take", "--reason", "by hand")
		want := "metasystem helm take: only a person at a terminal no agent started can take the helm, and this shell is not one (" + test.reason + "); nothing was done"
		if code != 3 || !strings.Contains(out, want) || strings.Contains(out, "helm take refused") {
			t.Errorf("invoker %d: exit %d, output:\n%s\nwant %q", test.invoker, code, out, want)
		}
	}
}
