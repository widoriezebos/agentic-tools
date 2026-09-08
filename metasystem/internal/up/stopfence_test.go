package up

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

func TestFenceReadersReturnStoppedForOrdinaryAndRecovery(t *testing.T) {
	root := t.TempDir()
	if err := stopfence.Write(root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 1,
		ChangedAt: "2026-09-07T00:00:00Z", Checkout: root,
	}); err != nil {
		t.Fatal(err)
	}
	for _, options := range []Options{{Root: root}, {Root: root, RecoverOnly: true, IfDown: true}} {
		result := Run(options)
		if result.ExitCode() != 0 || result.Outcome != "stopped" || len(result.Components) != 1 ||
			result.Components[0].Detail != "since 2026-09-07T00:00:00Z" ||
			!strings.Contains(result.Remedy, "metasystem arm --repo ") {
			t.Fatalf("stopped result = %#v", result)
		}
	}
}

func TestFenceReaderRemediesFollowTheDurableStopPhase(t *testing.T) {
	for _, test := range []struct {
		phase      string
		survivors  []stopfence.Survivor
		wantDetail func(string) string
		wantRemedy string
	}{
		{
			phase: stopfence.PhaseStopIncomplete, survivors: []stopfence.Survivor{{Component: "run", ID: "one", Reason: "survived"}},
			wantDetail: func(root string) string {
				return "stop incomplete for " + root + " since now by stop pid 71; 1 unresolved entries from the last stop"
			}, wantRemedy: "metasystem stop --repo",
		},
		{
			phase: stopfence.PhaseStopping,
			wantDetail: func(root string) string {
				return "stop unfinished for " + root + " since now by stop pid 71"
			}, wantRemedy: "metasystem stop --repo",
		},
	} {
		root := t.TempDir()
		if err := stopfence.Write(root, stopfence.Record{State: stopfence.StateClosed, Phase: test.phase, Generation: 2, ChangedAt: "now", Checkout: root, By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 71, PidStartedAt: 70}}, NotStopped: test.survivors}); err != nil {
			t.Fatal(err)
		}
		result := Run(Options{Root: root, Scope: root})
		lines := strings.Join(result.Lines(), "\n")
		if result.Outcome != "stopped" || len(result.Components) != 1 || result.Components[0].Detail != test.wantDetail(root) || !strings.Contains(result.Remedy, test.wantRemedy) || strings.Contains(result.Remedy, "metasystem arm") || strings.Count(lines, result.Remedy) != 1 {
			t.Fatalf("phase %s result = %#v", test.phase, result)
		}
	}
}

func TestEnsureStewardRunnerReportsFencedWithoutCreating(t *testing.T) {
	root := t.TempDir()
	if err := stopfence.Write(root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 3,
		ChangedAt: "2026-09-07T00:00:00Z", Checkout: root,
	}); err != nil {
		t.Fatal(err)
	}

	components, failed := ensureStewardRunner(Options{Root: root, WaitScaleMilli: 1}, nil, nil)
	if failed != nil || len(components) != 1 || components[0].Component != "steward-runner" ||
		components[0].Outcome != "FENCED" || components[0].Detail != "pid=0 generation=3" {
		t.Fatalf("fenced steward result = %#v, failure = %#v", components, failed)
	}
	if _, err := os.Stat(filepath.Join(root, "artifacts", "agents", "steward", "runner.json")); !os.IsNotExist(err) {
		t.Fatalf("fenced EnsureRunner created a runner record: %v", err)
	}
}
