package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func recoveryScope(t *testing.T, page string, rows ...goal.DesignUnitScope) string {
	t.Helper()
	for _, row := range rows {
		data, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		page += "Scope item: " + string(data) + "\n"
	}
	return page
}

// The public command keeps the real author-attempt, publication and build
// owners; only model execution and repository transport use fixture adapters.
func TestDesignReviewAutomaticFinalRevisionAndPartialSplit(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"partial", "person opened main", "all concrete", "all concrete dependent", "all concrete wrong owner", "dependent", "shared Decision", "nested Decision", "oversized", "six units", "goal-wide six", "commit failure", "opening failure", "lost commit response", "projection failure", "author display failure", "pending author", "failed author", "invalid proposal", "missing scope", "concurrent edit", "source revision moved", "held", "cut dependency", "cut ownership", "unknown dependency"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			allConcrete := strings.HasPrefix(scenario, "all concrete")
			units := acceptanceUnits + "| writer | Publish the result | 40 |\n"
			if scenario == "six units" {
				units += "| a | Third | 10 |\n| b | Fourth | 10 |\n| c | Fifth | 10 |\n| d | Sixth | 10 |\n"
			}
			if scenario == "oversized" {
				units = strings.Replace(units, "| writer | Publish the result | 40 |", "| writer | Publish the result | 300 |", 1)
			}
			b, dir, returned := designEvidenceBedConfigured(t, evidenceInventory, func(b *designLoopBed) {
				if scenario == "nested Decision" {
					b.writeFile(b.design, strings.Replace(string(mustRead(t, b.design)), "Publish the result.\n", "Publish the result.\n\n### Writer storage\nThe writer stores the result.\n", 1))
				}
				if scenario == "cut dependency" || scenario == "cut ownership" {
					page := string(mustRead(t, b.design)) + "\n## Acceptance items\n"
					reader := goal.DesignUnitScope{Unit: "reader", Decisions: []string{"## Collection:1"}, DependsOn: []string{"writer"}}
					if scenario == "cut ownership" {
						reader.Decisions, reader.DependsOn = []string{"## Collection:1", "## Publication"}, []string{}
					}
					page = recoveryScope(t, page, reader, goal.DesignUnitScope{Unit: "writer", Decisions: []string{"## Publication"}, DependsOn: []string{}})
					b.writeFile(b.design, page)
				}
			}, units)
			if err := os.MkdirAll(filepath.Join(b.root(), ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			source := b.goalFile(bedGoal)
			source.Origin = goal.OriginHuman
			if scenario == "person opened main" {
				source.Origin = goal.OriginMain
				source.History[0].Actor = "human:fixture-person"
			}
			b.addGoal(source)
			f1, f2 := evidenceFinding("F1", "## Collection:1", ""), evidenceFinding("F2", "## Publication", "")
			f2["change"] = "Choose the publication owner."
			findings, rigor := []any{f1, f2}, []any{evidenceRigor("F1"), evidenceRigor("F2")}
			if scenario == "oversized" || scenario == "six units" || scenario == "goal-wide six" {
				findings, rigor = findings[:1], rigor[:1]
			}
			returned["findings"], returned["rigor"], returned["verdictMaterialCount"] = findings, rigor, len(findings)
			b.writeJSON(filepath.Join(dir, "return.json"), returned)
			b.writeFile(filepath.Join(dir, "return.md"), fmt.Sprintf("VERDICT: REVISE material=%d\n", len(findings)))
			decisions := map[string]string{"F1": "accepted | concrete requirement | ## Collection:1"}
			if len(findings) > 1 {
				decisions["F2"] = "accepted | unresolved independent choice | ## Publication"
			}
			template := b.decide(b.review(), decisions)
			prior := string(mustRead(t, b.design))
			reader := goal.DesignUnitScope{Unit: "reader", Decisions: []string{"## Collection:1"}, DependsOn: []string{}}
			writer := goal.DesignUnitScope{Unit: "writer", Decisions: []string{"## Publication"}, DependsOn: []string{}}
			if scenario == "dependent" || scenario == "cut dependency" || scenario == "all concrete dependent" {
				reader.DependsOn = []string{"writer"}
			}
			if scenario == "shared Decision" {
				reader.Decisions = append(reader.Decisions, "## Publication")
			}
			if scenario == "unknown dependency" {
				reader.DependsOn = []string{"absent"}
			}
			if scenario == "all concrete wrong owner" {
				writer.Decisions = []string{"## Collection:1"}
			}
			if scenario == "cut dependency" {
				reader.DependsOn = []string{}
			}
			mappings := []designFoldMapping{concreteFoldMapping()}
			if len(findings) > 1 {
				m := designFoldMapping{Finding: "F2", Unit: "writer", Decision: "## Publication", Unresolved: true}
				if allConcrete {
					m = designFoldMapping{Finding: "F2", Unit: "writer", Decision: "## Publication", Passage: "Choose the publication owner. The goal transaction owns publication.", Checklist: "check the goal transaction", Fixture: "group:writer", Tests: []string{"writer/TestPublication"}}
				}
				mappings = append(mappings, m)
			}
			proposal := suppliedFoldPage(t, prior, mappings...)
			if scenario == "cut dependency" || scenario == "cut ownership" {
				var lines []string
				for _, line := range strings.Split(proposal, "\n") {
					if !strings.HasPrefix(line, "Scope item: ") {
						lines = append(lines, line)
					}
				}
				proposal = strings.Join(lines, "\n")
			}
			if allConcrete {
				proposal = strings.Replace(proposal, "Publish the result |", "Publish the result; writer/TestPublication; check the goal transaction |", 1)
			}
			proposal = recoveryScope(t, proposal, reader, writer)
			if scenario == "missing scope" {
				var lines []string
				for _, line := range strings.Split(proposal, "\n") {
					if !strings.HasPrefix(line, "Scope item: ") || !strings.Contains(line, `"Unit":"writer"`) {
						lines = append(lines, line)
					}
				}
				proposal = strings.Join(lines, "\n")
			}
			if scenario == "six units" {
				for _, unit := range []string{"a", "b", "c", "d"} {
					proposal = recoveryScope(t, proposal, goal.DesignUnitScope{Unit: unit, Decisions: []string{"## Publication"}, DependsOn: []string{}})
				}
			}
			if scenario == "goal-wide six" {
				other := strings.Replace(prior, "01DESIGNREADER", "other-design", 1)
				other = strings.Replace(other, units, "\n## Units\n\n| Unit | Production lines |\n| --- | --- |\n| a | 10 |\n| b | 10 |\n| c | 10 |\n| d | 10 |\n", 1)
				b.writeFile(filepath.Join(filepath.Dir(b.design), "other.md"), other)
			}
			work := newWorkBed(t)
			work.manager.Prober = &treeProber{dead: true}
			displayManager := work.manager
			owners := b.intentBed.terminalOwners()
			owners.delivery = b.owners
			owners.work.units = func(stateroot.Layout) *launch.UnitRunner {
				return &launch.UnitRunner{Manager: displayManager, ReviewPolicy: func() (string, error) {
					if scenario == "held" {
						return "person", nil
					}
					return "auto", nil
				}}
			}
			work.starter.author = func(record launch.Record) {
				command, err := launch.ClaudeHeadless{}.Command(record, t.TempDir())
				if err != nil {
					t.Error(err)
					return
				}
				output, _, err := authorPromptTask(command.Stdin)
				if err != nil {
					t.Error(err)
					return
				}
				if !strings.Contains(command.Stdin, "Exit operation:") || !strings.Contains(command.Stdin, "Fold item:") || !strings.Contains(command.Stdin, "Scope item:") || !strings.Contains(command.Stdin, "F1") {
					t.Error("author lacks the bound fold and dependency request")
				}
				if strings.Contains(command.Stdin, "First version.") {
					t.Error("the author brief duplicates the frozen page already supplied as its prior")
				}
				draft := proposal
				if scenario == "invalid proposal" {
					draft = prior
				}
				b.writeFile(output, draft)
				if scenario == "concurrent edit" {
					b.writeFile(b.design, prior+"A person's concurrent note.\n")
				}
				if scenario == "source revision moved" {
					changed := b.goalFile(bedGoal)
					changed.Revision++
					b.addGoal(changed)
				}
			}
			if scenario == "pending author" {
				work.starter.hold = "design"
			}
			if scenario == "failed author" {
				work.starter.fail["design"] = true
			}
			failed := false
			if strings.Contains(scenario, "failure") || scenario == "lost commit response" {
				endpoint := owners.dependencies.endpoint
				owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
					ep, err := endpoint(root)
					ep.Repository = &designAcceptanceRepository{Repository: ep.Repository, before: func() error {
						if !failed && (scenario == "commit failure" || scenario == "opening failure" && len(b.goalFile(bedGoal).DesignExits) > 0) {
							failed = true
							return errors.New("injected publication failure")
						}
						return nil
					}, after: func() (goal.CASOutcome, error) {
						if !failed && scenario == "lost commit response" {
							failed = true
							return goal.CASUnknown, errors.New("injected lost response")
						}
						if !failed && scenario == "projection failure" {
							failed = true
							b.writeFile(b.design, prior+"Concurrent projection edit.\n")
						}
						if !failed && scenario == "author display failure" {
							failed = true
							displayManager = &launch.Manager{Store: launch.Store{Root: t.TempDir()}}
						}
						return goal.CASLanded, nil
					}}
					return ep, err
				}
			}
			args := []string{"design", "review", b.design, "--tool-calls", "30", "--dispositions", template}
			code, result := b.runJSON(owners, args...)
			if scenario == "held" {
				if result.Outcome != intentInProgress || len(work.starter.launched()) != 0 || len(b.goalFile(bedGoal).DesignExits) != 0 || string(mustRead(t, b.design)) != prior {
					t.Fatalf("held revision launched or published: %d %+v", code, result)
				}
				return
			}
			if scenario == "pending author" || scenario == "failed author" || scenario == "invalid proposal" || scenario == "missing scope" || scenario == "concurrent edit" || scenario == "source revision moved" || scenario == "cut dependency" || scenario == "cut ownership" || scenario == "all concrete wrong owner" || scenario == "unknown dependency" {
				if len(b.goalFile(bedGoal).DesignExits) != 0 || b.closes != 0 {
					t.Fatalf("unready author accepted: %d %+v", code, result)
				}
				for i := 0; i < 2; i++ {
					b.runJSON(owners, args...)
				}
				if len(work.starter.launched()) != 1 {
					t.Fatalf("replay started another author: %v", work.starter.launched())
				}
				if scenario == "invalid proposal" {
					attempts, _ := work.manager.DesignAttempts(b.design)
					b.writeFile(attempts[0].Draft, proposal)
					code, result = b.runJSON(owners, args...)
				} else {
					return
				}
			}
			if scenario == "commit failure" || scenario == "opening failure" || scenario == "lost commit response" || scenario == "projection failure" {
				if scenario == "opening failure" {
					acceptanceBuild(t, b, true)
					if b.closes != 1 {
						t.Fatal("destination opening delayed source closure")
					}
				}
				if scenario == "commit failure" || scenario == "projection failure" {
					acceptanceBuild(t, b, false)
				}
				if scenario == "projection failure" {
					if !strings.Contains(string(mustRead(t, b.design)), "Concurrent projection edit.") {
						t.Fatal("concurrent edit overwritten")
					}
					b.writeFile(b.design, prior)
				}
				code, result = b.runJSON(owners, args...)
			}
			file := b.goalFile(bedGoal)
			if code != 0 || len(file.DesignExits) != 1 || b.closes != 1 || len(work.starter.launched()) != 1 {
				t.Fatalf("final revision: %d %+v exits=%+v launches=%v", code, result, file.DesignExits, work.starter.launched())
			}
			exit := file.DesignExits[0]
			attempts, err := work.manager.DesignAttempts(b.design)
			if err != nil || len(attempts) != 1 || exit.AuthorAttempt != attempts[0].Attempt || exit.Operation != attempts[0].Operation || scenario != "author display failure" && attempts[0].Outcome != "published" || exit.Expected != prior || exit.AuthorPrior == "" || len(exit.Scope) < 2 {
				t.Fatalf("lost bound author/scope: %+v attempts=%+v %v", exit, attempts, err)
			}
			if scenario == "author display failure" && !strings.Contains(strings.Join(result.Details, "\n"), "author status needs repair") {
				t.Fatal("lost local author metadata was not reported")
			}
			wholly := scenario == "dependent" || scenario == "shared Decision"
			if wholly {
				if len(exit.Units) != 0 || len(exit.TransferUnits) != 2 || len(file.ReviewObligations) != 0 || strings.Contains(exit.Page, "Status: accepted") {
					t.Fatalf("dependent scope accepted: %+v", exit)
				}
				acceptanceBuild(t, b, false)
			} else {
				if len(exit.Units) == 0 || exit.Units[0] != "reader" || len(file.ReviewObligations) == 0 || !strings.Contains(exit.Page, mappings[0].Passage) || !strings.Contains(exit.Page, "Status: accepted") {
					t.Fatalf("remainder missing: %+v", exit)
				}
				acceptanceBuild(t, b, true)
				if !allConcrete && (strings.Contains(exit.Page, "## Publication") || !strings.Contains(result.Summary, "accepted for")) {
					t.Fatalf("unresolved scope remained accepted: %+v", result)
				}
				if scenario == "nested Decision" && strings.Contains(exit.Page, "### Writer storage") {
					t.Fatal("a transferred Decision's child section remains accepted")
				}
			}
			if allConcrete {
				if exit.Destination != "" || len(exit.Units) != 2 || len(exit.Items) != 2 {
					t.Fatalf("concrete scope split before folding: %+v", exit)
				}
			} else {
				destination := b.goalFile(exit.Destination)
				if destination.Approved != nil || !reflect.DeepEqual(destination.Blocked, []string{bedGoal}) || len(file.Blocked) != 0 || !strings.Contains(exit.DestinationBrief, proposal) || len(exit.TransferUnits) == 0 {
					t.Fatalf("transfer/authority/order lost: %+v destination=%+v", exit, destination)
				}
			}
			for i := 0; i < 2; i++ {
				if code, replay := b.runJSON(owners, args...); code != 0 || replay.Outcome != intentUnchanged {
					t.Fatalf("replay: %d %+v", code, replay)
				}
			}
			if len(work.starter.launched()) != 1 || len(b.goalFile(bedGoal).DesignExits) != 1 || b.closes != 1 {
				t.Fatal("replay duplicated author, exit or closure")
			}
			if code, done := b.runJSON(owners, "goal", "done", bedGoal); !wholly && (code == 0 || done.Outcome != intentRefused) {
				t.Fatalf("unproved remainder completed: %d %+v", code, done)
			}
		})
	}
}

