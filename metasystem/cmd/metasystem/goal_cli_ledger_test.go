package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// These tests are the Go port of the goal CLI shell bed's ledger scenarios
// (risk-basis, labels-and-filtering, archive-and-prune,
// abandoned-with-a-reason, seat-blocker, landing-slot). Each drives the public
// goal commands, or a family owner where no public action exists, against the
// Git-free goalCLIBed.

var gcliLedgerHuman = []string{"--by", "Wido", "--fixture-human-authority"}

func gcliLedgerOpen(t *testing.T, bed *goalCLIBed, id string, extra ...string) {
	t.Helper()
	args := append([]string{"goal", "open", id, "--intent", "Fixture goal " + id + ".", "--next", "Continue.",
		"--risk", "severity=1,novelty=1,exposure=1,accumulation=1", "--basis", "fixture risk"}, extra...)
	if code, out, errOut := bed.public(args...); code != 0 {
		t.Fatalf("goal open %s: code=%d out=%q err=%q", id, code, out, errOut)
	}
}

func gcliLedgerHumanOpen(t *testing.T, bed *goalCLIBed, id string, extra ...string) {
	t.Helper()
	gcliLedgerOpen(t, bed, id, append(append([]string{"--origin", "human"}, gcliLedgerHuman...), extra...)...)
}

func gcliLedgerMust(t *testing.T, bed *goalCLIBed, args ...string) string {
	t.Helper()
	code, out, errOut := bed.public(args...)
	if code != 0 {
		t.Fatalf("%v: code=%d out=%q err=%q", args, code, out, errOut)
	}
	return out
}

func gcliLedgerRefused(t *testing.T, bed *goalCLIBed, want string, args ...string) string {
	t.Helper()
	before := bed.tip()
	code, out, errOut := bed.public(args...)
	if code == 0 || !strings.Contains(out+errOut, want) {
		t.Fatalf("%v: want a refusal naming %q, got code=%d out=%q err=%q", args, want, code, out, errOut)
	}
	if bed.tip() != before {
		t.Fatalf("%v: the refusal moved the ledger from %s to %s", args, before, bed.tip())
	}
	return out + errOut
}

// risk-basis: intake records the four answers and the derived tier; a recorded
// tier above the derivation stands until a person lowers it.
func TestGoalCLILedgerRiskBasis(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	refusal := gcliLedgerRefused(t, bed, "--risk", "goal", "open", "unanswered-risk",
		"--intent", "Refuse a tier without the four answers.", "--next", "Answer the risk questions.", "--tier", "2")
	if !strings.Contains(refusal, "--basis") {
		t.Fatalf("goal open --tier alone did not name both unanswered inputs: %q", refusal)
	}
	gcliLedgerMust(t, bed, append([]string{"goal", "open", "risk-basis", "--origin", "human",
		"--intent", "Exercise risk-derived intake.", "--next", "Keep the risk basis visible.",
		"--risk", "severity=2,novelty=1,exposure=1,accumulation=1", "--basis", "moderate consequence with landed precedent"}, gcliLedgerHuman...)...)
	lines := strings.Split(bed.goalRecord("risk-basis"), "\n")
	riskAt, tierAt := -1, -1
	for index, line := range lines {
		switch line {
		case `- Risk: severity=2 novelty=1 exposure=1 accumulation=1 basis="moderate consequence with landed precedent"`:
			riskAt = index
		case "- Tier: 2":
			tierAt = index
		}
	}
	if riskAt < 0 || tierAt != riskAt+1 {
		t.Fatalf("open did not render Risk immediately above the derived Tier:\n%s", bed.goalRecord("risk-basis"))
	}

	gcliLedgerMust(t, bed, append([]string{"goal", "open", "exposed-legacy", "--origin", "human",
		"--intent", "Wide but routine, tiered by the earlier formula.", "--next", "Lower it.",
		"--tier", "3", "--reason", "the earlier formula", "--risk", "severity=1,novelty=1,exposure=3,accumulation=1",
		"--basis", "wide but routine"}, gcliLedgerHuman...)...)
	tiers := gcliLedgerMust(t, bed, "goal", "list", "--tiers")
	if !strings.Contains(tiers, "exposed-legacy") || !strings.Contains(tiers, "recorded=3") || !strings.Contains(tiers, "derived=1") {
		t.Fatalf("the tier listing does not name the exposure-lifted goal as lowerable: %q", tiers)
	}
	gcliLedgerMust(t, bed, "goal", "edit", "exposed-legacy", "--risk", "severity=1,novelty=1,exposure=3,accumulation=1",
		"--basis", "wide but routine, reworded")
	if goalCLILine(bed.goalRecord("exposed-legacy"), "- Tier: ") != "- Tier: 3" {
		t.Fatalf("a re-stated basis lowered the recorded tier:\n%s", bed.goalRecord("exposed-legacy"))
	}
	gcliLedgerRefused(t, bed, "is a human act", "goal", "edit", "exposed-legacy", "--tier", "1",
		"--risk", "severity=1,novelty=1,exposure=3,accumulation=1", "--basis", "wide but routine, reworded", "--reason", "the formula changed")
	gcliLedgerMust(t, bed, append([]string{"goal", "edit", "exposed-legacy", "--tier", "1",
		"--risk", "severity=1,novelty=1,exposure=3,accumulation=1", "--basis", "wide but routine, reworded",
		"--reason", "the formula changed"}, gcliLedgerHuman...)...)
	if goalCLILine(bed.goalRecord("exposed-legacy"), "- Tier: ") != "- Tier: 1" {
		t.Fatalf("the person's lowering with unchanged answers did not land:\n%s", bed.goalRecord("exposed-legacy"))
	}

}

