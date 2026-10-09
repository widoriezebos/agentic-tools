package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func writeProcessRootJSON(t *testing.T, path string, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
}

func separatedProcessRun(t *testing.T) (*workBed, *intentInvocation, launch.UnitRunRecord) {
	t.Helper()
	b := newWorkBed(t)
	installation := t.TempDir()
	installationShapeAt(t, installation)
	now, err := b.commandNow(b.root())
	if err != nil {
		t.Fatal(err)
	}
	b.manager.Now = func() time.Time { return now.Add(10 * time.Minute) }
	plan := filepath.Join(b.unitRoot, "separated", "plan.json")
	writeProcessRootJSON(t, plan, launch.UnitPlan{Estimate: &launch.UnitEstimate{ElapsedMinutes: 1}})
	record := launch.UnitRunRecord{ID: "separated", Goal: b.id, Unit: "evidence", Plan: plan,
		Rounds: []launch.UnitRound{{Number: 1, Steps: []launch.UnitStep{{LaunchID: "separated-build", Kind: "build", State: launch.StepPassed,
			StartedAt: now.Format(time.RFC3339Nano), FinishedAt: now.Add(5 * time.Minute).Format(time.RFC3339Nano)}}}}}
	if err := b.manager.Store.Create(launch.Record{ID: "separated-build", Goal: b.id, State: launch.Completed,
		StartedAt: now.Format(time.RFC3339Nano), FinishedAt: now.Add(5 * time.Minute).Format(time.RFC3339Nano)}); err != nil {
		t.Fatal(err)
	}
	writeProcessRootJSON(t, filepath.Join(b.unitRoot, record.ID, "run.json"), record)
	owners := b.workOwners()
	endpoint := owners.dependencies.endpoint
	owners.dependencies.endpoint = func(string) (goal.Endpoint, error) { return endpoint(b.root()) }
	inv := &intentInvocation{cwd: b.root(), stateRoot: b.root(), owners: owners,
		layout: stateroot.Layout{GitRoot: b.root(), InstallationRoot: stateroot.Installation(installation)}}
	return b, inv, record
}

func TestUnitMeasureInputReadsQuestionsFromInstallation(t *testing.T) {
	t.Parallel()
	b, inv, record := separatedProcessRun(t)
	start, _ := time.Parse(time.RFC3339Nano, record.Rounds[0].Steps[0].StartedAt)
	question := channel.Question{ID: "person-wait", Goal: b.id, State: "answered", OpenedAt: start.Add(time.Minute),
		Answer: &channel.Answer{At: start.Add(3 * time.Minute)}}
	writeProcessRootJSON(t, filepath.Join(inv.layout.InstallationRoot.Path(), "artifacts", "agents", "channel", "questions", question.ID+".json"), question)
	question.ID, question.OpenedAt = "wrong-root", start.Add(2*time.Minute)
	writeProcessRootJSON(t, filepath.Join(b.root(), "artifacts", "agents", "channel", "questions", question.ID+".json"), question)
	input := inv.unitMeasureInput(launch.NamedWork{Unit: record.Unit, Run: record.ID, Record: &record})
	if len(input.Unknown) != 0 || len(input.Questions) != 1 || !input.Questions[0].Start.Equal(start.Add(time.Minute)) || !input.Questions[0].End.Equal(start.Add(3*time.Minute)) {
		t.Fatalf("person wait did not come from the installation: %+v", input)
	}
}

func TestGoalUnitStatusSeparatesProcessHistoryAndQuestions(t *testing.T) {
	t.Parallel()
	b, inv, _ := separatedProcessRun(t)
	act := processchange.ProcessAct{ID: "project-act", Goal: b.id, Actor: "agent", Key: "launch.read.model", After: "selected-model", AppliedAt: b.manager.Now()}
	writeProcessRootJSON(t, filepath.Join(b.root(), "process", "acts", act.ID+".json"), act)
	broken := filepath.Join(inv.layout.InstallationRoot.Path(), "artifacts", "agents", "channel", "questions", "broken.json")
	writeProcessRootJSON(t, broken, "unreadable question")
	var stdout, stderr bytes.Buffer
	inv.stdout, inv.stderr = &stdout, &stderr
	result := intentResult{Outcome: intentConfirmed, Summary: "goal status", Data: map[string]any{"goal": b.id}}
	if code := inv.renderGoalUnitStatus(result, 0); code != 0 {
		t.Fatalf("status exit %d: %s", code, stderr.String())
	}
	for _, want := range []string{"Process act project-act:", broken} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("status omitted evidence %q: %s", want, stdout.String())
		}
	}
}

