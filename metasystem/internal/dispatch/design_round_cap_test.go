package dispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func criticRecordParams(t *testing.T, root, role string) BuildRecordParams {
	t.Helper()
	tmp := t.TempDir()
	capResolution := filepath.Join(tmp, "cap.json")
	if err := WriteCapResolution(capResolution, 45, "built-in", "default"); err != nil {
		t.Fatal(err)
	}
	workspace, _ := filepath.Abs(filepath.Join("..", "..", ".."))
	p := BuildRecordParams{Output: filepath.Join(tmp, "record.json"), Job: role, Role: role, Root: root, Runtime: "fake", Workspace: workspace, CapResolution: capResolution, Model: "fake-model", Snapshot: "snapshot", InputBytes: 1, InputHash: "hash", Permissions: writeJSONFile(t, tmp, "permissions.json", map[string]any{}), Fallbacks: "[]", Signal: true, HandshakeBudget: 20, MainID: "main", ClaimEpoch: "1", DestructiveReach: HazardMechanical, ReasoningEffort: "medium", LaunchMode: LaunchModeSharedCheckout, OutputStream: filepath.Join(tmp, "stream.jsonl")}
	if role == "design-critic" {
		p.DeclaredOutputs = filepath.Join(tmp, "outputs.txt")
		if err := os.WriteFile(p.DeclaredOutputs, []byte("metasystem/internal/dispatch/build.go\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		p.Design = "metasystem/plans/critique-closes-on-folded-proof-design.md"
	}
	return p
}

// A design-critic chain takes the goal's own review-round member, clamped by
// metasystem.budget.review-round-max, exactly like a code critic (Wido
// 2026-10-02): the count is a far-away backstop, the loop ends on materiality.
func TestDesignCriticDispatchTakesGoalRoundLimit(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)
	p := criticRecordParams(t, bed.root, "design-critic")
	p.GoalID, p.GoalRevision, p.GoalTier, p.GateWidth, p.MachineID = "bounded", 2, 3, "full", "bed-m1"
	if limit := buildCriticLimitWithReads(t, p, bed.reads); limit != 3 {
		t.Fatalf("design-critic dispatch limit = %d, want the goal's 3", limit)
	}
}

// setGoalReviewRounds rewrites the bed goal's review-round member and the
// configured ceiling, then re-accepts the ledger.
func setGoalReviewRounds(t *testing.T, bed *goalAdmissionBed, member int64, ceiling int) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte(fmt.Sprintf("metasystem.budget.review-round-max=%d\n", ceiling)), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(bed.root, "plans", "goals", "bounded.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse goal fixture: %v", problems)
	}
	file.Budget.ReviewRoundLimit = member
	file.NormApproval.ReviewRounds = member
	file.Approved.Digest = legacyBudgetApprovalDigest(file.Intent, *file.Budget)
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.accept(t)
}

// Decision 2 caps design examinations while preserving the code-critic budget.
func TestDesignCriticLimitIsGoalMemberUnderCeiling(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ member, want int64 }{{12, 12}, {30, 20}} {
		bed := newGoalAdmissionBed(t, 2)
		setGoalReviewRounds(t, bed, c.member, 20)
		for _, role := range []string{"design-critic", "code-critic"} {
			p := criticRecordParams(t, bed.root, role)
			p.GoalID, p.GoalRevision, p.GoalTier, p.GateWidth, p.MachineID = "bounded", 2, 3, "full", "bed-m1"
			want := c.want
			if role == "design-critic" {
				want = 4
			}
			if limit := buildCriticLimitWithReads(t, p, bed.reads); limit != want {
				t.Fatalf("box %d under ceiling 20: %s dispatch limit = %d, want %d", c.member, role, limit, want)
			}
		}
		resolution, err := goalReviewRoundLimitWithReads(bed.root, "bounded", 2, "design-critic", bed.reads)
		if err != nil || resolution.roleLimit != 4 || resolution.rebindLimit() != 4 {
			t.Fatalf("box %d under ceiling 20: resolution %+v (rebind %d), %v; want %d", c.member, resolution, resolution.rebindLimit(), err, c.want)
		}
	}
}

