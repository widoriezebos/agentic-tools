package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

func TestGoalDoneMaterialNotes(t *testing.T) {
	t.Parallel()
	material, nonMaterial := true, false
	for _, tc := range []struct {
		name     string
		block    bool
		material *bool
	}{{"ten non-material notes", false, nil}, {"material note", true, &material}, {"unspecified materiality", true, nil}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bed := newGoalCLIBed(t, goalCLISeed{amend: func(files map[string]*goal.GoalFile) {
				for i := 1; i <= 10; i++ {
					files["fix-docs"].ReadItems = append(files["fix-docs"].ReadItems, goal.ReadItem{
						ID: fmt.Sprintf("note-%d", i), Read: "note", Text: "Follow up.", Material: &nonMaterial,
						State: goal.ReadItemOpen, AddedAt: goalCLISeedNow.Format("2006-01-02T15:04:05Z"),
					})
				}
				if tc.block {
					files["fix-docs"].ReadItems = append(files["fix-docs"].ReadItems, goal.ReadItem{
						ID: "critic-1", Read: "critic", Text: "Fix the defect.", Material: tc.material,
						State: goal.ReadItemOpen, AddedAt: goalCLISeedNow.Format("2006-01-02T15:04:05Z"),
					})
				}
			}})
			before := bed.tip()
			code, out, errOut := bed.public("goal", "done", "fix-docs", "--reason", "Finished.", "--by", "Wido", "--json")
			if tc.block {
				var result intentResult
				if code == 0 || json.Unmarshal([]byte(out), &result) != nil || bed.tip() != before ||
					!slices.Contains(result.Details, "refusal code: "+goal.DoneReadItemsOpenCode) ||
					!strings.Contains(result.Summary, "critic-1") || strings.Contains(result.Summary, "note-") {
					t.Fatalf("material refusal: code=%d out=%q err=%q", code, out, errOut)
				}
				return
			}
			file, problems := goal.ParseFile([]byte(bed.goalRecord("fix-docs")))
			if code != 0 || len(problems) != 0 || file.State != goal.StateDone || len(file.ReadItems) != 10 {
				t.Fatalf("conclude with notes: code=%d out=%q err=%q problems=%v", code, out, errOut, problems)
			}
			notes := gcliLedgerMust(t, bed, "goal", "notes", "fix-docs", "--all")
			for _, item := range file.ReadItems {
				if item.State != goal.ReadItemOpen || item.Material == nil || *item.Material || !strings.Contains(notes, item.ID+" [open]") {
					t.Fatalf("concluded goal lost its open non-material note: %+v notes=%q", item, notes)
				}
			}
		})
	}
}

func TestGoalNotesMaterialityThroughPublicVerb(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"--not-material", "--material", ""} {
		t.Run("materiality="+flag, func(t *testing.T) {
			t.Parallel()
			bed := newGoalCLIBed(t, goalCLISeed{})
			for i := 1; i <= 10; i++ {
				gcliLedgerMust(t, bed, "goal", "notes", "fix-docs", "--read", "critic", "--add", fmt.Sprintf("Observation %d.", i), "--not-material")
			}
			if flag != "--not-material" {
				args := []string{"goal", "notes", "fix-docs", "--read", "critic", "--add", "Fix the defect."}
				if flag != "" {
					args = append(args, flag)
				}
				gcliLedgerMust(t, bed, args...)
			}
			file, problems := goal.ParseFile([]byte(bed.goalRecord("fix-docs")))
			if len(problems) > 0 || file.ReadItems[0].Material == nil || *file.ReadItems[0].Material {
				t.Fatalf("added notes lost materiality: %+v %v", file.ReadItems, problems)
			}
			if flag != "--not-material" {
				// The impact is printed on stderr before the person's act; JSON remains on stdout.
				before := bed.tip()
				code, out, errOut := bed.public("goal", "done", "fix-docs", "--reason", "Finished.", "--by", "Wido", "--json")
				if code == 0 || bed.tip() != before || !strings.Contains(errOut, "Impact:") {
					t.Fatalf("material refusal changed state or lost impact: code=%d out=%q err=%q", code, out, errOut)
				}
				var result intentResult
				if json.Unmarshal([]byte(out), &result) != nil || !strings.Contains(result.Summary, "critic-11") || strings.Contains(result.Summary, "critic-1,") || !slices.Contains(result.Details, "refusal code: "+goal.DoneReadItemsOpenCode) {
					t.Fatalf("refusal must name only the material note: %s", out)
				}
				return
			}
			gcliLedgerMust(t, bed, "goal", "done", "fix-docs", "--reason", "Finished.", "--by", "Wido")
			file, problems = goal.ParseFile([]byte(bed.goalRecord("fix-docs")))
			notes := gcliLedgerMust(t, bed, "goal", "notes", "fix-docs", "--all")
			if len(problems) > 0 || file.State != goal.StateDone || len(file.ReadItems) != 10 {
				t.Fatalf("goal did not conclude with ten notes: %+v %v", file, problems)
			}
			for _, item := range file.ReadItems {
				if item.Material == nil || *item.Material || item.State != goal.ReadItemOpen || !strings.Contains(notes, item.ID+" [open]") {
					t.Fatalf("concluded goal lost its note: %+v %q", item, notes)
				}
			}
		})
	}
}

func TestGoalNotesLegacyMateriality(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	gcliLedgerMust(t, bed, "goal", "notes", "fix-docs", "--read", "critic", "--add", "Fix the legacy defect.")
	files, err := bed.repo.Files(bed.tip(), "")
	if err != nil {
		t.Fatal(err)
	}
	path := "plans/goals/fix-docs.md"
	body, _, _ := strings.Cut(strings.ReplaceAll(string(files[path]), " material=true", ""), "Integrity: sha256=")
	files[path] = []byte(body + "Integrity: sha256=" + goal.IntegrityDigest([]byte(body)) + "\n")
	bed.repo = testgoal.New(files, bed.clock(), bed.tip())
	if strings.Contains(bed.goalRecord("fix-docs"), "material=") {
		t.Fatal("legacy fixture carries a materiality key")
	}
	gcliLedgerRefused(t, bed, goal.DoneReadItemsOpenCode, "goal", "done", "fix-docs", "--reason", "Finished.", "--by", "Wido", "--json")
}

func TestGoalNotesMaterialityFlagsRefuseConflicts(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	gcliLedgerRefused(t, bed, "choose one", "goal", "notes", "fix-docs", "--read", "critic", "--add", "Observation.", "--material", "--not-material")
	gcliLedgerRefused(t, bed, "only goes with adding", "goal", "notes", "fix-docs", "--not-material")
}
