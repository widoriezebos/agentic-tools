package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/deploy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func landedHolderWithDeploy(t *testing.T, lineage ...string) (*deliveryBed, func(...deploy.Line), string) {
	t.Helper()
	b, owners, _ := holdingBoundaryBed(t)
	if len(lineage) > 0 {
		b.lineage = lineage[0]
		file := b.goalFile("standing-validation")
		file.Claimed.Lineage = b.lineage
		waitingToLandBed(file)
		last := &file.History[len(file.History)-1]
		last.Actor, last.Opid = "mac-cli+"+b.lineage, goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FRV", "mac-cli", b.lineage)
		file.Landing.Opid = last.Opid
		b.addGoal(file)
	}
	b.owners.recordLanded = productionRecordLanded
	code, result := b.do("work", "land", "standing-validation", "--whole", "--delivered", "x")
	expectOutcome(t, "holder's end declaration", code, result, intentConfirmed)
	landing := owners.status.BranchTip
	owners.status.EndpointTip = landing
	owners.status.Status, owners.status.Sources = branch.Status{}, nil
	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "holder records landing", code, result, intentUnchanged)
	file := b.goalFile("standing-validation")
	if file.History[len(file.History)-1].Verb != goal.LandedVerb {
		t.Fatalf("landing unrecorded: %+v", result)
	}
	home, origin := t.TempDir(), "https://example.test/team/project.git"
	key, err := deploy.ProjectKey(origin)
	if err != nil {
		t.Fatal(err)
	}
	b.writeFile(filepath.Join(b.root(), "deploy.json"), `{"schema":1,"adapter":{"argv":["fixture-adapter"]}}`)
	laneInputs := b.laneInputs
	b.laneInputs = func(owners *intentOwners) {
		laneInputs(owners)
		owners.deploy = deployOwners{home: func() (string, error) { return home, nil }, origin: func(string) (string, error) { return origin, nil }, now: b.owners.now}
	}
	old := strings.Repeat("0", 40)
	b.owners.laneContains = func(sha, main string) (bool, error) { return sha == main || sha == landing && main != old, nil }
	write := func(lines ...deploy.Line) {
		t.Helper()
		var record strings.Builder
		for _, line := range lines {
			line.Kind, line.StartedAt, line.EndedAt = deploy.KindDeploy, b.owners.now(), b.owners.now()
			data, err := json.Marshal(line)
			if err != nil {
				t.Fatal(err)
			}
			record.Write(data)
			record.WriteByte('\n')
		}
		b.writeFile(filepath.Join(deploy.Dir(home, key), "deploys.jsonl"), record.String())
	}
	return b, write, landing
}

func TestTheSeatsDoneWaitsForTheDeploy(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"pending", "failed", "successful", "person"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b, write, landing := landedHolderWithDeploy(t)
			old := deploy.Line{Commit: strings.Repeat("0", 40), Outcome: deploy.OutcomeActive}
			write(old)
			switch state {
			case "failed":
				write(old, deploy.Line{Commit: landing, Outcome: deploy.OutcomeBuildFailed})
			case "successful":
				write(old, deploy.Line{Commit: strings.Repeat("9", 40), Outcome: deploy.OutcomeActive})
			}
			args := []string{"goal", "done", "standing-validation", "--reason", "r"}
			if state == "person" {
				b.lineage = ""
				_, reader := enrollGoalSyncTerminal(t, b.root(), "ttys:fixture_deploy_done")
				b.facts.reader = &reader
				laneInputs := b.laneInputs
				b.laneInputs = func(owners *intentOwners) {
					laneInputs(owners)
					owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
						return humanauthority.Prove(root, reader.exact.Pid, reader, at)
					}
				}
				args = append(args, "--by", "Wido")
			}
			code, result := b.do(args...)
			if state == "successful" || state == "person" {
				expectOutcome(t, state, code, result, intentConfirmed)
				if b.goalFile("standing-validation").State != goal.StateDone {
					t.Fatal("conclusion left the goal live")
				}
				return
			}
			expectOutcome(t, state, code, result, intentRefused)
			want := "not deployed yet"
			if state == "failed" {
				want = "its deploy failed (build-failed of " + landing + ")"
			}
			if !strings.Contains(result.Summary, want) || result.Next == nil || strings.Join(result.Next.Argv, " ") != "metasystem deploy status" || b.goalFile("standing-validation").State != goal.StateClaimed {
				t.Fatalf("deploy refusal: %+v", result)
			}
		})
	}
}