// gcliLedgerListed are the goal ids of a goal list's summary rows.
func gcliLedgerListed(listing string) []string {
	var ids []string
	for _, line := range strings.Split(listing, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 5 && fields[2] == "tier" && strings.Contains(fields[0], ":") {
			ids = append(ids, fields[4])
		}
	}
	return ids
}

// labels-and-filtering: labels are one canonical whole field; edits compute
// the replacement; lists filter by every named label; the ready frontier
// answers a held claim first and names an empty filtered set distinctly.
func TestGoalCLILedgerLabelsAndFiltering(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	gcliLedgerHumanOpen(t, bed, "labeled-one", "--label", "beta", "--label", "alpha", "--label", "beta")
	gcliLedgerHumanOpen(t, bed, "plain-goal")
	if line := goalCLILine(bed.goalRecord("labeled-one"), "- Labels:"); line != "- Labels: alpha, beta" {
		t.Fatalf("open did not store sorted, deduplicated labels: %q", line)
	}
	if line := goalCLILine(bed.goalRecord("plain-goal"), "- Labels:"); line != "" {
		t.Fatalf("an unlabeled open wrote a Labels line: %q", line)
	}

	gcliLedgerMust(t, bed, "goal", "edit", "labeled-one", "--label", "shared", "--unlabel", "beta")
	edited := bed.goalRecord("labeled-one")
	if line := goalCLILine(edited, "- Labels:"); line != "- Labels: alpha, shared" {
		t.Fatalf("label add/remove produced the wrong whole field: %q", line)
	}
	revisionBefore := goalCLILine(edited, "- Revision: ")
	gcliLedgerMust(t, bed, "goal", "edit", "labeled-one", "--label", "alpha")
	// Idempotency follow-up (R-129): an edit whose final label set is
	// unchanged still records an edit and advances the revision today.
	if before, after := revisionBefore, goalCLILine(bed.goalRecord("labeled-one"), "- Revision: "); before == after {
		t.Fatalf("an equal final label set did not follow the shipped edit behaviour: revision stayed %q", after)
	}
	gcliLedgerRefused(t, bed, "both --label and --unlabel", "goal", "edit", "labeled-one", "--label", "alpha", "--unlabel", "alpha")
	gcliLedgerRefused(t, bed, "must match ^[a-z][a-z0-9-]{0,31}$", append([]string{"goal", "open", "bad-label", "--origin", "human",
		"--intent", "Must refuse.", "--next", "Stop.", "--risk", "severity=1,novelty=1,exposure=1,accumulation=1", "--basis", "fixture risk",
		"--label", "Bad_Label"}, gcliLedgerHuman...)...)
	gcliLedgerRefused(t, bed, "--label chooses among ready goals; a named goal takes none", "goal", "claim", "plain-goal", "--label", "x")

	gcliLedgerHumanOpen(t, bed, "labeled-two", "--label", "shared", "--label", "alpha")
	contains := func(ids []string, id string) bool {
		for _, candidate := range ids {
			if candidate == id {
				return true
			}
		}
		return false
	}
	one := gcliLedgerListed(gcliLedgerMust(t, bed, "goal", "list", "--label", "shared"))
	if !contains(one, "labeled-one") || !contains(one, "labeled-two") || contains(one, "plain-goal") {
		t.Fatalf("one-label list filtering: %v", one)
	}
	two := gcliLedgerListed(gcliLedgerMust(t, bed, "goal", "list", "--label", "alpha", "--label", "shared"))
	if !contains(two, "labeled-one") || !contains(two, "labeled-two") {
		t.Fatalf("two-label AND filtering lost a match: %v", two)
	}
	gcliLedgerHumanOpen(t, bed, "and-a", "--label", "a")
	gcliLedgerHumanOpen(t, bed, "and-ab", "--label", "a", "--label", "b")
	if both := gcliLedgerListed(gcliLedgerMust(t, bed, "goal", "list", "--label", "a", "--label", "b")); strings.Join(both, ",") != "and-ab" {
		t.Fatalf("the two-label filter did not return exactly the goal carrying both labels: %v", both)
	}

	if held := gcliLedgerMust(t, bed, "goal", "list", "--ready", "--label", "absent"); strings.TrimSpace(held) != "continue your claimed goal: ship-widget" {
		t.Fatalf("a label filter hid the held claim: %q", held)
	}
	gcliLedgerMust(t, bed, "goal", "release", "ship-widget", "--reason", "free the seat")
	// The public ready frontier names an empty filtered set by machine; the
	// family verb's label-specific wording is TestGoalPrioritySelection's.
	if empty := gcliLedgerMust(t, bed, "goal", "list", "--ready", "--label", "absent"); strings.TrimSpace(empty) != "no ready goal for fixture-machine" {
		t.Fatalf("the empty filtered frontier: %q", empty)
	}
	gcliLedgerHumanOpen(t, bed, "machine-only", "--label", "machine-only")
	gcliLedgerMust(t, bed, append([]string{"goal", "pause", "machine-only", "--reason", "Keep the matching goal unavailable."}, gcliLedgerHuman...)...)
	if machine := gcliLedgerMust(t, bed, "goal", "list", "--ready", "--machine", "fixture-machine", "--label", "machine-only"); strings.TrimSpace(machine) != "no ready goal for fixture-machine" {
		t.Fatalf("a parked matching goal was offered: %q", machine)
	}
}

