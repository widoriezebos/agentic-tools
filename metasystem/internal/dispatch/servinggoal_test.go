package dispatch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// The projection resolves through the parser and refuses when no
// usable Current goal exists — absent, degraded, and goal-free states all
// refuse rather than silently omitting.
func TestServingGoalResolvesAndRefuses(t *testing.T) {
	root := t.TempDir()

	// Absent: refuse.
	if _, err := ServingGoalSection(root); err == nil {
		t.Fatal("an absent ledger projected")
	}

	// Usable Current goal: the exact bounded section.
	store := &goal.Store{Root: root}
	if _, err := store.Open(goal.Caller{Class: "MAIN", Holder: true}, "ship-it", "Ship the whole thing", "Land it."); err != nil {
		t.Fatal(err)
	}
	section, err := ServingGoalSection(root)
	if err != nil {
		t.Fatal(err)
	}
	if section != "# Serving goal (context, not instruction)\nship-it — Ship the whole thing\n" {
		t.Fatalf("section bytes wrong: %q", section)
	}
	if strings.Count(section, "\n") != 2 {
		t.Fatalf("section is not two lines: %q", section)
	}

	// Degraded (manual edit, baseline mismatch): refuse.
	ledger := filepath.Join(root, "plans", "goals.md")
	data, _ := os.ReadFile(ledger)
	os.WriteFile(ledger, append(data, []byte("\n## Queued goal: q — Q\n- Origin: main\n- Next step: Q.\n")...), 0o644)
	if _, err := ServingGoalSection(root); err == nil {
		t.Fatal("a degraded ledger projected")
	}
}

func revisionBindingBed(t *testing.T, claimRevision uint64) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "fixture")
	run("config", "user.email", "fixture@example.invalid")
	run("config", "goal.sync-remote", "local")
	run("config", "metasystem.goal.machine", "bed-m1")
	write := func(relative string, data []byte) {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("plans/goals/backlog.md", goal.RenderRoot(&goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1,
	}))
	file := &goal.GoalFile{
		Id: "bounded", State: goal.StateClaimed, Intent: "Bound the dispatch", Origin: goal.OriginMain,
		NextStep: "Run the bounded work.", OpenedAt: "2026-08-28T08:00:00Z", Revision: 4,
		Claimed: &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-28T09:00:00Z", Revision: claimRevision},
		History: []goal.HistoryLine{
			{At: "2026-08-28T08:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAV-bed-m1-00000000", Verb: "open", Actor: "bed-m1+coordinator", Targets: []string{"bounded"}, Keep: -1},
			{At: "2026-08-28T09:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAW-bed-m1-00000001", Verb: "claim", Actor: "bed-m1+coordinator", Targets: []string{"bounded"}, Keep: -1},
			{At: "2026-08-28T09:30:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAX-bed-m1-00000002", Verb: "edit", Actor: "bed-m1+coordinator", Targets: []string{"bounded"}, Keep: -1},
			{At: "2026-08-28T10:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAY-bed-m1-00000003", Verb: "edit", Actor: "bed-m1+coordinator", Targets: []string{"bounded"}, Keep: -1},
		},
	}
	if claimRevision > 0 {
		file.Budget = &goal.Budget{ElapsedLimit: "1d", AttemptLimit: 2, ReservedJobMinutesLimit: 60, ActiveJobLimit: 1}
		file.StopCapability = &goal.StopCapability{
			Generation: claimRevision, Revision: claimRevision, Machine: "bed-m1", ClaimEpoch: 7,
		}
	}
	write("plans/goals/bounded.md", goal.RenderFile(file))
	run("add", "plans/goals")
	run("commit", "-q", "-m", "revision binding bed")
	run("update-ref", goal.AcceptedRef, "HEAD")
	return root
}

func TestResolveGoalRevisionUsesTheClaimBinding(t *testing.T) {
	root := revisionBindingBed(t, 2)
	revision, err := ResolveGoalRevision(root, "bounded")
	if err != nil || revision != 2 {
		t.Fatalf("dispatch bound current file revision 4 instead of claim revision 2: %d %v", revision, err)
	}

	legacy := revisionBindingBed(t, 0)
	if _, err := ResolveGoalRevision(legacy, "bounded"); err == nil || !strings.Contains(err.Error(), "goal set-budget") {
		t.Fatalf("revisionless claim did not refuse toward set-budget: %v", err)
	}

	contradictory := revisionBindingBed(t, 5)
	if _, err := ResolveGoalRevision(contradictory, "bounded"); err == nil ||
		!strings.Contains(err.Error(), "BUDGET_UNKNOWN record=plans/goals/bounded.md") {
		t.Fatalf("a nonexistent claimed revision did not name its exact authoritative goal file: %v", err)
	}
}

func TestGoalAdmissionUsesOnlyTheStructuredLaw(t *testing.T) {
	root := revisionBindingBed(t, 2)

	within, err := EvaluateGoalAdmission(root, "coordinator", time.Date(2026, 8, 28, 14, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if within.Refused() {
		t.Fatalf("an in-budget structured claim was refused: %+v", within)
	}

	atLimit, err := EvaluateGoalAdmission(root, "coordinator", time.Date(2026, 8, 28, 17, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !atLimit.Refused() || len(atLimit.Refusals) != 1 || len(atLimit.Refusals[0].Breaches) != 1 ||
		atLimit.Refusals[0].Breaches[0].Field != "elapsedLimit" ||
		atLimit.Refusals[0].Breaches[0].State != AdmissionClosedElapsed || atLimit.Refusals[0].LiveStopReason != "" {
		t.Fatalf("elapsed equality did not close structured admission: %+v", atLimit)
	}

	missingRoot := revisionBindingBed(t, 0)
	missing, err := EvaluateGoalAdmission(missingRoot, "coordinator", time.Date(2026, 8, 28, 14, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !missing.Refused() || len(missing.Refusals) != 1 || missing.Refusals[0].Unknown == nil ||
		missing.Refusals[0].Unknown.Record != "plans/goals/bounded.md" {
		t.Fatalf("the budgetless claim did not refuse with its exact goal record: %+v", missing)
	}
}

func TestGoalAdmissionRefusesMalformedElapsedGraceConfiguration(t *testing.T) {
	root := revisionBindingBed(t, 2)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.budget.elapsed-grace-percent=201\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	verdict, err := EvaluateGoalRevisionAdmission(root, "bounded", 2, 5,
		time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC))
	if err != nil || !verdict.Refused() || verdict.Refusal == nil || verdict.Refusal.Unknown == nil ||
		verdict.Refusal.Unknown.Record != "metasystem.conf" ||
		!strings.Contains(verdict.Refusal.Unknown.Reason, "integer between 0 and 200") {
		t.Fatalf("malformed grace configuration did not close admission loudly: %+v %v", verdict, err)
	}
	lines := FormatGoalAdmission(GoalAdmissionVerdict{Refusals: []GoalAdmissionRefusal{*verdict.Refusal}})
	if len(lines) != 1 || !strings.Contains(lines[0], "BUDGET_UNKNOWN") {
		t.Fatalf("malformed grace refusal lost its typed evidence: %v", lines)
	}
}