func TestTheSeatFinishesAGoalWithNoPerson(t *testing.T) {
	t.Parallel()
	b, write, landing := landedHolderWithDeploy(t)
	write(deploy.Line{Commit: landing, Outcome: deploy.OutcomeActive})
	code, result := b.do("goal", "done", "standing-validation", "--reason", "r")
	expectOutcome(t, "holder concludes deployed goal", code, result, intentConfirmed)
	file := b.goalFile("standing-validation")
	last := file.History[len(file.History)-1]
	if file.State != goal.StateDone || last.Verb != "done" || last.Actor != "mac-cli+agent-session" {
		t.Fatalf("goal not concluded by its holder: %+v %+v", file, last)
	}
}

func TestALandedGoalIsConcludedAfterItsDeploy(t *testing.T) {
	t.Parallel()
	seatLineage := func() string {
		for _, entry := range launch.SeatEnvironment() {
			if lineage, ok := strings.CutPrefix(entry, "METASYSTEM_OWNER_LINEAGE="); ok {
				return lineage
			}
		}
		t.Fatal("the seat launch supplies no owner lineage")
		return ""
	}
	b, write, landing := landedHolderWithDeploy(t, seatLineage())
	old := deploy.Line{Commit: strings.Repeat("0", 40), Outcome: deploy.OutcomeActive}
	write(old)
	stop := func(session string) string {
		t.Helper()
		owners := b.deliveryOwners()
		b.laneInputs(&owners)
		endpoint, err := owners.dependencies.endpoint(b.root())
		if err != nil {
			t.Fatal(err)
		}
		steps := goal.HolderStepsDue([]*goal.GoalFile{b.goalFile("standing-validation")}, goal.GateSettings{}, b.owners.now())
		if len(steps) != 1 || !steps[0].Conclude || steps[0].Revise {
			t.Fatalf("the holder has no completion step: %+v", steps)
		}
		store := &goal.Store{Root: b.root(), Now: b.owners.now, TakeHolderStep: holderStepTaker(b.root(), owners)}
		verdict, err := store.TurnVerdictAtEndpoint(endpoint, "mac-cli", goal.ScanResult{}, session, "", "main-1",
			goal.TurnVerdictOptions{SeatActor: goal.Actor{Machine: "mac-cli", Lineage: b.lineage}})
		if err != nil {
			t.Fatal(err)
		}
		t.Log(verdict.Display)
		return verdict.Display
	}
	if display := stop("seat-before-deploy"); !strings.Contains(display, "CONCLUSION WAITS standing-validation:") || !strings.Contains(display, "not deployed yet") || b.goalFile("standing-validation").State != goal.StateClaimed {
		t.Fatalf("pending deploy did not keep the goal live: %s", display)
	}
	// A new seat gets the same owner lineage from the launch environment.
	b.lineage = seatLineage()
	write(old, deploy.Line{Commit: landing, Outcome: deploy.OutcomeBuildFailed})
	for _, session := range []string{"successor-failed-deploy", "successor-failed-deploy-again"} {
		if display := stop(session); !strings.Contains(display, "CONCLUSION WAITS standing-validation:") || !strings.Contains(display, "its deploy failed") || b.goalFile("standing-validation").State != goal.StateClaimed {
			t.Fatalf("failed deploy did not keep the step due: %s", display)
		}
	}
	write(old, deploy.Line{Commit: landing, Outcome: deploy.OutcomeActive})
	if display := stop("successor-deployed"); !strings.Contains(display, "CONCLUDED standing-validation") {
		t.Fatalf("the successor did not conclude: %s", display)
	}
	file := b.goalFile("standing-validation")
	last := file.History[len(file.History)-1]
	if file.State != goal.StateDone || last.Verb != "done" || last.Actor != "mac-cli+"+seatLineage() {
		t.Fatalf("the successor did not conclude as the holder: %+v %+v", file, last)
	}
	if steps := goal.HolderStepsDue([]*goal.GoalFile{file}, goal.GateSettings{}, b.owners.now()); len(steps) != 0 {
		t.Fatalf("the concluded goal still has a due step: %+v", steps)
	}
}
