package plain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestLandingStewardAlreadyProvenAllowsLaterRowsAndNextPass(t *testing.T) {
	t.Parallel()
	b, scoped := changedMainStewardFixture(t)
	seedMainProofReuse(t, scoped, "landed batch")
	scoped.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
		return []goal.TrunkRedEntry{
			{ID: "first", Class: goal.TrunkRedClassTrunkRed, Sightings: []goal.TrunkRedSighting{{BaseCommit: "older-main"}}},
			{ID: "later", Class: goal.TrunkRedClassTrunkRed, Sightings: []goal.TrunkRedSighting{{BaseCommit: "older-main"}}},
		}, nil
	}
	calls := 0
	scoped.seams.Steward.Trunk = func(install, checkout string, s ProveSeams) (string, error) {
		calls++
		if calls == 1 {
			outcome, err := stewardTrunk(install, checkout, s)
			if outcome != stewardAlreadyProven || err != nil {
				t.Fatalf("applicable proof = %q, %v", outcome, err)
			}
			return outcome, err
		}
		return "proving main", nil
	}
	run, err := StewardStep(b.home, bedNow, scoped.seams)
	if err != nil || calls != 2 || len(run.Interventions) != 1 || run.Interventions[0].Subject != "later" {
		t.Fatalf("first pass=%+v calls=%d err=%v", run, calls, err)
	}
	entries, err := readLines[Intervention](interventionsPath(b.install))
	if err != nil || len(entries) != 1 || entries[0].Subject != "later" {
		t.Fatalf("invented an intervention: %+v %v", entries, err)
	}
	run, err = StewardStep(b.home, bedNow.Add(time.Minute), scoped.seams)
	if err != nil || calls != 3 || len(run.Interventions) != 1 || run.Interventions[0].Subject != "first" {
		t.Fatalf("next pass=%+v calls=%d err=%v", run, calls, err)
	}
}

func TestLandingStewardAlreadyProvenKeepsWithdrawalAndBound(t *testing.T) {
	t.Parallel()
	b, scoped := changedMainStewardFixture(t)
	seedMainProofReuse(t, scoped, "landed batch")
	b.staleQuestion()
	b.seams = scoped.seams
	b.seams.Steward.Trunk = stewardTrunk
	run := b.step(bedNow)
	if len(b.calls) != 1 || b.calls[0] != "withdraw" || len(run.Interventions) != 1 || run.Interventions[0].Row != "stale-question" || run.Interventions[0].Outcome != stewardAlreadyProven {
		t.Fatalf("withdrawal lost: %+v calls=%v", run, b.calls)
	}
	// Restore the same question to prove that the withdrawal still owns its bound.
	question := b.question
	question.State = "open"
	data, err := json.Marshal(question)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.install, "artifacts", "agents", "channel", "questions", question.ID+".json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	b.calls = nil
	run = b.step(bedNow.Add(time.Minute))
	if len(b.calls) != 0 || len(run.Interventions) != 0 || !strings.Contains(run.Line, "already acted within 60 minutes") {
		t.Fatalf("withdrawal repeated: %+v calls=%v", run, b.calls)
	}
	entries, err := readLines[Intervention](interventionsPath(b.install))
	if err != nil || len(entries) != 1 || entries[0].Row != "stale-question" {
		t.Fatalf("withdrawal record=%+v err=%v", entries, err)
	}
}
