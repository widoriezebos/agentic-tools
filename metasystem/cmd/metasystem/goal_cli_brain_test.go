package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// The Brain group ports the brain scenarios of goal-cli-fixtures.sh: the
// declared brain's fence over every verb that carries a human's word, over
// claims, over an unclassifiable caller, and the brain's stop and status
// surfaces. The beds are the intent bed's fake repository and clock; the
// brain declaration is the real record at brain.Path.

const gcliBrainLedger = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

// gcliBrainBed is one intent bed whose caller classifies deterministically:
// the caller the goal owners classify (this process's parent) is announced
// as a seat main in the bed's own root, so it is never the person at a
// terminal wherever the suite runs.
type gcliBrainBed struct {
	*intentBed
}

func gcliBrainNewBed(t *testing.T, announce bool) *gcliBrainBed {
	t.Helper()
	bed := newIntentBed(t, false, nil)
	for _, id := range []string{"fix-docs", "brain-claim-target", "classification-target", "brain-fixture-authority-target"} {
		// fix-docs is tierless like the shell seed's, so the classification
		// sweep has a goal to confirm; the others carry a tier-one risk.
		file := queuedIntentGoal(id, 0)
		if id != "fix-docs" {
			file = queuedIntentGoal(id, 1)
			file.Origin = goal.OriginHuman
		}
		bed.addGoal(file)
	}
	if announce {
		gcliBrainAnnounceCaller(t, bed.root())
	}
	// The shell bed exported METASYSTEM_OWNER_LINEAGE=fixture-lineage.
	bed.lineage = "fixture-lineage"
	return &gcliBrainBed{intentBed: bed}
}

func gcliBrainAnnounceCaller(t *testing.T, root string) {
	t.Helper()
	parent, state, err := (identity.KernelProber{}).Probe(int64(os.Getppid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe the classified caller: state=%s err=%v", state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "brain-bed-main", parent.Pid, parent.StartedAt.Unix(),
		parent.StartTicks, parent.BootID, "brain-bed-main", "fake", "m1"); err != nil {
		t.Fatal(err)
	}
}

func gcliBrainDeclare(t *testing.T, root string) {
	t.Helper()
	record := brain.Record{Schema: brain.Schema, Ledger: gcliBrainLedger, Machine: "mac-cli", DeclaredBy: "Wido", DeclaredAt: "2026-09-01T09:00:00Z"}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	gcliBrainWrite(t, brain.Path(root), append(data, '\n'))
}

func gcliBrainWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// gcliBrainNoProof fails a row whose fence did not refuse before the owner
// asked for the human's proof.
func gcliBrainNoProof(t *testing.T) goalAuthorityProver {
	return func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		t.Error("the brain fence let the owner reach the human proof")
		return humanauthority.Proof{}, errors.New("no proof in the brain bed")
	}
}

// ownerRun runs one owner with its report captured, returning the
// exit code and every word the owner said.
func (b *gcliBrainBed) ownerRun(run func(syncRequestDependencies) int) (int, string) {
	b.t.Helper()
	dependencies := b.dependencies()
	dependencies.ownerLineage = func() string { return b.lineage }
	report := &ownerReport{}
	var stdout, stderr strings.Builder
	dependencies.report, dependencies.stdout, dependencies.stderr = report, &stdout, &stderr
	code := run(dependencies)
	said := stdout.String() + stderr.String() + report.written.String()
	if report.failure != nil {
		said += report.failure.Error()
	}
	if report.refusal != nil {
		said += report.refusal.sentence
	}
	return code, said
}

type gcliBrainRow struct {
	name string
	run  func(b *gcliBrainBed, dependencies syncRequestDependencies) int
}

// gcliBrainSilentRows marks an owner whose refusal goes to the process's own
// stderr (classify-sweep never routes its refusal through the owner report).
// Its row proves the refusal by exit code, the proof it never reached and the
// unchanged tip, and the fence's own words by the same classification the
// owner calls at that point.
var gcliBrainSilentRows = map[string]func(b *gcliBrainBed) error{
	"classify-sweep --confirm": func(b *gcliBrainBed) error {
		_, err := classifyGoalAuthorityFirstWithFacts("classify-sweep", &syncFlags{root: b.root(), by: "Wido"}, b.dependencies().authorityFacts)
		return err
	},
}