func TestGoalFreeDesignRetainsTransferWithoutOpeningGoal(t *testing.T) {
	t.Parallel()
	b, dir, returned := designEvidenceBedConfigured(t, evidenceInventory, func(b *designLoopBed) {
		b.writeFile(b.design, strings.Replace(string(mustRead(t, b.design)), "- Goals: standing-validation\n", "", 1))
	}, acceptanceUnits+"| writer | Publish the result | 40 |\n")
	f1, f2 := evidenceFinding("F1", "## Collection:1", ""), evidenceFinding("F2", "## Publication", "")
	returned["findings"], returned["rigor"], returned["verdictMaterialCount"] = []any{f1, f2}, []any{evidenceRigor("F1"), evidenceRigor("F2")}, 2
	b.writeJSON(filepath.Join(dir, "return.json"), returned)
	b.writeFile(filepath.Join(dir, "return.md"), "VERDICT: REVISE material=2\n")
	root := b.job("rev1")
	root["designExaminationLimit"], root["reviewRoundLimit"] = 1, 1
	b.writeJob(root)
	template := b.decide(b.review(), map[string]string{"F1": "accepted | concrete requirement | ## Collection:1", "F2": "accepted | unresolved choice | ## Publication"})
	page := suppliedFoldPage(t, string(mustRead(t, b.design)), concreteFoldMapping(), designFoldMapping{Finding: "F2", Unit: "writer", Decision: "## Publication", Unresolved: true})
	page = recoveryScope(t, page, goal.DesignUnitScope{Unit: "reader", Decisions: []string{"## Collection:1"}, DependsOn: []string{}}, goal.DesignUnitScope{Unit: "writer", Decisions: []string{"## Publication"}, DependsOn: []string{}})
	b.writeFile(b.design, page)
	for i := 0; i < 2; i++ {
		result := b.review("--dispositions", template)
		if result.Outcome != intentInProgress || !strings.Contains(result.Summary, "goal-free transfer") || b.closes != 0 || len(b.goalFile(bedGoal).DesignExits) != 0 || string(mustRead(t, b.design)) != page {
			t.Fatalf("goal-free scope published: %+v", result)
		}
	}
	var entry designReviewEntry
	path := filepath.Join(b.install, "artifacts", "agents", "intent-review", "design-01designreader", "chain.json")
	if err := json.Unmarshal(mustRead(t, path), &entry); err != nil || entry.Fold == nil || entry.Fold.Destination != "" || entry.Fold.DestinationBrief == "" || !reflect.DeepEqual(entry.Fold.TransferUnits, []string{"writer"}) {
		t.Fatalf("goal-free draft not retained: %+v %v", entry.Fold, err)
	}
}
