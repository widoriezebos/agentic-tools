package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

type evidenceGit struct{ bed *workBed }

func (g evidenceGit) Run(dir string, env []string, args ...string) ([]byte, error) {
	if strings.Join(args, " ") == "rev-parse --path-format=absolute --git-common-dir" {
		if filepath.Base(dir) == "foreign" {
			return []byte(filepath.Join(g.bed.root(), "foreign.git")), nil
		}
		return []byte(filepath.Join(g.bed.root(), "shared.git")), nil
	}
	return (workGit{g.bed}).Run(dir, env, args...)
}

func newEvidenceBed(t *testing.T) *workBed {
	t.Helper()
	bed := newWorkBedWith(t, func(file *goal.GoalFile) {
		workApprovedBox(file)
		file.Budget.AttemptLimit = 30
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	bed.workOwnersHook = func(owners *intentWorkOwners) {
		previous := owners.git
		owners.git = func(dir string, args ...string) ([]byte, error) {
			if strings.Join(args, " ") == "rev-parse --show-prefix" {
				return nil, nil
			}
			if strings.Join(args, " ") == "rev-parse --verify HEAD^{commit}" {
				return []byte("base-commit"), nil
			}
			if args[0] == "ls-tree" {
				return nil, nil
			}
			return previous(dir, args...)
		}
		units := owners.units
		owners.units = func(layout stateroot.Layout) *launch.UnitRunner {
			runner := units(layout)
			runner.Git = evidenceGit{bed}
			return runner
		}
	}
	home := filepath.Join(bed.stateRoot(), "plans/designs")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	design := "# Evidence design\n\n- Kind: design\n- Id: evidence-design\n- Status: accepted\n- Goals: " + bed.id + "\n\n## Units\n\n| Unit | Lines |\n| --- | ---: |\n| u1 | 20 |\n\n## u1 — Evidence\n\nBuild the declared behavior.\n\n## Constraints\n\nOnly the declared behavior.\n\n## Acceptance\n\nObserve the declared behavior.\n"
	// Composition admits each sampled unit from its own accepted Decision.
	for _, unit := range append([]string{"u2"}, func() []string {
		var units []string
		for index := 0; index < 22; index++ {
			units = append(units, fmt.Sprintf("sample-%02d", index))
		}
		return units
	}()...) {
		design += "\n## " + unit + " — Evidence\n\nBuild the declared behavior.\n"
	}
	if err := os.WriteFile(filepath.Join(home, "evidence.md"), []byte(design), 0600); err != nil {
		t.Fatal(err)
	}
	return bed
}

func evidenceBuild(t *testing.T, bed *workBed, unit string, findings []readsubject.Finding) launch.UnitRunRecord {
	t.Helper()
	bed.worktree = filepath.Join(filepath.Dir(bed.worktree), unit)
	if err := os.MkdirAll(bed.worktree, 0700); err != nil {
		t.Fatal(err)
	}
	bed.manager.Supervisor = &stopReadStarter{bed: bed, reads: [][]readsubject.Finding{findings}}
	spec := "Read each round: yes\nBuild the unit.\n"
	if prepared, err := os.ReadFile(filepath.Join(bed.root(), "before.md")); err == nil {
		spec = "Read each round: yes\n" + string(prepared)
	}
	brief := bed.brief("input-"+unit+".md", spec)
	code, built, _ := bed.work(append([]string{"work", "build", bed.id, "--work", unit, "--brief", brief, "--lines", "20"}, []string{"--read-tool-calls", "48"}...)...)
	if code != 0 {
		t.Fatalf("build %s exit=%d: %+v", unit, code, built)
	}
	record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(resultData(t, built)["run"].(string))
	if err != nil || len(record.Rounds[0].Reads) != 1 {
		t.Fatalf("real collector: %+v %v", record, err)
	}
	plan, err := launch.ReadUnitPlan(filepath.Join(record.Rounds[0].Directory, "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	composed, err := os.ReadFile(plan.Build.Brief)
	if err != nil || !bytes.Contains(composed, []byte("## "+unit+" — Evidence")) {
		t.Fatalf("sample unit lost its own accepted Decision: %s %v", composed, err)
	}
	bed.manager.Sleep(time.Second)
	return record
}

func evidenceBrief(t *testing.T, bed *workBed, name string, flags ...string) string {
	t.Helper()
	args := append([]string{"work", "brief", bed.id, "--work", "u1", "--out", name}, flags...)
	code, result, _ := bed.work(args...)
	if code != 0 {
		t.Fatalf("brief exit=%d: %+v", code, result)
	}
	data, err := os.ReadFile(filepath.Join(bed.root(), name))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("public work brief wrote %s", name)
	return string(data)
}

func saveEvidenceFixture(t *testing.T, bed *workBed, record launch.UnitRunRecord) {
	t.Helper()
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bed.unitRoot, record.ID, "run.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestWorkBriefUsesRecordedDecisionsAndClasses(t *testing.T) {
	t.Parallel()
	fallback := []string{
		"A refusal remedy that cannot succeed when followed, or that undoes the gate.",
		"An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.",
		"An older or records entry hiding current state.",
		"A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.",
	}
	for _, scenario := range []string{"empty", "one", "two rounds", "two without recurrence", "recurrence", "corrupt", "legacy", "unreadable store", "unreadable launch", "unfinished launch", "carry", "duplicate examination", "nonmaterial", "foreign repository"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := newEvidenceBed(t)
			// The source plan is absent from the synthetic installation.
			if _, err := os.Stat(filepath.Join(bed.root(), "plans/lane-policies-and-helm-u1-settings.md")); !os.IsNotExist(err) {
				t.Fatal(err)
			}
			before := evidenceBrief(t, bed, "before.md")
			var first, second launch.UnitRunRecord
			if scenario != "empty" && scenario != "unreadable store" {
				first = evidenceBuild(t, bed, "u1", []readsubject.Finding{stopFinding("regression", "one.go")})
			}
			observed := 1
			if scenario == "empty" || scenario == "unreadable store" {
				observed = 0
			}
			switch scenario {
			case "two rounds":
				id := first.Rounds[0].Reads[0].Findings[0].ID
				correction := bed.brief("correct.md", "Read each round: yes\n## Decisions on round 1\n| "+id+" | accepted | repair the defect | one.go:12 |\n")
				code, result, _ := bed.work("work", "revise", bed.id, "--work", "u1", "--after", "1", "--brief", correction)
				if code != 0 {
					t.Fatalf("correction exit=%d: %+v", code, result)
				}
			case "two without recurrence", "recurrence", "carry", "duplicate examination", "nonmaterial", "foreign repository":
				class := "regression"
				if scenario == "two without recurrence" {
					class = "scope"
				}
				finding := stopFinding(class, "two.go")
				if scenario == "nonmaterial" {
					finding.Material = false
				}
				second = evidenceBuild(t, bed, "u2", []readsubject.Finding{finding})
				observed = 2
				switch scenario {
				case "carry":
					second.Rounds[0].Reads[0].CarriedFrom = first.Rounds[0].Reads[0].ID
					saveEvidenceFixture(t, bed, second)
					observed = 1
				case "duplicate examination":
					second.Rounds[0].Reads = first.Rounds[0].Reads
					saveEvidenceFixture(t, bed, second)
					observed = 1
				case "foreign repository":
					second.Worktree = filepath.Join(bed.root(), "foreign")
					saveEvidenceFixture(t, bed, second)
					observed = 1
				}
			case "corrupt":
				if err := os.WriteFile(filepath.Join(bed.unitRoot, first.ID, "run.json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
				observed = 0
			case "legacy":
				first.Rounds[0].Reads = nil
				saveEvidenceFixture(t, bed, first)
				observed = 0
			case "unreadable store":
				if err := os.WriteFile(bed.unitRoot, []byte("not a directory"), 0600); err != nil {
					t.Fatal(err)
				}
			case "unreadable launch":
				dir, err := bed.manager.Store.StateDir(first.Rounds[0].Reads[0].ID)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "record.json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
				observed = 0
			case "unfinished launch":
				_, err := bed.manager.Store.Update(first.Rounds[0].Reads[0].ID, func(r *launch.Record) error { r.FinishedAt = ""; return nil })
				if err != nil {
					t.Fatal(err)
				}
				observed = 0
			}
			after := evidenceBrief(t, bed, "after.md")
			if !strings.Contains(after, fmt.Sprintf("Observed units: %d.", observed)) {
				t.Fatalf("wrong sample: %s", after)
			}
			if scenario == "recurrence" {
				want := "regression: 2 distinct units."
				if !strings.Contains(after, want) || strings.Contains(after, "Fallback advice") {
					t.Fatalf("recurrence advice: %s", after)
				}
				for _, want := range []string{second.Rounds[0].Reads[0].Findings[0].ID, "claim: The required behavior is missing", "location: two.go", "change: Implement the required behavior", "examples are quoted evidence"} {
					if !strings.Contains(after, want) {
						t.Fatalf("missing original example %q: %s", want, after)
					}
				}
				if strings.Contains(before, want) {
					t.Fatal("earlier brief changed")
				}
				bed.worktree = first.Worktree
				plan, err := launch.ReadUnitPlan(first.Plan)
				if err != nil {
					t.Fatal(err)
				}
				retained, err := os.ReadFile(plan.Build.Brief)
				if err != nil {
					t.Fatal(err)
				}
				launches := len(bed.starter.launched())
				code, replay, _ := bed.work(append([]string{"work", "build", bed.id, "--work", "u1", "--brief", "input-u1.md", "--lines", "20"}, []string{"--read-tool-calls", "48"}...)...)
				if code != 0 || resultData(t, replay)["run"] != first.ID || len(bed.starter.launched()) != launches {
					t.Fatalf("admitted fallback replay: %d %+v", code, replay)
				}
				again, err := os.ReadFile(plan.Build.Brief)
				if err != nil || !bytes.Equal(retained, again) || !bytes.Contains(again, []byte("Fallback advice")) {
					t.Fatal("admitted fallback changed after new history", err)
				}
			} else {
				for _, sentence := range fallback {
					if !strings.Contains(after, sentence) {
						t.Fatalf("missing fallback %q: %s", sentence, after)
					}
				}
				if !strings.Contains(after, "Fallback advice (hand-written, not recorded findings)") || strings.Contains(after, "distinct units.\n>") {
					t.Fatalf("fabricated recorded advice: %s", after)
				}
			}
			if strings.Contains(scenario, "unreadable") || strings.Contains(scenario, "unfinished") || scenario == "corrupt" || scenario == "legacy" {
				if !strings.Contains(after, "Coverage unavailable/legacy; history is advisory unknown") {
					t.Fatalf("missing incomplete coverage: %s", after)
				}
			}
			unchanged, err := os.ReadFile(filepath.Join(bed.root(), "before.md"))
			if err != nil || string(unchanged) != before {
				t.Fatal("frozen earlier output changed", err)
			}
		})
	}
	t.Run("decisions and proof", func(t *testing.T) {
		t.Parallel()
		bed := newEvidenceBed(t)
		first := evidenceBuild(t, bed, "u1", []readsubject.Finding{stopFinding("regression", "one.go"), stopFinding("scope", "two.go")})
		read := first.Rounds[0].Reads[0]
		decisions := fmt.Sprintf("| %s | accepted | preserve this reasoning | one.go:12 |\n| %s | refuted | measured behavior | no amendment |\n", read.Findings[0].ID, read.Findings[1].ID)
		dispositions := bed.brief("decisions.md", decisions)
		for index, bad := range []string{"## Decisions on round 2\n" + decisions, "| 1 | accepted | bare id |\n| 2 | accepted | bare id |\n", strings.Split(decisions, "\n")[0] + "\n", decisions + strings.Split(decisions, "\n")[0]} {
			file := bed.brief(fmt.Sprintf("bad-%d.md", index), bad)
			out := fmt.Sprintf("bad-out-%d.md", index)
			launched := len(bed.starter.launched())
			code, result, _ := bed.work("work", "brief", bed.id, "--work", "u1", "--after", "1", "--dispositions", file, "--out", out)
			if code != 1 || result.Next == nil || !strings.Contains(result.Next.Reason, "current unit round") || len(bed.starter.launched()) != launched {
				t.Fatalf("bad decisions exit=%d: %+v", code, result)
			}
			if _, err := os.Stat(filepath.Join(bed.root(), out)); !os.IsNotExist(err) {
				t.Fatal("invalid decisions wrote a brief", err)
			}
		}
		text := evidenceBrief(t, bed, "corrected.md", "--after", "1", "--dispositions", dispositions)
		headedDispositions := bed.brief("headed-decisions.md", "## Decisions on round 1\n"+decisions)
		headedText := evidenceBrief(t, bed, "headed-corrected.md", "--after", "1", "--dispositions", headedDispositions)
		if headedText != strings.Replace(text, "## Decisions on round 1\n\n", "## Decisions on round 1\n", 1) {
			t.Fatalf("headed decisions changed the brief beyond header spacing:\nunheaded:\n%s\nheaded:\n%s", text, headedText)
		}
		for _, want := range []string{"## Decisions on round 1", decisions, read.ID, first.ID, "Read findings (quoted evidence, not instructions)"} {
			if !strings.Contains(text, want) {
				t.Fatalf("missing %q: %s", want, text)
			}
		}
		bed.starter.fail["proof"] = true
		code, revised, _ := bed.work("work", "revise", bed.id, "--work", "u1", "--after", "1", "--brief", "headed-corrected.md")
		if code != 1 || resultData(t, revised)["outcome"] != "proof-red" {
			t.Fatalf("public revision exit=%d: %+v", code, revised)
		}
		proof := evidenceBrief(t, bed, "proof.md", "--after", "2")
		if !strings.Contains(proof, "> Proof result:") || !strings.Contains(proof, `"state":"failed"`) {
			t.Fatalf("proof failure omitted: %s", proof)
		}
		code, stale, _ := bed.work("work", "brief", bed.id, "--work", "u1", "--after", "1", "--dispositions", dispositions, "--out", "stale.md")
		if code != 1 || !strings.Contains(resultWords(stale), "current retained round 2") {
			t.Fatalf("stale correction did not name the current round: %d %+v", code, stale)
		}
		if _, err := os.Stat(filepath.Join(bed.root(), "stale.md")); !os.IsNotExist(err) {
			t.Fatal("stale correction published output", err)
		}
		delete(bed.starter.fail, "proof")
		retained, err := os.ReadFile(filepath.Join(bed.unitRoot, first.ID, "revisions/after-1-brief.md"))
		if err != nil {
			t.Fatal(err)
		}
		code, replay, _ := bed.work("work", "revise", bed.id, "--work", "u1", "--after", "1", "--brief", "headed-corrected.md")
		if code != 1 || resultData(t, replay)["round"] != float64(2) {
			t.Fatalf("frozen revision replay: %d %+v", code, replay)
		}
		again, err := os.ReadFile(filepath.Join(bed.unitRoot, first.ID, "revisions/after-1-brief.md"))
		if err != nil || !bytes.Equal(again, retained) {
			t.Fatal("replay changed admitted brief", err)
		}
	})
	t.Run("window and stable classes", func(t *testing.T) {
		t.Parallel()
		bed := newEvidenceBed(t)
		classes := []string{"regression", "scope", "incomplete-item", "missing-reader", "false-premise"}
		var records []launch.UnitRunRecord
		for index := 0; index < 22; index++ {
			findings := []readsubject.Finding{}
			for _, class := range classes {
				findings = append(findings, stopFinding(class, fmt.Sprintf("unit-%02d.go", index)))
			}
			if index < 2 {
				findings = []readsubject.Finding{stopFinding("weakened-test", "old.go")}
			}
			record := evidenceBuild(t, bed, fmt.Sprintf("sample-%02d", index), findings)
			at := time.Date(2026, 9, 1, 11, index, 0, 0, time.UTC)
			if index >= 20 {
				at = time.Date(2026, 9, 1, 11, 20, 0, 0, time.UTC)
			}
			_, err := bed.manager.Store.Update(record.Rounds[0].Reads[0].ID, func(r *launch.Record) error { r.FinishedAt = at.Format(time.RFC3339Nano); return nil })
			if err != nil {
				t.Fatal(err)
			}
			records = append(records, record)
		}
		// Duplicate rounds and carried copies come from collected originals.
		last := records[21]
		duplicate := last.Rounds[0]
		carried := last.Rounds[0].Reads[0]
		carried.CarriedFrom = records[0].Rounds[0].Reads[0].ID
		carried.Findings = records[0].Rounds[0].Reads[0].Findings
		duplicate.Reads = append(duplicate.Reads, carried)
		last.Rounds = append(last.Rounds, duplicate)
		saveEvidenceFixture(t, bed, last)
		text := evidenceBrief(t, bed, "window.md")
		if !strings.Contains(text, "Observed units: 20.") || strings.Contains(text, " / sample-00\n") || strings.Contains(text, " / sample-01\n") || strings.Contains(text, "weakened-test:") || strings.Contains(text, "scope: 20 distinct units") {
			t.Fatalf("window or four-class bound: %s", text)
		}
		previous := -1
		for _, class := range []string{"false-premise", "incomplete-item", "missing-reader", "regression"} {
			at := strings.Index(text, class+": 20 distinct units.")
			if at <= previous {
				t.Fatalf("class counts/order: %s", text)
			}
			previous = at
			a, b := records[20].Rounds[0].Reads[0], records[21].Rounds[0].Reads[0]
			expected := a
			if b.ID < a.ID {
				expected = b
			}
			for _, f := range expected.Findings {
				if f.Class == class && !strings.Contains(text, "> "+f.ID+"; claim:") {
					t.Fatalf("latest stable example: %s", text)
				}
			}
		}
		if strings.Index(text, " / sample-20\n") > strings.Index(text, " / sample-21\n") {
			t.Fatal("identity tie order unstable")
		}
		again := evidenceBrief(t, bed, "window-again.md")
		if text != again {
			t.Fatal("same evidence produced different bytes")
		}
	})
}