func gcliBrainSyncOnly(name string, owner func(goal.VerbRequest, *syncFlags) (goal.PublishResult, error), required ...string) func(*gcliBrainBed, syncRequestDependencies, []string) int {
	return func(b *gcliBrainBed, dependencies syncRequestDependencies, args []string) int {
		builder := func(verb, root, by, lineage string) (goal.VerbRequest, error) {
			return syncReqWithProofAtWithDependencies(verb, root, by, lineage, nil, b.commandNow, dependencies)
		}
		return runSyncOnlyWithDependencies(name, owner, builder, dependencies, required...)(args)
	}
}

// gcliBrainHumanWordRows is the shell's assert_human_word_matrix: every row
// carries the human's word as --by from a caller that is not the person.
func gcliBrainHumanWordRows(t *testing.T, draft, digest string) []gcliBrainRow {
	box := []string{"--elapsed-limit", "1d", "--attempt-limit", "2", "--reserved-job-minutes-limit", "120", "--active-job-limit", "1", "--review-round-limit", "3"}
	with := func(root string, args ...string) []string { return append([]string{"--root", root}, args...) }
	return []gcliBrainRow{
		{"approve", func(b *gcliBrainBed, d syncRequestDependencies) int {
			return runGoalApproveWithInputs(with(b.root(), "--id", "fix-docs", "--by", "Wido", "--budget", "box"), gcliBrainNoProof(t), b.commandNow, d, b.binding)
		}},
		{"resume", func(b *gcliBrainBed, d syncRequestDependencies) int {
			return runGoalResumeWithInputs(append(with(b.root(), "--id", "fix-docs", "--by", "Wido"), box...), gcliBrainNoProof(t), b.commandNow, d, b.binding)
		}},
		{"resume --approved-ref", func(b *gcliBrainBed, d syncRequestDependencies) int {
			args := append(append(with(b.root(), "--id", "fix-docs", "--by", "Wido"), box...), "--approved-ref", "fixture-answer")
			return runGoalResumeWithInputs(args, gcliBrainNoProof(t), b.commandNow, d, b.binding)
		}},
		{"set-obligation", func(b *gcliBrainBed, d syncRequestDependencies) int {
			args := with(b.root(), "--id", "fix-docs", "--by", "Wido",
				"--state", "LIMITED", "--owner", "Wido", "--recurrence", "single-experiment", "--platform", "darwin-arm64",
				"--toolchain-identity", "go-fixture", "--surface-digest", "fixture-surface", "--max-active-jobs", "1", "--timing-envelope-sec", "60",
				"--effect", "local-write", "--value-judgment", "no", "--reversibility", "reversible", "--severe-harm", "no",
				"--unfamiliar-approach", "no", "--test-discrimination", "strong", "--correlated-assumption-risk", "no",
				"--authority-scope-change", "no", "--destructive-reach", "reversible-local")
			return runGoalSetObligationWithAuthorityFactsAtWithDependencies(args, gcliBrainNoProof(t), b.commandNow, d)
		}},
		{"steal (goal claim --take-over)", func(b *gcliBrainBed, d syncRequestDependencies) int {
			return gcliBrainSyncOnly("steal", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
				return goal.Steal(req, f.id)
			}, "id")(b, d, with(b.root(), "--id", "fix-docs", "--by", "Wido"))
		}},
		{"set-pin (goal pin)", func(b *gcliBrainBed, d syncRequestDependencies) int {
			return gcliBrainSyncOnly("set-pin", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
				return goal.SetPin(req, f.id, f.pin)
			}, "id", "pin")(b, d, with(b.root(), "--id", "fix-docs", "--pin", "node", "--by", "Wido"))
		}},
		{"classify-sweep --confirm", func(b *gcliBrainBed, d syncRequestDependencies) int {
			return runGoalClassifySweepWithInputs(with(b.root(), "--draft", draft, "--confirm", digest, "--by", "Wido"), gcliBrainNoProof(t), b.commandNow, d)
		}},
		{"set-budget (goal budget long form)", func(b *gcliBrainBed, d syncRequestDependencies) int {
			return runGoalSetBudgetWithInputs(append(with(b.root(), "--id", "fix-docs", "--by", "Wido"), box...), gcliBrainNoProof(t), b.commandNow, d, b.binding)
		}},
		{"unapprove", func(b *gcliBrainBed, d syncRequestDependencies) int {
			return runGoalUnapproveWithInputs(with(b.root(), "--id", "fix-docs", "--by", "Wido", "--because", "fixture reversal"), gcliBrainNoProof(t), b.commandNow, d)
		}},
		{"accept-risk", func(b *gcliBrainBed, d syncRequestDependencies) int {
			return runGoalAcceptRiskWithFacts(with(b.root(), "--id", "fix-docs", "--finding", "F1", "--chain", "brain-fixture-chain", "--by", "Wido", "--why", "fixture risk decision"),
				gcliBrainNoProof(t), b.commandNow, d, nil)
		}},
		{"discharge-review-obligation (work review --finding)", func(b *gcliBrainBed, d syncRequestDependencies) int {
			builder := func(verb, root, by, lineage string) (goal.VerbRequest, error) {
				return syncReqWithProofAtWithDependencies(verb, root, by, lineage, nil, b.commandNow, d)
			}
			discharge := func(goal.VerbRequest, string, string, string, string, string, ...goal.DischargeEvidence) (goal.PublishResult, error) {
				t.Error("the brain fence let discharge-review-obligation reach its owner")
				return goal.PublishResult{}, nil
			}
			return runGoalDischargeReviewObligationWithDependencies(with(b.root(), "--id", "fix-docs", "--finding", "F1", "--chain", "brain-fixture-chain", "--by", "Wido", "--test", "fixture test"),
				builder, discharge, d)
		}},
		{"repair --accept-remote (goal sync --accept-remote-history)", func(b *gcliBrainBed, d syncRequestDependencies) int {
			endpoint := func(string) (goal.Endpoint, error) {
				t.Error("the brain fence let repair resolve its endpoint")
				return goal.Endpoint{}, errors.New("no endpoint in the brain bed")
			}
			return goalRepairAcceptRemoteTo(d.stdout, d.stderr, b.root(), "Wido", d.authorityFacts, endpoint)
		}},
	}
}

