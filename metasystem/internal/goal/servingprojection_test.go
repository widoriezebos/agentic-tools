package goal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// servingBed builds a converted checkout with the given goal files —
// the world the mission prompt's goal line reads after migration.
func servingBed(t *testing.T, machine string, files map[string]*GoalFile) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "metasystem.goal.machine", machine)
	run("config", "goal.sync-remote", "local")
	run("config", "user.name", "serving-fixture")
	run("config", "user.email", "serving-fixture@example.invalid")
	write := func(rel string, data []byte) {
		t.Helper()
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, data, 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", rel)
	}
	write("plans/goals/backlog.md", RenderRoot(&RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1",
		SyncMode: SyncLocal, Revision: 1,
	}))
	history := []HistoryLine{{
		At: "2026-08-23T00:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAV-bed-00000000",
		Verb: "open", Actor: machine + "+coordinator", Targets: []string{"any"}, Keep: -1,
	}}
	for id, f := range files {
		f.History = history
		write("plans/goals/"+id+".md", RenderFile(f))
	}
	run("commit", "-q", "-m", "serving bed")
	run("update-ref", AcceptedRef, "HEAD")
	return root
}

func TestServingProjectionConvertedClaimCarriesAppetite(t *testing.T) {
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"ship-it": {
			Id: "ship-it", State: "claimed", Intent: "Ship the whole thing", Origin: "main",
			NextStep: "Appetite: 4h — land it in pieces.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
			Claimed: &ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
		},
	})
	id, intent, appetite, ok := (&Store{Root: root}).ServingProjection()
	if !ok || id != "ship-it" || intent != "Ship the whole thing" || appetite != "4h" {
		t.Fatalf("this machine's claim serves with its appetite: %q %q %q %v", id, intent, appetite, ok)
	}
}

func TestServingProjectionForeignClaimServesNothing(t *testing.T) {
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"theirs": {
			Id: "theirs", State: "claimed", Intent: "Someone else's", Origin: "main",
			NextStep: "Appetite: 2h — elsewhere.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
			Claimed: &ClaimRecord{Machine: "bed-m2", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
		},
	})
	if _, _, _, ok := (&Store{Root: root}).ServingProjection(); ok {
		t.Fatal("a foreign claim must serve nothing here")
	}
}

func TestServingProjectionProseNextStepServesWithoutAppetite(t *testing.T) {
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"loose": {
			Id: "loose", State: "claimed", Intent: "Prose only", Origin: "main",
			NextStep: "Just do the thing when possible.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
			Claimed: &ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
		},
	})
	id, _, appetite, ok := (&Store{Root: root}).ServingProjection()
	if !ok || id != "loose" || appetite != "" {
		t.Fatalf("a prose next step serves with no appetite token: %q %q %v", id, appetite, ok)
	}
}
