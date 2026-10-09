package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestWorkBriefCarriesSelectedDecision(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"named", "named colon", "named backticks", "quoted selection", "scope moved before units", "scope split before units", "empty units section", "numbered units", "qualified units", "numbered decisions", "references", "invalid range", "link", "second page", "sole unit", "no selection", "missing mapping", "ambiguous mapping", "ambiguous reference", "duplicate ownership", "unpublished items"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			header := func(id string) string {
				return "# Selected design\n\n- Kind: design\n- Id: " + id + "\n- Status: accepted\n- Goals: " + bed.id + "\n"
			}
			row := "| B1 | 140 | 360 | selected requirement |"
			passage := "### B1 — Selected behavior\n\nCarry this exact folded change: item BR1-F2, tested by TestSelectedPublicVerb.\n\n#### Folded acceptance\n\nKeep punctuation,  spacing and `SelectedOwner` verbatim.\n"
			extra := ""
			switch scenario {
			case "named backticks", "quoted selection":
				row = strings.Replace(row, "B1", "`B1`", 1)
				passage = strings.Replace(passage, "B1 — Selected behavior", "`B1` — Selected behavior", 1)
			case "numbered decisions":
				row = "| B1 | 140 | 360 | Decisions 2–3 |"
				passage = strings.Replace(passage, "B1 — Selected behavior", "2. Selected behavior", 1)
				extra = "\n### 3. Selected companion\n\nCarry item BR1-F3 and TestSelectedCompanion.\n"
			case "named colon":
				passage = strings.Replace(passage, "B1 — Selected behavior", "B1: Selected behavior", 1)
			case "ambiguous reference":
				row = "| B1 | 140 | 360 | Decision 2 |"
				passage = strings.Replace(passage, "B1 — Selected behavior", "Decision 2 — Selected behavior", 1)
				extra = "\n### Decision 2 — Different behavior\n\nA contradictory mapping.\n"
			case "references":
				row = "| B1 | 140 | 360 | Decisions 2–3 |"
				passage = strings.Replace(passage, "B1 — Selected behavior", "Decision 2 — Selected behavior", 1)
				extra = "\n### Decision 3 — Selected companion\n\nCarry item BR1-F3 and TestSelectedCompanion.\n"
			case "invalid range":
				row = "| B1 | 140 | 360 | Decisions 3–2 |"
			case "link":
				row = "| B1 | 140 | 360 | [Decision](#decision-2) |"
				passage = strings.Replace(passage, "B1 — Selected behavior", "Decision 2", 1)
			case "missing mapping":
				row = "| B1 | 140 | 360 | Decision 99 |"
				passage = strings.Replace(passage, "B1 — Selected behavior", "Unmapped behavior", 1)
			case "ambiguous mapping":
				extra = "\n### B1 — Selected behavior\n\nAnother owner for the same heading.\n"
			}
			b2row := "| B2 | 230 | 650 | Decision 1 |\n"
			if scenario == "sole unit" {
				b2row = ""
			}
			table := "\n## Units\n\n| Unit | Production lines | Changed lines | Specification |\n| --- | ---: | ---: | --- |\n" + b2row + row + "\n"
			tests := "\n## A test through the public verb\n\n| Unit / proposed test | Scenario | Mutation |\n| --- | --- | --- |\n| B1 `TestSelectedPublicVerb` | BR1-F2 exact folded result | Omit folded behavior |\n| B2 `TestOtherPublicVerb` | OTHER_UNIT_TEST | Omit other behavior |\n"
			items := "\n## Acceptance items\n\n| Unit | Item | Test |\n| --- | --- | --- |\n| B1 | BR1-F2 | TestSelectedPublicVerb |\n| B2 | OTHER_UNIT_ITEM | TestOtherPublicVerb |\n"
			limits := "\n## Global limits\n\nAt most five units, each at most 250 production lines. Do not open local settings.\n\n## Return\n\nReport proof and exits.\n"
			body := header("selected-design") + table + "\n### Decision 1 — Other behavior\n\nOTHER_UNIT_SCOPE\n\n#### Folded acceptance\n\nOTHER_UNIT_ACCEPTANCE\n\n#### Scope limits\n\nOTHER_UNIT_LIMITS\n\n" + passage + extra + tests + items + limits + "\n## Estimates\n\n| Unit | Changed-line allocation |\n| --- | ---: |\n| B1 | 360 |\n"
			switch scenario {
			case "scope moved before units":
				body = strings.Replace(body, table, "\n## Scope moved before the units\n\nTransferred scope.\n"+table, 1)
			case "scope split before units":
				body = strings.Replace(body, table, "\n### Scope split before the units\n\nTransferred scope.\n"+table, 1)
			case "empty units section":
				body = strings.Replace(body, table, "\n## Units (deferred)\n\nNo table here.\n"+table, 1)
			case "numbered units":
				body = strings.Replace(body, "## Units\n", "## 8. Units\n", 1)
			case "qualified units":
				body = strings.Replace(body, "## Units\n", "## 7. Units, in landing order (revision 2)\n", 1)
			case "numbered decisions":
				body = strings.Replace(body, "### Decision 1 — Other behavior", "## Other numbered sections\n\n### 2. Unrelated behavior\n\nOTHER_NUMBERED_SCOPE\n\n## Decisions, with code sites read\n\n### 1. Other behavior", 1)
				body = strings.Replace(body, tests, "\n## After decisions\n\n### 3. Unrelated companion\n\nOTHER_NUMBERED_SCOPE\n"+tests, 1)
			}
			if scenario == "unpublished items" {
				body = strings.Replace(body, "- Status: accepted\n", "- Status: accepted\n- Convergence: unpublished-exit\n", 1)
			}
			home := filepath.Join(bed.stateRoot(), "plans", "designs")
			if err := os.MkdirAll(home, 0700); err != nil {
				t.Fatal(err)
			}
			write := func(name, data string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(home, name), []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "second page" {
				write("a-other.md", header("other-design")+strings.Replace(table, row+"\n", "", 1)+"\n### Decision 1 — Other behavior\n\nOTHER_UNIT_SCOPE\n")
				body = strings.Replace(body, b2row, "", 1)
			}
			write("b-selected.md", body)
			if scenario == "duplicate ownership" {
				write("duplicate.md", strings.Replace(body, "selected-design", "duplicate-design", 1))
			}
			args := []string{"work", "brief", bed.id, "--out", "selected.md"}
			if scenario != "no selection" && scenario != "sole unit" {
				selected := "B1"
				if scenario == "quoted selection" {
					selected = "`B1`"
				}
				args = append(args, "--work", selected)
			}
			code, result, _ := bed.work(args...)
			path := filepath.Join(bed.root(), "selected.md")
			output, err := os.ReadFile(path)
			if scenario == "duplicate ownership" {
				if code != 1 || result.Outcome != intentRefused || !os.IsNotExist(err) || !strings.Contains(result.Summary, "duplicate accepted ownership") {
					t.Fatalf("duplicate ownership wrote a brief: %d %+v %v", code, result, err)
				}
				return
			}
			if code != 0 || err != nil {
				t.Fatalf("brief: %d %+v %v", code, result, err)
			}
			text := string(output)
			if scenario == "invalid range" || scenario == "no selection" || scenario == "missing mapping" || scenario == "ambiguous mapping" || scenario == "ambiguous reference" || scenario == "unpublished items" {
				want := map[string]string{"invalid range": "the Decision range", "no selection": "select one declared unit", "missing mapping": "Decision 99", "ambiguous mapping": "ambiguous Decision heading", "ambiguous reference": "ambiguous Decision heading", "unpublished items": "published convergence body/items"}[scenario]
				if !strings.Contains(text, "MISSING DECISION") || !strings.Contains(text, want) {
					t.Fatalf("missing evidence silently accepted (%s):\n%s", want, text)
				}
				if scenario == "unpublished items" {
					build := append([]string{"work", "build", bed.id, "B1", "--brief", "selected.md", "--lines", "360"}, workCheck...)
					if exit, refused, _ := bed.work(build...); exit == 0 || refused.Outcome != intentRefused || len(bed.starter.launched()) != 0 {
						t.Fatalf("unpublished items launched: %d %+v", exit, refused)
					}
				}
				return
			}
			for _, want := range []string{passage, extra, "BR1-F2", "TestSelectedPublicVerb", "Omit folded behavior", "Changed-line allocation: 360.", "Production estimate: 140.", "At most five units, each at most 250 production lines.", "id: selected-design; body sha256:"} {
				if !strings.Contains(text, want) {
					t.Fatalf("brief lacks %q:\n%s", want, text)
				}
			}
			for _, forbidden := range []string{"OTHER_UNIT_SCOPE", "OTHER_NUMBERED_SCOPE", "OTHER_UNIT_ACCEPTANCE", "OTHER_UNIT_LIMITS", "OTHER_UNIT_TEST", "OTHER_UNIT_ITEM", "TestOtherPublicVerb", "| B2 |"} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("another unit leaked %q:\n%s", forbidden, text)
				}
			}
			if exit, replay, _ := bed.work(args...); exit != 0 || replay.Outcome != intentUnchanged {
				t.Fatalf("same-byte replay: %d %+v", exit, replay)
			}
			if err := os.WriteFile(path, []byte("human edit\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if exit, refused, _ := bed.work(args...); exit != 1 || refused.Outcome != intentRefused {
				t.Fatalf("overwrote human edit: %d %+v", exit, refused)
			}
			kept, err := os.ReadFile(path)
			if err != nil || string(kept) != "human edit\n" {
				t.Fatalf("human file changed: %q %v", kept, err)
			}
			fresh := append([]string(nil), args...)
			for i, arg := range fresh {
				if arg == "selected.md" {
					fresh[i] = "selected-new.md"
				}
			}
			if exit, repaired, _ := bed.work(fresh...); exit != 0 || repaired.Outcome != intentConfirmed {
				t.Fatalf("new-path remedy failed: %d %+v", exit, repaired)
			}
			if scenario == "named" {
				build := append([]string{"work", "build", bed.id, "B1", "--brief", "selected-new.md"}, workCheck...)
				exit, built, _ := bed.work(build...)
				if exit != 0 {
					t.Fatalf("build: %d %+v", exit, built)
				}
				plan, err := launch.ReadUnitPlan(resultData(t, built)["plan"].(string))
				if err != nil {
					t.Fatal(err)
				}
				retained, err := os.ReadFile(plan.Build.Brief)
				if err != nil {
					t.Fatal(err)
				}
				for _, want := range []string{passage, "Production estimate: 140.", "| B1 | 360 |", "BR1-F2", "TestSelectedPublicVerb"} {
					if !strings.Contains(string(retained), want) {
						t.Fatalf("retained build lacks %s:\n%s", fmt.Sprintf("%q", want), retained)
					}
				}
			}
		})
	}
}