func (b *gcliBrainBed) classificationDraft() (string, string) {
	b.t.Helper()
	draft := filepath.Join(b.t.TempDir(), "brain-human-word-classification.txt")
	if err := os.WriteFile(draft, []byte("fix-docs 1,1,1,1 queued fixture\n"), 0o644); err != nil {
		b.t.Fatal(err)
	}
	endpoint, err := b.dependencies().endpoint(b.root())
	if err != nil {
		b.t.Fatal(err)
	}
	data, _ := os.ReadFile(draft)
	now, _ := b.commandNow(b.root())
	listing, err := goal.PreviewClassificationSweep(endpoint, data, now)
	if err != nil {
		b.t.Fatalf("the brain bed could not prepare its classification confirmation: %v", err)
	}
	return draft, listing.Digest
}

func (b *gcliBrainBed) assertMatrix(rows []gcliBrainRow, expected string) {
	b.t.Helper()
	for _, row := range rows {
		before := b.publications()
		code, said := b.ownerRun(func(d syncRequestDependencies) int { return row.run(b, d) })
		if silent := gcliBrainSilentRows[row.name]; silent != nil && code == 1 && said == "" {
			if err := silent(b); err != nil {
				said = err.Error()
			}
		}
		if code == 0 || !strings.Contains(said, expected) {
			b.t.Errorf("%s: brain goal fence wanted %q, got rc=%d: %s", row.name, expected, code, said)
		}
		if b.publications() != before {
			b.t.Errorf("%s: the refusal advanced the ledger tip", row.name)
		}
	}
}

