package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestJudgementLineNamesBuildReadAndProof(t *testing.T) {
	manager := &launch.Manager{Store: launch.Store{Root: t.TempDir()}}
	yes := true
	record := launch.UnitRunRecord{ID: "run", State: "awaiting-judgement", Rounds: []launch.UnitRound{{Number: 1, Directory: filepath.Join(t.TempDir(), "round-1"), Outcome: "proof-red", Steps: []launch.UnitStep{
		{Name: "build", LaunchID: "build-id", State: launch.StepPassed},
		{Name: "read", LaunchID: "read-id", State: launch.StepPassed, Verdict: "pass", VerdictCounts: &yes},
		{Name: "proof:one", LaunchID: "proof-one", State: launch.StepPassed},
		{Name: "proof:two", LaunchID: "proof-two", State: launch.StepFailed},
	}}}}
	manager.Store.Create(launch.Record{ID: "build-id", Kind: "build", Adapter: "codex-exec", State: launch.Completed, AdapterData: map[string]json.RawMessage{}})
	line := unitJudgementLine(record, manager)
	for _, want := range []string{"build=build-id:completed", "read=read-id:pass", "verdict-counts=true", "proof=1/2", "red=two", "state=awaiting-judgement"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line=%q missing=%q", line, want)
		}
	}
}

type fakeUnitAdvancer struct {
	result launch.UnitResult
	err    error
}

func (runner fakeUnitAdvancer) Advance(launch.UnitRequest) (launch.UnitResult, error) {
	return runner.result, runner.err
}

func TestUnitRunPrintsOneLine(t *testing.T) {
	terminal := launch.UnitResult{Record: launch.UnitRunRecord{ID: "terminal", State: "awaiting-judgement", Rounds: []launch.UnitRound{{
		Number: 1, Directory: t.TempDir(), Outcome: "green", Steps: []launch.UnitStep{{Name: "build", State: launch.StepPassed}},
	}}}, Round: 1}
	capped := launch.UnitResult{Record: launch.UnitRunRecord{ID: "capped"}, Round: 1, Step: "proof:check", Launch: "launch", Capped: true}
	for _, row := range []struct {
		name string
		code int
		give launch.UnitResult
	}{{"terminal", 0, terminal}, {"capped", 3, capped}} {
		t.Run(row.name, func(t *testing.T) {
			old := unitRunner
			unitRunner = func() unitAdvancer { return fakeUnitAdvancer{result: row.give} }
			t.Cleanup(func() { unitRunner = old })
			code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
				return runUnitRun([]string{"--resume", "run"})
			})
			if code != row.code || stderr != "" || !strings.HasSuffix(stdout, "\n") || strings.Count(stdout, "\n") != 1 || strings.TrimSpace(stdout) == "" {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
}

func TestUnitFamilyIsRegistered(t *testing.T) {
	for _, family := range families() {
		if family.name == "unit" && len(family.verbs) == 1 && family.verbs[0].name == "run" {
			return
		}
	}
	t.Fatal("unit run is not registered")
}
