package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

const evidenceInventory = "\n" + readsubject.DesignInventoryHeading + "\n\n" +
	"| Function / record / act | Production caller | Freshness at the decision | Person or agent | Remedy that can succeed | Unreadable input |\n" +
	"| --- | --- | --- | --- | --- | --- |\n" +
	"| reader | review | frozen page | evidence | restore return | unknown |\n" +
	"| publisher | close | current page | person | same operation | pending |\n"

func designEvidenceBed(t *testing.T, inventory string) (*designLoopBed, string, map[string]any) {
	t.Helper()
	b := newDesignLoopBed(t)
	b.writeFile(b.design, "# Reader\n\n- Kind: design\n- Id: 01DESIGNREADER\n- Status: draft\n- Goals: standing-validation\n\n## Collection:1\nFirst version.\n\n## Publication\nPublish the result.\n"+inventory)
	dispatch := b.handler
	var subject readsubject.ReadSubject
	b.handler = func(p intentProcess) intentProcessResult {
		result := dispatch(p)
		if flagValue(p.argv, "--role") == "design-critic" {
			workspace, err := filepath.EvalSymlinks(b.root())
			if err != nil {
				t.Fatal(err)
			}
			var present bool
			subject, present, err = dispatchcore.ComputeReadSubjectWithFacts(dispatchcore.ReadSubjectRequest{RepoRoot: b.install, Role: "design-critic", Workspace: workspace,
				Design: flagValue(p.argv, "--design"), DeclaredOutputs: flagValue(p.argv, "--outputs")}, admissionFacts{})
			if err != nil || !present {
				t.Fatalf("admit subject: %v", err)
			}
			dir := filepath.Join(b.install, "artifacts", "agents", "rev1", "rounds", "1")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := dispatchcore.WriteReadSubject(filepath.Join(dir, "subject.json"), subject); err != nil {
				t.Fatal(err)
			}
			record := b.job("rev1")
			record["engineBuild"], record["effectiveModel"] = "fixture-engine", "fixture-critic"
			record["reviewRoundLimit"], record["criticRoundsConsumed"] = 2, 0
			record["declaredOutputs"], record["declaredOutputsDigest"] = []string{subject.DesignPath}, subject.DeclaredOutputsDigest
			b.writeJob(record)
		}
		return result
	}
	if result := b.review(); result.Outcome != intentInProgress {
		t.Fatalf("launch: %+v", result)
	}
	briefs, _ := filepath.Glob(filepath.Join(b.install, "artifacts", "agents", "intent-review", "design-*", "brief.md"))
	if len(briefs) != 1 || !strings.Contains(string(mustRead(t, briefs[0])), "Review the entire frozen page") {
		t.Fatal("generated brief does not require whole-page evidence")
	}
	b.finish("rev1", 1, "completed")
	coverage := []readsubject.DesignCoverage{}
	if inventory == evidenceInventory {
		for _, row := range []string{"reader", "publisher"} {
			coverage = append(coverage, evidenceCoverage(row, readsubject.DesignInventoryHeading))
		}
	}
	returned := map[string]any{"schemaVersion": 3, "jobId": "rev1", "round": 1, "runtime": "codex", "sessionId": "fixture-session",
		"model": map[string]any{"requested": "fixture-critic", "effective": "fixture-critic"}, "claimed": map[string]any{"sessionId": nil, "model": nil},
		"evidence": []any{}, "gaps": []string{}, "mode": "design-critique", "reviewedCommit": subject.ReviewedCommit,
		"findings": []any{}, "rigor": []any{}, "verdictMaterialCount": 0, "wholePageDigest": subject.ContentDigest, "coverage": coverage}
	dir := filepath.Join(b.install, "artifacts", "agents", "rev1", "rounds", "1")
	b.writeFile(filepath.Join(dir, "return.md"), "VERDICT: REVISE material=0\n")
	b.writeJSON(filepath.Join(dir, "return.json"), returned)
	return b, dir, returned
}

func evidenceCoverage(row, where string) readsubject.DesignCoverage {
	coverage := readsubject.DesignCoverage{Row: row, Where: where}
	for question := 1; question <= 5; question++ {
		coverage.Answers = append(coverage.Answers, readsubject.DesignAnswer{Question: question, Answer: "specified", Evidence: "owning Decision"})
	}
	return coverage
}

