package main

import (
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
)

func TestDesignItemsBindBuildReadDoneAndLand(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"accepted", "head edit", "body edit", "missing marker", "missing marker changed body other unit", "missing exit", "missing item", "entirely transferred", "other unit", "clone", "newer acceptance", "title edit", "line endings", "space edit"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := newDesignGateBed(t, 2)
			bed.lineage = "builder"
			bed.workOwnersHook = func(owners *intentWorkOwners) {
				original := owners.git
				owners.git = func(dir string, args ...string) ([]byte, error) {
					if strings.Join(args, " ") == "rev-parse base-commit^{tree}" {
						return []byte(strings.Repeat("b", 40)), nil
					}
					if len(args) == 2 && args[0] == "show" && strings.HasSuffix(args[1], ":metasystem.conf") {
						return []byte("proof.cheap=true\nproof.audits=true\nproof.deadline=15\n"), nil
					}
					return original(dir, args...)
				}
			}
			setDesignGateMode(t, bed, "refuse")
			path, data := designGatePage(t, bed, "- Critique: closed at round 2 on 1 material finding folded as 1 unit acceptance item (convergence exit-one)")
			data = []byte(strings.Replace(string(data), "\nBuild the gate.", "\n\nBuild the gate.", 1))
			body, err := project.DesignBodyDigest(path, data)
			if err != nil {
				record, problems, _ := project.ParseRecord(path, string(data))
				t.Fatalf("%v: %+v %+v", err, record, problems)
			}
			file := bed.goalFile(bed.id)
			file.DesignExits = []goal.DesignExit{{Operation: "exit-one", DesignID: "gate-design", BodySHA256: body, Units: []string{"u"}, Items: []string{"design-read:M1"}}}
			file.ReviewObligations = []goal.ReviewObligation{{Finding: "design-read:M1", Chain: "design-read", Artifact: "gate.go", Test: "TestPublicGate", Fixture: "group:gate", State: "open", DesignItem: &goal.DesignItem{Exit: "exit-one", DesignID: "gate-design", BodySHA256: body, Unit: "u", Decision: "6", Tests: []string{"gate/TestPublicGate"}}}}
			unit := "u"
			switch scenario {
			case "head edit":
				data = []byte(strings.Replace(string(data), "- Id: gate-design", "- Id: gate-design\n- By: Wido", 1))
			case "title edit":
				data = []byte(strings.Replace(string(data), "# Gate design", "# Renamed design", 1))
			case "line endings":
				data = []byte(strings.ReplaceAll(string(data), "\n", "\r\n"))
			case "space edit":
				data = []byte(strings.Replace(string(data), "Build the gate.", "Build the gate. ", 1))
			case "body edit":
				data = append(data, []byte("A requirement outside the examined section.\n")...)
			case "missing marker", "missing marker changed body other unit":
				data = []byte(strings.Replace(string(data), " (convergence exit-one)", "", 1))
				if scenario == "missing marker changed body other unit" {
					data = append(data, []byte("A requirement outside the examined section.\n")...)
					unit = "other"
				}
			case "missing exit":
				file.DesignExits = nil
			case "missing item":
				file.ReviewObligations = nil
			case "entirely transferred":
				file.DesignExits[0].Units = nil
			case "other unit":
				unit = "other"
			case "newer acceptance":
				file.DesignExits = append(file.DesignExits, goal.DesignExit{Operation: "exit-two", DesignID: "gate-design", BodySHA256: body, Units: []string{"u"}})
			case "clone":
				rendered := goal.RenderFile(file)
				parsed, errs := goal.ParseFile(rendered)
				if len(errs) != 0 {
					t.Fatalf("clone reload: %v", errs)
				}
				file = parsed
				hook := bed.workOwnersHook
				bed = newDesignGateBed(t, 2)
				bed.lineage = "builder"
				bed.workOwnersHook = hook
				setDesignGateMode(t, bed, "refuse")
				path, _ = designGatePage(t, bed, "- Critique: closed at round 2 on 1 material finding folded as 1 unit acceptance item (convergence exit-one)")
				bed.designGate.chains = nil
			}
			bed.addGoal(file)
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			brief := bed.brief("items.md", "Read each round: yes\nBuild the required gate.\n")
			code, result, output := bed.work("work", "build", bed.id, unit, "--brief", brief, "--lines", "40", "--read-tool-calls", "12")
			good := scenario == "accepted" || scenario == "head edit" || scenario == "clone"
			if !good {
				if code != 1 || result.Outcome != intentRefused || len(bed.starter.launched()) != 0 || !strings.Contains(result.Summary, "nothing was built") {
					t.Fatalf("admission: %d %+v %s", code, result, output)
				}
				return
			}
			if code != 0 {
				t.Fatalf("build: %d %+v %s", code, result, output)
			}
			plan, err := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
			if err != nil {
				t.Fatal(err)
			}
			bound, err := os.ReadFile(plan.Build.Brief)
			if err != nil || !strings.Contains(string(bound), "design-read:M1") || !strings.Contains(string(bound), body) || !strings.Contains(string(bound), "gate/TestPublicGate") {
				t.Fatalf("build dropped items: %s %v", bound, err)
			}
			read, err := os.ReadFile(plan.Read.Brief)
			if err != nil || !strings.Contains(string(read), "coversFindings") {
				t.Fatalf("read dropped requirement: %s %v", read, err)
			}
			identity, _ := bed.designGate.identity(stateroottest.Installation(t, bed.stateRoot()))
			_, recorded := designGateRead(t, bed, identity, unit)
			if !strings.Contains(recorded.Designs[0].Acceptance, "gate/TestPublicGate") || recorded.Designs[0].BodySHA256 != body {
				t.Fatalf("landing binding: %+v", recorded)
			}
			watchWriteJSON(t, bed.stateRoot(), "artifacts/agents/jobs/design-read.json", map[string]any{"jobId": "design-read", "role": "design-critic", "goalId": bed.id, "status": "completed", "round": 1})
			bed.lineage = file.Claimed.Lineage
			reviewCode, reviewed, _ := bed.work("work", "review", bed.id, "--review", "design-read", "--finding", "design-read:M1", "--test", "TestPublicGate")
			if reviewCode != 1 || !strings.Contains(resultWords(reviewed), "can't be closed by this test run") || bed.goalFile(bed.id).ReviewObligations[0].State != "open" {
				t.Fatalf("text review minted proof: %d %+v", reviewCode, reviewed)
			}
			doneCode, done, _ := bed.work("goal", "done", bed.id, "--reason", "Finished.", "--lineage", bed.lineage)
			if doneCode != 1 || !strings.Contains(resultWords(done), "open review obligation") || bed.goalFile(bed.id).State != goal.StateClaimed {
				t.Fatalf("done ignored required item: %d %+v", doneCode, done)
			}

		})
	}
	t.Run("completion", func(t *testing.T) {
		t.Parallel()
		b := newDeliveryBedWith(t, nil, true)
		o := goal.ReviewObligation{Finding: "design-read:M1", Chain: "design-read", Artifact: "gate spec.go", Test: "TestPublicGate", Fixture: "group:gate", State: "open"}
		b.owners.branchState = func(string, string) (intentBranchState, error) {
			state := readBranch(1, "critic")
			state.Status.ReviewObligations = []goal.ReviewObligation{o}
			state.ReadsWaived = true
			return state, nil
		}
		code, result := b.do("work", "land", "standing-validation")
		if code != 1 || !strings.Contains(result.Summary, "required design finding") || result.Outcome != intentRefused || result.Next == nil || len(result.Next.Argv) != 16 || result.Next.Argv[11] != o.Artifact {
			t.Fatalf("land accepted open requirement: %d %+v", code, result)
		}
	})
}