// seat-blocker (R-93-m1e): a seat opens only the defect that blocks its
// claimed goal; the open parks the blocked goal behind it and clears the
// claim; no agent lifts that park; concluding the blocker returns the goal in
// the same publish, and it is claimable again.
func TestGoalCLILedgerSeatBlocker(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	bed.announceHolder()
	refusal := gcliLedgerRefused(t, bed, "R-93-m1e", "goal", "open", "stray-idea", "--intent", "An improvement that blocks nothing.",
		"--next", "Do it.", "--risk", "severity=1,novelty=1,exposure=1,accumulation=1", "--basis", "seat blocker fixture")
	if !strings.Contains(refusal, "--blocks") {
		t.Fatalf("the seat open refusal does not name --blocks: %q", refusal)
	}
	gcliLedgerOpen(t, bed, "widget-defect", "--blocks", "ship-widget")
	if bed.goalRecord("widget-defect") == "" {
		t.Fatal("the blocker did not open")
	}
	parked := bed.goalRecord("ship-widget")
	if goalCLILine(parked, "- State: ") != "- State: parked" || goalCLILine(parked, "- BlockedBy: ") != "- BlockedBy: widget-defect" ||
		!strings.Contains(parked, " blocker=widget-defect because=blocked by widget-defect") || goalCLILine(parked, "- Claimed:") != "" {
		t.Fatalf("the blocked goal did not park behind its blocker with the claim cleared:\n%s", parked)
	}
	gcliLedgerRefused(t, bed, "returns by itself", "goal", "resume", "ship-widget")
	gcliLedgerMust(t, bed, append([]string{"goal", "approve", "widget-defect"}, gcliLedgerHuman...)...)
	gcliLedgerMust(t, bed, "goal", "claim", "widget-defect")
	gcliLedgerMust(t, bed, "goal", "done", "widget-defect", "--reason", "Fixed; ship-widget continues.")
	returned := bed.goalRecord("ship-widget")
	if goalCLILine(returned, "- State: ") == "- State: parked" {
		t.Fatalf("the blocked goal did not return when its blocker was done:\n%s", returned)
	}
	if !strings.Contains(returned, " unpark actor=fixture-machine+fixture-lineage ") || !strings.Contains(returned, "reason=blocker widget-defect is done") {
		t.Fatalf("the return is not recorded on the goal:\n%s", returned)
	}
	if goalCLILine(returned, "- State: ") == "- State: queued" {
		gcliLedgerMust(t, bed, append([]string{"goal", "approve", "ship-widget", "--budget", "4h/2/120m/1/0"}, gcliLedgerHuman...)...)
	}
	gcliLedgerMust(t, bed, "goal", "claim", "ship-widget")
	if line := goalCLILine(bed.goalRecord("ship-widget"), "- Claimed: "); !strings.HasPrefix(line, "- Claimed: machine=fixture-machine lineage=fixture-lineage") {
		t.Fatalf("the returned goal is not claimable again: %q", line)
	}
}