func evidenceFinding(id, where, key string) map[string]any {
	return map[string]any{"id": id, "class": "incomplete-item", "where": where, "change": "Specify the implementation and its acceptance test", "coverage": key, "relation": "",
		"severity": "high", "material": true, "claim": "The Decision misses the implementation requirement", "evidence": "Frozen owning Decision"}
}

func evidenceRigor(id string) map[string]any {
	return map[string]any{"findingId": id, "rigorClass": "bounded", "reopeningTrigger": "requirement changes", "facts": map[string]any{
		"local": true, "recoverable": true, "proofBoundaryCrossed": false, "authorityBoundaryCrossed": false, "secretsBoundaryCrossed": false,
		"irreversibleDataBoundaryCrossed": false, "externalSideEffectBoundaryCrossed": false}}
}

func TestDesignReviewValidatesWholePageEvidence(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"clean", "notes", "repair-cell", "whole-page", "same-section", "missing-inventory", "malformed-inventory", "unanswered", "equivalent-finding", "silent-cell", "silent-row", "duplicate-cell", "duplicate-row", "corrupt-return", "missing-class", "raw-count", "prose-count", "missing-prose", "wrong-job", "wrong-digest", "wrong-engine", "unmapped-heading", "damaged-after-fold"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			inventory := evidenceInventory
			if scenario == "missing-inventory" {
				inventory = ""
			}
			if scenario == "malformed-inventory" {
				inventory = "\n" + readsubject.DesignInventoryHeading + "\n\n| broken | table |\n"
			}
			b, dir, raw := designEvidenceBed(t, inventory)
			coverage := raw["coverage"].([]readsubject.DesignCoverage)
			if scenario == "whole-page" || scenario == "same-section" || scenario == "missing-class" || scenario == "raw-count" || scenario == "unmapped-heading" {
				raw["findings"] = []any{evidenceFinding("F1", "## Collection:1", ""), evidenceFinding("F2", "## Publication", "")}
				raw["rigor"] = []any{evidenceRigor("F1"), evidenceRigor("F2")}
				raw["verdictMaterialCount"] = 2
				b.writeFile(filepath.Join(dir, "return.md"), "VERDICT: REVISE material=2\n")
			}
			switch scenario {
			case "same-section":
				// Two findings in the same first-read section remain two findings.
				raw["findings"].([]any)[1].(map[string]any)["where"] = "## Collection:1"
			case "unanswered", "equivalent-finding", "repair-cell":
				added := evidenceCoverage("omitted validator", "## Publication")
				added.Answers[4] = readsubject.DesignAnswer{Question: 5, Unanswered: true}
				coverage = append(coverage, added)
				if scenario == "equivalent-finding" {
					raw["findings"], raw["rigor"], raw["verdictMaterialCount"] = []any{evidenceFinding("F1", "## Publication", "omitted validator:5")}, []any{evidenceRigor("F1")}, 1
					b.writeFile(filepath.Join(dir, "return.md"), "VERDICT: REVISE material=1\n")
				}
			case "notes":
				note := evidenceFinding("N1", "", "")
				note["material"], note["change"] = false, ""
				raw["findings"] = []any{note}
			case "silent-cell":
				coverage[0].Answers = coverage[0].Answers[:4]
			case "silent-row":
				coverage = coverage[:1]
			case "duplicate-cell":
				coverage[0].Answers[4].Question = 1
			case "duplicate-row":
				coverage = append(coverage, coverage[0])
			case "missing-class":
				delete(raw["findings"].([]any)[0].(map[string]any), "class")
			case "raw-count":
				raw["verdictMaterialCount"] = 0
			case "prose-count":
				b.writeFile(filepath.Join(dir, "return.md"), "VERDICT: REVISE material=1\n")
			case "damaged-after-fold":
				if result := b.review(); result.Outcome != intentConfirmed {
					t.Fatalf("initial valid read: %+v", result)
				}
				raw["wholePageDigest"] = "changed"
			case "wrong-job":
				raw["jobId"] = "foreign-job"
			case "wrong-digest":
				raw["wholePageDigest"] = strings.Repeat("a", 64)
			case "wrong-engine":
				record := b.job("rev1")
				delete(record, "engineBuild")
				b.writeJob(record)
			case "unmapped-heading":
				raw["findings"].([]any)[0].(map[string]any)["where"] = "## Renamed collection"
			}
			raw["coverage"] = coverage
			b.writeJSON(filepath.Join(dir, "return.json"), raw)
			if scenario == "corrupt-return" {
				b.writeFile(filepath.Join(dir, "return.json"), "{")
			}
			if scenario == "missing-prose" {
				if err := os.Remove(filepath.Join(dir, "return.md")); err != nil {
					t.Fatal(err)
				}
			}
			before := mustRead(t, filepath.Join(dir, "return.json"))
			result := b.review()
			known := scenario == "clean" || scenario == "notes" || scenario == "repair-cell" || scenario == "same-section" || scenario == "whole-page" || scenario == "missing-inventory" || scenario == "malformed-inventory" || scenario == "unanswered" || scenario == "equivalent-finding"
			if !known {
				if result.Outcome != intentFailed || !strings.Contains(result.Summary, "unknown") {
					t.Fatalf("unknown evidence became readable: %+v", result)
				}
				answer := filepath.Join(b.root(), "answer.md")
				b.writeFile(answer, "# Dispositions\n\n| Finding | Decision | Reason | Evidence |\n| --- | --- | --- | --- |\n")
				closed := b.review("--dispositions", answer)
				if closed.Outcome != intentFailed || b.closes != 0 || b.job("rev1")["chainClosed"] == true || b.fresh != 1 || len(b.followUps) != 0 {
					t.Fatalf("unknown evidence closed or relaunched: %+v", closed)
				}
				return
			}
			if result.Outcome != intentConfirmed {
				t.Fatalf("readable evidence failed: %+v", result)
			}
			encoded, err := json.Marshal(result.Data.(map[string]any)["read"])
			if err != nil {
				t.Fatal(err)
			}
			var read readsubject.Read
			if err := json.Unmarshal(encoded, &read); err != nil {
				t.Fatal(err)
			}
			want := 1
			if scenario == "clean" || scenario == "notes" {
				want = 0
			}
			if scenario == "whole-page" || scenario == "same-section" {
				want = 2
			}
			if read.Material != want || read.Design.RecordID != "01DESIGNREADER" || read.Design.Root != "rev1" || len(read.Design.Goals) != 1 || read.Design.Sections["## Collection:1"] == "" {
				t.Fatalf("whole-page count or frozen identity: %+v", read)
			}
			if scenario == "unanswered" && (read.Design.RawMaterial != 0 || len(read.Design.Synthetic) != 1 || len(read.Design.Coverage) != 3) {
				t.Fatalf("critic-added coverage lost: %+v", read.Design)
			}
			if scenario == "equivalent-finding" && len(read.Design.Synthetic) != 0 {
				t.Fatalf("missing answer counted twice: %+v", read)
			}
			if _, err := dispatchcore.CritiqueRegisterAdvance(b.install, "rev1", "rev1"); err != nil {
				t.Fatal(err)
			}
			retained := mustRead(t, filepath.Join(dir, "read.json"))
			var saved readsubject.Read
			if err := json.Unmarshal(retained, &saved); err != nil {
				t.Fatal(err)
			}
			if saved.Material != want || len(b.job("rev1")["findingRegister"].([]any)) != want {
				t.Fatalf("register lost derived material: %s %v", retained, b.job("rev1"))
			}
			if !bytes.Equal(before, mustRead(t, filepath.Join(dir, "return.json"))) {
				t.Fatal("raw return was rewritten")
			}
			if scenario == "repair-cell" {
				decided := b.decide(result, map[string]string{read.Findings[0].ID: "accepted | specify unreadable input | ## Publication: retain unknown evidence"})
				b.writeFile(b.design, string(mustRead(t, b.design))+"\nThe validator retains unknown evidence.\n")
				closed := b.review("--dispositions", decided)
				if closed.Outcome != intentConfirmed || b.closes != 1 || len(b.followUps) != 0 || b.job("rev1")["chainClosed"] != true {
					t.Fatalf("known defect has no successful repair: %+v", closed)
				}
				if !bytes.Equal(retained, mustRead(t, filepath.Join(dir, "read.json"))) {
					t.Fatal("dispositions subtracted material from the immutable examination")
				}
			}
			if !strings.Contains(result.Summary, fmt.Sprintf("%d material", want)) {
				t.Fatalf("public count: %+v", result)
			}
		})
	}
}

