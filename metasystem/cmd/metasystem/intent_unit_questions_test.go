package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

type unitImpactWriter struct {
	bytes.Buffer
	observe func(string)
}

func (w *unitImpactWriter) Write(data []byte) (int, error) {
	w.observe(string(data))
	return w.Buffer.Write(data)
}

func TestIntentAcceptRiskClosesOnlyOneUnitFindingAsk(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	writeCriticChain(t, bed.root())
	owners := bed.owners()
	owners.processes.question = channel.ReadQuestion
	path := filepath.Join(bed.root(), "artifacts", "agents", "jobs", "critic.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var critic map[string]any
	if err := json.Unmarshal(data, &critic); err != nil {
		t.Fatal(err)
	}
	first := critic["findingRegister"].([]any)[0].(map[string]any)
	second := map[string]any{}
	for key, value := range first {
		second[key] = value
	}
	second["findingId"] = "S-2"
	critic["findingRegister"] = []any{first, second}
	writeTemp(t, filepath.Dir(path), "critic.json", critic)
	var ids []string
	for _, finding := range []string{"S-1", "S-2"} {
		command := "metasystem goal accept-risk " + bedGoal + " --finding " + finding + " --review critic --reason 'bounded exposure'"
		q, err := channel.Ask(channel.AskRequest{RepoRoot: bed.root(), Goal: bedGoal, Kind: "stop", Machine: "machine", Facts: []string{"finding remains unresolved"}, Now: time.Unix(10, 0), UnitStop: &channel.UnitStopQuestion{Loop: "unit-round", Subject: bedGoal + "/extra/run", Attempt: 3, Finding: finding, Review: "critic", Needs: command, AcceptableActs: []string{"goal-accept-risk", "work-revise", "goal-done"}}})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, q.ID)
		code, out, _ := bed.run(owners, "question", "show", "channel:"+q.ID)
		if code != 0 || !strings.Contains(strings.Join(strings.Fields(out), " "), command) {
			t.Fatalf("question show has no executable act: %d %q", code, out)
		}
	}
	code, _, stderr := bed.run(bed.owners(), "goal", "accept-risk", bedGoal, "--finding", "S-1", "--review", "critic", "--reason", "bounded exposure")
	if code != 0 || !strings.Contains(stderr, "Impact: accepting this finding's risk") {
		t.Fatalf("risk act: %d %q", code, stderr)
	}
	for i, id := range ids {
		q, err := channel.ReadQuestion(bed.root(), id)
		want := "open"
		if i == 0 {
			want = "closed"
		}
		if err != nil || q.State != want {
			t.Fatalf("finding %d question: %+v %v", i+1, q, err)
		}
	}
	impacts, err := filepath.Glob(filepath.Join(bed.root(), "artifacts", "agents", "channel", "unit-stop-overrides", "*.json"))
	if err != nil || len(impacts) != 1 {
		t.Fatalf("prior impact records: %v %v", impacts, err)
	}
}

func TestIntentGoalDoneRecordsImpactBeforeClosingUnitAsks(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	_, reader := enrollGoalSyncTerminal(t, bed.root(), "ttys:fixture_unit_done")
	bed.facts.reader = &reader
	q, err := channel.Ask(channel.AskRequest{RepoRoot: bed.root(), Goal: bedGoal, Kind: "stop", Machine: "machine", Facts: []string{"extra work remains unresolved"}, Now: time.Unix(10, 0), UnitStop: &channel.UnitStopQuestion{Loop: "unit-round", Subject: bedGoal + "/extra/run", Attempt: 3, Finding: "read:1", Review: "critic", Needs: "metasystem goal done " + bedGoal + " --reason 'conclude explicitly'", AcceptableActs: []string{"goal-done"}}})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	stderr := &unitImpactWriter{observe: func(text string) {
		if !strings.Contains(text, "Impact:") {
			return
		}
		if bed.goalFile(bedGoal).State == goal.StateDone {
			t.Error("impact was printed after conclusion")
		}
		paths, _ := filepath.Glob(filepath.Join(bed.root(), "artifacts", "agents", "channel", "unit-stop-overrides", "*.json"))
		if len(paths) != 1 {
			t.Errorf("impact was printed before being recorded: %v", paths)
		}
	}}
	command, rest, ok := resolveIntentArgv([]string{"goal", "done", bedGoal, "--reason", "conclude explicitly"})
	if !ok {
		t.Fatal("public goal done command is missing")
	}
	code := runIntentIn(command, rest, &out, stderr, bed.root(), bed.owners())
	if code != 0 || !strings.Contains(stderr.String(), "Impact:") {
		t.Fatalf("goal done: %d stdout=%q stderr=%q", code, out.String(), stderr.String())
	}
	stored, err := channel.ReadQuestion(bed.root(), q.ID)
	if err != nil || stored.State != "closed" {
		t.Fatalf("successful conclusion did not close the unit ask: %+v %v", stored, err)
	}
	impacts, err := filepath.Glob(filepath.Join(bed.root(), "artifacts", "agents", "channel", "unit-stop-overrides", "*.json"))
	if err != nil || len(impacts) != 1 {
		t.Fatalf("conclusion impact records: %v %v", impacts, err)
	}
}
