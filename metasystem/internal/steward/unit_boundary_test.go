package steward

import (
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestHeldWorkCompletedMatchesTheBoundaryRule(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"dead-open-step", "failed-without-review", "missing-read", "unpublished-read", "published-terminal-steps"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed, units, work, _ := seatBusyFixture(t)
			seatBusyRun(t, units, bed.now, 41)
			step := launch.UnitStep{Name: "build", LaunchID: "launch-1", State: launch.StepPassed, FinishedAt: bed.now.Format("2006-01-02T15:04:05Z07:00")}
			state := "awaiting-judgement"
			stage := "reviewed; its read is collected and published"
			switch scenario {
			case "dead-open-step":
				state = "running"
				step.State, stage = launch.StepRunning, "building"
			case "failed-without-review":
				step.State, stage = launch.StepFailed, "build failed; awaiting read"
			case "missing-read":
				stage = "built; awaiting read"
			case "unpublished-read":
				stage = "reviewed; its read is collected but not published"
			}
			writeStewardRecord(t, filepath.Join(units, "run-1", "run.json"), map[string]any{"id": "run-1", "goal": "held", "state": state, "rounds": []launch.UnitRound{{Number: 1, Outcome: "green", Steps: []launch.UnitStep{step}}}})
			calls := 0
			reader := func(root, id string) ([]UnitStage, error) {
				calls++
				if root != bed.root || id != "held" {
					t.Fatalf("unexpected units read: %s %s", root, id)
				}
				return []UnitStage{{Run: "run-1", Stage: stage}}, nil
			}
			complete, runs, err := heldWorkCompleted(bed.root, filepath.Dir(units), work, reader, seatBusyDeadProber{})
			want := scenario == "published-terminal-steps"
			if err != nil || complete != want || (len(runs) > 0) != want || calls != 1 {
				t.Fatalf("completion=%t runs=%d units reads=%d error=%v; want complete=%t", complete, len(runs), calls, err, want)
			}
		})
	}
}
