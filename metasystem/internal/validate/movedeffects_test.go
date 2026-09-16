package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func movedEffectsPage(row string) []byte {
	return []byte("## Moved effects\n\n| Effect | From | To | Code |\n|---|---|---|---|\n" + row + "\n")
}

func TestMovedEffectsRules(t *testing.T) {
	cases := []struct{ name, row, code string }{
		{"NO-EFFECT", "| | old | new | `a/b` |", "MOVED-EFFECT-NO-EFFECT"},
		{"NO-OLD-OWNER", "| effect | | new | `a/b` |", "MOVED-EFFECT-NO-OLD-OWNER"},
		{"NO-NEW-OWNER", "| effect | old | `todo` | `a/b` |", "MOVED-EFFECT-NO-NEW-OWNER"},
		{"SAME-OWNER", "| effect | Owner | owner | `a/b` |", "MOVED-EFFECT-SAME-OWNER"},
		{"NO-CODE", "| effect | old | new | code.go |", "MOVED-EFFECT-NO-CODE"},
		{"CODE-ABSENT", "| effect | old | new | `missing/file.go:2` |", "MOVED-EFFECT-CODE-ABSENT"},
		{"MALFORMED-ROW", "| effect | old | new |", "MOVED-EFFECT-MALFORMED-ROW"},
		{"NO-ROWS", "", "MOVED-EFFECTS-NO-ROWS"},
		{"CONTRADICTORY", "| effect | old | new | `a/b` |\nNo owner moves.", "MOVED-EFFECTS-CONTRADICTORY"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			report := CheckMovedEffects(movedEffectsPage(test.row), func(path string) bool { return path == "a/b" })
			for _, problem := range report.Problems {
				if problem.Code == test.code {
					return
				}
			}
			t.Fatalf("problems = %+v, want %s", report.Problems, test.code)
		})
	}
	t.Run("none-declared", func(t *testing.T) {
		report := CheckMovedEffects([]byte("## Moved effects\nNo owner moves.\n"), func(string) bool { return false })
		if report.Inventory != "none-declared" || len(report.Problems) != 0 {
			t.Fatalf("report = %+v", report)
		}
	})
	for _, test := range []struct {
		name, page, inventory string
		rows                  int
	}{
		{"well-formed", string(movedEffectsPage("| effect | old | new | `a/b#symbol` |")), "present", 1},
		{"numbered-heading", "## 7. Moved effects\nNo owner moves.\n", "none-declared", 0},
		{"fenced", "```\n## Moved effects\nNo owner moves.\n```\n", "absent", 0},
		{"later-heading", "## Moved effects\nNo owner moves.\n## Later\n" + string(movedEffectsPage("| effect | old | new | `a/b` |")), "none-declared", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			report := CheckMovedEffects([]byte(test.page), func(string) bool { return true })
			if report.Inventory != test.inventory || len(report.Rows) != test.rows || len(report.Problems) != 0 {
				t.Fatalf("report = %+v", report)
			}
		})
	}
}

func TestStopDecisionsPageCarriesTheFirstInventory(t *testing.T) {
	metasystem, _ := filepath.Abs("../..")
	page, err := os.ReadFile(filepath.Join(metasystem, "plans/stop-decisions-record-deadline-evidence-design.md"))
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Dir(metasystem)
	report := CheckMovedEffects(page, func(path string) bool {
		info, err := os.Stat(filepath.Join(repo, path))
		return err == nil && (info.Mode().IsRegular() || info.IsDir())
	})
	if report.Inventory != "present" || len(report.Rows) < 12 || len(report.Problems) != 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestMovesOwnerFixtureCarriesNoInventory(t *testing.T) {
	page, err := os.ReadFile("testdata/moved-effects/moves-owner-without-inventory.md")
	if err != nil {
		t.Fatal(err)
	}
	report := CheckMovedEffects(page, func(string) bool { return false })
	if report.Inventory != "absent" || len(report.Problems) != 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestDesignCriticPacketCarriesMovedEffectsCheck(t *testing.T) {
	metasystem, _ := filepath.Abs("../..")
	data, err := os.ReadFile(filepath.Join(metasystem, "scripts/agents/role-packets.json"))
	if err != nil {
		t.Fatal(err)
	}
	var recipe struct {
		Roles map[string]struct {
			Sources []struct {
				Path string `json:"path"`
			} `json:"sources"`
		} `json:"roles"`
	}
	if err := json.Unmarshal(data, &recipe); err != nil {
		t.Fatal(err)
	}
	var packet strings.Builder
	for _, source := range recipe.Roles["design-critic"].Sources {
		root := metasystem
		if strings.HasPrefix(source.Path, "metasystem/") {
			root = filepath.Dir(metasystem)
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(source.Path)))
		if err != nil {
			t.Fatal(err)
		}
		packet.Write(content)
	}
	for _, required := range []string{"validate moved-effects", "Moved effects", "MOVED-EFFECTS-"} {
		if !strings.Contains(packet.String(), required) {
			t.Errorf("design-critic packet does not contain %q", required)
		}
	}
}