// landing-slot (goal 20): built work lands beside the seat's claim. The
// land-ready act keeps the claim and frees the machine's one claim; a second
// slot is refused; the frontier continues the working claim and names the
// landing goal; a person concludes the landing goal; the same pair's release
// and re-claim keep the accounting episode with the gap as idle seconds.
func TestGoalCLILedgerLandingSlot(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	bed.announceHolder()
	landAt := goalCLISeedNow.Add(time.Minute)
	stamp := func(at time.Time) string { return at.UTC().Format(time.RFC3339) }
	bed.setNow(landAt)
	gcliLedgerMust(t, bed, "work", "land", "ship-widget", "--queue-only")
	landing := bed.goalRecord("ship-widget")
	if !strings.HasPrefix(goalCLILine(landing, "- Landing: "), "- Landing: at="+stamp(landAt)+" opid=") ||
		!strings.HasPrefix(goalCLILine(landing, "- Claimed: "), "- Claimed: machine=fixture-machine lineage=fixture-lineage") ||
		!strings.Contains(landing, " land-ready actor=fixture-machine+fixture-lineage targets=ship-widget") {
		t.Fatalf("land-ready did not record the slot beside the kept claim:\n%s", landing)
	}
	// A repeated land-ready is nothing to do (R-129): success, and no second
	// record.
	tip := bed.tip()
	if code, out, errOut := bed.public("work", "land", "ship-widget", "--queue-only"); code != 0 || !strings.Contains(out, "already in landing") || bed.tip() != tip {
		t.Fatalf("a repeated land-ready was not nothing to do: code=%d out=%q err=%q", code, out, errOut)
	}

	bed.setNow(landAt.Add(5 * time.Minute))
	gcliLedgerHumanOpen(t, bed, "next-widget")
	gcliLedgerMust(t, bed, append([]string{"goal", "approve", "next-widget"}, gcliLedgerHuman...)...)
	gcliLedgerMust(t, bed, "goal", "claim", "next-widget")
	gcliLedgerRefused(t, bed, "one landing slot per machine", "work", "land", "next-widget", "--queue-only")
	frontier := gcliLedgerMust(t, bed, "goal", "list", "--ready", "--machine", "fixture-machine")
	if !strings.Contains(frontier, "continue your claimed goal: next-widget") {
		t.Fatalf("the frontier does not continue the working claim: %q", frontier)
	}
	code, listing, errOut := bed.public("goal", "list", "--json")
	if code != 0 || !strings.Contains(listing, stamp(landAt)) {
		t.Fatalf("goal list does not show the landing slot: code=%d out=%s err=%s", code, listing, errOut)
	}
	var listed struct {
		Data struct {
			Open []struct {
				Id      string
				Landing *struct{ At string }
			}
		}
	}
	if err := json.Unmarshal([]byte(listing), &listed); err != nil {
		t.Fatal(err)
	}
	slot := ""
	for _, file := range listed.Data.Open {
		if file.Id == "ship-widget" && file.Landing != nil {
			slot = file.Landing.At
		}
	}
	if slot != stamp(landAt) {
		t.Fatalf("goal list --json does not carry the landing slot at %s: %s", stamp(landAt), listing)
	}
	gcliLedgerMust(t, bed, "goal", "done", "ship-widget", "--reason", "Landed by the person while next-widget was claimed.", "--by", "Wido")
	archived := bed.accepted("records/goals/ship-widget.md")
	if !strings.Contains(archived, " land-ready actor=") || goalCLILine(archived, "- Landing:") != "" {
		t.Fatalf("the archive did not keep the land-ready line and drop the slot:\n%s", archived)
	}

	claimed := goalCLILine(bed.goalRecord("next-widget"), "- Claimed: ")
	accounting := ""
	for _, field := range strings.Fields(claimed) {
		if value, ok := strings.CutPrefix(field, "accountingRevision="); ok {
			accounting = value
		}
	}
	if accounting == "" {
		t.Fatalf("the claim carries no accounting revision: %q", claimed)
	}
	releaseAt := landAt.Add(time.Hour)
	bed.setNow(releaseAt)
	gcliLedgerMust(t, bed, "goal", "release", "next-widget", "--reason", "the seat pauses")
	episode := goalCLILine(bed.goalRecord("next-widget"), "- Episode: ")
	if !strings.HasPrefix(episode, "- Episode: machine=fixture-machine lineage=fixture-lineage accountingRevision="+accounting+" ") ||
		!strings.HasSuffix(episode, " idleSeconds=0 released="+stamp(releaseAt)) {
		t.Fatalf("the own pair's release did not keep the episode: %q", episode)
	}
	reclaimAt := releaseAt.Add(2 * time.Hour)
	bed.setNow(reclaimAt)
	gcliLedgerMust(t, bed, "goal", "claim", "next-widget")
	reclaimed := bed.goalRecord("next-widget")
	line := goalCLILine(reclaimed, "- Claimed: ")
	if !strings.HasPrefix(line, "- Claimed: machine=fixture-machine lineage=fixture-lineage at="+stamp(reclaimAt)+" revision=") ||
		!strings.Contains(line, " accountingRevision="+accounting+" episodeAt=") || !strings.HasSuffix(line, " idleSeconds=7200") {
		t.Fatalf("the re-claim did not keep the episode with the gap idle: %q", line)
	}
	if goalCLILine(reclaimed, "- Episode:") != "" {
		t.Fatalf("the re-claim did not consume the kept episode:\n%s", reclaimed)
	}
}

