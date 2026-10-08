package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func writeRunRootJSON(t *testing.T, path string, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, path, body, 0600)
}

func TestContextBoundaryStatusReadsInstallation(t *testing.T) {
	t.Parallel()
	checkout, installation := separatedContextRoots(t)
	installation, err := filepath.EvalSymlinks(installation)
	if err != nil {
		t.Fatal(err)
	}
	event := steward.UnitBoundary{Seat: installation, Session: "session", Goal: "goal", Unit: "unit/1", Outcome: "green"}
	writeRunRootJSON(t, filepath.Join(installation, "artifacts", "agents", "steward", "unit-boundaries.json"), []steward.UnitBoundary{event})
	event.Seat, event.Unit = checkout, "wrong-root/1"
	writeRunRootJSON(t, filepath.Join(checkout, "artifacts", "agents", "steward", "unit-boundaries.json"), []steward.UnitBoundary{event})
	var stdout, stderr bytes.Buffer
	code := runContextStatus([]string{"--root", checkout, "--runtime", "codex", "--session", "session", "--json"}, &stdout, &stderr)
	var output contextStatusOutput
	if code != 0 || json.Unmarshal(stdout.Bytes(), &output) != nil || output.BoundaryError != "" || len(output.Boundaries) != 1 || output.Boundaries[0].Unit != "unit/1" {
		t.Fatalf("installation boundary status: exit %d, output %s, error %s", code, stdout.String(), stderr.String())
	}
	writeTestingFixtureFile(t, filepath.Join(installation, "artifacts", "agents", "steward", "unit-boundaries.json"), []byte("broken"), 0600)
	stdout.Reset()
	code = runContextStatus([]string{"--root", checkout, "--runtime", "codex", "--session", "session", "--json"}, &stdout, &stderr)
	if code != 0 || json.Unmarshal(stdout.Bytes(), &output) != nil || output.BoundaryError == "" {
		t.Fatalf("unreadable installation boundary was hidden: exit %d, output %s", code, stdout.String())
	}
}

func TestGoalBudgetViewsReadInstallation(t *testing.T) {
	t.Parallel()
	for _, claimed := range []bool{true, false} {
		t.Run(map[bool]string{true: "claim", false: "episode"}[claimed], func(t *testing.T) {
			t.Parallel()
			b := newIntentBed(t, false, nil)
			installation := filepath.Join(b.root(), "tools", "metasystem")
			installationShapeAt(t, installation)
			installation, err := filepath.EvalSymlinks(installation)
			if err != nil {
				t.Fatal(err)
			}
			file := b.goalFile("standing-validation")
			if !claimed {
				file.State, file.Claimed = goal.StateApproved, nil
				b.addGoal(file)
			}
			// Corrupt run evidence belongs to the installation and must make
			// spending unknown even when the project's directory is empty.
			writeTestingFixtureFile(t, filepath.Join(installation, "artifacts", "agents", "jobs", "broken.json"), []byte("broken"), 0600)
			owners := b.owners()
			endpoint := owners.dependencies.endpoint
			owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
				if root != installation {
					t.Fatalf("ledger endpoint received %q, want installation %q", root, installation)
				}
				return endpoint(b.root())
			}
			for _, verb := range []string{"show", "budget"} {
				code, result := b.runJSON(owners, "goal", verb, file.Id, "--repo", installation)
				if code != 0 {
					t.Fatalf("%s exit %d: %+v", verb, code, result)
				}
				body, _ := json.Marshal(result.Data)
				var data struct {
					Budget intentBudgetView `json:"budget"`
				}
				if err := json.Unmarshal(body, &data); err != nil {
					t.Fatal(err)
				}
				requireInstallationBudgetUnknown(t, data.Budget)
			}
			var stdout bytes.Buffer
			inv := &intentInvocation{stateRoot: b.root(), owners: owners, stdout: &stdout, stderr: io.Discard,
				layout: stateroot.Layout{GitRoot: b.root(), InstallationRoot: stateroot.Installation(installation)}}
			after := inv.afterGoalAct(file.Id, "approve")
			requireInstallationBudgetUnknown(t, after.Data.(map[string]any)["budget"].(intentBudgetView))
			result := intentResult{Outcome: intentConfirmed, Targets: inv.targets(file.Id), Summary: "goal status", Data: map[string]any{"goal": file.Id}}
			if code := inv.renderGoalUnitStatus(result, 0); code != 0 {
				t.Fatalf("status exit %d", code)
			}
			requireInstallationBudgetUnknown(t, result.Data.(map[string]any)["budget"].(intentBudgetView))
		})
	}
}

func requireInstallationBudgetUnknown(t *testing.T, view intentBudgetView) {
	t.Helper()
	if view.Projection == nil || view.Projection.Status != dispatchcore.BudgetUnknown || view.Projection.Unknown == nil || !strings.Contains(view.Projection.Unknown.Record, "broken.json") {
		t.Fatalf("installation's unreadable spending evidence was hidden: %+v", view.Projection)
	}
}