func TestDesignReviewFailedAdvanceWritesNoDecisions(t *testing.T) {
	t.Parallel()
	b, dir, returned := designEvidenceBed(t, evidenceInventory)
	returned["findings"] = []any{evidenceFinding("F1", "## Collection:1", ""), evidenceFinding("F2", "## Publication", "")}
	returned["rigor"], returned["verdictMaterialCount"] = []any{evidenceRigor("F1"), evidenceRigor("F2")}, 2
	b.writeJSON(filepath.Join(dir, "return.json"), returned)
	b.writeFile(filepath.Join(dir, "return.md"), "VERDICT: REVISE material=2\n")
	record := b.job("rev1")
	record["findingRegisterRound"] = -1
	b.writeJob(record)
	if _, err := dispatchcore.CollectExamination(b.install, "rev1"); err != nil {
		t.Fatalf("collection must succeed before the register fails: %v", err)
	}
	result := b.review()
	if result.Outcome != intentFailed || !strings.Contains(strings.Join(result.Details, " "), "register round state") {
		t.Fatalf("expected the real register failure: %+v", result)
	}
	template := filepath.Join(dir, "decisions.md")
	if _, err := os.Stat(template); !os.IsNotExist(err) {
		t.Fatalf("failed collection wrote a permanent decisions template: %v", err)
	}
	if _, present := result.Data.(map[string]any)["template"]; present {
		t.Fatal("failed collection offers a decisions template")
	}
	record["findingRegisterRound"] = 0
	b.writeJob(record)
	result = b.review()
	if result.Outcome != intentConfirmed || result.Data.(map[string]any)["template"] != template {
		t.Fatalf("repaired collection did not offer its template: %+v", result)
	}
	content := string(mustRead(t, template))
	for _, id := range []string{"F1", "F2"} {
		if !strings.Contains(content, "| "+id+" |") {
			t.Fatalf("recovered template omits %s: %s", id, content)
		}
	}
}