func TestDesignItemsAllowRecordsHandInButRefuseCode(t *testing.T) {
	t.Parallel()
	item := goal.ReviewObligation{Finding: "design-read:M1", Chain: "design-read", Artifact: "gate spec.go", Test: "TestPublicGate", Fixture: "group:gate", State: "open", DesignItem: &goal.DesignItem{Exit: "exit-one", DesignID: "gate-design", BodySHA256: strings.Repeat("b", 64), Unit: "u", Decision: "6", Tests: []string{"gate/TestPublicGate"}}}
	t.Run("records", func(t *testing.T) {
		t.Parallel()
		b := newRecordsBed(t)
		readState := b.owners.delivery.branchState
		b.owners.delivery.branchState = func(root, id string) (intentBranchState, error) {
			state, err := readState(root, id)
			state.Status.ReviewObligations = []goal.ReviewObligation{item}
			return state, err
		}
		code, result := b.land(recordsPath)
		entries, err := plain.Entries(b.lane)
		if code != 0 || result.Outcome != intentConfirmed || len(b.requests) != 1 || len(b.pushes) != 1 || err != nil || len(entries) != 1 || !entries[0].Records || entries[0].SHA != b.published {
			t.Fatalf("records with an open design item: %d %+v entries=%+v err=%v", code, result, entries, err)
		}
	})
	t.Run("code", func(t *testing.T) {
		t.Parallel()
		b := newDeliveryBedWith(t, nil, true)
		b.owners.branchState = func(string, string) (intentBranchState, error) {
			state := readBranch(1, "critic")
			state.Status.ReviewObligations = []goal.ReviewObligation{item}
			state.ReadsWaived = true
			return state, nil
		}
		code, result := b.do("work", "land", "standing-validation")
		if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "required design finding") || len(b.calls) != 0 {
			t.Fatalf("code with an open design item: %d %+v calls=%v", code, result, b.calls)
		}
	})
}