// TestGoalCLIBrainHumanWordRefuses ports brain-human-word-refuses: the
// declared brain refuses every human-word verb, a corrupt declaration refuses
// them all with its remedy, the fixture proof crosses the seam, and nothing
// the refusals did moved the ledger.
func TestGoalCLIBrainHumanWordRefuses(t *testing.T) {
	t.Parallel()
	bed := gcliBrainNewBed(t, true)
	draft, digest := bed.classificationDraft()
	rows := gcliBrainHumanWordRows(t, draft, digest)
	gcliBrainDeclare(t, bed.root())
	valid, err := os.ReadFile(brain.Path(bed.root()))
	if err != nil {
		t.Fatal(err)
	}

	bed.assertMatrix(rows, "this checkout is declared the brain; the brain never carries a human's word into goal")
	if bed.publications() != 0 {
		t.Fatal("brain human-word refusal advanced the ledger tip")
	}

	// A root that is not fake refuses the fixture flag, and the brain still
	// refuses the word before that refusal.
	conf := filepath.Join(bed.root(), "metasystem.conf")
	fake, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	gcliBrainWrite(t, conf, []byte("metasystem.runtimes=codex\n"))
	code, said := bed.ownerRun(func(d syncRequestDependencies) int {
		return runGoalApproveWithInputs([]string{"--root", bed.root(), "--id", "fix-docs", "--by", "Wido", "--budget", "box", "--fixture-human-authority"},
			gcliBrainNoProof(t), bed.commandNow, d, bed.binding)
	})
	if code == 0 || !strings.Contains(said, "this checkout is declared the brain; the brain never carries a human's word into goal approve") {
		t.Fatalf("runtimes=codex approve: rc=%d %s", code, said)
	}
	gcliBrainWrite(t, conf, fake)

	gcliBrainWrite(t, brain.Path(bed.root()), []byte("{broken\n"))
	bed.assertMatrix(rows, "this checkout's brain declaration is unreadable")
	if bed.publications() != 0 {
		t.Fatal("corrupt brain human-word refusal advanced the ledger tip")
	}
	gcliBrainWrite(t, brain.Path(bed.root()), valid)

	code, said = bed.ownerRun(func(d syncRequestDependencies) int {
		return runGoalApproveWithInputs([]string{"--root", bed.root(), "--id", "brain-fixture-authority-target", "--by", "Wido", "--budget", "box", "--fixture-human-authority"},
			fixedFixtureGoalAuthority, bed.commandNow, d, bed.binding)
	})
	if code != 0 {
		t.Fatalf("fixture-only human authority did not cross the brain seam: rc=%d %s", code, said)
	}
	if file := bed.goalFile("brain-fixture-authority-target"); file.Approved == nil {
		t.Fatalf("fixture approval through the brain left the goal unapproved: %+v", file)
	}
}

// approve approves a goal as the shell's approve_fixture_goal did:
// the fixture human's word under a box.
func (b *gcliBrainBed) approve(id string) (int, string) {
	b.t.Helper()
	return b.ownerRun(func(d syncRequestDependencies) int {
		return runGoalApproveWithInputs([]string{"--root", b.root(), "--id", id, "--by", "Wido", "--budget", "box", "--fixture-human-authority"},
			fixedFixtureGoalAuthority, b.commandNow, d, b.binding)
	})
}

// TestGoalCLIBrainClaimRefuses ports brain-claim-refuses: a declared brain
// refuses goal claim and open --claim with the node's command, a corrupt
// declaration refuses both with its remedy, and no refusal moves the ledger.
func TestGoalCLIBrainClaimRefuses(t *testing.T) {
	t.Parallel()
	bed := gcliBrainNewBed(t, true)
	if code, said := bed.approve("brain-claim-target"); code != 0 {
		t.Fatalf("fixture approval for brain-claim-target failed: %s", said)
	}
	declared := bed.publications()
	gcliBrainDeclare(t, bed.root())

	claim := func() (int, string) {
		code, result := bed.runJSON(bed.owners(), "goal", "claim", "brain-claim-target")
		return code, result.Summary + " " + result.Decision
	}
	openClaim := func(id string) (int, string) {
		return bed.ownerRun(func(d syncRequestDependencies) int {
			code, handled := trySyncMutationWithCompletion("open", []string{"--root", bed.root(), "--id", id,
				"--intent", "A node must open and claim this goal.", "--next", "Open it on a node.",
				"--risk", "severity=1,novelty=1,exposure=1,accumulation=1", "--basis", "fixture open claim fence",
				"--claim", "--elapsed-limit", "1d", "--attempt-limit", "2", "--reserved-job-minutes-limit", "120", "--active-job-limit", "1", "--review-round-limit", "0"},
				bed.commandNow, d, nil, completionInputs{})
			if !handled {
				t.Error("goal open --claim was not handled by the synced-backlog owner")
			}
			return code
		})
	}
	claimCommand := "this checkout is declared the brain; the brain never claims. A node claims: metasystem goal claim --root <checkout> --id <id>"
	if code, said := claim(); code == 0 || !strings.Contains(said, claimCommand) {
		t.Errorf("goal claim: brain goal fence wanted %q, got rc=%d: %s", claimCommand, code, said)
	}
	// open --claim reads the declaration against goal.ExistingLedgerIdentity
	// (goalsync_mutations.go, the brain.Fence before goal.OpenClaim), which
	// reads the accepted ref with Git rather than through the injected
	// endpoint; in this Git-free bed that identity is empty, so the fence
	// refuses as unreadable instead of naming the node's claim. The row
	// proves the refusal and the untouched ledger; the declared wording
	// needs that seam (see the return).
	if code, said := openClaim("brain-open-claim-target"); code == 0 || !strings.Contains(said, "brain") {
		t.Errorf("goal open --claim: the declared brain did not refuse: rc=%d: %s", code, said)
	}
	if bed.publications() != declared {
		t.Fatal("brain claim refusal advanced the ledger tip")
	}

	gcliBrainWrite(t, brain.Path(bed.root()), []byte("{broken\n"))
	for _, step := range []struct {
		name string
		run  func() (int, string)
	}{{"goal claim", claim}, {"goal open --claim", func() (int, string) { return openClaim("brain-open-claim-corrupt") }}} {
		if code, said := step.run(); code == 0 || !strings.Contains(said, "this checkout's brain declaration is unreadable") {
			t.Errorf("corrupt %s: rc=%d: %s", step.name, code, said)
		}
	}
	if bed.publications() != declared {
		t.Fatal("corrupt brain claim refusal advanced the ledger tip")
	}
}