func TestDesignReviewCollectsLegacyEvidence(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"no-subject", "saved-without-page", "folded-without-page"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b, dir, returned := designEvidenceBed(t, evidenceInventory)
			var subject readsubject.ReadSubject
			if err := json.Unmarshal(mustRead(t, filepath.Join(dir, "subject.json")), &subject); err != nil {
				t.Fatal(err)
			}
			if scenario == "no-subject" {
				if err := os.Remove(filepath.Join(dir, "subject.json")); err != nil {
					t.Fatal(err)
				}
			} else {
				subject.DesignPage = ""
				b.writeJSON(filepath.Join(dir, "subject.json"), subject)
			}
			delete(returned, "wholePageDigest")
			delete(returned, "coverage")
			returned["findings"] = []any{evidenceFinding("F1", "## Collection:1", "")}
			returned["rigor"], returned["verdictMaterialCount"] = []any{evidenceRigor("F1")}, 1
			b.writeJSON(filepath.Join(dir, "return.json"), returned)
			record := b.job("rev1")
			delete(record, "engineBuild")
			b.writeJob(record)
			if scenario == "folded-without-page" {
				b.register(1, 20, []int64{1}, map[string]any{"findingId": "F1"})
			} else if _, err := dispatchcore.CritiqueRegisterAdvance(b.install, "rev1", "rev1"); err != nil {
				t.Fatalf("dispatcher could not fold legacy findings: %v", err)
			}
			if outcome, err := dispatchcore.CritiqueRegisterAdvance(b.install, "rev1", "rev1"); err != nil || outcome != "unchanged" {
				t.Fatalf("folded legacy round was recollected: %s %v", outcome, err)
			}
			result := b.review()
			if result.Outcome != intentConfirmed || !strings.Contains(result.Summary, "1 material") {
				t.Fatalf("legacy collection is stranded: %+v", result)
			}
			decided := b.decide(result, map[string]string{"F1": "accepted | specify the requirement | ## Collection:1"})
			closed := b.review("--dispositions", decided)
			if closed.Outcome != intentConfirmed || b.closes != 1 || b.job("rev1")["chainClosed"] != true {
				t.Fatalf("legacy recovery cannot close: %+v", closed)
			}
			if len(b.followUps) != 0 || b.fresh != 1 {
				t.Fatal("legacy recovery requested another examination")
			}
		})
	}
}
