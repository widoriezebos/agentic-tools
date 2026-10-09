package channel

import (
	"strings"
	"testing"
	"time"
)

func TestUnitStopAndProcessQuestionsRetainDistinctRemedies(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	request := AskRequest{RepoRoot: root, Goal: "g", Kind: "stop", Machine: "m", Now: time.Unix(10, 0),
		ProcessAct: "process-choice", Wants: "metasystem settings apply process-choice --by Wido", Facts: []string{"The process choice needs approval"}}
	process, err := Ask(request)
	if err != nil {
		t.Fatal(err)
	}
	unit, err := Ask(unitStopAskFixture(root, "read:1"))
	if err != nil {
		t.Fatal(err)
	}
	process, err = ReadQuestion(root, process.ID)
	if err != nil || process.ProcessAct != request.ProcessAct || process.UnitStop != nil || !strings.Contains(ReplyInstructions(process), request.Wants+"; applying this exact process act closes the question.") {
		t.Fatalf("process question lost its binding or remedy: %+v %v", process, err)
	}
	unit, err = ReadQuestion(root, unit.ID)
	if err != nil || unit.ProcessAct != "" || unit.UnitStop == nil || !strings.Contains(ReplyInstructions(unit), unit.UnitStop.Needs+"; a successful matching act or unit closure closes this question.") {
		t.Fatalf("unit question lost its binding or remedy: %+v %v", unit, err)
	}
	if unit.ID == process.ID {
		t.Fatal("a unit stop joined a process question")
	}
}

func TestUnitAndLaneStopQuestionsKeepIndependentClosure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	unit, err := Ask(unitStopAskFixture(root, "read:1"))
	if err != nil {
		t.Fatal(err)
	}
	lane, err := Ask(AskRequest{RepoRoot: root, About: "lane", Kind: "other", Machine: "m", Lineage: "landing-agent", Now: time.Unix(10, 0),
		Facts: []string{"metasystem landing prove --trunk", "lane stop: {}", "evidence: proof"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := Close(root, lane.ID, "matching proof admitted", nil, DestinationConfig{}); err != nil {
		t.Fatal(err)
	}
	lane, err = ReadQuestion(root, lane.ID)
	if err != nil || lane.State != "closed" || lane.ClosedBecause != "matching proof admitted" || ActCommand(lane) != "metasystem landing prove --trunk" {
		t.Fatalf("lane closure lost its reason or command: %+v %v", lane, err)
	}
	unit, err = ReadQuestion(root, unit.ID)
	if err != nil || unit.State != "open" || unit.UnitStop == nil || unit.UnitStop.Finding != "read:1" || ActCommand(unit) != unit.UnitStop.Needs {
		t.Fatalf("lane closure changed the unit stop: %+v %v", unit, err)
	}
	if err := RecordUnitStopAct(root, UnitStopAct{ID: "act", Goal: "g", Loop: "unit-round", Subject: "g/u/run", Attempt: 2,
		Findings: []string{"read:1"}, Kind: "goal-accept-risk", Reason: "local exposure", At: time.Unix(11, 0)}); err != nil {
		t.Fatal(err)
	}
	unit, err = ReadQuestion(root, unit.ID)
	if err != nil || unit.State != "closed" || unit.UnitStop == nil {
		t.Fatalf("matching act did not close the retained unit stop: %+v %v", unit, err)
	}
	lane, err = ReadQuestion(root, lane.ID)
	if err != nil || lane.ClosedBecause != "matching proof admitted" {
		t.Fatalf("unit act changed lane closure: %+v %v", lane, err)
	}
}