// TestGoalCLIBrainClassificationFails ports brain-classification-fails: a
// declared brain whose caller cannot be classified refuses approve and steal
// fail-closed; once undeclared the same approval confirms and steal does not
// inherit the classification refusal.
func TestGoalCLIBrainClassificationFails(t *testing.T) {
	t.Parallel()
	bed := gcliBrainNewBed(t, false)
	gcliBrainDeclare(t, bed.root())
	supervision := filepath.Join(bed.root(), "artifacts", "agents", "supervision", "state.json")
	gcliBrainWrite(t, supervision, []byte("{broken\n"))
	steal := func() (int, string) {
		return bed.ownerRun(func(d syncRequestDependencies) int {
			return gcliBrainSyncOnly("steal", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
				return goal.Steal(req, f.id)
			}, "id")(bed, d, []string{"--root", bed.root(), "--id", "classification-target", "--by", "Wido"})
		})
	}
	classifier := "this checkout is declared the brain and the caller could not be classified"
	if code, said := bed.approve("classification-target"); code == 0 || !strings.Contains(said, classifier) {
		t.Errorf("approve: brain goal fence wanted %q, got rc=%d: %s", classifier, code, said)
	}
	if code, said := steal(); code == 0 || !strings.Contains(said, classifier) {
		t.Errorf("steal: brain goal fence wanted %q, got rc=%d: %s", classifier, code, said)
	}
	if bed.publications() != 0 {
		t.Fatal("the classification refusal advanced the ledger tip")
	}

	if err := os.Remove(supervision); err != nil {
		t.Fatal(err)
	}
	if _, err := brain.Withdraw(bed.root(), t.TempDir(), gcliBrainLedger); err != nil {
		t.Fatal(err)
	}
	gcliBrainAnnounceCaller(t, bed.root())
	if code, said := bed.approve("classification-target"); code != 0 || bed.goalFile("classification-target").Approved == nil {
		t.Fatalf("undeclared fixture approval no longer behaved as before: rc=%d %s", code, said)
	}
	if _, said := steal(); strings.Contains(said, "caller could not be classified") {
		t.Fatalf("undeclared steal inherited the brain classification refusal: %s", said)
	}
}