func TestWorkRunStatusUsesInstallationClockAndQuestionEvidence(t *testing.T) {
	t.Parallel()
	b := newAdoptedBed(t)
	var err error
	b.installation, err = filepath.EvalSymlinks(b.installation)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	clockCalls := 0
	b.owners.commandNow = func(root string) (time.Time, error) {
		clockCalls++
		if root != b.installation {
			return time.Time{}, errors.New("clock read from the project instead of the installation")
		}
		return now, nil
	}
	record := launch.UnitRunRecord{ID: "separated", Goal: "example", Unit: "evidence", Rounds: []launch.UnitRound{{Number: 1, Steps: []launch.UnitStep{{Kind: "build", State: launch.StepPassed,
		StartedAt: now.Add(-5 * time.Minute).Format(time.RFC3339Nano), FinishedAt: now.Format(time.RFC3339Nano)}}}}}
	writeProcessRootJSON(t, filepath.Join(filepath.Dir(b.launchDir), "unit", record.ID, "run.json"), record)
	act := processchange.ProcessAct{ID: "project-act", Goal: record.Goal, Actor: "agent", AppliedAt: now}
	writeProcessRootJSON(t, filepath.Join(b.app, "process", "acts", act.ID+".json"), act)
	broken := filepath.Join(b.installation, "artifacts", "agents", "channel", "questions", "broken.json")
	writeProcessRootJSON(t, broken, "unreadable question")
	code, result := b.run(b.installation, "work", "status", "run:"+record.ID)
	if code != 0 || clockCalls != 1 {
		t.Fatalf("run status clock: exit %d, calls %d, result %+v", code, clockCalls, result)
	}
	body, err := json.Marshal(result.Data)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Process act project-act:", broken} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("run status omitted %q: %s", want, body)
		}
	}
}

func TestProcessDriftKeepsStateAndAsksThroughInstallation(t *testing.T) {
	t.Parallel()
	b, inv, record := separatedProcessRun(t)
	if err := inv.observeProcessDrift(record); err != nil {
		t.Fatal(err)
	}
	state, err := processchange.ReadState(b.root(), b.id)
	if err != nil || len(state.Unknown) != 0 || len(state.Stops) != 1 || state.Stops[0].Question == "" {
		t.Fatalf("project process state: %+v, %v", state, err)
	}
	question, err := channel.ReadQuestion(inv.layout.InstallationRoot.Path(), state.Stops[0].Question)
	if err != nil || question.State != "open" || question.Goal != b.id {
		t.Fatalf("installation question: %+v, %v", question, err)
	}
	if _, err := channel.ReadQuestion(b.root(), question.ID); !os.IsNotExist(err) {
		t.Fatalf("question appeared in project state: %v", err)
	}
	if err := inv.observeProcessDrift(record); err != nil {
		t.Fatal(err)
	}
	questions, unknown := channel.WalkQuestions(inv.layout.InstallationRoot.Path())
	if len(questions) != 1 || len(unknown) != 0 {
		t.Fatalf("repeated observation duplicated the question: %+v, %v", questions, unknown)
	}
}

func TestDirectPersonProofUsesCheckoutAuthorityAndInstallationClock(t *testing.T) {
	t.Parallel()
	b := newGrantEverythingBed(t)
	installation := t.TempDir()
	now, err := b.commandNow(b.root())
	if err != nil {
		t.Fatal(err)
	}
	owners := b.owners()
	owners.commandNow = func(root string) (time.Time, error) {
		if root != installation {
			return time.Time{}, errors.New("clock read outside installation")
		}
		return now, nil
	}
	projectState := t.TempDir()
	inv := &intentInvocation{stateRoot: projectState, owners: owners,
		layout: stateroot.Layout{GitRoot: b.root(), InstallationRoot: stateroot.Installation(installation)}}
	if refusal := inv.directPersonProof("process setting"); refusal != nil {
		t.Fatalf("enrolled checkout person refused: %+v", refusal)
	}
	grantProof, err := humanauthority.HelmProof(b.root(), humanauthority.HelmGrant{By: "Wido", Grant: "attorney"}, now)
	if err != nil {
		t.Fatal(err)
	}
	owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return grantProof, nil
	}
	inv.owners = owners
	if refusal := inv.directPersonProof("process setting"); refusal == nil || refusal.Outcome != intentRefused {
		t.Fatalf("grant admitted a direct-person act: %+v", refusal)
	}
	body, err := os.ReadFile(humanauthority.AttorneyLogPath(installation))
	if err != nil || !strings.Contains(string(body), "refused grant=attorney") {
		t.Fatalf("installation refusal log: %s, %v", body, err)
	}
	if _, err := os.Stat(humanauthority.AttorneyLogPath(projectState)); !os.IsNotExist(err) {
		t.Fatalf("refusal logged under project state: %v", err)
	}
}
