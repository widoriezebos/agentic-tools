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
)

func suppliedFoldPage(t *testing.T, prior string, mappings ...designFoldMapping) string {
	t.Helper()
	page := prior
	for _, m := range mappings {
		if m.Decision == "## Collection:1" {
			page = strings.Replace(page, "First version.", m.Passage, 1)
		} else {
			page = strings.Replace(page, m.Decision+"\n", m.Decision+"\n"+m.Passage+"\n", 1)
		}
		page = strings.Replace(page, "Read the whole page", "Read the whole page; "+strings.Join(m.Tests, ",")+"; "+m.Checklist, 1)
	}
	page += "\n## Acceptance items\n\n"
	for _, m := range mappings {
		encoded, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		page += "Fold item: " + string(encoded) + "\n"
	}
	return page
}

func concreteFoldMapping() designFoldMapping {
	return designFoldMapping{Finding: "F1", Unit: "reader", Decision: "## Collection:1", Passage: "Specify the implementation and its acceptance test. Retain unknown evidence instead of accepting it.", Tests: []string{"reader/TestUnknownEvidence"}, Checklist: "check unknown evidence handling", Fixture: "group:reader"}
}

// Supplied Decision revisions accept concrete scope and keep unresolved
// split prerequisites pending.
func TestDesignReviewFoldsDecisionsAndSplitsRemainder(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"concrete", "critical", "finding list only", "missing checklist", "missing test", "duplicate mapping", "unrelated edit", "oversized", "clean oversized", "six units", "goal-wide six", "earlier unresolved", "split prerequisite", "commit failure"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			units := acceptanceUnits
			if scenario == "clean oversized" {
				units = strings.Replace(units, "| 40 |", "| 300 |", 1)
			}
			b, dir, returned := designEvidenceBed(t, evidenceInventory, units)
			if err := os.MkdirAll(filepath.Join(b.root(), ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			b.lineage = b.goalFile(bedGoal).Claimed.Lineage
			f := evidenceFinding("F1", "## Collection:1", "")
			if scenario == "critical" {
				f["severity"] = "critical"
			}
			findings, rigor := []any{f}, []any{evidenceRigor("F1")}
			if scenario == "split prerequisite" {
				choice := evidenceFinding("F2", "## Publication", "")
				choice["change"] = "Decide which publication authority owns the follow-up"
				findings = append(findings, choice)
				rigor = append(rigor, evidenceRigor("F2"))
			}
			if scenario == "clean oversized" {
				findings, rigor = []any{}, []any{}
			}
			returned["findings"], returned["rigor"], returned["verdictMaterialCount"] = findings, rigor, len(findings)
			b.writeJSON(filepath.Join(dir, "return.json"), returned)
			b.writeFile(filepath.Join(dir, "return.md"), fmt.Sprintf("VERDICT: REVISE material=%d\n", len(findings)))
			rows := map[string]string{"F1": "accepted | concrete requirement | ## Collection:1"}
			if len(findings) > 1 {
				rows["F2"] = "accepted | unresolved independent choice | ## Publication"
			}
			if scenario == "clean oversized" {
				rows = nil
			}
			template := b.decide(b.review(), rows)
			if scenario == "critical" {
				root := b.job("rev1")
				root["reviewRoundLimit"] = 1
				b.writeJob(root)
			}
			m := concreteFoldMapping()
			page := suppliedFoldPage(t, string(mustRead(t, b.design)), m)
			switch scenario {
			case "finding list only":
				page = strings.Replace(page, m.Passage, "First version.", 1)
			case "missing checklist":
				page = strings.Replace(page, m.Checklist, "", 1)
			case "missing test":
				page = strings.Replace(page, m.Tests[0], "", 1)
			case "duplicate mapping":
				encoded, _ := json.Marshal(m)
				page += "Fold item: " + string(encoded) + "\n"
			case "unrelated edit":
				page = strings.Replace(page, "Publish the result.", "Discard the result.", 1)
			case "clean oversized":
				page = string(mustRead(t, b.design))
			case "oversized":
				page = strings.Replace(page, "| 40 |", "| 300 |", 1)
			case "six units":
				page = strings.Replace(page, "| reader |", "| a | extra | 10 |\n| b | extra | 10 |\n| c | extra | 10 |\n| d | extra | 10 |\n| e | extra | 10 |\n| reader |", 1)
			case "goal-wide six":
				other := strings.Replace(string(mustRead(t, b.design)), "01DESIGNREADER", "other-design", 1)
				other = strings.Replace(other, acceptanceUnits, "\n## Units\n\n| Unit | Production lines |\n| --- | --- |\n| a | 10 |\n| b | 10 |\n| c | 10 |\n| d | 10 |\n| e | 10 |\n", 1)
				b.writeFile(filepath.Join(filepath.Dir(b.design), "other.md"), other)
			case "earlier unresolved":
				b.register(1, 2, []int64{1}, map[string]any{"findingId": "F1"}, map[string]any{"findingId": "earlier:F2"})
			}
			b.writeFile(b.design, page)
			owners := b.intentBed.owners()
			owners.delivery = b.owners
			failed := false
			if scenario == "commit failure" {
				endpoint := owners.dependencies.endpoint
				owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
					ep, err := endpoint(root)
					ep.Repository = &designAcceptanceRepository{Repository: ep.Repository, before: func() error {
						if !failed {
							failed = true
							return errors.New("injected goal write failure")
						}
						return nil
					}}
					return ep, err
				}
			}
			code, result := b.runJSON(owners, "design", "review", b.design, "--tool-calls", "30", "--dispositions", template)
			success := scenario == "concrete" || scenario == "critical"
			if !success {
				if code == 0 || len(b.goalFile(bedGoal).DesignExits) != 0 || b.job("rev1")["chainClosed"] == true || string(mustRead(t, b.design)) != page {
					t.Fatalf("invalid or pending revision accepted: %d %+v", code, result)
				}
				if scenario == "clean oversized" && !strings.Contains(result.Summary, "production lines") {
					t.Fatalf("the unchanged clean page must fail on resulting size: %+v", result)
				}
				if scenario != "commit failure" {
					return
				}
				acceptanceBuild(t, b, false)
				code, result = b.runJSON(owners, "design", "review", b.design, "--tool-calls", "30", "--dispositions", template)
			}
			file := b.goalFile(bedGoal)
			if code != 0 || len(file.DesignExits) != 1 || len(file.ReviewObligations) != 1 || b.job("rev1")["chainClosed"] != true {
				t.Fatalf("fold publication: %d %+v goal=%+v", code, result, file)
			}
			exit, item := file.DesignExits[0], file.ReviewObligations[0]
			if exit.Page != string(mustRead(t, b.design)) || exit.Expected != page || len(exit.Items) != 1 || exit.Items[0] != item.Finding || item.Finding != "rev1:F1:reader" || item.State != "open" || item.DesignItem.Passage != m.Passage || item.DesignItem.BodySHA256 != exit.BodySHA256 || item.DesignItem.Requirement != f["change"] || !reflect.DeepEqual(item.DesignItem.Tests, m.Tests) || item.DesignItem.Read != "rev1" || item.DesignItem.Finding != "F1" || item.DesignItem.Evidence.Change != f["change"] {
				t.Fatalf("fold lost provenance or mapping: exit=%+v item=%+v", exit, item)
			}
			if !strings.Contains(exit.Page, "on 1 material findings folded as 1 unit acceptance items") {
				t.Fatal("the accepted head hides the material finding or its item")
			}
			acceptanceBuild(t, b, true)
			if code, again := b.runJSON(owners, "design", "review", b.design, "--tool-calls", "30", "--dispositions", template); code != 0 || again.Outcome != intentUnchanged || len(b.goalFile(bedGoal).DesignExits) != 1 || b.closes != 1 || b.fresh != 1 || len(b.followUps) != 0 {
				t.Fatalf("fold replay repeated publication or critique: %d %+v", code, again)
			}
			if code, done := b.runJSON(owners, "goal", "done", bedGoal); code == 0 || done.Outcome != intentRefused {
				t.Fatalf("unproved acceptance item completed: %d %+v", code, done)
			}
			t.Logf("review exit=0; one exact Decision revision and open item; build exit=0; replay exit=0; completion refused")
		})
	}
}