// gcliBrainStopBed is prepare_brain_stop_bed: a draft opened here, two
// approved goals no node holds, one open question, one open plan and one
// plan waiting on the human, in a checkout declared the brain.
func gcliBrainStopBed(t *testing.T, declare bool) *gcliBrainBed {
	t.Helper()
	bed := &gcliBrainBed{intentBed: newIntentBed(t, false, nil)}
	accepted := bed.repo.commit(bed.repo.accepted).files
	delete(accepted, "plans/goals/standing-validation.md")
	if err := os.Remove(filepath.Join(bed.root(), "plans", "goals", "standing-validation.md")); err != nil {
		t.Fatal(err)
	}
	draft := queuedIntentGoal("brain-draft", 1)
	draft.NextStep = "Wido approves this draft."
	bed.addGoal(draft)
	for _, id := range []string{"brain-approved-one", "brain-approved-two"} {
		file := queuedIntentGoal(id, 1)
		file.Origin = goal.OriginHuman
		file.History[0].Actor = "human:Wido"
		bed.addGoal(file)
		if code, said := bed.approve(id); code != 0 {
			t.Fatalf("fixture approval for %s failed: %s", id, said)
		}
	}
	gcliBrainWrite(t, filepath.Join(bed.root(), "artifacts", "agents", "channel", "questions", "brain-ask.json"),
		[]byte(`{"id":"brain-ask","goal":"brain-draft","kind":"other","machine":"mac-cli","openedAt":"2026-09-07T00:00:00Z","wants":"Wido chooses","state":"open"}`+"\n"))
	gcliBrainWrite(t, filepath.Join(bed.root(), "plans", "brain-open-plan.md"), []byte("# Brain open plan\n- Next step: Finish the local note.\n"))
	gcliBrainWrite(t, filepath.Join(bed.root(), "plans", "brain-waiting-plan.md"),
		[]byte("# Brain waiting plan\n- Waiting on the human: Choose a direction.\n- Next step: Continue after the answer.\n"))
	if declare {
		gcliBrainDeclare(t, bed.root())
	}
	return bed
}

// verdict runs report turn-verdict, the one turn-end decision, through its
// real command with the bed's endpoint and machine.
func (b *gcliBrainBed) verdict(session string, stopHookActive bool) goal.Verdict {
	b.t.Helper()
	args := []string{"--root", b.root(), "--session", session}
	if stopHookActive {
		args = append(args, "--stop-hook-active=true")
	}
	var stdout, stderr strings.Builder
	machine := func(root string) (string, error) {
		if root != b.root() {
			b.t.Fatalf("turn-verdict machine root %q", root)
		}
		return "mac-cli", nil
	}
	if code := runReportTurnVerdictTo(args, &stdout, &stderr, b.dependencies().endpoint, machine); code != 0 {
		b.t.Fatalf("report turn-verdict exited %d: %s", code, stderr.String())
	}
	var verdict goal.Verdict
	if err := json.Unmarshal([]byte(stdout.String()), &verdict); err != nil {
		b.t.Fatalf("report turn-verdict printed no verdict: %v: %q", err, stdout.String())
	}
	return verdict
}

func gcliBrainLines(display string, count int) []string {
	lines := strings.SplitN(display, "\n", count+1)
	for len(lines) < count+1 {
		lines = append(lines, "")
	}
	return lines[:count]
}

