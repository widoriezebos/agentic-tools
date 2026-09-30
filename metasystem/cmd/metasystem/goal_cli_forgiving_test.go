package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// The forgiving budget scenarios of goal-cli-fixtures.sh (forgiving-*), run
// in the Git-free goal CLI bed through the public object-action grammar. The
// public goal budget G BOX is the forgiving budget command. Its refusals are
// rendered by the public router, so their shape differs from the goal
// family's two-line refusal:
//
//	family "goal budget: S" + "run: C"
//	  public "metasystem goal budget: S" + "run: C"
//	family "goal budget: S" + "no command completes this: W"
//	  public "metasystem goal budget: S" + "needed first: W"
//
// A printed remedy is parsed and executed in process through the public
// router, never through a shell.

const gcliForgivingFixture = "--fixture-human-authority"

// The shell bed's forgiving clock: the scenario's base is the bed's starting
// clock, forgiving_breach_at is six hours later and the second breach thirty.
var (
	gcliForgivingBase         = goalCLISeedNow.Add(time.Minute)
	gcliForgivingBreachAt     = gcliForgivingBase.Add(6 * time.Hour)
	gcliForgivingSecondBreach = gcliForgivingBase.Add(30 * time.Hour)
)

// gcliForgivingBinding resolves a claimed goal's dispatch binding from the
// bed's ledger, as the production command does from the checkout: the goal
// budget of a breach-stopped goal reads it.
func gcliForgivingBinding(bed *goalCLIBed) goalBindingResolver {
	reads := dispatchcore.ProofAdmissionReads{
		ResolveEndpoint: bed.endpoint,
		ResolveMachine:  func(string) (string, error) { return bed.machine, nil },
		Receipt: dispatchcore.ReceiptAdmissionSource{
			AcceptedLedgerTip: func(string) (string, bool, error) { return bed.repo.Accepted() },
			TopLevel:          func(string) (string, error) { return bed.root, nil },
			FileAt: func(_, tip, path string) ([]byte, bool, error) {
				files, err := bed.repo.Files(tip, path)
				if err != nil {
					return nil, false, err
				}
				data, ok := files[path]
				return data, ok, nil
			},
		},
	}
	return func(root, id string, now time.Time) (dispatchcore.GoalBinding, error) {
		return dispatchcore.ResolveGoalBindingWithReads(root, id, now, reads)
	}
}

// gcliForgivingPublic is the bed's public runner with the binding reader
// production supplies.
func gcliForgivingPublic(bed *goalCLIBed, args ...string) (int, string, string) {
	bed.t.Helper()
	command, rest, ok := resolveIntentArgv(args)
	if !ok {
		bed.t.Fatalf("no public command %q", args)
	}
	var stdout, stderr bytes.Buffer
	owners := bed.owners(&stdout, &stderr)
	owners.binding = gcliForgivingBinding(bed)
	code := runIntentIn(command, rest, &stdout, &stderr, bed.root, owners)
	return code, stdout.String(), stderr.String()
}

// gcliForgivingBoxed seeds queued tier-three goals that already carry the
// standing box 3h/5/300m/1/2, as the shell bed's goal open with the five long
// limits did. The public goal open takes no limits (it records the tier box),
// so the opened-with-a-box state is seeded.
func gcliForgivingBoxed(ids ...string) func(map[string]*goal.GoalFile) {
	return func(goals map[string]*goal.GoalFile) {
		at := goalCLISeedNow.Format(time.RFC3339)
		for index, id := range ids {
			box := goal.Budget{ElapsedLimit: "3h", AttemptLimit: 5, ReservedJobMinutesLimit: 300, ActiveJobLimit: 1, ReviewRoundLimit: 2}
			goals[id] = &goal.GoalFile{
				Id: id, State: goal.StateQueued, Tier: 3,
				Risk:   &goal.RiskRecord{Severity: 3, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "fixture risk"},
				Intent: "Exercise the forgiving budget command for " + id + ".", Origin: goal.OriginHuman,
				NextStep: "Inspect the recorded box.", OpenedAt: at, Revision: 1, Budget: &box,
				History: []goal.HistoryLine{{
					At: at, Opid: goal.Opid(fmt.Sprintf("01ARZ3NDEKTSV4RRFFQ69G6%03d", index), "fixture-machine", "fixture-lineage"),
					Verb: "open", Actor: "human:Wido", Targets: []string{id}, Keep: -1,
				}},
			}
		}
	}
}

// gcliForgivingOpen is open_forgiving_fixture_goal without limits: a human
// opens a tier-three goal through the public goal open.
func gcliForgivingOpen(t *testing.T, bed *goalCLIBed, ids ...string) {
	t.Helper()
	for _, id := range ids {
		gcliForgivingMust(t, bed, "goal", "open", id,
			"--intent", "Exercise the forgiving budget command for "+id+".", "--next", "Inspect the recorded box.",
			"--origin", "human", "--by", "Wido", gcliForgivingFixture,
			"--tier", "3", "--risk", "severity=3,novelty=1,exposure=1,accumulation=1", "--basis", "fixture risk")
	}
}

func gcliForgivingMust(t *testing.T, bed *goalCLIBed, args ...string) string {
	t.Helper()
	code, stdout, stderr := gcliForgivingPublic(bed, args...)
	if code != 0 {
		t.Fatalf("%q: code=%d stdout=%q stderr=%q", args, code, stdout, stderr)
	}
	return stdout
}

func gcliForgivingBudget(record string) string {
	return strings.TrimPrefix(goalCLILine(record, "- Budget: "), "- Budget: ")
}

// gcliForgivingLastHistory is the shell's awk '$4 " " $5' of the newest
// history line: its verb and actor.
func gcliForgivingLastHistory(record string) string {
	value, inHistory := "", false
	for _, line := range strings.Split(record, "\n") {
		if line == "History:" {
			inHistory = true
			continue
		}
		if inHistory && strings.HasPrefix(line, "- ") {
			if fields := strings.Fields(line); len(fields) >= 5 {
				value = fields[3] + " " + fields[4]
			}
		}
	}
	return value
}

