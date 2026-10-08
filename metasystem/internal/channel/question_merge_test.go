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
