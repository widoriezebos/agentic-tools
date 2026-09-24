package steward

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func budgetHealthProjectionBed(t *testing.T, now time.Time, goals map[string]*goal.GoalFile) (string, goal.Projection, error) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	record := &goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1",
		SyncMode: goal.SyncLocal, Revision: 1,
	}
	files := map[string][]byte{"plans/goals/backlog.md": goal.RenderRoot(record)}
	for id, file := range goals {
		files["plans/goals/"+id+".md"] = goal.RenderFile(file)
	}
	for relative, data := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tree, problems := goal.ParseTreeFiles(files)
	if len(problems) != 0 {
		return root, goal.Projection{}, &goal.TreeReadError{Tip: "fixture", Problems: problems, Files: files}
	}
	return root, goal.Projection{Root: root, Tree: tree, Horizon: goal.ApprovalHorizon{Now: now}}, nil
}
