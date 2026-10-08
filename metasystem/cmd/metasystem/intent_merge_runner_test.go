package main

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestIntentUnitRunnerKeepsCheckHooksAndIsolatesCallState(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	owners := bed.workOwners()
	owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("agent process")
	}
	source := owners.work.units(stateroot.Layout{})
	source.Actor = "person"
	owners.work.units = func(stateroot.Layout) *launch.UnitRunner { return source }
	for _, check := range []bool{true, false} {
		argv := []string{"work", "build", bed.id}
		if check {
			argv = append(argv, "--check", "fixture-check")
		}
		command, args, ok := resolveIntentArgv(argv)
		if !ok {
			t.Fatal("work build is unavailable")
		}
		input, problem := parseIntentArgs(command, args)
		if problem != nil {
			t.Fatal(problem)
		}
		inv := &intentInvocation{command: command, input: input, raw: args, owners: owners, cwd: bed.root(), stdout: io.Discard, stderr: io.Discard}
		runner := inv.unitRunner()
		if runner == source || runner.Manager == source.Manager || runner.Actor != "" {
			t.Fatal("an agent call reused the runner, manager, or person's actor")
		}
		if runner.Manager.BuildPolicy.Key != "host.builds" {
			t.Fatalf("host admission lost its policy: %+v", runner.Manager.BuildPolicy)
		}
		if runner.FreezeCheck == nil || runner.CriticCustody == nil || runner.ExaminationRead == nil || runner.CollectLaunch == nil {
			t.Fatal("host admission discarded the unit's check or examination hooks")
		}
		policy, err := runner.ReviewPolicy()
		want := "auto"
		if check {
			want = "person"
		}
		if err != nil || policy != want {
			t.Fatalf("check=%v: review policy=%q, %v; want %s", check, policy, err, want)
		}
	}
	policy, err := source.ReviewPolicy()
	if source.Actor != "person" || source.FreezeCheck != nil || source.CriticCustody != nil || err != nil || policy != "auto" {
		t.Fatal("per-call state changed the shared unit runner")
	}
}
