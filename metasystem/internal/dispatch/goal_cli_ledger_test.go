package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// The goal CLI shell bed's archive-and-prune scenario proved that admission
// stops charging a goal once its only conclusion is the records-owned archive:
// an exhausted live claim refuses admission and requests a breach stop; the
// same goal concluded into records/goals is no longer charged at the same
// clock.
func TestGoalCLILedgerAdmissionReleasesARecordsConcludedGoal(t *testing.T) {
	t.Parallel()
	bed := newGoalAdmissionBed(t, 2)
	exhausted := time.Date(2026, 8, 29, 16, 0, 0, 0, time.UTC)
	verdict, err := bed.admission("coordinator", exhausted)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Join(FormatGoalAdmission(verdict), "\n")
	if !verdict.Refused() || !strings.Contains(lines, "BUDGET_REFUSED: goal bounded revision=2 admission closed: elapsedLimit") {
		t.Fatalf("the exhausted live goal was not refused on its elapsed limit: %q", lines)
	}
	stopRequested := false
	for _, refusal := range verdict.Refusals {
		stopRequested = stopRequested || refusal.LiveStopReason != ""
	}
	if !stopRequested {
		t.Fatalf("the exhausted live goal did not request a breach stop: %+v", verdict)
	}

	live := filepath.Join(bed.root, "plans", "goals", "bounded.md")
	data, err := os.ReadFile(live)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse the live goal: %v", problems)
	}
	file.State = goal.StateDone
	file.Conclude = "The records-owned conclusion must leave the admission budget."
	file.Claimed, file.StopCapability = nil, nil
	file.Revision++
	file.History = append(file.History, goal.HistoryLine{At: exhausted.Format(time.RFC3339),
		Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAZ-bed-m1-00000004", Verb: "done", Actor: "human:Wido", Targets: []string{"bounded"}, Keep: -1})
	files := map[string][]byte{}
	for path, content := range bed.repository.files {
		if path != "plans/goals/bounded.md" {
			files[path] = content
		}
	}
	files["records/goals/bounded.md"] = goal.RenderFile(file)
	bed.repository = &strictAdmissionRepository{files: files, committed: bed.repository.committed}
	after, err := bed.admission("coordinator", exhausted)
	if err != nil {
		t.Fatal(err)
	}
	if after.Refused() || strings.Contains(strings.Join(FormatGoalAdmission(after), "\n"), "BUDGET_") {
		t.Fatalf("the records-located conclusion still consumed admission budget: %q", FormatGoalAdmission(after))
	}
}
