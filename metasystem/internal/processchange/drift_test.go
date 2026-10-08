package processchange

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/roots"
)

func TestObserveRetainsStopWhenInstallationQuestionFails(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	installation := roots.Installation(filepath.Join(t.TempDir(), "installation"))
	if err := os.WriteFile(installation.Path(), []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	elapsed, estimate := 1.0, 1.0
	drift := processmeasure.Decide("example/unit/run", processmeasure.Measures{
		ObservedAt: time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC), ElapsedHours: &elapsed, EstimateMinutes: &estimate}, nil)
	drift.Stops[0].Cause = &loopstop.Cause{Kind: "process-change", Name: "original-act"}
	if err := Observe(project, installation, "example", "episode", drift); err == nil {
		t.Fatal("an unavailable installation admitted the question")
	}
	state, err := ReadState(project, "example")
	if err != nil || len(state.Stops) != 1 || state.Stops[0].Question != "" {
		t.Fatalf("failed question lost the retained stop: %+v, %v", state, err)
	}
	retainedID := state.Stops[0].ID
	if err := os.Remove(installation.Path()); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(installation.Path(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installation.Path("metasystem.conf"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Observe(project, installation, "example", "episode", drift); err != nil {
		t.Fatal(err)
	}
	state, err = ReadState(project, "example")
	if err != nil || len(state.Stops) != 1 || state.Stops[0].ID != retainedID || state.Stops[0].Question == "" {
		t.Fatalf("retry replaced the retained stop: %+v, %v", state, err)
	}
	questions, unknown := channel.WalkQuestions(installation.Path())
	if len(questions) != 1 || len(unknown) != 0 || questions[0].ID != state.Stops[0].Question {
		t.Fatalf("retry did not bind the installation question: %+v, %v", questions, unknown)
	}
	if questions, unknown := channel.WalkQuestions(project); len(questions) != 0 || len(unknown) != 0 {
		t.Fatalf("question written into project state: %+v, %v", questions, unknown)
	}
	if err := resolveUndo(project, installation, ProcessAct{ID: "undo-act", Goal: "example", Undo: "original-act"}); err != nil {
		t.Fatal(err)
	}
	question, err := channel.ReadQuestion(installation.Path(), state.Stops[0].Question)
	if err != nil || question.State != "closed" {
		t.Fatalf("undo did not close the installation question: %+v, %v", question, err)
	}
}
