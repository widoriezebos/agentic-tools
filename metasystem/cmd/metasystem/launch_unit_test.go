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

func TestJudgementLineSkipsReadSupersededByCountingRerun(t *testing.T) {
	t.Parallel()
	yes, no := true, false
	record := launch.UnitRunRecord{ID: "run", State: "awaiting-judgement", Rounds: []launch.UnitRound{{Number: 1, Directory: t.TempDir(), Outcome: "green", Steps: []launch.UnitStep{
		{Name: "read:pkg/a", LaunchID: "compacted", State: launch.StepPassed, Package: "pkg/a", Verdict: "fix first", VerdictCounts: &no},
		{Name: "read:pkg/a/a.go:rerun", LaunchID: "rerun", State: launch.StepPassed, Package: "pkg/a", File: "pkg/a/a.go", Rerun: true, Verdict: "pass", VerdictCounts: &yes},
	}}}}
	line := unitJudgementLine(record, nil)
	if strings.Contains(line, "compacted:fix first") || !strings.Contains(line, "read=rerun:pass verdict-counts=true") {
		t.Fatalf("line=%q", line)
	}
}
