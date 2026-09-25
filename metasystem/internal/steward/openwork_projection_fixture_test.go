package steward

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const unenrolledMachineError = "no machine nickname is enrolled and hostnames are never published: run  git config metasystem.goal.machine <nickname>  once on this machine"

// convertedProjectionBed declares accepted goal bytes while leaving live
// process and job records on disk for the normal liveness join.
func convertedProjectionBed(t *testing.T, machine string, goals map[string]*goal.GoalFile) (string, openWorkDependencies) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	rootRecord := &goal.RootRecord{
		Identity:      "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		FormatVersion: "1",
		SyncMode:      goal.SyncLocal,
		Revision:      1,
	}
	files := map[string][]byte{"plans/goals/backlog.md": goal.RenderRoot(rootRecord)}
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
	writeStewardRecord(t, ledgerAttentionStatePath(root), map[string]any{
		"schema": ledgerAttentionStateSchema, "lastOutcome": "local",
	})

	var routedAt time.Time
	dependencies := openWorkDependencies{
		NewWorld: func(actualRoot string) bool {
			if actualRoot != root {
				t.Fatalf("converted world routed at %q, want %q", actualRoot, root)
			}
			routedAt = time.Now()
			return true
		},
		ReadClaimableBudgetedWork: func(actualRoot string, now time.Time) (goal.ClaimableBudgetedWork, error) {
			if actualRoot != root || routedAt.IsZero() || now.Before(routedAt) || now.After(time.Now()) {
				t.Fatalf("work reader received root %q and time %s after route at %s; want root %q and a fresh call time", actualRoot, now, routedAt, root)
			}
			if machine == "" {
				return goal.ClaimableBudgetedWork{}, errors.New(unenrolledMachineError)
			}
			tree, problems := goal.ParseTreeFiles(files)
			if len(problems) > 0 {
				t.Fatalf("declared accepted goal files did not parse: %v", problems)
			}
			projection := goal.Projection{
				Root: root, Tree: tree,
				Horizon: goal.ApprovalHorizon{Now: now},
			}
			return goal.ClaimableWorkFromProjection(projection, machine, identity.KernelProber{})
		},
	}
	return root, dependencies
}
