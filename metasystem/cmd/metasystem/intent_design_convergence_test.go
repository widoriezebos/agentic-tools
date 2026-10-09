package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
)

func TestDesignReviewConvergesAndStopsOnOneRoot(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name    string
		counts  []int
		changed bool
		stop    bool
	}{
		{"converge", []int{5, 3, 1, 0}, false, false},
		{"equal", []int{3, 3}, false, true},
		{"rising", []int{2, 3}, false, true},
		{"fourth-positive", []int{5, 4, 3, 2}, false, true},
		{"fixed-section", []int{3, 1}, true, true},
		{"unchanged-section-author-mark", []int{3, 1}, false, false},
		{"different-section", []int{3, 1}, true, false},
		{"corrupt-history", []int{3, 1}, false, false},
		{"corrupt-allowance", []int{3, 1}, false, false},
		{"refuted-section", []int{3, 1}, true, false},
		{"refuted-unchanged", []int{3, 1, 0}, false, false},
		{"held-continuation", []int{3, 1}, false, false},
		{"fixed-other-same-rule", []int{3, 1}, true, true},
		{"fixed-other-new-rule", []int{3, 1}, true, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			b, _, raw := designEvidenceBed(t, evidenceInventory, acceptanceUnits)
			b.lineage = b.goalFile(bedGoal).Claimed.Lineage
			if err := os.MkdirAll(filepath.Join(b.root(), ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			root := b.job("rev1")
			root["reviewRoundLimit"] = 4
			b.writeJob(root)
			dispatch := b.handler
			b.handler = func(p intentProcess) intentProcessResult {
				result := dispatch(p)
				if flagValue(p.argv, "--follow-up") == "rev1" {
					round := len(b.followUps) + 1
					job := fmt.Sprintf("rev1-r%d", round)
					child := b.job(job)
					if round > 2 {
						child["parentJob"] = fmt.Sprintf("rev1-r%d", round-1)
					}
					child["engineBuild"], child["effectiveModel"] = "fixture-engine", "fixture-critic"
					b.writeJob(child)
					workspace, err := filepath.EvalSymlinks(b.root())
					if err != nil {
						t.Fatal(err)
					}
					subject, present, err := dispatchcore.ComputeReadSubjectWithFacts(dispatchcore.ReadSubjectRequest{RepoRoot: b.install, Role: "design-critic", Workspace: workspace, RootJob: "rev1"}, admissionFacts{})
					if err != nil || !present {
						t.Fatalf("freeze follow-up: %v", err)
					}
					b.writeJSON(filepath.Join(b.install, "artifacts", "agents", "rev1", "rounds", fmt.Sprint(round), "subject.json"), subject)
				}
				return result
			}
			for index, count := range scenario.counts {
				round := index + 1
				job := "rev1"
				if round > 1 {
					job = fmt.Sprintf("rev1-r%d", round)
				}
				b.finish(job, round, "completed")
				dir := filepath.Join(b.install, "artifacts", "agents", "rev1", "rounds", fmt.Sprint(round))
				var subject dispatchcore.ReadSubject
				if err := json.Unmarshal(mustRead(t, filepath.Join(dir, "subject.json")), &subject); err != nil {
					t.Fatal(err)
				}
				findings, rigor, rows := []any{}, []any{}, map[string]string{}
				for n := 0; n < count; n++ {
					id := fmt.Sprintf("R%dF%d", round, n)
					finding := evidenceFinding(id, "## Publication", "")
					if scenario.name == "different-section" && round > 1 {
						finding["where"] = "## Collection:1"
					}
					if strings.HasPrefix(scenario.name, "fixed-other-") {
						finding["class"], finding["relation"] = "other", "rule A"
						if scenario.name == "fixed-other-new-rule" && round > 1 {
							finding["relation"] = "rule B"
						}
					}
					if scenario.name == "fourth-positive" {
						finding["severity"] = "critical"
					}
					findings = append(findings, finding)
					rigor = append(rigor, evidenceRigor(id))
					rows[id] = "accepted | specified the requirement | ## Publication"
					if (scenario.name == "refuted-section" || scenario.name == "refuted-unchanged") && round == 1 {
						rows[id] = "refuted | searched reader.md for page and found line 3 | "
					}
				}
				raw["jobId"], raw["round"], raw["wholePageDigest"] = job, round, subject.ContentDigest
				raw["findings"], raw["rigor"], raw["verdictMaterialCount"] = findings, rigor, count
				b.writeJSON(filepath.Join(dir, "return.json"), raw)
				b.writeFile(filepath.Join(dir, "return.md"), fmt.Sprintf("VERDICT: REVISE material=%d\n", count))
				if (scenario.name == "corrupt-history" || scenario.name == "corrupt-allowance") && round == 2 {
					root := b.job("rev1")
					field := "designExaminations"
					if scenario.name == "corrupt-allowance" {
						field = "designExaminationLimit"
					}
					prior := root[field]
					root[field] = []any{map[string]any{"id": "unbound history"}}
					b.writeJob(root)
					if unknown := b.review(); unknown.Outcome != intentFailed || !strings.Contains(unknown.Summary, "unknown") || b.job("rev1")["findingRegisterRound"] != float64(1) {
						t.Fatalf("unreadable history was admitted: %+v", unknown)
					}
					root = b.job("rev1")
					root[field] = prior
					b.writeJob(root)
				}
				result := b.review()
				if result.Outcome != intentConfirmed {
					t.Fatalf("round %d: %+v", round, result)
				}
				root = b.job("rev1")
				var decision loopstop.Stop
				encoded, _ := json.Marshal(root["designDecision"])
				if err := json.Unmarshal(encoded, &decision); err != nil {
					t.Fatal(err)
				}
				want := "continue"
				if count == 0 {
					want = "close"
				} else if index == len(scenario.counts)-1 && scenario.stop {
					want = "stop"
				}
				if decision.Decision != want || decision.Attempt != round || decision.Budget != 4 {
					t.Fatalf("round %d decision: %+v; want %s", round, decision, want)
				}
				if round == 1 {
					root["reviewRoundLimit"] = 20
					b.writeJob(root)
				}
				answer := b.decide(result, rows)
				if scenario.name == "unchanged-section-author-mark" {
					b.writeFile(answer, string(mustRead(t, answer))+"\nFixed: incomplete-item at ## Publication\n")
				}
				if want == "stop" {
					for _, field := range []string{"designStop", "findingRegisterStop"} {
						data, _ := json.Marshal(root[field])
						var stop loopstop.Stop
						if json.Unmarshal(data, &stop) != nil || stop.Decision != "stop" || stop.Subject != "rev1" {
							t.Fatalf("missing %s: %v", field, root[field])
						}
					}
					brief := filepath.Join(b.root(), "successor.md")
					b.writeFile(brief, "No new allowance.\n")
					if _, err := dispatchcore.CritiqueExhaustionAdvance(b.install, "rev1", "design-critic", brief, "fifth"); err == nil {
						t.Fatal("stopped design admitted another examination")
					}
					mappings := []designFoldMapping{}
					for _, f := range findings {
						id := f.(map[string]any)["id"].(string)
						m := concreteFoldMapping()
						m.Finding, m.Decision = id, "## Publication"
						mappings = append(mappings, m)
					}
					b.writeFile(b.design, suppliedFoldPage(t, string(mustRead(t, b.design)), mappings...))
				}
				if index < len(scenario.counts)-1 {
					page := string(mustRead(t, b.design))
					if scenario.changed {
						page = strings.Replace(page, "Publish the result.", "Publish the changed result.", 1)
					} else {
						page = strings.Replace(page, "First version.", fmt.Sprintf("First version. Correction %d.", round), 1)
					}
					if scenario.name != "refuted-unchanged" || round != 1 {
						b.writeFile(b.design, page)
					}
					if scenario.name == "held-continuation" {
						conf := filepath.Join(b.install, "metasystem.conf")
						original := string(mustRead(t, conf))
						b.writeFile(conf, original+"\nreview.stop=person\n")
						if held := b.review("--dispositions", answer); held.Outcome != intentInProgress || len(b.followUps) != 0 {
							t.Fatalf("person hold admitted a correction: %+v", held)
						}
						b.writeFile(conf, original)
					}
					continued := b.review("--dispositions", answer)
					if continued.Outcome != intentInProgress || len(b.followUps) != round {
						t.Fatalf("continuation %d: %+v", round, continued)
					}
				} else if want != "continue" {
					closed := b.review("--dispositions", answer)
					if closed.Outcome != intentConfirmed || b.job("rev1")["chainClosed"] != true || b.closes != 1 {
						t.Fatalf("durable exit: %+v; root=%v", closed, b.job("rev1"))
					}
					if len(b.goalFile(bedGoal).DesignExits) != 1 || b.fresh != 1 || len(b.followUps) != round-1 {
						t.Fatal("exit or root duplicated")
					}
					if replay := b.review("--dispositions", answer); replay.Outcome != intentUnchanged {
						t.Fatalf("exit replay: %+v", replay)
					}
				}
			}
		})
	}
}
