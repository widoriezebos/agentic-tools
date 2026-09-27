package main

// The hint rides the two acts that make a goal someone's work — opening it and
// claiming it — and it rides them on standard error. These tests drive the
// real routes over captured streams, because the contract is about which
// stream carries what: a caller parses the standard output, a person reads the
// standard error.

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// designHintLine is the line as a person reads it, written out here rather
// than composed from the constant the code prints: a test that built the
// string the same way would survive a rewrite of both.
const designHintLine = "A design for %s is a record: " +
	"see docs/design/design-obligation-gate.md, A design is a record.\n"

func designHintFor(id string) string { return strings.Replace(designHintLine, "%s", id, 1) }

// designHintStreams are the real owner dependencies writing to this test's own
// standard output and standard error: the contract is which stream carries
// what, and a process-wide capture would also collect whatever a parallel test
// prints while it is open.
func designHintStreams() (syncRequestDependencies, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	dependencies := defaultSyncRequestDependencies()
	dependencies.stdout, dependencies.stderr = &stdout, &stderr
	return dependencies, &stdout, &stderr
}

// designHintConfirmedJSON is the half of the contract the standard output
// owns: the confirmation stays one JSON object, whatever is said beside it.
func designHintConfirmedJSON(t *testing.T, stdout string) {
	t.Helper()
	var confirmation map[string]any
	if err := json.Unmarshal([]byte(stdout), &confirmation); err != nil {
		t.Fatalf("the standard output is not one JSON object: %v: %q", err, stdout)
	}
	if confirmation["outcome"] != string(goal.OutcomeConfirmed) {
		t.Fatalf("the act did not confirm: %q", stdout)
	}
}

// A confirmed open says it twice: once as the JSON a caller parses, once as
// the sentence a person reads. Neither is on the other's stream.
func TestDesignRecordHintFollowsAConfirmedOpen(t *testing.T) {
	t.Parallel()

	root := syncedClaimedGoalFixture(t)
	dependencies, out, errOut := designHintStreams()
	code, handled := trySyncMutationWithDependencies("open", []string{
		"--root", root, "--id", "designed-goal",
		"--intent", "Unblock the standing goal.", "--next", "Design it.",
		"--risk", "severity=1,novelty=1,exposure=1,accumulation=1", "--basis", "fixture risk",
		"--lineage", "m1", "--blocks", "standing-validation",
	}, goalCommandNow, dependencies, goalParkBranchCheck)
	stdout, stderr := out.String(), errOut.String()
	if !handled || code != 0 {
		t.Fatalf("goal open handled=%v code=%d stdout=%q stderr=%q", handled, code, stdout, stderr)
	}
	designHintConfirmedJSON(t, stdout)
	if stderr != designHintFor("designed-goal") {
		t.Fatalf("the standard error carried %q", stderr)
	}
}

// Claiming is the other moment the hint belongs to: the work starts there for
// the seat that did not open the goal. The engine's own call is the seam — a
// confirmed claim needs an authenticated lease holder's claim epoch, which
// this process is not — so the wrapper is driven over a confirming stub, which
// is the code that decides what is printed and on which stream.
func TestDesignRecordHintFollowsAConfirmedClaim(t *testing.T) {
	t.Parallel()

	root := syncedClaimedGoalFixture(t)
	confirm := func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.PublishResult{Outcome: goal.OutcomeConfirmed, Tip: "fixture-tip"}, nil
	}
	dependencies, out, errOut := designHintStreams()
	code := runSyncOnlyWithDependencies("claim", confirm, nil, dependencies, "id")(
		[]string{"--root", root, "--id", "standing-validation", "--lineage", "m1"})
	stdout, stderr := out.String(), errOut.String()
	if code != 0 {
		t.Fatalf("the claim wrapper returned %d: stdout %q stderr %q", code, stdout, stderr)
	}
	designHintConfirmedJSON(t, stdout)
	if stderr != designHintFor("standing-validation") {
		t.Fatalf("the standard error carried %q", stderr)
	}

	// The hint belongs to claiming, not to every verb that shares the
	// wrapper: releasing a goal confirms the same way and says nothing.
	dependencies, out, errOut = designHintStreams()
	code = runSyncOnlyWithDependencies("release", confirm, nil, dependencies, "id")(
		[]string{"--root", root, "--id", "standing-validation", "--lineage", "m1"})
	stdout, stderr = out.String(), errOut.String()
	if code != 0 || stderr != "" {
		t.Fatalf("release carried the hint: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

// A refused act is told nothing about designs: the refusal is what the seat
// must read, and a hint beside it would be advice about work that did not
// start. Both refusals are real ones — the retired open --claim, and a claim
// this process cannot make.
func TestDesignRecordHintIsSilentWhenTheActIsRefused(t *testing.T) {
	t.Parallel()

	root := syncedClaimedGoalFixture(t)
	dependencies, _, errOut := designHintStreams()
	code, handled := trySyncMutationWithDependencies("open", []string{
		"--root", root, "--id", "claimed-on-open",
		"--intent", "Open and claim in one act.", "--next", "Design it.",
		"--risk", "severity=1,novelty=1,exposure=1,accumulation=1", "--basis", "fixture risk",
		"--lineage", "m1", "--blocks", "standing-validation", "--claim",
		"--elapsed-limit", "8h", "--attempt-limit", "2", "--reserved-job-minutes-limit", "120",
		"--active-job-limit", "1", "--review-round-limit", "0",
	}, goalCommandNow, dependencies, goalParkBranchCheck)
	stderr := errOut.String()
	if !handled || code == 0 {
		t.Fatalf("open --claim confirmed: handled=%v code=%d stderr=%q", handled, code, stderr)
	}
	if strings.Contains(stderr, "is a record") {
		t.Fatalf("a refused open carried the hint: %q", stderr)
	}

	dependencies, out, errOut := designHintStreams()
	code = runSyncOnlyWithDependencies("claim", claimGoalOwner, nil, dependencies, "id")(
		[]string{"--root", root, "--id", "standing-validation", "--lineage", "m1"})
	stdout, stderr := out.String(), errOut.String()
	if code == 0 {
		t.Fatalf("the claim confirmed; the fixture no longer refuses: stdout %q", stdout)
	}
	if strings.Contains(stderr, "is a record") {
		t.Fatalf("a refused claim carried the hint: %q", stderr)
	}
}