// gcliLedgerFamily runs one synced-ledger family mutation (a verb with no
// public action, such as prune) with the bed's dependencies.
func gcliLedgerFamily(t *testing.T, bed *goalCLIBed, name string, args ...string) (int, string, string) {
	t.Helper()
	return bed.owner(func(dependencies syncRequestDependencies) int {
		code, _ := trySyncMutationWithCompletion(name, append([]string{"--root", bed.root}, args...), bed.commandNow, dependencies,
			func(string, goal.Endpoint) func(string, string) (string, error) {
				return func(string, string) (string, error) { return "", nil }
			}, completionInputs{})
		return code
	})
}

// gcliLedgerInstall commits changes straight onto the accepted ledger, the
// in-memory equivalent of the shell bed's plumbing commit and ref update for
// a shape production admission no longer writes.
func gcliLedgerInstall(t *testing.T, bed *goalCLIBed, opid string, changes ...goal.Change) {
	t.Helper()
	parent, err := bed.repo.Capture(opid)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := bed.repo.Build(opid, parent, changes, "fixture install")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := bed.repo.Publish(parent, commit); err != nil || outcome != goal.CASLanded {
		t.Fatalf("install publish: outcome=%v err=%v", outcome, err)
	}
	if err := bed.repo.AcceptedCAS(parent, commit); err != nil {
		t.Fatal(err)
	}
	if err := bed.repo.Release(opid); err != nil {
		t.Fatal(err)
	}
}

