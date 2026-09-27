package report

// Ported from scripts/agents/supervision-fixtures.sh, scenario
// stop-hook-monitor, S4-14/S4-15(a): the report behind a Stop that refuses a
// turn ending with open work retains the verdict display (OPEN WORK and the
// plan's next step) and carries the health evidence section.

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestSupCOpenWorkStopReportRetainsTheVerdictAndHealth(t *testing.T) {
	t.Parallel()
	root := stopPresentationRoot(t)
	input := stopPresentationFixture(root, strings.Repeat("3", 32), true)
	source := "open-work"
	input.Control.Class, input.Control.BlockSource = "open-work", &source
	display := "OPEN WORK (1)\nOPEN-WORK plans/stream.md: dispatch the runner"
	input.Judgment.Verdict.Class, input.Judgment.Verdict.BlockSource = "open-work", &source
	input.Judgment.Verdict.Display, input.Judgment.FullDisplay = display, display
	input.Judgment.Scan.Open = []goal.Item{{Kind: "plan", Id: "plans/stream.md", Detail: "dispatch the runner",
		FullDetail: "dispatch the runner", SourcePath: "plans/stream.md", RequestedAction: "dispatch the runner"}}
	input.Judgment.Refusal = goal.TurnRefusalFacts{Class: "open-work", BlockSource: source, Occurrence: 1}
	inputPath := filepath.Join(t.TempDir(), "input.json")
	writeStopInput(t, inputPath, input)
	result, err := PresentStop(root, inputPath, filepath.Join(t.TempDir(), "presentation.json"), time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Control.ShouldBlock {
		t.Fatalf("the presentation dropped the open-work refusal: %+v", result.Control)
	}
	reportBytes, _, err := ReadStopStatus(root, result.Report.Id)
	if err != nil {
		t.Fatal(err)
	}
	report := string(reportBytes)
	for _, want := range []string{"OPEN WORK (1)", "dispatch the runner", "## Health\n\n```json\n", `"runner alive"`} {
		if !strings.Contains(report, want) {
			t.Fatalf("the Stop report omitted %q:\n%s", want, report)
		}
	}
}