func TestGoalFreeDesignCriticDispatchAndFallbackTakeCeiling(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.budget.review-round-max=7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reads := goalFreeCriticReads(t)
	for role, want := range map[string]int64{"design-critic": 4, "code-critic": 7} {
		if got := buildCriticLimitWithReads(t, criticRecordParams(t, root, role), reads); got != want {
			t.Fatalf("goal-free %s dispatch limit = %d, want %d", role, got, want)
		}
	}
	account, err := critiqueRoundAccountingWithReads(root, critiqueState{}, "critic", map[string]any{"role": "design-critic", "goalId": nil, criticRoundsConsumedField: 0}, reads)
	if err != nil || account.limit != 4 {
		t.Fatalf("goal-free design-critic fallback = %+v, %v; want limit 4", account, err)
	}
	path := filepath.Join(root, "artifacts", "agents", "jobs", "critic.json")
	writeJSONFile(t, filepath.Dir(path), filepath.Base(path), map[string]any{"jobId": "critic", "role": "design-critic", "goalId": nil, reviewRoundLimitField: 3, criticRoundsConsumedField: 0, "critiqueBudgetBinding": map[string]any{"opid": "critique-budget-rebind-critic-r0"}})
	if outcome, err := critiqueBudgetRebindWithReads(root, "critic", reads); err != nil || outcome != "rebound" {
		t.Fatalf("goal-free rebind = %q, %v", outcome, err)
	}
	record := readJSONFile(t, path)
	if limit, _ := numInt(record[reviewRoundLimitField]); limit != 4 {
		t.Fatalf("goal-free rebound limit = %v, want 4", record[reviewRoundLimitField])
	}
}

// Design critique exists at tiers 2 and 3 (Wido 2026-10-01); tier 1 is refused.
func TestDesignCriticDispatchRequiresTierTwoOrThree(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)
	p := criticRecordParams(t, bed.root, "design-critic")
	p.Workspace = t.TempDir()
	p.GoalID, p.GoalRevision, p.GoalTier, p.GateWidth, p.MachineID = "bounded", 2, 1, "full", "bed-m1"
	err := buildRecordWithReads(p, recordFacts(t, p.Workspace, 1, ""), bed.reads)
	if err == nil || !strings.Contains(err.Error(), "design-critic at goal tier 1") || !strings.Contains(err.Error(), "design critique exists at tiers 2 and 3") {
		t.Fatalf("tier-1 refusal = %v", err)
	}
	if err := validateReviewRoundTier("design-critic", true, 2); err != nil {
		t.Fatalf("tier 2 design critique refused: %v", err)
	}
	if _, err := os.Stat(p.Output); !os.IsNotExist(err) {
		t.Fatalf("tier-1 refusal wrote record: %v", err)
	}
}
func TestDesignCriticBudgetRebindTakesGoalMember(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)
	goalRoot := bed.root
	path := filepath.Join(goalRoot, "artifacts", "agents", "jobs", "critic.json")
	writeJSONFile(t, filepath.Dir(path), filepath.Base(path), map[string]any{"jobId": "critic", "role": "design-critic", "goalId": "bounded", "goalRevision": 2, "goalTier": 3, reviewRoundLimitField: 6, criticRoundsConsumedField: 1})
	// Decision 2 keeps a legacy review budget while the examination cap stays separate.
	if account, err := critiqueRoundAccounting(goalRoot, critiqueState{}, "critic", readJSONFile(t, path)); err != nil || account.limit != 6 {
		t.Fatalf("stored design-critic limit 6 = %+v, %v; want 6", account, err)
	}
	if outcome, err := critiqueBudgetRebindWithReads(goalRoot, "critic", bed.reads); err != nil || outcome != "rebound" {
		t.Fatalf("goal-bound repair = %q, %v; want rebound", outcome, err)
	}
	record := readJSONFile(t, path)
	// The rebind takes the goal's stored member (3).
	if limit, _ := numInt(record[reviewRoundLimitField]); limit != 3 {
		t.Fatalf("goal-bound repaired limit = %v, want 3", record[reviewRoundLimitField])
	}
	account, err := critiqueRoundAccounting(goalRoot, critiqueState{}, "critic", record)
	if err != nil || account.limit != 3 {
		t.Fatalf("repaired accounting = %+v, %v; want limit 3", account, err)
	}
	delete(record, reviewRoundLimitField)
	delete(record, "goalTier")
	if account, err = critiqueRoundAccountingWithReads(goalRoot, critiqueState{}, "critic", record, bed.reads); err != nil || account.limit != 3 {
		t.Fatalf("tier-free fallback accounting = %+v, %v; want the goal's 3", account, err)
	}
	if limit := reviewRoundLimitForRole("design-critic", true, 1).rebindLimit(); limit != 1 {
		t.Fatalf("goal-bound rebind = %d, want 1", limit)
	}
	if limit := reviewRoundLimitForRole("design-critic", true, 12).rebindLimit(); limit != 4 {
		t.Fatalf("goal-bound rebind = %d, want the design cap of 4", limit)
	}
	if limit := reviewRoundLimitForRole("design-critic", false, 20).rebindLimit(); limit != 4 {
		t.Fatalf("goal-free rebind = %d, want the design cap of 4", limit)
	}
}
func TestNonDesignCriticsKeepGoalRoundLimit(t *testing.T) {
	for _, role := range []string{"code-critic", "warden"} {
		bed := newGoalAdmissionBed(t, 2)
		p := criticRecordParams(t, bed.root, role)
		p.GoalID, p.GoalRevision, p.GoalTier, p.GateWidth, p.MachineID = "bounded", 2, 3, "full", "bed-m1"
		if got := buildCriticLimitWithReads(t, p, bed.reads); got != 3 {
			t.Errorf("%s limit = %d, want 3", role, got)
		}
	}
}