// archive-and-prune: a conclusion is written to the records-owned archive
// with its Integrity line; reopen moves it back with a History event; the
// soak reader counts a legacy plans/goals/done conclusion beside records;
// prune removes concluded files from both locations and leaves a tombstone
// on the root record.
func TestGoalCLILedgerArchiveAndPrune(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	gcliLedgerHumanOpen(t, bed, "archive-roundtrip")
	gcliLedgerMust(t, bed, "goal", "done", "archive-roundtrip", "--reason", "Archived in the records-owned location.", "--by", "Wido")
	archived := bed.accepted("records/goals/archive-roundtrip.md")
	if !strings.Contains(archived, "\nIntegrity: sha256=") {
		t.Fatalf("the records-owned conclusion lost its Integrity line:\n%s", archived)
	}
	if bed.accepted("plans/goals/done/archive-roundtrip.md") != "" || bed.accepted("plans/goals/archive-roundtrip.md") != "" {
		t.Fatal("goal done wrote the legacy archive or left the live record")
	}
	gcliLedgerMust(t, bed, "goal", "reopen", "archive-roundtrip", "--next", "Conclude it again.")
	reopened := bed.accepted("plans/goals/archive-roundtrip.md")
	if !strings.Contains(reopened, " reopen actor=") || bed.accepted("records/goals/archive-roundtrip.md") != "" {
		t.Fatalf("goal reopen did not move the record back with its History event:\n%s", reopened)
	}
	gcliLedgerMust(t, bed, "goal", "done", "archive-roundtrip", "--reason", "Archived again after the recorded reopen.", "--by", "Wido")
	if !strings.Contains(bed.accepted("records/goals/archive-roundtrip.md"), "\nIntegrity: sha256=") {
		t.Fatal("the second conclusion lost its Integrity line")
	}

	// The soak reader: move the conclusion to the legacy location on the
	// accepted ledger and read both locations.
	legacy := bed.accepted("records/goals/archive-roundtrip.md")
	gcliLedgerInstall(t, bed, "fixture-legacy-archive",
		goal.Change{Path: "plans/goals/done/archive-roundtrip.md", Content: []byte(legacy)},
		goal.Change{Path: "records/goals/archive-roundtrip.md", Delete: true})
	listing := gcliLedgerMust(t, bed, "goal", "list", "--all")
	if !strings.Contains(listing, " done=2 abandoned=0 tip=") {
		t.Fatalf("the dual-location soak reader did not count both conclusions: %q", listing)
	}
	code, shown, errOut := bed.public("goal", "show", "archive-roundtrip", "--json")
	if code != 0 || !strings.Contains(shown, `"State": "done"`) || !strings.Contains(shown, "Archived again after the recorded reopen.") {
		t.Fatalf("goal show did not read the legacy conclusion during the soak: code=%d out=%s err=%s", code, shown, errOut)
	}

	if code, out, errOut := gcliLedgerFamily(t, bed, "prune", "--keep", "0"); code != 0 {
		t.Fatalf("goal prune: code=%d out=%q err=%q", code, out, errOut)
	}
	tip, _, err := bed.repo.Accepted()
	if err != nil {
		t.Fatal(err)
	}
	concluded, err := bed.repo.Files(tip, "records/goals/", "plans/goals/done/")
	if err != nil {
		t.Fatal(err)
	}
	if len(concluded) != 0 {
		t.Fatalf("goal prune left concluded records outside its retention closure: %v", concluded)
	}
	if !strings.Contains(bed.accepted("plans/goals/backlog.md"), " prune actor=") {
		t.Fatalf("goal prune removed files without its root History tombstone:\n%s", bed.accepted("plans/goals/backlog.md"))
	}
}

