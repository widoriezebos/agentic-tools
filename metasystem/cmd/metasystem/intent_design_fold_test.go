package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
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
	// Decision 4 routes size overflow through the separate split replay scenario.
	for _, scenario := range []string{"concrete", "critical", "finding list only", "missing checklist", "missing test", "duplicate mapping", "unrelated edit", "goal-wide six", "earlier unresolved", "split prerequisite", "commit failure"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			units := acceptanceUnits
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
			returned["findings"], returned["rigor"], returned["verdictMaterialCount"] = findings, rigor, len(findings)
			b.writeJSON(filepath.Join(dir, "return.json"), returned)
			b.writeFile(filepath.Join(dir, "return.md"), fmt.Sprintf("VERDICT: REVISE material=%d\n", len(findings)))
			rows := map[string]string{"F1": "accepted | concrete requirement | ## Collection:1"}
			if len(findings) > 1 {
				rows["F2"] = "accepted | unresolved independent choice | ## Publication"
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

// The public split commits its scope before opening one unapproved destination.
// Interrupted opening and a failed channel post resume their existing owners.
func TestDesignReviewSplitOpensFollowUpOnce(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"oversized", "clean oversized", "six units", "open failure", "person recovery", "post failure", "no channel", "agent source", "destination conflict", "missing brief", "brief conflict"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			units := strings.Replace(acceptanceUnits, "| 40 |", "| 300 |", 1)
			if scenario == "six units" {
				units = acceptanceUnits + "| a | extra | 10 |\n| b | extra | 10 |\n| c | extra | 10 |\n| d | extra | 10 |\n| e | extra | 10 |\n"
			}
			b, dir, returned := designEvidenceBed(t, evidenceInventory, units)
			source := b.goalFile(bedGoal)
			source.Origin = goal.OriginHuman
			if scenario == "agent source" {
				source.Origin = goal.OriginMain
			}
			b.addGoal(source)
			b.lineage = source.Claimed.Lineage
			if err := os.MkdirAll(filepath.Join(b.root(), ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			var findings, rigor []any
			rows := map[string]string{}
			if scenario != "clean oversized" {
				findings, rigor = []any{evidenceFinding("F1", "## Collection:1", "")}, []any{evidenceRigor("F1")}
				rows["F1"] = "accepted | concrete requirement | ## Collection:1"
			}
			if findings == nil {
				findings, rigor = []any{}, []any{}
			}
			returned["findings"], returned["rigor"], returned["verdictMaterialCount"] = findings, rigor, len(findings)
			b.writeJSON(filepath.Join(dir, "return.json"), returned)
			b.writeFile(filepath.Join(dir, "return.md"), fmt.Sprintf("VERDICT: REVISE material=%d\n", len(findings)))
			template := b.decide(b.review(), rows)
			page := string(mustRead(t, b.design))
			if len(findings) > 0 {
				page = suppliedFoldPage(t, page, concreteFoldMapping())
			}
			b.writeFile(b.design, page)
			owners := b.intentBed.terminalOwners()
			owners.delivery = b.owners
			manager := &launch.Manager{Store: launch.Store{Root: t.TempDir()}}
			owners.work.units = func(stateroot.Layout) *launch.UnitRunner { return &launch.UnitRunner{Manager: manager} }
			var mu sync.Mutex
			var messages []string
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				requests++
				if scenario == "post failure" && requests == 1 {
					http.Error(w, "temporary provider failure", 500)
					return
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				text, _ := body["text"].(string)
				messages = append(messages, text)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"ok":true,"result":{"message_id":1,"chat":{"id":1000},"date":1791453600}}`)
			}))
			t.Cleanup(server.Close)
			if scenario != "no channel" {
				fakeDir := t.TempDir()
				b.writeFile(filepath.Join(fakeDir, "base-url"), server.URL)
				conf := filepath.Join(b.install, "metasystem.conf")
				b.writeFile(conf, string(mustRead(t, conf))+"\nchannel.destination.fleet.adapter=fake\nchannel.destination.fleet.fake.face=telegram\nchannel.destination.fleet.fake.dir="+fakeDir+"\n")
			}
			failed := false
			if scenario == "open failure" || scenario == "destination conflict" || scenario == "person recovery" {
				endpoint := owners.dependencies.endpoint
				owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
					ep, err := endpoint(root)
					ep.Repository = &designAcceptanceRepository{Repository: ep.Repository, before: func() error {
						if len(b.goalFile(bedGoal).DesignExits) > 0 && !failed {
							failed = true
							return errors.New("injected destination write failure")
						}
						return nil
					}}
					return ep, err
				}
			}
			args := []string{"design", "review", b.design, "--tool-calls", "30", "--dispositions", template}
			code, result := b.runJSON(owners, args...)
			if scenario == "agent source" {
				if code == 0 || len(b.goalFile(bedGoal).DesignExits) != 0 || b.closes != 0 {
					t.Fatalf("agent gained follow-up authority: %d %+v", code, result)
				}
				return
			}
			file := b.goalFile(bedGoal)
			if len(file.DesignExits) != 1 {
				t.Fatalf("missing committed split: %d %+v file=%+v", code, result, file)
			}
			exit := file.DesignExits[0]
			if exit.Destination == "" || exit.AuthorAttempt != 1 || len(exit.Units) != 0 || len(exit.TransferUnits) == 0 || len(exit.TransferObligations) != len(findings) || !strings.Contains(exit.DestinationBrief, page) || !strings.Contains(exit.OpenCommand, "--blocked-by") || strings.Contains(string(mustRead(t, b.design)), "- Status: accepted") {
				t.Fatalf("incomplete split: %+v", exit)
			}
			if scenario == "open failure" || scenario == "destination conflict" || scenario == "person recovery" {
				if code == 0 || b.closes != 0 || !strings.Contains(result.Summary, "opening remains pending") {
					t.Fatalf("opening failure hidden: %d %+v", code, result)
				}
				if doneCode, done := b.runJSON(owners, "goal", "done", bedGoal, "--by", "Wido", "--reason", "Transfer the design scope."); doneCode == 0 || !strings.Contains(done.Summary, "split destination") {
					t.Fatalf("source completed with unopened destination: %d %+v", doneCode, done)
				}
				if scenario == "destination conflict" {
					other := queuedIntentGoal(exit.Destination, source.Tier)
					other.Intent = "Unrelated work"
					b.addGoal(other)
					code, result = b.runJSON(owners, args...)
					if code == 0 || b.goalFile(exit.Destination).Intent != "Unrelated work" || b.closes != 0 {
						t.Fatalf("destination overwritten: %d %+v", code, result)
					}
					return
				}
				if scenario == "person recovery" {
					opening := shellWords(exit.OpenCommand)[1:]
					for i, word := range opening {
						if word == "NAME" {
							opening[i] = "Wido"
						}
					}
					if openCode, opened := b.runJSON(owners, append(opening, "--fixture-human-authority")...); openCode != 0 {
						t.Fatalf("printed opening failed: %d %+v command=%s", openCode, opened, exit.OpenCommand)
					}
				}
				code, result = b.runJSON(owners, args...)
			}
			if code != 0 || b.closes != 1 {
				t.Fatalf("split failed: %d %+v", code, result)
			}
			check := func() {
				destination := b.goalFile(exit.Destination)
				if destination.Approved != nil || destination.State != goal.StateParked || !reflect.DeepEqual(destination.Blocked, []string{bedGoal}) || destination.Origin != goal.OriginMain || destination.NextStep != "Reshape scope in "+exit.DestinationBriefPath || string(mustRead(t, filepath.Join(b.install, exit.DestinationBriefPath))) != exit.DestinationBrief {
					t.Fatalf("destination approved or incorrectly ordered: %+v", destination)
				}
				if current := b.goalFile(bedGoal); current.State != goal.StateClaimed || len(current.Blocked) != 0 || len(current.DesignExits) != 1 {
					t.Fatalf("source changed: %+v", current)
				}
				attempts, err := manager.DesignAttempts(b.design)
				if err != nil || len(attempts) != 1 || attempts[0].Operation != exit.Operation || attempts[0].ExpectedSHA256 != exit.ExaminedSHA256 || string(mustRead(t, attempts[0].Draft)) != exit.Expected {
					t.Fatalf("author attempt repeated or lost: %+v %v", attempts, err)
				}
			}
			check()
			briefPath := filepath.Join(b.install, exit.DestinationBriefPath)
			if scenario == "missing brief" {
				if err := os.Remove(briefPath); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "brief conflict" {
				b.writeFile(briefPath, "A person's changed scope.\n")
				if code, result := b.runJSON(owners, args...); code == 0 || !strings.Contains(result.Summary, "brief") || string(mustRead(t, briefPath)) != "A person's changed scope.\n" {
					t.Fatalf("brief edit overwritten: %d %+v", code, result)
				}
				b.writeFile(briefPath, exit.DestinationBrief)
			}
			if scenario == "post failure" && len(channel.LoadLandedState(b.install).Pending) != 1 {
				t.Fatal("failed notification was not retained")
			}
			for i := 0; i < 2; i++ {
				if code, result := b.runJSON(owners, args...); code != 0 || result.Outcome != intentUnchanged {
					t.Fatalf("replay: %d %+v", code, result)
				}
				check()
			}
			ep, err := owners.dependencies.endpoint(b.root())
			if err != nil {
				t.Fatal(err)
			}
			projection, err := goal.Project(ep, false, b.owners.now())
			if err != nil || len(projection.Tree.Live) != 2 {
				t.Fatalf("duplicate destination: %+v %v", projection, err)
			}
			mu.Lock()
			defer mu.Unlock()
			if scenario == "no channel" {
				if requests != 0 {
					t.Fatal("unconfigured channel posted")
				}
			} else if len(messages) != 1 || !strings.Contains(messages[0], "metasystem goal approve "+exit.Destination) || !strings.Contains(messages[0], bedGoal) {
				t.Fatalf("approval request missing or repeated: %v", messages)
			}
			acceptanceBuild(t, b, false)
			t.Logf("review/replay exit=0; one unapproved destination, one retained author attempt, %d approval messages; empty source build refused", len(messages))
		})
	}
}