// Decision 2 binds dispatch before a return or a later goal budget can move it.
func TestDesignCriticAdmissionFreezesAllowance(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name      string
		goalBound bool
		member    int64
		ceiling   int
		want      int64
	}{
		{"approved-zero", true, 0, 20, 0},
		{"approved-zero-unlimited-config", true, 0, 0, 0},
		{"approved-two", true, 2, 0, 2},
		{"approved-twenty", true, 20, 20, 4},
		{"goal-free-zero", false, 0, 0, 4},
		{"goal-free-two", false, 0, 2, 2},
		{"goal-free-twenty", false, 0, 20, 4},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			bed := newGoalAdmissionBed(t, 2)
			setGoalReviewRounds(t, bed, c.member, c.ceiling)
			p := criticRecordParams(t, bed.root, "design-critic")
			p.Workspace = t.TempDir()
			reads := goalFreeCriticReads(t)
			if c.goalBound {
				p.GoalID, p.GoalRevision, p.GoalTier, p.GateWidth, p.MachineID = "bounded", 2, 3, "full", "bed-m1"
				reads = bed.reads
			}
			designPath := p.Design
			if c.want == 0 {
				designPath = ""
			}
			err := buildRecordWithReads(p, recordFacts(t, p.Workspace, 1, designPath), reads)
			if c.want == 0 {
				if err == nil || strings.Contains(err.Error(), "<nil>") || !strings.Contains(err.Error(), "goal budget is zero") || !strings.Contains(err.Error(), "revise and approve") {
					t.Fatalf("approved zero has no plain reason and remedy: %v", err)
				}
				if _, err := os.Stat(p.Output); !os.IsNotExist(err) {
					t.Fatalf("zero wrote a record: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			record := readJSONFile(t, p.Output)
			if n, ok := numInt(record["designExaminationLimit"]); !ok || n != c.want {
				t.Fatalf("admission did not freeze %d: %v", c.want, record)
			}
			jobPath := filepath.Join(bed.root, "artifacts", "agents", "jobs", "critic.json")
			record[reviewRoundLimitField] = 20
			if err := os.MkdirAll(filepath.Dir(jobPath), 0755); err != nil {
				t.Fatal(err)
			}
			if err := writeRecord(jobPath, record); err != nil {
				t.Fatal(err)
			}
			if n := DesignRoundLimit(bed.root, "critic", 20); n != c.want {
				t.Fatalf("budget raise moved frozen allowance: %d", n)
			}
		})
	}
}