// gcliLedgerBreachStop closes the claimed revision's launch fence through the
// goal stop owner, as the stop custodian does after a breach, and records the
// completed stop batch (no jobs were running). It returns the stop id.
func gcliLedgerBreachStop(t *testing.T, bed *goalCLIBed, id string) string {
	t.Helper()
	file, problems := goal.ParseFile([]byte(bed.goalRecord(id)))
	if file == nil || len(problems) != 0 || file.Claimed == nil || file.StopCapability == nil {
		t.Fatalf("goal %s is not a claimed goal with a stop capability: problems=%v\n%s", id, problems, bed.goalRecord(id))
	}
	capability := *file.StopCapability
	stopID := fmt.Sprintf("stop-%s-r%d-f%d", id, file.Revision, capability.FenceEpoch+1)
	endpoint, err := bed.endpoint(bed.root)
	if err != nil {
		t.Fatal(err)
	}
	now := bed.clock()
	result, err := goal.CloseStop(goal.CloseStopRequest{
		VerbRequest: goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Machine: capability.Machine, Lineage: "goal-stop-custodian"},
			Ulid: "01ARZ3NDEKTSV4RRFFQ69G5FS0", Now: now, ClaimEpoch: capability.ClaimEpoch},
		GoalID: id, StopID: stopID, Reason: goal.StopReasonElapsedLimit, Capability: capability,
	})
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("close the launch fence of %s: result=%+v err=%v", id, result, err)
	}
	fenced, _ := goal.ParseFile([]byte(bed.goalRecord(id)))
	if fenced == nil || fenced.StopFence == nil {
		t.Fatalf("goal %s carries no launch fence after the stop:\n%s", id, bed.goalRecord(id))
	}
	stamp := now.UTC().Format(time.RFC3339)
	if err := goal.WriteStopBatch(bed.root, goal.StopBatch{
		StopID: stopID, GoalID: id, GoalRevision: fenced.StopFence.Revision, FenceEpoch: fenced.StopFence.Epoch,
		CapabilityGeneration: fenced.StopFence.CapabilityGeneration, Machine: capability.Machine, ClaimEpoch: capability.ClaimEpoch,
		Reason: goal.StopReasonElapsedLimit, State: goal.StopBatchComplete, OpenedAt: stamp, UpdatedAt: stamp, CompletedAt: stamp, Pass: 1,
	}); err != nil {
		t.Fatal(err)
	}
	return stopID
}