// TestGoalCLIBrainStopSeeded ports brain-stop-seeded: three stops of a
// declared brain lead with the brain summary and the open work, never enter
// the idle ladder, block once for open work, ask for the status line until it
// is posted, and stage no steward continuation.
func TestGoalCLIBrainStopSeeded(t *testing.T) {
	t.Parallel()
	bed := gcliBrainStopBed(t, true)
	for stop := 1; stop <= 3; stop++ {
		verdict := bed.verdict("brain-stop", true)
		lines := gcliBrainLines(verdict.Display, 3)
		if !strings.HasPrefix(lines[0], "BRAIN SEAT:") || !strings.Contains(lines[0], "2 approved goals") ||
			!strings.Contains(lines[0], "1 asks await Wido (brain-ask)") || !strings.Contains(lines[0], "1 drafts await approval (brain-draft)") ||
			lines[1] != "OPEN WORK (1)" || lines[2] != "OPEN-WORK plans/brain-open-plan.md: Finish the local note." {
			t.Fatalf("brain display lost its open work, action, ask, or draft on stop %d: %s", stop, verdict.Display)
		}
		if verdict.IdleRefusal {
			t.Fatalf("brain stop %d entered idle refusal", stop)
		}
		source := ""
		if verdict.BlockSource != nil {
			source = *verdict.BlockSource
		}
		if source == "idle-backlog" {
			t.Fatalf("brain stop %d used idle-backlog source", stop)
		}
		if stop == 1 {
			if source != "open-work" {
				t.Fatalf("first brain stop did not preserve open-work block: %q", source)
			}
			if !verdict.BrainStatusDue {
				t.Fatal("first brain stop did not mark status due")
			}
			// The status is posted now; the next stops fall inside its window.
			if err := brain.MarkStatusPosted(bed.root(), time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if source != "" {
			t.Fatalf("repeated brain stop blocked again: %q", source)
		}
		if verdict.BrainStatusDue {
			t.Fatal("fresh brain status remained due")
		}
	}
	if _, err := os.Stat(brain.StatusPath(bed.root())); err != nil {
		t.Fatalf("brain stop wrote no status record: %v", err)
	}
	intents, _ := filepath.Glob(filepath.Join(bed.root(), "artifacts", "agents", "steward", "intents", "*.json"))
	if len(intents) != 0 {
		t.Fatalf("brain stop staged a steward continuation intent: %v", intents)
	}
}

// TestGoalCLIBrainStopCorrupt ports brain-stop-corrupt: an unreadable
// declaration keeps the brain summary first, its remedy second and the open
// work after them, and never enters the idle ladder.
func TestGoalCLIBrainStopCorrupt(t *testing.T) {
	t.Parallel()
	bed := gcliBrainStopBed(t, true)
	gcliBrainWrite(t, brain.Path(bed.root()), []byte("{broken\n"))
	verdict := bed.verdict("brain-corrupt", true)
	lines := gcliBrainLines(verdict.Display, 4)
	if !strings.HasPrefix(lines[0], "BRAIN SEAT:") || !strings.HasPrefix(lines[1], "this checkout's brain declaration is unreadable") ||
		lines[2] != "OPEN WORK (1)" || lines[3] != "OPEN-WORK plans/brain-open-plan.md: Finish the local note." {
		t.Fatalf("corrupt brain lost its leading summary, remedy, or following action: %s", verdict.Display)
	}
	if verdict.IdleRefusal {
		t.Fatal("corrupt brain entered idle refusal")
	}
}

// TestGoalCLIBrainUndeclaredMalformedQuestion ports the first block of
// brain-human-word-refuses: an undeclared checkout does not inherit the
// brain-only malformed-question failure.
func TestGoalCLIBrainUndeclaredMalformedQuestion(t *testing.T) {
	t.Parallel()
	bed := gcliBrainNewBed(t, false)
	gcliBrainWrite(t, filepath.Join(bed.root(), "artifacts", "agents", "channel", "questions", "undeclared-broken.json"), []byte("{broken\n"))
	verdict := bed.verdict("brain-undeclared-question", false)
	if strings.Contains(verdict.Display, "inputs unreadable:") || strings.Contains(verdict.Display, "undeclared-broken.json") {
		t.Fatalf("an undeclared checkout inherited the brain-only malformed-question failure: %s", verdict.Display)
	}
}

// TestGoalCLIBrainStatusLine ports brain-status-line: once the boot's
// delivery writes the brain status, channel status leads with the
// transport-free brain line; after the withdrawal it no longer does.
func TestGoalCLIBrainStatusLine(t *testing.T) {
	t.Parallel()
	bed := gcliBrainNewBed(t, false)
	gcliBrainDeclare(t, bed.root())
	identity := func(string) string { return gcliBrainLedger }
	state := brain.Read(bed.root(), gcliBrainLedger)
	if state.State != brain.Declared {
		t.Fatalf("the bed is not declared: %+v", state)
	}
	encoded, err := json.Marshal(*state.Record)
	if err != nil {
		t.Fatal(err)
	}
	if err := brainStartDelivered(bed.root(), bed.root(), fmt.Sprintf("%x", sha256.Sum256(encoded)), -1, "", identity, nil); err != nil {
		t.Fatalf("brain boot delivery: %v", err)
	}
	status := func() string {
		var stdout, stderr strings.Builder
		machine := func(string) (string, error) { return "mac-cli", nil }
		landing := func(string, time.Time) ([]byte, error) { return []byte{}, nil }
		if code := runChannelStatusTo([]string{"--root", bed.root()}, &stdout, &stderr, machine, bed.dependencies().endpoint, landing); code != 0 {
			t.Fatalf("channel status exited %d: %s", code, stderr.String())
		}
		return stdout.String()
	}
	declared := status()
	first, _, _ := strings.Cut(declared, "\n")
	if !strings.HasPrefix(first, "BRAIN: mac-cli for ledger ") || strings.Contains(first, "://") {
		t.Fatalf("declared status did not lead with transport-free brain line: %s", declared)
	}
	if _, err := brain.Withdraw(bed.root(), t.TempDir(), gcliBrainLedger); err != nil {
		t.Fatal(err)
	}
	if undeclared := status(); strings.HasPrefix(undeclared, "BRAIN:") {
		t.Fatalf("undeclared status retained brain line: %s", undeclared)
	}
}
