package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// These tests are the Go port of the goal CLI shell bed's budget and
// authority scenarios (budget-extension, structured-budget, scope-bounds,
// classification-sweep, power-of-attorney). Each drives the public goal
// commands, or a family owner where no public action exists, against the
// Git-free goalCLIBed. The dispatch-owned halves (job goal-admission, job
// breach-stop, job stop-batch-reconcile) and fenced-set-budget are in
// internal/dispatch/goal_cli_budget_test.go.

var gcliBudgetHuman = []string{"--by", "Wido", "--fixture-human-authority"}

const (
	gcliBudgetTierThree = "severity=3,novelty=1,exposure=1,accumulation=1"
	gcliBudgetTierTwo   = "severity=2,novelty=1,exposure=1,accumulation=1"
	gcliBudgetTierOne   = "severity=1,novelty=1,exposure=1,accumulation=1"
)

func gcliBudgetMust(t *testing.T, bed *goalCLIBed, args ...string) string {
	t.Helper()
	code, out, errOut := bed.public(args...)
	if code != 0 {
		t.Fatalf("%v: code=%d out=%q err=%q", args, code, out, errOut)
	}
	return out
}

func gcliBudgetHumanMust(t *testing.T, bed *goalCLIBed, args ...string) string {
	t.Helper()
	return gcliBudgetMust(t, bed, append(append([]string(nil), args...), gcliBudgetHuman...)...)
}

// gcliBudgetRefused runs a public command that must refuse naming want and
// leave the accepted ledger where it was.
func gcliBudgetRefused(t *testing.T, bed *goalCLIBed, want string, args ...string) string {
	t.Helper()
	before := bed.tip()
	code, out, errOut := bed.public(args...)
	// A refusal wraps at the width, so it is read as its words.
	if code == 0 || !strings.Contains(strings.Join(strings.Fields(out+errOut), " "), want) {
		t.Fatalf("%v: want a refusal naming %q, got code=%d out=%q err=%q", args, want, code, out, errOut)
	}
	if bed.tip() != before {
		t.Fatalf("%v: the refusal moved the ledger from %s to %s", args, before, bed.tip())
	}
	return out + errOut
}

// gcliBudgetOpen opens a person's goal with the given risk answers.
func gcliBudgetOpen(t *testing.T, bed *goalCLIBed, id, risk, next string, extra ...string) {
	t.Helper()
	gcliBudgetHumanMust(t, bed, append([]string{"goal", "open", id, "--origin", "human",
		"--intent", "Fixture goal " + id + ".", "--next", next, "--risk", risk, "--basis", "fixture risk"}, extra...)...)
}

// gcliBudgetFamily runs one synced-ledger family mutation (a verb with no
// public action) with the bed's dependencies.
func gcliBudgetFamily(t *testing.T, bed *goalCLIBed, name string, args ...string) (int, string, string) {
	t.Helper()
	return bed.owner(func(dependencies syncRequestDependencies) int {
		code, _ := trySyncMutationWithCompletion(name, append([]string{"--root", bed.root}, args...), bed.commandNow, dependencies,
			func(string, goal.Endpoint) func(string, string) (string, error) {
				return func(string, string) (string, error) { return "", nil }
			}, completionInputs{})
		return code
	})
}

// gcliBudgetReads are the dispatch owners' repository reads over the bed's
// accepted ledger; receipts serves the accepted receipt ledger's bytes.
func gcliBudgetReads(bed *goalCLIBed, receipts func() []byte) dispatchcore.ProofAdmissionReads {
	return dispatchcore.ProofAdmissionReads{
		ResolveEndpoint: bed.endpoint,
		ResolveMachine:  func(string) (string, error) { return bed.machine, nil },
		Receipt: dispatchcore.ReceiptAdmissionSource{
			AcceptedLedgerTip: func(string) (string, bool, error) { return bed.tip(), true, nil },
			TopLevel:          func(string) (string, error) { return bed.root, nil },
			FileAt: func(_, _, path string) ([]byte, bool, error) {
				if path != "memory/receipts.log" || receipts == nil {
					return nil, false, nil
				}
				data := receipts()
				return data, data != nil, nil
			},
		},
	}
}

