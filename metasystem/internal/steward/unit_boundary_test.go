package steward

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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

func TestRetiredUnitBoundaryStages(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		stage   string
		settled bool
	}{
		{"cancelled; new work starts under another name", true},
		{"dropped; prior reads retained", true},
		{"dropped; question repair pending", false},
		{"drop pending: recorded", false},
		{"transferred; its destinations remain required", true},
		{"stopped: needs revision", false},
		{"superseded by Ua2, read under attestation", true},
		{"committed, ready to land without a read", true},
		{"reviewed; its read is collected and published", true},
		{"committed and under independent examination", false},
		{"committed, ready for review", false},
		{"running", false}, {"starting", false},
	} {
		t.Run(tc.stage, func(t *testing.T) {
			bed, units, work, _ := seatBusyFixture(t)
			reader := func(string, string) ([]UnitStage, error) { return []UnitStage{{Unit: "Ub", Stage: tc.stage}}, nil }
			if err := ObserveUnitBoundary(bed.root, filepath.Dir(units), "session", bed.now.Add(-time.Minute), work, reader, bed.now, seatBusyDeadProber{}); err != nil {
				t.Fatal(err)
			}
			events, err := ReadUnitBoundaries(bed.root)
			if err != nil || (len(events) > 0) != tc.settled {
				t.Fatalf("boundary=%+v err=%v want=%v", events, err, tc.settled)
			}
		})
	}
}

func TestRetiredBoundaryBindsTheSessionHandoff(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "metasystem.conf"), []byte("seat.driver=auto\n"))
	work := goal.ClaimableBudgetedWork{Claimed: []string{"fix-it"}}
	now := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	session := "session / original"
	reader := func(string, string) ([]UnitStage, error) {
		return []UnitStage{
			{Unit: "Ua", Stage: "superseded by Ua2, read under attestation"},
			{Unit: "Ub", Stage: "cancelled; new work starts under another name"},
			{Unit: "Uc", Stage: "reviewed; its read is collected and published"},
			{Unit: "Ud", Stage: "committed, ready to land without a read"},
		}, nil
	}
	if err := ObserveUnitBoundary(root, t.TempDir(), session, now.Add(-time.Minute), work, reader, now, seatBusyDeadProber{}); err != nil {
		t.Fatal(err)
	}
	nonce := "2000000000000012"
	staged := writeStagedHandoffFixture(t, root, nonce)
	writeStewardRecord(t, filepath.Join(root, "artifacts", "agents", "mains", "worktree-lease.json"), map[string]any{"holderMainId": "main-1", "ownerLineage": SeatLineage, "pid": 4242, "revision": 1, "claimEpoch": 1})
	if err := MintIntent(root, Intent{Nonce: nonce, Goal: "fix-it", Reason: seatHandoffReason, Handoff: &staged.binding}); err != nil {
		t.Fatal(err)
	}
	if err := BindUnitHandoff(root, session); err != nil {
		t.Fatal(err)
	}
	events, err := ReadUnitBoundaries(root)
	if err != nil || len(events) == 0 || events[0].Handoff != nonce {
		t.Fatalf("handoff not bound: %+v %v", events, err)
	}
}

func TestRetiredBoundaryDoesNotPublishAChangedObservation(t *testing.T) {
	t.Parallel()
	bed, units, work, _ := seatBusyFixture(t)
	calls := 0
	reader := func(string, string) ([]UnitStage, error) {
		calls++
		stage := "cancelled; new work starts under another name"
		if calls > 1 {
			stage = "running"
		}
		return []UnitStage{{Unit: "Ub", Stage: stage}}, nil
	}
	if err := ObserveUnitBoundary(bed.root, filepath.Dir(units), "session", bed.now.Add(-time.Minute), work, reader, bed.now, seatBusyDeadProber{}); err != nil {
		t.Fatal(err)
	}
	events, err := ReadUnitBoundaries(bed.root)
	if err != nil || len(events) != 0 {
		t.Fatalf("changed work bound a handoff: %+v %v", events, err)
	}
}
