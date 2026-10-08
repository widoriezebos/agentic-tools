package launch

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestUnitBuildAdmissionRefusalKeepsStartingStep(t *testing.T) {
	t.Parallel()
	for _, code := range []string{"LAUNCH_BUILD_CAPACITY", "LAUNCH_BUILD_PERSON"} {
		t.Run(code, func(t *testing.T) {
			t.Parallel()
			m, _, _, _ := manager(t)
			round := UnitRound{Number: 1, Directory: t.TempDir(), Steps: []UnitStep{{Name: "build", State: StepPending}}}
			calls := 0
			driver := stepDriver{manager: m, unit: true, round: &round,
				launchID: func(int) string { return "held-build" }, save: func() error { return nil },
				start: func(spec StartSpec) (Record, error) {
					calls++
					if spec.ID != "held-build" {
						t.Fatalf("admission changed its launch identity: %s", spec.ID)
					}
					return Record{}, coded(code, "", errors.New("build admission is waiting"))
				}}
			for range 2 {
				_, err := driver.startStep(0, StartSpec{Kind: "build"})
				step := round.Steps[0]
				if !IsCode(err, code) || step.State != StepStarting || step.FinishedAt != "" || step.Cause != "" || round.Outcome != "" || len(step.LaunchIDs) != 1 {
					t.Fatalf("admission became a failed execution: step=%+v round=%+v error=%v", step, round, err)
				}
			}
			if calls != 2 {
				t.Fatalf("waiting admission was not retried: %d calls", calls)
			}
			if _, err := m.Store.Read("held-build"); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("waiting admission created a launch or unreadable state: %v", err)
			}
		})
	}
}

func TestUnitBuildUsesCurrentActorInsteadOfRetainedActor(t *testing.T) {
	t.Parallel()
	for _, current := range []string{"", "person"} {
		t.Run("current="+current, func(t *testing.T) {
			t.Parallel()
			fixture := baseUnitFixture(t)
			fixture.manager.Prober = &fakeProber{states: map[int64]identity.Liveness{10: identity.Dead, 20: identity.Dead}}
			fixture.manager.CapacitySources.Load = func(at time.Time) hostload.Sample {
				return hostload.Sample{At: at.Format(time.RFC3339Nano), Available: true, Load1m: 9}
			}
			fixture.runner.Actor = current
			retained := "person"
			if current == "person" {
				retained = ""
			}
			driver := fixture.runner.driver(&UnitRunRecord{ID: "unit"}, &UnitRound{Number: 1})
			record, err := driver.start(StartSpec{Kind: "build", Goal: "goal", Actor: retained, WorkingDirectory: fixture.worktree, Brief: brief(t)})
			if current == "person" {
				if err != nil || record.State != Completed || len(fixture.starter.order) != 1 {
					t.Fatalf("the person's current act could not start the retained build: %+v %v", record, err)
				}
			} else if !IsCode(err, "LAUNCH_BUILD_CAPACITY") || record.ID != "" || len(fixture.starter.order) != 0 {
				t.Fatalf("retained actor granted the agent authority: %+v %v", record, err)
			}
		})
	}
}