// abandoned-with-a-reason: a breach-stopped claim keeps its fence against
// release; abandon refuses an uncovered dependent and, with a successor,
// archives the stopped record with its reason; listings keep done and
// abandoned apart; the frontier skips it; show carries the reason; split
// refuses the archive; prune keeps the abandoned record.
func TestGoalCLILedgerAbandonedWithAReason(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	bed.announceHolder()
	gcliLedgerMust(t, bed, "goal", "release", "ship-widget", "--reason", "free the seat")
	gcliLedgerHumanOpen(t, bed, "abandon-b")
	gcliLedgerMust(t, bed, append([]string{"goal", "approve", "abandon-b"}, gcliLedgerHuman...)...)
	gcliLedgerMust(t, bed, "goal", "claim", "abandon-b")
	gcliLedgerOpen(t, bed, "abandon-a", "--blocks", "abandon-b")
	gcliLedgerMust(t, bed, append([]string{"goal", "approve", "abandon-a", "--budget", "1m/2/20m/1/0"}, gcliLedgerHuman...)...)
	gcliLedgerMust(t, bed, "goal", "claim", "abandon-a")
	bed.setNow(bed.clock().Add(2 * time.Minute))
	stopID := gcliLedgerBreachStop(t, bed, "abandon-a")

	gcliLedgerRefused(t, bed, "only goal resume may clear its launch fence", "goal", "release", "abandon-a", "--reason", "try to clear the fence")
	gcliLedgerRefused(t, bed, "goal abandon-b is blocked by abandon-a", "goal", "abandon", "abandon-a", "--reason", "fixture", "--by", "Wido")

	gcliLedgerHumanOpen(t, bed, "abandon-successor")
	doneBefore := ""
	for _, field := range strings.Fields(gcliLedgerMust(t, bed, "goal", "list", "--all")) {
		if value, ok := strings.CutPrefix(field, "done="); ok {
			doneBefore = value
		}
	}
	gcliLedgerMust(t, bed, "goal", "abandon", "abandon-a", "--reason", "fixture", "--successor", "abandon-successor", "--by", "Wido")
	archived := bed.accepted("records/goals/abandon-a.md")
	if goalCLILine(archived, "- State: ") != "- State: abandoned" || goalCLILine(archived, "- StopFence:") == "" ||
		!strings.HasPrefix(goalCLILine(archived, "- Abandoned: "), "- Abandoned: by=human:Wido ") ||
		!strings.Contains(goalCLILine(archived, "- Abandoned: "), "stopId="+stopID+" carried=abandon-successor ") ||
		!strings.HasSuffix(goalCLILine(archived, "- Abandoned: "), " because=fixture") {
		t.Fatalf("the abandoned stopped record is incomplete:\n%s", archived)
	}
	if bed.accepted("plans/goals/abandon-a.md") != "" {
		t.Fatal("the abandoned goal remained in the live path")
	}
	summary := gcliLedgerMust(t, bed, "goal", "list", "--all")
	if !strings.Contains(summary, " done="+doneBefore+" abandoned=1 tip=") {
		t.Fatalf("listing conflated done and abandoned (done before %s): %q", doneBefore, summary)
	}
	code, listing, errOut := bed.public("goal", "list", "--all", "--json")
	var listed struct {
		Data struct {
			Abandoned []struct{ Id string }
			Done      []struct{ Id string }
		}
	}
	if err := json.Unmarshal([]byte(listing), &listed); code != 0 || err != nil {
		t.Fatalf("goal list --json: code=%d err=%v stderr=%s", code, err, errOut)
	}
	inAbandoned, inDone := false, false
	for _, file := range listed.Data.Abandoned {
		inAbandoned = inAbandoned || file.Id == "abandon-a"
	}
	for _, file := range listed.Data.Done {
		inDone = inDone || file.Id == "abandon-a"
	}
	if !inAbandoned || inDone {
		t.Fatalf("JSON listing omitted or conflated the abandoned id: abandoned=%v done=%v\n%s", inAbandoned, inDone, listing)
	}
	if next := gcliLedgerMust(t, bed, "goal", "list", "--ready"); strings.Contains(next, "abandon-a") || strings.Contains(next, "next ready goal: abandon-b") {
		t.Fatalf("the frontier offered abandoned or blocked work: %q", next)
	}
	if code, shown, errOut := bed.public("goal", "show", "abandon-a", "--json"); code != 0 || !strings.Contains(shown, `"Because": "fixture"`) {
		t.Fatalf("goal show omitted the abandon reason: code=%d out=%s err=%s", code, shown, errOut)
	}
	plan := filepath.Join(bed.root, "abandon-split.md")
	if err := os.WriteFile(plan, []byte("# split abandon-a\n\n## member abandon-a-one\n- Intent: First impossible split member.\n- Next step: Never run.\n\n## member abandon-a-two\n- Intent: Second impossible split member.\n- Next step: Never run.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gcliLedgerRefused(t, bed, "in the archive; there is nothing to split", "goal", "split", "abandon-a", "--plan", plan)
	if code, out, errOut := gcliLedgerFamily(t, bed, "prune", "--keep", "0"); code != 0 {
		t.Fatalf("goal prune: code=%d out=%q err=%q", code, out, errOut)
	}
	if bed.accepted("records/goals/abandon-a.md") == "" {
		t.Fatal("prune dropped the abandoned record")
	}
}

// The migration CLI takes its actor's lineage from the runner's real export
// (F16: a second spelling once collapsed every session to the literal
// "session"). goalActor reads the process environment, so the witness is its
// source: the one environment read it makes is METASYSTEM_OWNER_LINEAGE.
// TestGoalCLILedgerMigrationRecovery (internal/goal) proves the owner half:
// the synthesized claim carries the actor's lineage.
func TestGoalCLILedgerMigrationActorReadsTheRunnersLineageExport(t *testing.T) {
	t.Parallel()
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, "goalsync_verbs.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var reads []string
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "goalActor" {
			continue
		}
		ast.Inspect(function, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Getenv" && len(call.Args) == 1 {
				if literal, ok := call.Args[0].(*ast.BasicLit); ok {
					reads = append(reads, literal.Value)
				}
			}
			return true
		})
	}
	if strings.Join(reads, ",") != `"METASYSTEM_OWNER_LINEAGE"` {
		t.Fatalf("goalActor must read exactly the runner's METASYSTEM_OWNER_LINEAGE export, read %v", reads)
	}
}