// gcliBudgetInstall commits changes straight onto the accepted ledger, the
// in-memory equivalent of the shell bed's plumbing commit and ref update.
func gcliBudgetInstall(t *testing.T, bed *goalCLIBed, opid string, changes ...goal.Change) {
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

// gcliBudgetAnnounceParent records the test binary's parent process as the
// checkout's MAIN lease holder and makes it the classified caller. Owners
// that classify os.Getppid() (goal split's main-origin ratification) then
// see the same holder the claim-bearing acts do.
func gcliBudgetAnnounceParent(t *testing.T, bed *goalCLIBed) {
	t.Helper()
	parent, state, err := (identity.KernelProber{}).Probe(int64(os.Getppid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe the test binary's parent: state=%s err=%v", state, err)
	}
	if _, err := lease.AnnounceWithPair(bed.root, "goal-cli-fixture", parent.Pid, parent.StartedAt.Unix(), parent.StartTicks, parent.BootID,
		"goal-cli-fixture", "fake", bed.lineage); err != nil {
		t.Fatal(err)
	}
	bed.caller = ownercall.Process{Pid: parent.Pid, StartedAt: parent.StartedAt.Unix()}
}

// gcliBudgetTemplateRoot moves the bed's checkout into the template layout
// (<top>/metasystem, declared by metasystem.template=true) so the state
// root resolves without Git, as the receipt ledger's path needs.
func gcliBudgetTemplateRoot(t *testing.T, bed *goalCLIBed) {
	t.Helper()
	top := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(top); err == nil {
		top = resolved
	}
	root := filepath.Join(top, "metasystem")
	if err := os.CopyFS(root, os.DirFS(bed.root)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(top, "development"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(top, "development", "metasystem-design.md"), []byte("fixture template marker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	conf, err := os.ReadFile(filepath.Join(root, "metasystem.conf"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), append(conf, []byte("metasystem.template=true\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.root = root
}

// gcliBudgetGrant records a power of attorney and returns its entry id.
func gcliBudgetGrant(t *testing.T, bed *goalCLIBed, tiers, acts, until string) string {
	t.Helper()
	out := gcliBudgetHumanMust(t, bed, "grant", "add", "--tiers", tiers, "--acts", acts, "--until", until, "--json")
	var result struct {
		Outcome string
		Data    struct{ Grant string }
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil || result.Outcome != "confirmed" || result.Data.Grant == "" {
		t.Fatalf("grant add did not confirm with an entry id: %v %s", err, out)
	}
	return result.Data.Grant
}

// budget-extension: one completed attempt exhausts a narrow box; an accepted
// shipped implementation receipt earns exactly one extension whose marker
// names the landing evidence by epoch and line digest; the next exhaustion is
// refused naming the marker.
func TestGoalCLIBudgetEarnedExtension(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{config: "metasystem.budget.tier-3=1h/1/60m/1/3\n"})
	gcliBudgetTemplateRoot(t, bed)
	bed.announceHolder()
	gcliBudgetMust(t, bed, "goal", "release", "ship-widget", "--reason", "free the seat")
	bed.setNow(time.Date(2026, 9, 12, 20, 0, 0, 0, time.UTC))
	gcliBudgetOpen(t, bed, "earned-extension", gcliBudgetTierThree, "Apply the earned extension.", "--tier", "3")
	gcliBudgetHumanMust(t, bed, "goal", "approve", "earned-extension", "--elapsed-limit", "8h", "--attempt-limit", "1",
		"--reserved-job-minutes-limit", "60", "--active-job-limit", "1", "--review-round-limit", "3")
	gcliBudgetMust(t, bed, "goal", "claim", "earned-extension")
	claimed := goalCLILine(bed.goalRecord("earned-extension"), "- Claimed: ")
	match := regexp.MustCompile(` revision=([0-9]+)`).FindStringSubmatch(claimed)
	if match == nil {
		t.Fatalf("the claim carries no revision: %q", claimed)
	}
	revision, err := strconv.ParseUint(match[1], 10, 64)
	if err != nil {
		t.Fatal(err)
	}

	jobs := filepath.Join(bed.root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJob := func(id, started, ended string) {
		record := `{"jobId":"` + id + `","operationId":"` + id + `","goalId":"earned-extension","goalRevision":` + match[1] +
			`,"capMin":1,"status":"completed","startedAt":"` + started + `","endedAt":"` + ended + `"}` + "\n"
		if err := os.WriteFile(filepath.Join(jobs, id+".json"), []byte(record), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeJob("earned-first", "2026-09-12T20:05:00Z", "2026-09-12T20:06:00Z")
	receipt := "1789245000|2026-09-12T20:30:00Z|RECEIPT|type=implement|outcome=shipped|goal=earned-extension|built_by=fixture|note=fixture advancement"
	digest := sha1.Sum([]byte(receipt))
	reads := gcliBudgetReads(bed, func() []byte { return []byte(receipt + "\n") })

	extendAt := time.Date(2026, 9, 12, 21, 0, 0, 0, time.UTC)
	bed.setNow(extendAt)
	args := []string{"--root", bed.root, "--id", "earned-extension", "--revision", match[1], "--proposed-cap", "1",
		"--role", "implementer", "--dispatch-mode", "fresh", "--destructive-reach", "DESIGN-BEARING", "--lineage", bed.lineage}
	extend := func() int {
		code, _, _ := bed.owner(func(dependencies syncRequestDependencies) int {
			return goalExtendBudgetTo(args, bed.commandNow, dependencies, reads, dependencies.outStream(), dependencies.errStream())
		})
		return code
	}
	if code := extend(); code != 0 {
		t.Fatalf("the earned extension did not confirm: code=%d\n%s", code, bed.goalRecord("earned-extension"))
	}
	marker := goalCLILine(bed.goalRecord("earned-extension"), "- BudgetExtension: ")
	want := " evidence=landing:1789245000-" + hex.EncodeToString(digest[:]) + "@2026-09-12T20:30:00Z"
	if !strings.HasSuffix(marker, want) {
		t.Fatalf("the extension marker does not name its landing evidence by epoch and line digest: %q, want suffix %q", marker, want)
	}

	writeJob("earned-second", "2026-09-12T21:01:00Z", "2026-09-12T21:02:00Z")
	// Idempotency follow-up (R-129): a second extend-budget after the one
	// earned extension is refused naming the marker today.
	tip := bed.tip()
	if code := extend(); code != 1 || bed.tip() != tip {
		t.Fatalf("the second extension was not refused leaving the ledger alone: code=%d", code)
	}
	verdict, err := dispatchcore.EvaluateGoalRevisionAdmissionForDispatchWithReads(bed.root, "earned-extension", revision, 1, extendAt,
		"implementer", "fresh", reads, dispatchcore.HazardClass("DESIGN-BEARING"))
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Join(dispatchcore.FormatGoalRevisionAdmission(verdict), "\n"); verdict.Extension != nil ||
		!strings.Contains(lines, "extended once at 2026-09-12T21:00:00Z") {
		t.Fatalf("the second exhaustion did not name the extension marker: offer=%+v lines=%q", verdict.Extension, lines)
	}
}

// structured-budget: labels survive the separated open, approve and claim; a
// queued goal needs no budget and the claim stores the complete tuple bound
// to its revision while Appetite prose stays inert; a person's set-budget
// replaces the tuple and rebinds the claim's revision and elapsed origin; a
// second machine claims a goal under the tuple its approval bound.
func TestGoalCLIBudgetStructuredBudget(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	bed.setNow(goalCLISeedNow)
	bed.announceHolder()
	gcliBudgetMust(t, bed, "goal", "release", "ship-widget", "--reason", "free the seat")
	gcliBudgetOpen(t, bed, "plain-goal", gcliBudgetTierThree, "Continue.", "--tier", "3")

	gcliBudgetOpen(t, bed, "claimed-label", gcliBudgetTierThree, "Continue.", "--tier", "3", "--label", "custody")
	gcliBudgetHumanMust(t, bed, "goal", "approve", "claimed-label", "--budget", "norm")
	gcliBudgetMust(t, bed, "goal", "claim", "claimed-label")
	if line := goalCLILine(bed.goalRecord("claimed-label"), "- Labels:"); line != "- Labels: custody" {
		t.Fatalf("separated claim dropped its label: %q", line)
	}
	gcliBudgetMust(t, bed, "goal", "release", "claimed-label", "--reason", "free the seat")

	gcliBudgetOpen(t, bed, "budget-check", gcliBudgetTierThree, "Appetite: 4h is inert human prose, not a budget.", "--tier", "3")
	gcliBudgetHumanMust(t, bed, "goal", "approve", "budget-check", "--elapsed-limit", "8h", "--attempt-limit", "2",
		"--reserved-job-minutes-limit", "120", "--active-job-limit", "1", "--review-round-limit", "3")
	gcliBudgetMust(t, bed, "goal", "claim", "budget-check")
	claimed := bed.goalRecord("budget-check")
	if line := goalCLILine(claimed, "- Budget: "); line != "- Budget: elapsedLimit=1d attemptLimit=2 reservedJobMinutesLimit=120 activeJobLimit=1 reviewRoundLimit=3" {
		t.Fatalf("claim did not store the complete budget tuple: %q", line)
	}
	claim := goalCLILine(claimed, "- Claimed: ")
	if !strings.Contains(claim, " revision=3") {
		t.Fatalf("claim did not bind its goal revision: %q", claim)
	}
	if strings.Contains(claim, " appetite=") {
		t.Fatalf("claim froze inert prose into a budget field: %q", claim)
	}

	bed.setNow(time.Date(2026, 8, 20, 5, 1, 0, 0, time.UTC))
	gcliBudgetHumanMust(t, bed, "goal", "budget", "budget-check", "--elapsed-limit", "8h", "--attempt-limit", "3",
		"--reserved-job-minutes-limit", "180", "--active-job-limit", "2", "--review-round-limit", "3")
	rebudget := bed.goalRecord("budget-check")
	if line := goalCLILine(rebudget, "- Budget: "); line != "- Budget: elapsedLimit=1d attemptLimit=3 reservedJobMinutesLimit=180 activeJobLimit=2 reviewRoundLimit=3" {
		t.Fatalf("set-budget did not replace the complete tuple: %q", line)
	}
	if claim := goalCLILine(rebudget, "- Claimed: "); !strings.Contains(claim, " at=2026-08-20T05:01:00Z revision=4") {
		t.Fatalf("set-budget did not bind the new revision and elapsed origin: %q", claim)
	}

	// A separate machine and lineage claims the goal under the box its
	// approval bound, as the shell bed's second clone did.
	gcliBudgetHumanMust(t, bed, "goal", "approve", "plain-goal", "--budget", "norm")
	other := newGoalCLIBed(t, goalCLISeed{})
	other.repo, other.machine, other.lineage = bed.repo, "fixture-other", "other-lineage"
	other.setNow(bed.clock())
	other.announceHolder()
	gcliBudgetMust(t, other, "goal", "claim", "plain-goal")
	if line := goalCLILine(bed.goalRecord("plain-goal"), "- Claimed: "); !strings.HasPrefix(line, "- Claimed: machine=fixture-other lineage=other-lineage ") {
		t.Fatalf("an approved-tuple claim on the second machine did not confirm: %q", line)
	}
}

// scope-bounds: an over-norm set-budget refuses with the typed norm refusal
// and the split remedy; open --claim is retired; the split publishes both
// members, the parent's conclusion and the root's registry atomically and
// moves the blocked goal's edge to the members; a decomposed parent never
// reopens and its id survives prune as retired.
func TestGoalCLIBudgetScopeBounds(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	gcliBudgetAnnounceParent(t, bed)
	gcliBudgetMust(t, bed, "goal", "release", "ship-widget", "--reason", "free the seat")
	gcliBudgetOpen(t, bed, "norm-parent", gcliBudgetTierThree, "Split it first.", "--tier", "3")
	gcliBudgetHumanMust(t, bed, "goal", "approve", "norm-parent", "--budget", "norm")
	gcliBudgetMust(t, bed, "goal", "claim", "norm-parent")
	refusal := gcliBudgetRefused(t, bed, "goal norm-parent asks for 1441m", append([]string{"goal", "budget", "norm-parent",
		"--elapsed-limit", "1d", "--attempt-limit", "2", "--reserved-job-minutes-limit", "1441", "--active-job-limit", "1",
		"--review-round-limit", "3"}, gcliBudgetHuman...)...)
	if !strings.Contains(refusal, "split it, or pass --approved-ref") {
		t.Fatalf("the norm refusal did not name its split remedy: %q", refusal)
	}

	// The retired open-and-claim form: the public open has no --claim; the
	// family open still names the separated approval path.
	gcliBudgetRefused(t, bed, "--claim", "goal", "open", "norm-open-claim", "--intent", "Must not enter claimed over norm.",
		"--next", "Stop.", "--tier", "3", "--risk", gcliBudgetTierThree, "--basis", "fixture risk", "--claim")
	before := bed.tip()
	code, out, errOut := gcliBudgetFamily(t, bed, "open", "--id", "norm-open-claim", "--intent", "Must not enter claimed over norm.",
		"--next", "Stop.", "--tier", "3", "--risk", gcliBudgetTierThree, "--basis", "fixture risk", "--claim",
		"--elapsed-limit", "1d", "--attempt-limit", "2", "--reserved-job-minutes-limit", "1441", "--active-job-limit", "1", "--review-round-limit", "3")
	if code == 0 || !strings.Contains(out+errOut, "open --claim is gone") || bed.tip() != before {
		t.Fatalf("retired open --claim did not name the separated approval path: code=%d out=%q err=%q", code, out, errOut)
	}

	gcliBudgetMust(t, bed, "goal", "open", "split-parent", "--blocks", "norm-parent", "--intent", "Deliver the two-part fixture.",
		"--next", "Atomize it.", "--tier", "3", "--risk", gcliBudgetTierThree, "--basis", "fixture risk", "--label", "fixture")
	draft := filepath.Join(t.TempDir(), "split-parent.md")
	plan := strings.Join([]string{"# split split-parent", "", "## member split-parent-one", "- Intent: Deliver part one.", "- Next step: Build part one.",
		"", "## member split-parent-two", "- Intent: Deliver part two.", "- Next step: Build part two.", "- BlockedBy: split-parent-one", ""}, "\n")
	if err := os.WriteFile(draft, []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
	tip := bed.tip()
	gcliBudgetMust(t, bed, "goal", "split", "split-parent", "--plan", draft)
	if bed.tip() == tip {
		t.Fatal("goal split did not publish")
	}
	one := bed.accepted("plans/goals/split-parent-one.md")
	parent := bed.accepted("records/goals/split-parent.md")
	backlog := bed.accepted("plans/goals/backlog.md")
	if goalCLILine(one, "- Arc: ") != "- Arc: split-parent" || !strings.HasPrefix(goalCLILine(parent, "- Ratified: "), "- Ratified: tier=main ") ||
		!strings.Contains(parent, "goal:split-parent-one") || goalCLILine(backlog, "- split-parent opid=") == "" ||
		bed.accepted("plans/goals/split-parent-two.md") == "" {
		t.Fatalf("the atomic split records are incomplete:\n%s\n%s\n%s", one, parent, backlog)
	}
	blocked := bed.goalRecord("norm-parent")
	if goalCLILine(blocked, "- BlockedBy: ") != "- BlockedBy: split-parent-one, split-parent-two" || !strings.Contains(blocked, " blocker=split-parent-one because=") {
		t.Fatalf("the split did not move the blocked goal's edge and park to the members:\n%s", blocked)
	}
	gcliBudgetRefused(t, bed, "was split into member goals and never comes back", "goal", "reopen", "split-parent", "--next", "Bring it back.")
	if code, out, errOut := gcliBudgetFamily(t, bed, "prune", "--keep", "0"); code != 0 {
		t.Fatalf("goal prune: code=%d out=%q err=%q", code, out, errOut)
	}
	if bed.accepted("records/goals/split-parent.md") != "" {
		t.Fatal("prune kept the decomposed parent's record")
	}
	gcliBudgetRefused(t, bed, "goal id split-parent was used by a goal that was split", append([]string{"goal", "open", "split-parent", "--origin", "human",
		"--intent", "Illicit resurrection.", "--next", "Stop.", "--tier", "3", "--risk", gcliBudgetTierThree, "--basis", "fixture risk"}, gcliBudgetHuman...)...)
}

// gcliBudgetSweep runs the classify-sweep owner with the bed's dependencies.
// The owner prints its preview and refusals on the process streams, so the
// tests read the listing and the refusal codes from goal.PreviewClassificationSweep,
// the owner's own preview, and assert the command's exit and the ledger.
// gcliBudgetSweep runs classify-sweep on the bed's own streams: its listing,
// outcome and refusals never touch the process's, which a parallel test may
// be capturing.
func gcliBudgetSweep(bed *goalCLIBed, args ...string) (int, string, string) {
	return bed.owner(func(dependencies syncRequestDependencies) int {
		return runGoalClassifySweepWithInputs(append([]string{"--root", bed.root}, args...), bed.prove, bed.commandNow, dependencies)
	})
}

// classification-sweep: a tierless claim resolves under tier-three rules
// before TierLaw; the preview is inert, normalized and sorted with its
// digest; malformed drafts refuse by code; a changed draft refuses by digest;
// confirmation classifies every goal and installs TierLaw with the last edit;
// after TierLaw a tierless claimed goal refuses its dispatch binding.
func TestGoalCLIBudgetClassificationSweep(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	bed.announceHolder()
	reads := gcliBudgetReads(bed, nil)
	gcliBudgetHumanMust(t, bed, "goal", "budget", "ship-widget", "--elapsed-limit", "8h", "--attempt-limit", "10",
		"--reserved-job-minutes-limit", "1200", "--active-job-limit", "1", "--review-round-limit", "3")
	binding, err := dispatchcore.ResolveGoalBindingWithReads(bed.root, "ship-widget", bed.clock(), reads)
	if err != nil || binding.Tier != 3 {
		t.Fatalf("tierless pre-TierLaw claim did not resolve under tier-three rules: %+v %v", binding, err)
	}
	preSweep := bed.accepted("plans/goals/ship-widget.md")

	dir := t.TempDir()
	writeDraft := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	draftBody := "ship-widget 3,1,1,1 claimed migration\nfix-docs 1,1,1,1 queued migration\nperf-pass 2,1,1,1 parked migration"
	draft := writeDraft("classification-draft.txt", draftBody)
	endpoint, err := bed.endpoint(bed.root)
	if err != nil {
		t.Fatal(err)
	}
	before := bed.tip()
	listing, err := goal.PreviewClassificationSweep(endpoint, []byte(draftBody+"\n"), bed.clock())
	if err != nil {
		t.Fatal(err)
	}
	want := "fix-docs 1,1,1,1 tier=1 queued migration\nperf-pass 2,1,1,1 tier=2 parked migration\nship-widget 3,1,1,1 tier=3 claimed migration"
	if got := strings.Join(listing.Lines, "\n"); got != want {
		t.Fatalf("classification preview was not normalized and sorted:\n%s", got)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(listing.Digest) {
		t.Fatalf("classification preview did not carry its SHA-256 digest: %q", listing.Digest)
	}
	if code, stdout, stderr := gcliBudgetSweep(bed, "--draft", draft, "--preview"); code != 0 || bed.tip() != before ||
		stdout != want+"\nconfirm with: --confirm "+listing.Digest+"\n" || stderr != "" {
		t.Fatalf("the preview was not inert on the supplied streams: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	for _, row := range []struct{ code, words, body string }{
		{"SWEEP_UNKNOWN_GOAL", "goal absent-goal isn't an open goal still waiting for its risk", "fix-docs 1,1,1,1 queued migration\nperf-pass 2,1,1,1 parked migration\nship-widget 3,1,1,1 claimed migration\nabsent-goal 1,1,1,1 unknown"},
		{"SWEEP_DUPLICATE_GOAL", "goal fix-docs is in the draft twice", "fix-docs 1,1,1,1 queued migration\nfix-docs 2,1,1,1 duplicate\nperf-pass 2,1,1,1 parked migration\nship-widget 3,1,1,1 claimed migration"},
		{"SWEEP_INCOMPLETE", "goal perf-pass is missing from the draft", "fix-docs 1,1,1,1 queued migration\nship-widget 3,1,1,1 claimed migration"},
		{"SWEEP_MALFORMED_ROW", "draft line 1 (fix-docs) isn't", "fix-docs 1 invalid tier\nperf-pass 2,1,1,1 parked migration\nship-widget 3,1,1,1 claimed migration"},
	} {
		path := writeDraft("refusal-"+row.code+".txt", row.body)
		if _, err := goal.PreviewClassificationSweep(endpoint, []byte(row.body+"\n"), bed.clock()); err == nil || goal.RefusalCode(err) != row.code || !strings.Contains(err.Error(), row.words) {
			t.Fatalf("classification refusal %s did not fire: %v", row.code, err)
		}
		if code, stdout, stderr := gcliBudgetSweep(bed, "--draft", path, "--preview"); code == 0 || bed.tip() != before ||
			stdout != "" || !strings.HasPrefix(stderr, "goal classify-sweep: "+row.words) {
			t.Fatalf("classify-sweep --preview did not refuse %s on the supplied stderr: code=%d stdout=%q stderr=%q", row.code, code, stdout, stderr)
		}
	}
	changedBody := strings.Replace(draftBody, "claimed migration", "changed migration", 1)
	changedListing, err := goal.PreviewClassificationSweep(endpoint, []byte(changedBody+"\n"), bed.clock())
	if err != nil || changedListing.Digest == listing.Digest {
		t.Fatalf("a changed draft kept the listing digest: %+v %v", changedListing, err)
	}
	changed := writeDraft("changed-draft.txt", changedBody)
	if code, stdout, stderr := gcliBudgetSweep(bed, "--draft", changed, "--confirm", listing.Digest, "--by", "Wido", "--fixture-human-authority"); code != 1 || bed.tip() != before ||
		stdout != "" || !strings.HasPrefix(stderr, "goal classify-sweep: the list of goals changed since the preview") {
		t.Fatalf("changed classification draft did not refuse by digest on the supplied stderr: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	if code, stdout, stderr := gcliBudgetSweep(bed, "--draft", draft, "--confirm", listing.Digest, "--by", "Wido", "--fixture-human-authority"); code != 0 ||
		!strings.Contains(stdout, `"outcome":"confirmed"`) || !strings.Contains(stdout, `"classified":3`) || stderr != "" {
		t.Fatalf("classification confirmation did not confirm on the supplied stdout: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, row := range []struct{ id, tier, rounds string }{{"fix-docs", "1", "0"}, {"perf-pass", "2", "2"}, {"ship-widget", "3", "3"}} {
		record := bed.goalRecord(row.id)
		if goalCLILine(record, "- Tier: ") != "- Tier: "+row.tier || !strings.HasSuffix(goalCLILine(record, "- Budget: "), "reviewRoundLimit="+row.rounds) {
			t.Fatalf("classification did not normalize %s to tier %s and %s rounds:\n%s", row.id, row.tier, row.rounds, record)
		}
	}
	if !strings.HasPrefix(goalCLILine(bed.accepted("plans/goals/backlog.md"), "- TierLaw: "), "- TierLaw: since=") {
		t.Fatalf("classification confirmation did not install TierLaw:\n%s", bed.accepted("plans/goals/backlog.md"))
	}

	// Keep the classified root, restore the pre-sweep claimed record: the
	// active dispatch binding refuses a tierless goal after TierLaw.
	gcliBudgetInstall(t, bed, "fixture-tierless-after-tierlaw", goal.Change{Path: "plans/goals/ship-widget.md", Content: []byte(preSweep)})
	if _, err := dispatchcore.ResolveGoalBindingWithReads(bed.root, "ship-widget", bed.clock(), reads); err == nil ||
		!strings.Contains(err.Error(), "classify the goal first") {
		t.Fatalf("post-TierLaw tierless goal did not refuse dispatch binding: %v", err)
	}
}

// power-of-attorney (R-95-m1e, R-105-m1e): a person records an entry; the
// seat approves and budgets a tier-1 goal under it as its own act with the
// entry on the line; anything outside the entry's tiers, acts, form or dates
// is refused; an entry naming resume-parked lifts a person's park on a tier-1
// goal saying what it verified, never a tier-2 goal or a blocker park.
func TestGoalCLIBudgetPowerOfAttorney(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	bed.announceHolder()
	gcliBudgetMust(t, bed, "goal", "release", "ship-widget", "--reason", "free the seat")
	bed.setNow(time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC))
	gcliBudgetOpen(t, bed, "poa-small", gcliBudgetTierOne, "Approve it under the entry.")
	gcliBudgetOpen(t, bed, "poa-medium", gcliBudgetTierTwo, "Refuse it under the entry.")
	gcliBudgetOpen(t, bed, "poa-late", gcliBudgetTierOne, "Refuse it under the entry.")
	entry := gcliBudgetGrant(t, bed, "1", "approve,budget", "2026-08-25")
	if line := goalCLILine(bed.accepted("plans/goals/backlog.md"), "- "+entry+" "); line !=
		"- "+entry+" by=human:Wido tiers=1 verbs=approve,set-budget since=2026-08-20T09:00:00Z expires=2026-08-25" {
		t.Fatalf("the root record does not carry the entry: %q", line)
	}

	gcliBudgetMust(t, bed, "goal", "approve", "poa-small", "--under", entry)
	small := bed.goalRecord("poa-small")
	if approved := goalCLILine(small, "- Approved: "); !strings.HasPrefix(approved, "- Approved: by=fixture-machine+fixture-lineage ") ||
		!strings.Contains(approved, " authority=attorney ") {
		t.Fatalf("the approval is not the seat's own act under attorney: %q", approved)
	}
	if !strings.Contains(small, " approve actor=fixture-machine+fixture-lineage targets=poa-small authorityOutcome=POWER_OF_ATTORNEY authorityRuling="+entry) {
		t.Fatalf("the history line does not name the entry:\n%s", small)
	}
	gcliBudgetMust(t, bed, "goal", "claim", "poa-small")
	gcliBudgetMust(t, bed, "goal", "budget", "poa-small", "--under", entry, "--elapsed-limit", "1h", "--attempt-limit", "2",
		"--reserved-job-minutes-limit", "120", "--active-job-limit", "1", "--review-round-limit", "0")
	if line := goalCLILine(bed.goalRecord("poa-small"), "- Budget: "); line != "- Budget: elapsedLimit=1h attemptLimit=2 reservedJobMinutesLimit=120 activeJobLimit=1 reviewRoundLimit=0" {
		t.Fatalf("set-budget under attorney did not land its box: %q", line)
	}
	gcliBudgetRefused(t, bed, "only up to its tier 1 budget", "goal", "budget", "poa-small", "--under", entry, "--elapsed-limit", "8h", "--attempt-limit", "2",
		"--reserved-job-minutes-limit", "120", "--active-job-limit", "1", "--review-round-limit", "0")
	gcliBudgetRefused(t, bed, "covers tier 1 only", "goal", "approve", "poa-medium", "--under", entry)

	wide := gcliBudgetGrant(t, bed, "1,2", "approve", "2026-08-24")
	gcliBudgetMust(t, bed, "goal", "approve", "poa-medium", "--under", wide)
	if line := goalCLILine(bed.accepted("plans/goals/backlog.md"), "- "+wide+" "); !strings.HasPrefix(line, "- "+wide+" by=human:Wido tiers=1,2 verbs=approve ") {
		t.Fatalf("the root record does not carry the tiers 1,2 entry: %q", line)
	}
	gcliBudgetRefused(t, bed, "seat's own act", "goal", "approve", "poa-late", "--under", entry, "--by", "Wido")

	gcliBudgetHumanMust(t, bed, "goal", "pause", "poa-late", "--reason", "wait for the vendor's 1.2 release")
	gcliBudgetRefused(t, bed, "covers approve,set-budget, not unpark", "goal", "resume", "poa-late", "--under", entry, "--verified", "1.2 is on the vendor's page")
	gcliBudgetRefused(t, bed, "only a person, or a grant naming resume-parked, lifts that", "goal", "resume", "poa-late")
	lift := gcliBudgetGrant(t, bed, "1,2", "resume-parked", "2026-08-24")
	gcliBudgetRefused(t, bed, "--verified", "goal", "resume", "poa-late", "--under", lift)
	gcliBudgetRefused(t, bed, "seat's own act", "goal", "resume", "poa-late", "--under", lift, "--verified", "it holds", "--by", "Wido")
	gcliBudgetRefused(t, bed, "--under", "goal", "resume", "poa-late", "--verified", "it holds")
	gcliBudgetMust(t, bed, "goal", "resume", "poa-late", "--under", lift, "--verified", "1.2 is on the vendor's page")
	late := bed.goalRecord("poa-late")
	if goalCLILine(late, "- State: ") != "- State: queued" {
		t.Fatalf("the lifted goal did not return to its resting state:\n%s", late)
	}
	if !strings.Contains(late, " unpark actor=fixture-machine+fixture-lineage targets=poa-late authorityOutcome=POWER_OF_ATTORNEY authorityRuling="+lift+" reason=verified: 1.2 is on the vendor's page") {
		t.Fatalf("the attorney unpark's history line does not carry the entry and what was verified:\n%s", late)
	}
	gcliBudgetHumanMust(t, bed, "goal", "pause", "poa-medium", "--reason", "the person pauses a tier-2 goal")
	gcliBudgetRefused(t, bed, "tier-1 goals only", "goal", "resume", "poa-medium", "--under", lift, "--verified", "it holds")
	gcliBudgetMust(t, bed, "goal", "open", "poa-defect", "--blocks", "poa-small", "--intent", "The defect that blocks poa-small.",
		"--next", "Fix it.", "--risk", gcliBudgetTierOne, "--basis", "power of attorney fixture")
	gcliBudgetRefused(t, bed, "comes back by itself", "goal", "resume", "poa-small", "--under", lift, "--verified", "the defect is fixed")

	bed.setNow(time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC))
	gcliBudgetRefused(t, bed, "expired 2026-08-25", "goal", "approve", "poa-late", "--under", entry)
}

// The long set-budget form (retired with internal goal set-budget; a person
// runs goal budget G BOX), kept for these tests as a composition of the budget
// owners the public goal budget and goal resume call.

func runGoalSetBudgetWithInputs(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies, binding goalBindingResolver) int {
	values := newHumanVerbValues("set-budget", args)
	values.bindDependencies(dependencies)
	f, ok := parseHumanSyncFlags(values, "set-budget", args)
	if !ok {
		return 2
	}
	if f.under != "" {
		if !converted(f.root) || f.id == "" {
			fmt.Fprintln(dependencies.errStream(), "goal set-budget --under needs a synced backlog plus --id")
			return 2
		}
		return runGoalUnderAttorneyWithInputs("set-budget", f, commandNow, dependencies)
	}
	box := ""
	if parsed, err := f.budgetTuple(true); err == nil {
		box = goalbudget.FormatBox(*parsed)
	}
	fmt.Fprintln(dependencies.errStream(), "hint:", values.budgetCommand(box))
	if box != "" && stoppedGoalForSetBudgetWithInputs(f, commandNow, dependencies.endpoint) {
		return runGoalStoppedSetBudgetWithInputs(values, f, prove, commandNow, dependencies, binding)
	}
	return runGoalBudgetPreparedWithInputs(values, f, "", prove, commandNow, dependencies, binding)
}

func stoppedGoalForSetBudgetWithInputs(flags *syncFlags, commandNow func(string) (time.Time, error), resolveEndpoint func(string) (goal.Endpoint, error)) bool {
	if !converted(flags.root) || flags.id == "" {
		return false
	}
	if resolveEndpoint == nil || commandNow == nil {
		return false
	}
	endpoint, err := resolveEndpoint(flags.root)
	if err != nil {
		return false
	}
	now, err := commandNow(flags.root)
	if err != nil {
		return false
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return false
	}
	file := projection.Tree.Live[flags.id]
	return file != nil && file.StopFence != nil
}

func runGoalStoppedSetBudgetWithInputs(values *humanVerbValues, flags *syncFlags, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies, binding goalBindingResolver) int {
	budget, err := flags.budgetTuple(true)
	if err != nil {
		return runGoalBudgetPreparedWithInputs(values, flags, "", prove, commandNow, dependencies, binding)
	}
	values.box = budget
	classification, err := classifyGoalAuthorityFirstWithFacts("set-budget", flags, dependencies.authorityFacts)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the goal authority classification before retrying"})
	}
	proof, err := proveGoalHumanAuthorityAt("set-budget", flags, prove, commandNow)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanProofRemedy(values, flags.fixtureHumanAuthority, flags.temporaryWord, flags.reviewBy, err))
	}
	if err := resolveGoalHuman(flags, proof); err != nil {
		return refuseHumanVerb(values, 2, err.Error(), humanVerbRemedy{words: "re-enroll with goal enroll-terminal --by <your name>, or add --by <your name> to this command"})
	}
	values.by = flags.by
	req, err := syncReqClassifiedWithTerminalGradeAtWithDependencies(flags.root, flags.by, flags.lineage, &proof, classification, false, commandNow, dependencies)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the named checkout identity fact before retrying"})
	}
	req.ApprovedRef = flags.approvedRef
	result, err := goal.SetBudgetApproved(req, flags.id, *budget, &proof)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{command: values.budgetCommandWithoutApprovedRef("keep")})
	}
	if result.Outcome != goal.OutcomeConfirmed {
		writeJSONLine(dependencies.outStream(), dependencies.errStream(), map[string]any{"outcome": result.Outcome, "tip": result.Tip, "detail": result.Detail})
		return refuseHumanVerb(values, 1, result.Detail, humanVerbRemedy{command: values.budgetCommandWithoutApprovedRef("keep")})
	}
	if err := recordGoalApprovalProof(flags.root, goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage), "goal set-budget", proof); err != nil {
		return refuseHumanVerb(values, 1, "the act landed at tip "+result.Tip+", but its authority proof did not: "+err.Error(), humanVerbRemedy{words: "the act already landed; do not run it again"})
	}
	if proof.TemporaryResumeFor(flags.root) {
		fmt.Fprintf(dependencies.outStream(), "goal set-budget: TEMPORARY authority under a recorded relayed word (human provenance not verified); re-approval due %s at an agent-free terminal\n", flags.reviewBy)
	}
	return writeSyncResult(dependencies.outStream(), dependencies.errStream(), result, nil)
}