func gcliForgivingLines(stderr string) []string {
	trimmed := strings.TrimRight(stderr, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// gcliForgivingWords asserts the public words-only refusal: two lines, the
// verb's sentence and "needed first: WORDS", and no run line.
func gcliForgivingWords(t *testing.T, label, verb string, code int, stderr, sentence, words string) {
	t.Helper()
	lines := gcliForgivingLines(stderr)
	if code == 0 || len(lines) != 2 || !strings.HasPrefix(lines[0], "metasystem "+verb+": "+sentence) ||
		!strings.HasPrefix(lines[1], "needed first: "+words) || strings.Contains(stderr, "run:") {
		t.Fatalf("%s did not print the two-line words refusal (sentence %q, words %q): code=%d stderr=%q", label, sentence, words, code, stderr)
	}
}

// gcliForgivingRun asserts a two-line refusal whose second line is the
// command that resolves it ("Messages a Person Reads").
func gcliForgivingRun(t *testing.T, label, verb string, code int, stderr, sentence, run string) {
	t.Helper()
	lines := gcliForgivingLines(stderr)
	if code == 0 || len(lines) != 2 || !strings.HasPrefix(lines[0], "metasystem "+verb+": "+sentence) ||
		!strings.HasPrefix(lines[1], "run: "+run) || strings.Contains(stderr, "needed first") {
		t.Fatalf("%s did not print the two-line run refusal (sentence %q, run %q): code=%d stderr=%q", label, sentence, run, code, stderr)
	}
}

// gcliForgivingCommand asserts the public command refusal and returns the
// printed remedy: the verb's sentence and "run: COMMAND".
func gcliForgivingCommand(t *testing.T, label, verb string, code int, stderr string) []string {
	t.Helper()
	lines := gcliForgivingLines(stderr)
	if code == 0 || len(lines) != 2 || !strings.HasPrefix(lines[0], "metasystem "+verb+": ") ||
		!strings.HasPrefix(lines[1], "run: metasystem ") {
		t.Fatalf("%s did not print the two-line command refusal: code=%d stderr=%q", label, code, stderr)
	}
	return shellWords(strings.TrimPrefix(lines[1], "run: "))
}

// gcliForgivingRunRemedy runs a printed public remedy through the router.
func gcliForgivingRunRemedy(t *testing.T, bed *goalCLIBed, label string, remedy []string) string {
	t.Helper()
	if len(remedy) < 2 || remedy[0] != "metasystem" || remedy[1] == "internal" {
		t.Fatalf("%s printed %q, not a public command", label, remedy)
	}
	code, stdout, stderr := gcliForgivingPublic(bed, remedy[1:]...)
	if code != 0 {
		t.Fatalf("%s printed a command that did not complete: %q code=%d stdout=%q stderr=%q", label, remedy, code, stdout, stderr)
	}
	return stdout
}

// gcliForgivingRefusalRemedy is run_forgiving_refusal_remedy: the refused
// command prints a runnable remedy; running it lands the same Budget line and
// the same newest history verb and actor as the long-form twin command lands
// on the twin goal.
func gcliForgivingRefusalRemedy(t *testing.T, bed *goalCLIBed, label, goalID, twinID string, twin func(), refused ...string) []string {
	t.Helper()
	tip := bed.tip()
	code, _, stderr := gcliForgivingPublic(bed, refused...)
	remedy := gcliForgivingCommand(t, label, "goal budget", code, stderr)
	if bed.tip() != tip {
		t.Fatalf("%s published although it refused", label)
	}
	if len(remedy) < 5 || strings.Join(remedy[:4], " ") != "metasystem goal budget "+goalID {
		t.Fatalf("%s printed %q, not the public goal budget %s BOX", label, remedy, goalID)
	}
	gcliForgivingRunRemedy(t, bed, label, remedy)
	// The remedied goal is read before the twin runs: a twin may release it.
	remedied := bed.goalRecord(goalID)
	twin()
	gcliForgivingSameRecords(t, label, remedied, bed.goalRecord(twinID))
	return remedy
}

// gcliForgivingSameAct compares the Budget line and the newest history verb
// and actor of a remedied goal and its long-form twin.
func gcliForgivingSameAct(t *testing.T, bed *goalCLIBed, label, id, twin string) {
	t.Helper()
	gcliForgivingSameRecords(t, label, bed.goalRecord(id), bed.goalRecord(twin))
}

func gcliForgivingSameRecords(t *testing.T, label, remedied, long string) {
	t.Helper()
	budget, twinBudget := gcliForgivingBudget(remedied), gcliForgivingBudget(long)
	history, twinHistory := gcliForgivingLastHistory(remedied), gcliForgivingLastHistory(long)
	if budget == "" || budget != twinBudget || history == "" || history != twinHistory {
		t.Fatalf("%s's remedy did not match its long-form twin:\nremedy: %s | %s\ntwin:   %s | %s\n%s\n%s", label, budget, history, twinBudget, twinHistory, remedied, long)
	}
}

// gcliForgivingLongForm is the long-form twin: goal budget G with the five
// long limits.
func gcliForgivingLongForm(t *testing.T, bed *goalCLIBed, id, elapsed, attempts, minutes, active, rounds string, extra ...string) func() {
	return func() {
		t.Helper()
		args := append([]string{"goal", "budget", id, gcliForgivingFixture,
			"--elapsed-limit", elapsed, "--attempt-limit", attempts, "--reserved-job-minutes-limit", minutes,
			"--active-job-limit", active, "--review-round-limit", rounds}, extra...)
		gcliForgivingMust(t, bed, args...)
	}
}

// gcliForgivingFamily runs a goal family owner in process with the bed's
// dependencies and returns its typed report: the family's refusal sentence
// and remedy, and its notes (hints), which are its printed lines.
func gcliForgivingFamily(bed *goalCLIBed, run func(syncRequestDependencies) int) (int, *ownerReport) {
	report := &ownerReport{}
	code, _, _ := bed.owner(func(dependencies syncRequestDependencies) int {
		dependencies.report = report
		return run(dependencies)
	})
	return code, report
}

func gcliForgivingParse(t *testing.T, record string) *goal.GoalFile {
	t.Helper()
	file, problems := goal.ParseFile([]byte(record))
	if file == nil || len(problems) != 0 {
		t.Fatalf("goal record does not parse: %v\n%s", problems, record)
	}
	return file
}

func gcliForgivingClaimRevision(t *testing.T, record string) uint64 {
	t.Helper()
	file := gcliForgivingParse(t, record)
	if file.Claimed == nil {
		t.Fatalf("goal %s has no claim:\n%s", file.Id, record)
	}
	return file.Claimed.Revision
}

// gcliForgivingBreachStop is the stop custodian's job breach-stop followed
// by job stop-batch-reconcile, in process against the bed's ledger: the
// elapsed budget is proven breached at the bed's clock, the claimed
// revision's launch fence is closed through goal.CloseStop (the transaction
// dispatch EnsureBreachStop publishes), the stop batch is opened, and
// ReconcileStopBatch completes it (the bed has no jobs of the goal).
func gcliForgivingBreachStop(t *testing.T, bed *goalCLIBed, id, ulid string) {
	t.Helper()
	stopID := gcliForgivingOpenStop(t, bed, id, ulid)
	batch, err := dispatchcore.ReconcileStopBatch(bed.root, stopID, bed.clock())
	if err != nil || batch.State != goal.StopBatchComplete {
		t.Fatalf("stop batch %s did not complete: %+v %v", stopID, batch, err)
	}
}

// gcliForgivingOpenStop is the stop custodian's job breach-stop alone: the
// fence is closed and the stop batch recorded OPEN, not yet reconciled. It
// returns the stop id.
func gcliForgivingOpenStop(t *testing.T, bed *goalCLIBed, id, ulid string) string {
	t.Helper()
	file := gcliForgivingParse(t, bed.goalRecord(id))
	if file.State != goal.StateClaimed || file.Claimed == nil || file.StopCapability == nil {
		t.Fatalf("goal %s has no claimed revision for its breach-stop: %+v", id, file)
	}
	now := bed.clock()
	if projection := dispatchcore.ProjectBudget(bed.root, file, now); projection.ElapsedState != dispatchcore.ElapsedBreach {
		t.Fatalf("goal %s has no elapsed breach at %s: %+v", id, now, projection)
	}
	capability := *file.StopCapability
	stopID := fmt.Sprintf("stop-%s-r%d-f%d", id, file.Claimed.Revision, capability.FenceEpoch+1)
	endpoint, err := bed.endpoint(bed.root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := goal.CloseStop(goal.CloseStopRequest{
		VerbRequest: goal.VerbRequest{
			Endpoint: endpoint, Actor: goal.Actor{Machine: file.Claimed.Machine, Lineage: "goal-stop-custodian"},
			Ulid: ulid, Now: now, ClaimEpoch: capability.ClaimEpoch,
		},
		GoalID: id, StopID: stopID, Reason: goal.StopReasonElapsedLimit, Capability: capability,
	})
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("breach-stop of %s did not close its fence: %+v %v", id, result, err)
	}
	fenced := gcliForgivingParse(t, bed.goalRecord(id))
	if fenced.StopFence == nil || fenced.StopFence.StopID != stopID {
		t.Fatalf("goal %s carries no fence %s:\n%s", id, stopID, bed.goalRecord(id))
	}
	stamp := now.UTC().Format(time.RFC3339)
	if err := goal.WriteStopBatch(bed.root, goal.StopBatch{
		StopID: stopID, GoalID: id, GoalRevision: file.Claimed.Revision, FenceEpoch: fenced.StopFence.Epoch,
		CapabilityGeneration: fenced.StopCapability.Generation, Machine: fenced.StopCapability.Machine,
		ClaimEpoch: fenced.StopCapability.ClaimEpoch, Reason: fenced.StopFence.Reason, State: goal.StopBatchOpen,
		OpenedAt: stamp, UpdatedAt: stamp,
	}); err != nil {
		t.Fatal(err)
	}
	return stopID
}

func gcliForgivingReleaseIfClaimed(t *testing.T, bed *goalCLIBed, id string) {
	t.Helper()
	if record := bed.goalRecord(id); record != "" && goalCLILine(record, "- State: ") == "- State: claimed" {
		gcliForgivingMust(t, bed, "goal", "release", id, "--reason", "free the machine's claim for the next fixture row")
	}
}

func gcliForgivingClaim(t *testing.T, bed *goalCLIBed, id string) {
	t.Helper()
	stdout := gcliForgivingMust(t, bed, "goal", "claim", id)
	if !strings.HasPrefix(stdout, "claim: "+id+" is claimed") {
		t.Fatalf("claiming %s did not confirm: %q", id, stdout)
	}
}

// gcliForgivingHint finds the family's "hint: metasystem goal budget G ...BOX" note.
func gcliForgivingHint(notes []string, id, suffix string) bool {
	for _, note := range notes {
		if strings.HasPrefix(note, "hint: metasystem goal budget "+id+" ") && strings.HasSuffix(note, suffix) {
			return true
		}
	}
	return false
}

// forgiving-budget-states (goal-cli-fixtures.sh 1592-1659, clock 353-359).
func TestGoalCLIForgivingBudgetStates(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	bed.announceHolder()

	// A queued goal accepts the box after its flags; the enrolled name is the
	// approver.
	gcliForgivingMust(t, bed, "goal", "budget", "fix-docs", gcliForgivingFixture, "2h/4/240m/1/2")
	approved := bed.goalRecord("fix-docs")
	if goalCLILine(approved, "- State: ") != "- State: approved" ||
		gcliForgivingBudget(approved) != "elapsedLimit=2h attemptLimit=4 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=2" ||
		!strings.Contains(approved, " approve actor=human:Wido ") {
		t.Fatalf("queued goal budget did not publish an approval:\n%s", approved)
	}
	// Re-approval with the box before the flags changes only the tuple.
	gcliForgivingMust(t, bed, "goal", "budget", "fix-docs", "3h/5/300m/1/2", "--by", "Wido", gcliForgivingFixture)
	reapproved := bed.goalRecord("fix-docs")
	if strings.Count(reapproved, " approve actor=human:Wido ") != 2 ||
		gcliForgivingBudget(reapproved) != "elapsedLimit=3h attemptLimit=5 reservedJobMinutesLimit=300 activeJobLimit=1 reviewRoundLimit=2" {
		t.Fatalf("approved goal budget did not re-approve:\n%s", reapproved)
	}

	// The parked transaction keeps the pause intact.
	gcliForgivingMust(t, bed, "goal", "budget", "perf-pass", "2h/4/240m/1/2", "--by", "Wido", gcliForgivingFixture)
	parked := bed.goalRecord("perf-pass")
	if goalCLILine(parked, "- State: ") != "- State: parked" || goalCLILine(parked, "- Parked: ") == "" ||
		gcliForgivingBudget(parked) != "elapsedLimit=2h attemptLimit=4 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=2" ||
		!strings.Contains(parked, " approve actor=human:Wido ") {
		t.Fatalf("parked goal budget moved or erased the park:\n%s", parked)
	}

	// A claimed goal routes through set-budget and advances the claim binding.
	before := gcliForgivingClaimRevision(t, bed.goalRecord("ship-widget"))
	gcliForgivingMust(t, bed, "goal", "budget", "ship-widget", "4h/6/600m/1/2", "--by", "Wido", gcliForgivingFixture)
	rebound := bed.goalRecord("ship-widget")
	if after := gcliForgivingClaimRevision(t, rebound); after <= before || !strings.Contains(rebound, " set-budget actor=human:Wido ") {
		t.Fatalf("claimed goal budget did not rebind through set-budget (revision %d -> %d):\n%s", before, after, rebound)
	}

	// The stop custodian closes the claimed revision; keep then routes through
	// the standing resume transaction, and its remedy drops the rejected
	// approval reference.
	bed.setNow(gcliForgivingBreachAt)
	gcliForgivingBreachStop(t, bed, "ship-widget", "01ARZ3NDEKTSV4RRFFQ69G7S01")
	code, _, stderr := gcliForgivingPublic(bed, "goal", "budget", "ship-widget", "keep", "--approved-ref", "rejected-on-resume", "--by", "Wido", gcliForgivingFixture)
	remedy := gcliForgivingCommand(t, "the stopped-goal keep", "goal budget", code, stderr)
	if strings.Contains(strings.Join(remedy, " "), "--approved-ref") || remedy[len(remedy)-1] != "keep" {
		t.Fatalf("the stopped-goal keep remedy retained its rejected approval reference: %q", remedy)
	}
	gcliForgivingRunRemedy(t, bed, "the stopped-goal keep", remedy)
	resumed := bed.goalRecord("ship-widget")
	if !strings.Contains(resumed, " resume actor=human:Wido ") || goalCLILine(resumed, "- StopFence:") != "" {
		t.Fatalf("keep did not resume the breach-stopped goal:\n%s", resumed)
	}

	// A done goal refuses with words only.
	tip := bed.tip()
	code, _, stderr = gcliForgivingPublic(bed, "goal", "budget", "port-engine", "norm", "--by", "Wido", gcliForgivingFixture)
	gcliForgivingWords(t, "port-engine", "goal budget", code, stderr, "the goal is done and its archived budget is read-only", "reopen the goal")
	// An unknown goal: the public router answers before the owner and names
	// the listing as its next command (the family printed words only).
	code, _, stderr = gcliForgivingPublic(bed, "goal", "budget", "absent-goal", "norm", "--by", "Wido", gcliForgivingFixture)
	lines := gcliForgivingLines(stderr)
	if code == 0 || len(lines) != 2 || lines[0] != "metasystem goal budget: no goal absent-goal on the accepted ledger; nothing was done" ||
		lines[1] != "run: metasystem goal list --all  (list the goals by id)" {
		t.Fatalf("absent-goal did not refuse as an unknown goal: code=%d stderr=%q", code, stderr)
	}
	if bed.tip() != tip {
		t.Fatal("a refused budget published")
	}
}

// forgiving-budget-members (goal-cli-fixtures.sh 1661-1772).
func TestGoalCLIForgivingBudgetMembers(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{amend: gcliForgivingBoxed(
		"keep-preset", "empty-member", "four-members", "four-members-twin", "invalid-member", "invalid-member-twin",
		"mixed-box", "approved-completion")})
	norm := "elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3"
	standing := "elapsedLimit=3h attemptLimit=5 reservedJobMinutesLimit=300 activeJobLimit=1 reviewRoundLimit=2"

	gcliForgivingOpen(t, bed, "norm-preset")
	gcliForgivingMust(t, bed, "goal", "budget", "norm-preset", "norm", gcliForgivingFixture)
	if got := gcliForgivingBudget(bed.goalRecord("norm-preset")); got != norm {
		t.Fatalf("norm did not resolve to the tier-three fixture box: %s", got)
	}

	gcliForgivingMust(t, bed, "goal", "budget", "keep-preset", "keep", gcliForgivingFixture)
	if record := bed.goalRecord("keep-preset"); gcliForgivingBudget(record) != standing || goalCLILine(record, "- State: ") != "- State: approved" {
		t.Fatalf("keep did not preserve the opened box:\n%s", record)
	}

	gcliForgivingMust(t, bed, "goal", "budget", "empty-member", "4h////", gcliForgivingFixture)
	if got := gcliForgivingBudget(bed.goalRecord("empty-member")); got != "elapsedLimit=4h attemptLimit=5 reservedJobMinutesLimit=300 activeJobLimit=1 reviewRoundLimit=2" {
		t.Fatalf("empty compact members did not inherit the standing limits: %s", got)
	}

	// keep without a standing box: fix-docs is tierless, so the remedy names
	// the tier-three box itself where the family printed norm.
	gcliForgivingOpen(t, bed, "keep-without-standing-twin")
	gcliForgivingMust(t, bed, "goal", "budget", "keep-without-standing-twin", "norm", "--by", "Wido", gcliForgivingFixture)
	gcliForgivingMust(t, bed, "goal", "unapprove", "keep-without-standing-twin", "--reason", "prepare a fieldless budget twin", "--by", "Wido", gcliForgivingFixture)
	remedy := gcliForgivingRefusalRemedy(t, bed, "keep without a standing box", "fix-docs", "keep-without-standing-twin",
		gcliForgivingLongForm(t, bed, "keep-without-standing-twin", "1d", "10", "1200", "1", "3"),
		"goal", "budget", "fix-docs", "keep", gcliForgivingFixture)
	if remedy[len(remedy)-1] != "1d/10/1200m/1/3" {
		t.Fatalf("keep without a standing box did not print the tier-three box: %q", remedy)
	}
	if got := gcliForgivingBudget(bed.goalRecord("fix-docs")); got != norm {
		t.Fatalf("the printed norm remedy did not approve the goal: %s", got)
	}

	remedy = gcliForgivingRefusalRemedy(t, bed, "four-member compact box", "four-members", "four-members-twin",
		gcliForgivingLongForm(t, bed, "four-members-twin", "4h", "6", "360", "1", "2"),
		"goal", "budget", "four-members", "4h/6/360m/1", gcliForgivingFixture)
	if remedy[len(remedy)-1] != "4h/6/360m/1/2" {
		t.Fatalf("the four-member box was not completed from the standing box: %q", remedy)
	}
	remedy = gcliForgivingRefusalRemedy(t, bed, "invalid compact member", "invalid-member", "invalid-member-twin",
		gcliForgivingLongForm(t, bed, "invalid-member-twin", "4h", "5", "360", "1", "2"),
		"goal", "budget", "invalid-member", "4h/many/360m/1/2", gcliForgivingFixture)
	if remedy[len(remedy)-1] != "4h/5/360m/1/2" {
		t.Fatalf("the invalid member was not replaced by the standing one: %q", remedy)
	}

	// Compact and long form together: the public router refuses before the
	// owner with words only, where the family printed a compact-box command.
	tip := bed.tip()
	code, _, stderr := gcliForgivingPublic(bed, "goal", "budget", "mixed-box", "4h/6/360m/1/2", gcliForgivingFixture,
		"--elapsed-limit", "8h", "--attempt-limit", "10", "--reserved-job-minutes-limit", "1200", "--active-job-limit", "1", "--review-round-limit", "3")
	gcliForgivingRun(t, "compact and long form together", "goal budget", code, stderr,
		"the budget is given twice, as 4h/6/360m/1/2 and as --elapsed-limit", "metasystem goal budget mixed-box 4h/6/360m/1/2")
	if lines := gcliForgivingLines(stderr); strings.Contains(lines[len(lines)-1], "--attempt-limit") {
		t.Fatalf("the retry kept a long limit: %q", stderr)
	}
	if bed.tip() != tip {
		t.Fatal("the mixed box published")
	}

	gcliForgivingOpen(t, bed, "extra-member", "extra-member-twin")
	remedy = gcliForgivingRefusalRemedy(t, bed, "extra compact member", "extra-member", "extra-member-twin",
		gcliForgivingLongForm(t, bed, "extra-member-twin", "4h", "6", "360", "1", "2"),
		"goal", "budget", "extra-member", "4h/6/360m/1/2/ignored", gcliForgivingFixture)
	if remedy[len(remedy)-1] != "4h/6/360m/1/2" {
		t.Fatalf("the extra member was not dropped: %q", remedy)
	}

	gcliForgivingOpen(t, bed, "over-round-limit", "rejected-reference", "fixture-over-norm")
	gcliForgivingMust(t, bed, "goal", "budget", "approved-completion", "keep", gcliForgivingFixture)
	tip = bed.tip()
	code, _, stderr = gcliForgivingPublic(bed, "goal", "budget", "over-round-limit", "4h/6/360m/1/4", gcliForgivingFixture)
	gcliForgivingWords(t, "an over-limit review-round member", "goal budget", code, stderr,
		"reviewRoundLimit 4 exceeds configured maximum 3", "reviewRoundLimit 4 exceeds configured maximum 3")

	// R-129 (U-idem): the standing box repeated is a repeat whose effect
	// holds: success, nothing recorded.
	standingTip := bed.tip()
	if code, _, stderr = gcliForgivingPublic(bed, "goal", "budget", "approved-completion", "3h/5/300m/1", gcliForgivingFixture); code != 0 || stderr != "" || bed.tip() != standingTip {
		t.Fatalf("an approved standing-box completion was not unchanged: code=%d stderr=%q", code, stderr)
	}

	code, _, stderr = gcliForgivingPublic(bed, "goal", "budget", "rejected-reference", "norm", "--approved-ref", "missing-reference", gcliForgivingFixture)
	gcliForgivingWords(t, "a rejected approval reference", "goal budget", code, stderr,
		"GOAL_NORM_REFUSED: --approved-ref missing-reference", "the approved reference must cover this exact goal revision and box")

	code, _, stderr = gcliForgivingPublic(bed, "goal", "budget", "fixture-over-norm", "8h/10/1201m/1/3", gcliForgivingFixture)
	gcliForgivingWords(t, "fixture authority over the norm", "goal budget", code, stderr,
		"GOAL_NORM_REFUSED: goal fixture-over-norm", humanauthority.PersonActRemedy("the over-norm box"))
	if bed.tip() != tip {
		t.Fatal("a words refusal published")
	}
}

// forgiving-budget-identity-aliases (goal-cli-fixtures.sh 1774-1842).
func TestGoalCLIForgivingBudgetIdentityAliases(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	bed.announceHolder()
	gcliForgivingMust(t, bed, "goal", "release", "ship-widget", "--reason", "free the machine's claim for the alias rows")

	gcliForgivingOpen(t, bed, "default-human", "explicit-human")
	gcliForgivingMust(t, bed, "goal", "budget", "default-human", "2h/4/240m/1/2", gcliForgivingFixture)
	if record := bed.goalRecord("default-human"); !strings.Contains(record, " approve actor=human:Wido ") {
		t.Fatalf("the enrollment name did not default --by:\n%s", record)
	}
	gcliForgivingMust(t, bed, "goal", "budget", "explicit-human", "2h/4/240m/1/2", "--by", "Alice", gcliForgivingFixture)
	if record := bed.goalRecord("explicit-human"); !strings.Contains(record, " approve actor=human:Alice ") {
		t.Fatalf("an explicit --by did not win over the enrollment name:\n%s", record)
	}

	// A fieldless enrollment: words only.
	gcliForgivingOpen(t, bed, "nameless-enrollment")
	writeFixtureEnrollment(t, bed.root, "")
	tip := bed.tip()
	code, _, stderr := gcliForgivingPublic(bed, "goal", "budget", "nameless-enrollment", "norm", gcliForgivingFixture)
	gcliForgivingWords(t, "a fieldless enrollment", "goal budget", code, stderr, "the enrolled terminal has no recorded name", "")
	if bed.tip() != tip {
		t.Fatal("the nameless refusal published")
	}
	writeFixtureEnrollment(t, bed.root, "Wido")

	gcliForgivingOpen(t, bed, "alias-approve-box", "direct-approve-box", "public-approve-box",
		"alias-approve-long", "direct-approve-long", "public-approve-long", "alias-set-budget", "direct-set-budget")

	// The family approve --budget box prints the goal budget norm hint and
	// routes to the same act; the public goal approve G is its successor.
	code, report := gcliForgivingFamily(bed, func(dependencies syncRequestDependencies) int {
		return runGoalApproveWithInputs([]string{"--root", bed.root, "--id", "alias-approve-box", "--budget", "box", "--by", "Wido", gcliForgivingFixture},
			bed.prove, bed.commandNow, dependencies, gcliForgivingBinding(bed))
	})
	if code != 0 || report.refusal != nil || !gcliForgivingHint(report.notes, "alias-approve-box", " norm") {
		t.Fatalf("approve --budget box did not print its goal budget norm hint: code=%d notes=%q refusal=%+v", code, report.notes, report.refusal)
	}
	gcliForgivingMust(t, bed, "goal", "budget", "direct-approve-box", "norm", "--by", "Wido", gcliForgivingFixture)
	gcliForgivingMust(t, bed, "goal", "approve", "public-approve-box", "--by", "Wido", gcliForgivingFixture)

	long := []string{"--elapsed-limit", "3h", "--attempt-limit", "5", "--reserved-job-minutes-limit", "300", "--active-job-limit", "1", "--review-round-limit", "2"}
	code, report = gcliForgivingFamily(bed, func(dependencies syncRequestDependencies) int {
		return runGoalApproveWithInputs(append([]string{"--root", bed.root, "--id", "alias-approve-long", "--by", "Wido", gcliForgivingFixture}, long...),
			bed.prove, bed.commandNow, dependencies, gcliForgivingBinding(bed))
	})
	if code != 0 || report.refusal != nil || !gcliForgivingHint(report.notes, "alias-approve-long", " 3h/5/300m/1/2") {
		t.Fatalf("approve long form did not print its compact goal budget hint: code=%d notes=%q refusal=%+v", code, report.notes, report.refusal)
	}
	gcliForgivingMust(t, bed, "goal", "budget", "direct-approve-long", "3h/5/300m/1/2", "--by", "Wido", gcliForgivingFixture)
	gcliForgivingMust(t, bed, append([]string{"goal", "approve", "public-approve-long", "--by", "Wido", gcliForgivingFixture}, long...)...)

	// The family set-budget on a claimed goal (its hint goes to the process's
	// standard error, not to an injected stream) and the public goal budget
	// G BOX land the same act.
	gcliForgivingMust(t, bed, "goal", "budget", "alias-set-budget", "norm", "--by", "Wido", gcliForgivingFixture)
	gcliForgivingClaim(t, bed, "alias-set-budget")
	code, report = gcliForgivingFamily(bed, func(dependencies syncRequestDependencies) int {
		return runGoalSetBudgetWithInputs(append([]string{"--root", bed.root, "--id", "alias-set-budget", "--by", "Wido", gcliForgivingFixture}, long...),
			bed.prove, bed.commandNow, dependencies, gcliForgivingBinding(bed))
	})
	if code != 0 || report.refusal != nil {
		t.Fatalf("the family set-budget did not land: code=%d refusal=%+v failure=%v", code, report.refusal, report.failure)
	}
	gcliForgivingMust(t, bed, "goal", "release", "alias-set-budget", "--reason", "free the claim for the direct twin")
	gcliForgivingMust(t, bed, "goal", "budget", "direct-set-budget", "norm", "--by", "Wido", gcliForgivingFixture)
	gcliForgivingClaim(t, bed, "direct-set-budget")
	gcliForgivingMust(t, bed, "goal", "budget", "direct-set-budget", "3h/5/300m/1/2", "--by", "Wido", gcliForgivingFixture)

	for _, pair := range []struct{ alias, direct, verb string }{
		{"alias-approve-box", "direct-approve-box", "approve"},
		{"public-approve-box", "direct-approve-box", "approve"},
		{"alias-approve-long", "direct-approve-long", "approve"},
		{"public-approve-long", "direct-approve-long", "approve"},
		{"alias-set-budget", "direct-set-budget", "set-budget"},
	} {
		alias, direct := bed.goalRecord(pair.alias), bed.goalRecord(pair.direct)
		if gcliForgivingBudget(alias) == "" || gcliForgivingBudget(alias) != gcliForgivingBudget(direct) ||
			!strings.Contains(alias, " "+pair.verb+" actor=human:Wido ") || !strings.Contains(direct, " "+pair.verb+" actor=human:Wido ") {
			t.Fatalf("%s did not land the same budget and history verb as %s:\n%s\n%s", pair.alias, pair.direct, alias, direct)
		}
	}
}

// forgiving-human-refusals (goal-cli-fixtures.sh 1844-2047).
func TestGoalCLIForgivingHumanRefusals(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	bed.announceHolder()
	endpoint, err := bed.endpoint(bed.root)
	if err != nil {
		t.Fatal(err)
	}

	// A stray flag: the public router refuses it with a hint and no command
	// (the family printed the same command without the flag).
	gcliForgivingOpen(t, bed, "dropped-shape")
	tip := bed.tip()
	code, _, stderr := gcliForgivingPublic(bed, "goal", "budget", "dropped-shape", "norm", "--by", "Wido", gcliForgivingFixture, "--label", "stray")
	lines := gcliForgivingLines(stderr)
	if code == 0 || len(lines) != 2 || !strings.HasPrefix(lines[0], "metasystem goal budget: does not take --label;") ||
		lines[1] != "hint: see metasystem help goal budget" || bed.tip() != tip {
		t.Fatalf("the stray flag was not refused before any act: code=%d stderr=%q", code, stderr)
	}

	// Fixture authority with a temporary human word: the remedy drops the
	// pair. It is printed in the public form (U-idem: public remedies, with
	// --id), which the family owner also accepts; the twin is the public
	// long form.
	gcliForgivingOpen(t, bed, "fixture-pair", "fixture-pair-twin")
	tip = bed.tip()
	code, _, stderr = gcliForgivingPublic(bed, "goal", "budget", "fixture-pair", "norm", "--by", "Wido", gcliForgivingFixture,
		"--temporary-human-word", "Wido authorizes this relay", "--review-by", "2026-09-06")
	lines = gcliForgivingLines(stderr)
	if code == 0 || len(lines) != 2 || lines[0] != "metasystem goal budget: goal budget fixture authority does not combine with a temporary human word or review date." ||
		!strings.HasPrefix(lines[1], "run: metasystem goal budget ") || bed.tip() != tip {
		t.Fatalf("fixture and temporary authority did not print the dropped-pair command: code=%d stderr=%q", code, stderr)
	}
	pair := shellWords(strings.TrimPrefix(lines[1], "run: "))
	if strings.Contains(lines[1], "--temporary-human-word") || strings.Contains(lines[1], "--review-by") || pair[len(pair)-1] != "norm" {
		t.Fatalf("the remedy kept the temporary pair: %q", pair)
	}
	if code, report := gcliForgivingFamily(bed, func(dependencies syncRequestDependencies) int {
		return runGoalBudgetWithInputs(pair[3:], bed.prove, bed.commandNow, dependencies, gcliForgivingBinding(bed))
	}); code != 0 || report.refusal != nil {
		t.Fatalf("the printed family remedy did not complete: code=%d refusal=%+v failure=%v", code, report.refusal, report.failure)
	}
	gcliForgivingLongForm(t, bed, "fixture-pair-twin", "1d", "10", "1200", "1", "3", "--by", "Wido")()
	gcliForgivingSameAct(t, bed, "fixture and temporary authority", "fixture-pair", "fixture-pair-twin")

	// The approval sweep is not a public act (the public goal approve takes
	// no --sweep).
	if code, _, stderr := gcliForgivingPublic(bed, "goal", "approve", "--sweep", "--budget", "box", "--by", "Wido", gcliForgivingFixture); code == 0 ||
		!strings.HasPrefix(stderr, "metasystem goal approve: does not take --sweep;") {
		t.Fatalf("the public approve took --sweep: code=%d stderr=%q", code, stderr)
	}
	gcliForgivingOpen(t, bed, "sweep-with-id", "sweep-with-id-twin")
	code, report := gcliForgivingFamily(bed, func(dependencies syncRequestDependencies) int {
		return runGoalApproveWithInputs([]string{"--root", bed.root, "--sweep", "--id", "sweep-with-id", "--confirm", "stale", "--by", "Wido", gcliForgivingFixture},
			bed.prove, bed.commandNow, dependencies, gcliForgivingBinding(bed))
	})
	// U-idem: a direct approval is the public goal approve.
	if code == 0 || report.refusal == nil || !strings.HasPrefix(report.refusal.remedy.command, "metasystem goal approve ") ||
		strings.Contains(report.refusal.remedy.command, "--sweep") || strings.Contains(report.refusal.remedy.command, "--confirm") {
		t.Fatalf("approval sweep with a named goal did not print the direct approval: code=%d refusal=%+v", code, report.refusal)
	}
	direct := shellWords(report.refusal.remedy.command)
	if code, report := gcliForgivingFamily(bed, func(dependencies syncRequestDependencies) int {
		return runGoalApproveWithInputs(direct[3:], bed.prove, bed.commandNow, dependencies, gcliForgivingBinding(bed))
	}); code != 0 || report.refusal != nil {
		t.Fatalf("the direct approval remedy did not complete: code=%d refusal=%+v failure=%v", code, report.refusal, report.failure)
	}
	gcliForgivingLongForm(t, bed, "sweep-with-id-twin", "1d", "10", "1200", "1", "3", "--by", "Wido")()
	gcliForgivingSameAct(t, bed, "approval sweep with a named goal", "sweep-with-id", "sweep-with-id-twin")

	// A parked goal's incomplete box is completed from its standing box.
	gcliForgivingOpen(t, bed, "parked-completion", "parked-completion-twin")
	for _, id := range []string{"parked-completion", "parked-completion-twin"} {
		gcliForgivingMust(t, bed, "goal", "budget", id, "3h/5/300m/1/2", "--by", "Wido", gcliForgivingFixture)
		gcliForgivingMust(t, bed, "goal", "pause", id, "--reason", "hold the completed-box fixture", "--by", "Wido", gcliForgivingFixture)
	}
	gcliForgivingRefusalRemedy(t, bed, "parked compact completion", "parked-completion", "parked-completion-twin",
		gcliForgivingLongForm(t, bed, "parked-completion-twin", "4h", "6", "360", "1", "2", "--by", "Wido"),
		"goal", "budget", "parked-completion", "4h/6/360m/1", "--by", "Wido", gcliForgivingFixture)

	// Resume of an unfenced claim with a typed box (the family form; the public
	// goal resume takes no box and answers that there is nothing to resume).
	code, report = gcliForgivingFamily(bed, func(dependencies syncRequestDependencies) int {
		return runGoalResumeWithInputs([]string{"--root", bed.root, "--id", "ship-widget", "--by", "Wido", gcliForgivingFixture,
			"--elapsed-limit", "8h", "--attempt-limit", "10", "--reserved-job-minutes-limit", "1200", "--active-job-limit", "1", "--review-round-limit", "3"},
			bed.prove, bed.commandNow, dependencies, gcliForgivingBinding(bed))
	})
	if code == 0 || report.refusal == nil || !strings.HasPrefix(report.refusal.remedy.command, "metasystem goal budget ship-widget ") ||
		!strings.HasSuffix(report.refusal.remedy.command, " 1d/10/1200m/1/3") {
		t.Fatalf("resume without a fence did not print the tierless goal's tier-three box: code=%d refusal=%+v", code, report.refusal)
	}
	gcliForgivingRunRemedy(t, bed, "resume without a fence", shellWords(report.refusal.remedy.command))
	if record := bed.goalRecord("ship-widget"); !strings.Contains(record, " set-budget actor=human:Wido ") ||
		gcliForgivingBudget(record) != "elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3" {
		t.Fatalf("resume without a fence printed a command that did not land the norm box:\n%s", record)
	}
	if code, stdout, stderr := gcliForgivingPublic(bed, "goal", "resume", "ship-widget", "--by", "Wido", gcliForgivingFixture); code != 0 ||
		stdout != "ship-widget is running under its standing box; there is nothing to resume\n" {
		t.Fatalf("the public resume of an unfenced claim: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	// A claimed goal's incomplete box is completed from its standing box.
	gcliForgivingReleaseIfClaimed(t, bed, "ship-widget")
	gcliForgivingOpen(t, bed, "claimed-completion", "claimed-completion-twin")
	for _, id := range []string{"claimed-completion", "claimed-completion-twin"} {
		gcliForgivingMust(t, bed, "goal", "budget", id, "3h/5/300m/1/2", "--by", "Wido", gcliForgivingFixture)
	}
	gcliForgivingClaim(t, bed, "claimed-completion")
	gcliForgivingRefusalRemedy(t, bed, "claimed compact completion", "claimed-completion", "claimed-completion-twin", func() {
		gcliForgivingReleaseIfClaimed(t, bed, "claimed-completion")
		gcliForgivingClaim(t, bed, "claimed-completion-twin")
		gcliForgivingLongForm(t, bed, "claimed-completion-twin", "4h", "6", "360", "1", "2", "--by", "Wido")()
	}, "goal", "budget", "claimed-completion", "4h/6/360m/1", "--by", "Wido", gcliForgivingFixture)

	// R-129 (U-idem): the completed box repeated is a repeat whose effect
	// holds: success, nothing recorded.
	tip = bed.tip()
	for _, id := range []string{"parked-completion", "claimed-completion-twin"} {
		if code, _, stderr := gcliForgivingPublic(bed, "goal", "budget", id, "4h/6/360m/1", "--by", "Wido", gcliForgivingFixture); code != 0 || stderr != "" {
			t.Fatalf("%s: the completed box repeated was not unchanged: code=%d stderr=%q", id, code, stderr)
		}
	}
	if bed.tip() != tip {
		t.Fatal("a completed box published")
	}

	// The same completed box is an act when the claim is breach-stopped: the
	// compact refusal prints keep, and keep matches a resume of the twin.
	bed.setNow(gcliForgivingBreachAt)
	gcliForgivingBreachStop(t, bed, "claimed-completion-twin", "01ARZ3NDEKTSV4RRFFQ69G7S02")
	remedy := gcliForgivingRefusalRemedy(t, bed, "breach-stopped claimed compact completion", "claimed-completion-twin", "claimed-completion", func() {
		gcliForgivingReleaseIfClaimed(t, bed, "claimed-completion-twin")
		gcliForgivingClaim(t, bed, "claimed-completion")
		bed.setNow(gcliForgivingSecondBreach)
		gcliForgivingBreachStop(t, bed, "claimed-completion", "01ARZ3NDEKTSV4RRFFQ69G7S03")
		// The family resume took the explicit box; the public resume resumes
		// under the standing box, which is that box.
		gcliForgivingMust(t, bed, "goal", "resume", "claimed-completion", "--by", "Wido", gcliForgivingFixture)
	}, "goal", "budget", "claimed-completion-twin", "4h/6/360m/1", "--by", "Wido", gcliForgivingFixture)
	if remedy[len(remedy)-1] != "keep" {
		t.Fatalf("the breach-stopped completion did not print keep: %q", remedy)
	}
	if record := bed.goalRecord("claimed-completion"); !strings.Contains(record, " resume actor=human:Wido ") ||
		gcliForgivingBudget(record) != "elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=2" {
		t.Fatalf("the twin did not resume under the completed box:\n%s", record)
	}
	gcliForgivingReleaseIfClaimed(t, bed, "claimed-completion")
	gcliForgivingReleaseIfClaimed(t, bed, "claimed-completion-twin")
	gcliForgivingClaim(t, bed, "ship-widget")

	tip = bed.tip()
	code, _, stderr = gcliForgivingPublic(bed, "goal", "unapprove", "fix-docs", "--by", "Wido", gcliForgivingFixture)
	gcliForgivingRun(t, "unapprove's unseen reason", "goal unapprove", code, stderr, "withdrawing an approval needs a reason",
		"metasystem goal unapprove fix-docs --by Wido")
	if !strings.Contains(stderr, "--reason TEXT") {
		t.Fatalf("the retry names no --reason: %q", stderr)
	}

	// accept-risk with fixture authority and a temporary word.
	jobs := filepath.Join(bed.root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobs, "fixture-risk.json"), []byte(gcliForgivingRiskJob+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The public accept-risk refuses the pair with its reason alone: repeating
	// the act would not resolve it, and enrolling a terminal would not either.
	code, _, stderr = gcliForgivingPublic(bed, "goal", "accept-risk", "ship-widget", "--finding", "RISK-1", "--review", "fixture-risk", "--reason", "fixture pair",
		"--by", "Wido", gcliForgivingFixture, "--temporary-human-word", "Wido authorizes this relay")
	if want := "metasystem goal accept-risk: goal accept-risk fixture authority does not combine with a temporary human word or review date, so nothing was done\n"; code == 0 || stderr != want {
		t.Fatalf("the public accept-risk pair = %d %q, want %q", code, stderr, want)
	}
	if bed.tip() != tip {
		t.Fatal("a refused accept-risk published")
	}
	// The family accept-risk prints the public command (U-idem) without the word;
	// running it accepts the real fixture finding.
	code, report = gcliForgivingFamily(bed, func(dependencies syncRequestDependencies) int {
		return runGoalAcceptRiskWithFacts([]string{"--root", bed.root, "--id", "ship-widget", "--finding", "RISK-1", "--chain", "fixture-risk", "--why", "fixture pair",
			"--by", "Wido", gcliForgivingFixture, "--temporary-human-word", "Wido authorizes this relay"}, bed.prove, bed.commandNow, dependencies, nil)
	})
	if code == 0 || report.refusal == nil || !strings.HasPrefix(report.refusal.remedy.command, "metasystem goal accept-risk ") ||
		strings.Contains(report.refusal.remedy.command, "--temporary-human-word") {
		t.Fatalf("accept-risk's temporary pair did not produce a command refusal: code=%d refusal=%+v", code, report.refusal)
	}
	accept := shellWords(report.refusal.remedy.command)
	if code, report := gcliForgivingFamily(bed, func(dependencies syncRequestDependencies) int {
		return runGoalAcceptRiskWithFacts(accept[3:], bed.prove, bed.commandNow, dependencies, nil)
	}); code != 0 || report.refusal != nil {
		t.Fatalf("accept-risk's printed remedy did not complete: code=%d refusal=%+v failure=%v", code, report.refusal, report.failure)
	}
	if line := goalCLILine(bed.goalRecord("ship-widget"), "- AcceptedRisk: "); !strings.HasPrefix(line, "- AcceptedRisk: finding=RISK-1 chain=fixture-risk by=Wido opid=") {
		t.Fatalf("accept-risk's printed pair remedy did not accept the real fixture finding: %q\n%s", line, bed.goalRecord("ship-widget"))
	}

	// set-obligation (public goal edit --obligation) without its values.
	tip = bed.tip()
	code, _, stderr = gcliForgivingPublic(bed, "goal", "edit", "ship-widget", "--obligation", "DRAFT", gcliForgivingFixture)
	gcliForgivingWords(t, "set-obligation's absent values", "goal edit", code, stderr, "requires identity, recurrence", "supply every missing named flag")

	// enroll-terminal (public system enroll) without a name: the public form
	// prints its own usage as the command, where the family printed words.
	code, _, stderr = gcliForgivingPublic(bed, "system", "enroll")
	lines = gcliForgivingLines(stderr)
	// The name is filled in: the helm holder's, else this account's.
	if code == 0 || len(lines) != 2 || lines[0] != "metasystem system enroll: enroll needs your name; nothing was done" ||
		!strings.HasPrefix(lines[1], "run: metasystem system enroll --name ") || strings.Contains(lines[1], "--name NAME") {
		t.Fatalf("enroll without a name: code=%d stderr=%q", code, stderr)
	}
	if bed.tip() != tip {
		t.Fatal("a refused edit or enroll published")
	}

	// classify-sweep (installation maintenance: family only).
	draft := filepath.Join(bed.root, "forgiving-classification.txt")
	draftText := "ship-widget 3,1,1,1 claimed migration\nfix-docs 1,1,1,1 queued migration\nperf-pass 2,1,1,1 parked migration\n"
	if err := os.WriteFile(draft, []byte(draftText), 0o644); err != nil {
		t.Fatal(err)
	}
	// runGoalClassifySweepWithInputs prints its refusals and its preview
	// listing on the process's streams, not through dependencies.report or
	// the injected writers, so only its exit status and its effect on the
	// ledger are observable here (see the seam in the return). The remedy it
	// prints is the preview form below.
	classify := func(args ...string) int {
		code, _ := gcliForgivingFamily(bed, func(dependencies syncRequestDependencies) int {
			return runGoalClassifySweepWithInputs(args, bed.prove, bed.commandNow, dependencies)
		})
		return code
	}
	previewForm := []string{"--root", bed.root, "--draft", draft, "--preview"}
	if code := classify("--root", bed.root, "--draft", draft); code != 2 {
		t.Fatalf("classify-sweep without a mode did not refuse: code=%d", code)
	}
	if code := classify(previewForm...); code != 0 {
		t.Fatalf("classify-sweep's missing-mode preview did not run: code=%d", code)
	}
	if code := classify("--root", bed.root, "--draft", filepath.Join(bed.root, "missing-classification.txt"), "--preview"); code != 1 {
		t.Fatalf("classify-sweep's unreadable draft did not refuse: code=%d", code)
	}
	listing, err := goal.PreviewClassificationSweep(endpoint, []byte(draftText), bed.clock())
	if err != nil || listing.Digest == "" {
		t.Fatalf("the classification preview has no digest: %+v %v", listing, err)
	}
	changed := strings.ReplaceAll(draftText, "claimed migration", "changed migration")
	if err := os.WriteFile(draft, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	changedListing, err := goal.PreviewClassificationSweep(endpoint, []byte(changed), bed.clock())
	if err != nil || changedListing.Digest == listing.Digest {
		t.Fatalf("the changed draft did not change the listing: %+v %v", changedListing, err)
	}
	if code := classify("--root", bed.root, "--draft", draft, "--confirm", listing.Digest, "--by", "Wido", gcliForgivingFixture); code != 1 {
		t.Fatalf("classify-sweep's changed listing did not refuse: code=%d", code)
	}
	if code := classify(previewForm...); code != 0 {
		t.Fatalf("classify-sweep's changed-listing preview did not run: code=%d", code)
	}
	if bed.tip() != tip {
		t.Fatal("a refused classification published")
	}
}

const gcliForgivingRiskJob = `{"jobId":"fixture-risk","role":"code-critic","round":1,"parentJob":null,"status":"completed","goalId":"ship-widget","findingRegisterRound":1,"reviewRoundLimit":3,"criticRoundsConsumed":3,"demotions":[],"findingRegister":[{"findingId":"RISK-1","critic":"fixture-critic","rigorClass":"severe","factsDigest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","facts":{"local":true},"artifact":"metasystem/fixture.go","title":"fixture severe finding","status":"open","resolution":"","decisionOpid":"","evidence":"direct fixture evidence","evidenceDigest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","multiplicity":1}]}`
