package launch

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

func TestABuildRefusedForCapacityRetriesOnceAfterAWait(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, cause    string
		passes, capped bool
		resumeAfter    time.Duration
		sleeps         []time.Duration
	}{
		{name: "retry passes", cause: outage.ProviderLimit, passes: true, sleeps: []time.Duration{outage.ProbeInterval}},
		{name: "refused twice", cause: outage.ProviderLimit, sleeps: []time.Duration{outage.ProbeInterval}},
		{name: "at the deadline", cause: outage.ProviderLimit, capped: true, sleeps: []time.Duration{time.Second}},
		{name: "continued", cause: outage.ProviderLimit, capped: true, passes: true, resumeAfter: time.Minute, sleeps: []time.Duration{time.Second, time.Minute}},
		{name: "continued when due", cause: outage.ProviderLimit, capped: true, passes: true, resumeAfter: 200 * time.Second, sleeps: []time.Duration{time.Second}},
		{name: "other environment", cause: "process-lost", passes: true},
		{name: "plain exit"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			end := "failed"
			if row.passes {
				end = "round"
			}
			// Explicit events keep the full round script even while the first build fails.
			events := []string{"branch", "branch", "before"}
			if row.resumeAfter > 0 {
				events = append(events, "branch")
			}
			if !row.capped || row.resumeAfter > 0 {
				events = append(events, end)
			}
			f := newUnitFixture(t, "", events...)
			f.starter.readVerdict = "land"
			f.manager.Settings.WaitCapSeconds = 240
			advance := f.manager.Sleep
			var sleeps []time.Duration
			f.manager.Sleep = func(d time.Duration) { sleeps = append(sleeps, d); advance(d) }
			builds := 0
			f.starter.onStart = func(r Record) error {
				if r.Kind == "build" && builds == 1 && row.capped {
					advance(239 * time.Second)
				}
				if r.Kind == "build" && builds > 1 && row.capped && row.resumeAfter == 0 {
					f.starter.holdKind = "build"
				}
				return nil
			}
			f.starter.onBuild = func(r Record) error {
				builds++
				f.starter.failKind = "build"
				if row.passes && builds > 1 {
					f.starter.failKind = ""
					return nil
				}
				_, err := f.manager.Store.Update(r.ID, func(r *Record) error {
					r.Cause = row.cause
					if row.cause == outage.ProviderLimit {
						r.CauseWords = capacityWords
					}
					return nil
				})
				return err
			}
			result, err := f.runner.Advance(UnitRequest{Plan: f.plan})
			if err != nil {
				t.Fatal(err)
			}
			if row.capped {
				round, step := result.Record.Rounds[0], result.Record.Rounds[0].Steps[0]
				if !slices.Equal(sleeps, []time.Duration{time.Second}) || !result.Capped || step.State != StepFailed || len(step.LaunchIDs) != 1 || step.RetryBy != "machine" || round.Stop != nil {
					t.Fatalf("wait crossed the allowance: sleeps=%v capped=%t step=%+v stop=%+v", sleeps, result.Capped, step, round.Stop)
				}
				if row.resumeAfter > 0 {
					advance(row.resumeAfter - time.Second)
					result, err = f.runner.Continue(UnitRequest{Resume: result.Record.ID})
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			if !slices.Equal(sleeps, row.sleeps) {
				t.Fatalf("sleeps=%v, want %v", sleeps, row.sleeps)
			}
			round, step := result.Record.Rounds[0], result.Record.Rounds[0].Steps[0]
			wantBuilds := 2
			if row.cause == "" || row.capped && row.resumeAfter == 0 {
				wantBuilds = 1
			}
			if builds != wantBuilds || len(step.LaunchIDs) != wantBuilds {
				t.Fatalf("builds=%d step=%+v, want %d", builds, step, wantBuilds)
			}
			if row.cause == outage.ProviderLimit && (step.RetryBy != "machine" || step.RetryReason != "one retry 2m0s after the provider refused") {
				t.Fatalf("retry marker=%+v", step)
			}
			switch {
			case row.passes:
				if result.Capped || round.Cause != "" || round.Stop == nil || round.Stop.Decision != "close" || step.State != StepPassed {
					t.Fatalf("retry did not pass: %+v", round)
				}
			case row.capped:
			case row.cause == outage.ProviderLimit:
				if round.Cause != "environment" || round.Stop == nil || round.Stop.Handoff != "stopped environment" || !strings.Contains(round.Stop.Class, "the provider said: Selected model is at capacity. Please try a different model.") {
					t.Fatalf("second refusal=%+v", round)
				}
			default:
				if round.Cause != "unclassified" || round.Stop == nil || round.Stop.Class != "build: fixture-red" {
					t.Fatalf("plain exit=%+v", round)
				}
			}
		})
	}
}