func TestDirectPersonCheckSeparatesAuthorityClockAndRefusalLog(t *testing.T) {
	t.Parallel()
	b := newGrantEverythingBed(t)
	installation, projectState := t.TempDir(), t.TempDir()
	now, err := b.commandNow(b.root())
	if err != nil {
		t.Fatal(err)
	}
	owners := b.owners()
	owners.commandNow = func(root string) (time.Time, error) {
		if root != installation {
			return time.Time{}, errors.New("clock read outside the installation")
		}
		return now, nil
	}
	inv := &intentInvocation{stateRoot: projectState, owners: owners,
		layout: stateroot.Layout{GitRoot: b.root(), InstallationRoot: stateroot.Installation(installation)}}
	if problem := inv.checkDirectPersonProof("host builds", false); problem != nil {
		t.Fatalf("checkout's enrolled person refused: %+v", problem)
	}
	proof, err := humanauthority.HelmProof(b.root(), humanauthority.HelmGrant{By: "Wido", Grant: "attorney"}, now)
	if err != nil {
		t.Fatal(err)
	}
	inv.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return proof, nil
	}
	if problem := inv.checkDirectPersonProof("host builds", false); problem == nil || problem.Outcome != intentRefused {
		t.Fatalf("grant admitted as the person: %+v", problem)
	}
	if _, err := os.Stat(humanauthority.AttorneyLogPath(installation)); !os.IsNotExist(err) {
		t.Fatalf("observation recorded an attorney refusal: %v", err)
	}
	if problem := inv.directPersonProof("host builds"); problem == nil || problem.Outcome != intentRefused {
		t.Fatalf("grant admitted a person-only act: %+v", problem)
	}
	body, err := os.ReadFile(humanauthority.AttorneyLogPath(installation))
	if err != nil || !strings.Contains(string(body), "refused grant=attorney") {
		t.Fatalf("installation refusal log: %s, %v", body, err)
	}
	if _, err := os.Stat(humanauthority.AttorneyLogPath(projectState)); !os.IsNotExist(err) || inv.stateRoot != projectState {
		t.Fatalf("person check changed project state: %v, root %q", err, inv.stateRoot)
	}
}

func TestBuildPersonQuestionUsesInstallation(t *testing.T) {
	t.Parallel()
	installation := t.TempDir()
	var gotRoot string
	inv := &intentInvocation{stateRoot: t.TempDir(), layout: stateroot.Layout{InstallationRoot: stateroot.Installation(installation)}}
	inv.owners.processes.ask = func(root string, input channelAskInput) (channel.Question, []string, int, error) {
		gotRoot = root
		if input.Goal != "goal" || input.Kind != "other" {
			t.Fatalf("question lost its subject: %+v", input)
		}
		return channel.Question{ID: "person-question"}, nil, 0, nil
	}
	result := inv.unitOutcome(nil, launch.UnitResult{Record: launch.UnitRunRecord{Goal: "goal"}}, &launch.CodedError{Code: "LAUNCH_BUILD_PERSON"}, inv.targets("goal"), []string{"metasystem", "work", "build", "goal"})
	if gotRoot != installation || result.Outcome != intentRefused || len(result.Targets) != 2 || result.Targets[1].ID != "person-question" {
		t.Fatalf("person question route: root %q, result %+v", gotRoot, result)
	}
}

func TestDirectPersonCheckInitializesGoalState(t *testing.T) {
	t.Parallel()
	b := newGrantEverythingBed(t)
	installation := filepath.Join(b.root(), "tools", "metasystem")
	installationShapeAt(t, installation)
	owners := b.owners()
	owners.commandNow = func(root string) (time.Time, error) {
		if root != installation {
			t.Fatalf("person check clock root %q, want %q", root, installation)
		}
		return b.commandNow(b.root())
	}
	inv := &intentInvocation{cwd: installation, owners: owners,
		layout: stateroot.Layout{GitRoot: b.root(), InstallationRoot: stateroot.Installation(installation)}}
	if problem := inv.checkDirectPersonProof("work build", false); problem != nil || inv.stateRoot != b.root() {
		t.Fatalf("person check left the goal state unavailable: root %q, problem %+v", inv.stateRoot, problem)
	}
}

func TestQuestionAskUsesInstallation(t *testing.T) {
	t.Parallel()
	b := newIntentBed(t, false, nil)
	installation := filepath.Join(b.root(), "tools", "metasystem")
	installationShapeAt(t, installation)
	installation, err := filepath.EvalSymlinks(installation)
	if err != nil {
		t.Fatal(err)
	}
	owners := b.owners()
	var gotRoot string
	owners.processes.ask = func(root string, input channelAskInput) (channel.Question, []string, int, error) {
		gotRoot = root
		return channel.Question{ID: "question", Goal: input.Goal, Thread: &channel.MessageRef{ThreadID: "thread", ID: "message"}}, nil, 0, nil
	}
	code, result := b.runJSON(owners, "question", "ask", "standing-validation", "--question", "Proceed?", "--option", "yes: proceed", "--repo", installation)
	if code != 0 || gotRoot != installation {
		t.Fatalf("question route: exit %d, root %q, result %+v", code, gotRoot, result)
	}
}
