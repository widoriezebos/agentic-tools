package dispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

type goalMutationBed struct {
	root       string
	repository *testgoal.Repository
	reads      goalAdmissionReads
}

func newGoalMutationBed(t *testing.T) *goalMutationBed {
	t.Helper()
	seed := newGoalAdmissionBed(t, 2)
	files := make(map[string][]byte)
	err := filepath.WalkDir(filepath.Join(seed.root, "plans", "goals"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(seed.root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = append([]byte(nil), data...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	repository := testgoal.New(files, time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC), "0000000000000000000000000000000000000001")
	bed := &goalMutationBed{root: seed.root, repository: repository}
	bed.reads = goalAdmissionReads{
		NewWorld: func(root string) bool { return root == bed.root },
		ResolveEndpoint: func(root string) (goal.Endpoint, error) {
			if root != bed.root {
				return goal.Endpoint{}, fmt.Errorf("undeclared goal endpoint root %q", root)
			}
			return bed.endpoint(), nil
		},
		ResolveMachine: func(root string) (string, error) {
			if root != bed.root {
				return "", fmt.Errorf("undeclared goal machine root %q", root)
			}
			return "bed-m1", nil
		},
	}
	return bed
}

func (bed *goalMutationBed) endpoint() goal.Endpoint {
	return goal.Endpoint{Root: bed.root, Remote: "local", Branch: goal.LocalLedgerBranch, Repository: bed.repository}
}

func (bed *goalMutationBed) binding(id string, now time.Time) (GoalBinding, error) {
	return resolveGoalBindingWithReads(bed.root, id, now, bed.reads)
}

func (bed *goalMutationBed) admission(id string, revision, cap uint64, now time.Time) (GoalRevisionAdmission, error) {
	return evaluateGoalRevisionAdmissionWithReads(bed.root, id, revision, cap, now, bed.reads)
}

func (bed *goalMutationBed) stops(now time.Time) ([]StopRoute, error) {
	return findBreachStopsWithReads(bed.root, now, bed.reads)
}

func (bed *goalMutationBed) stop(id string, revision uint64, now time.Time) (goal.StopBatch, error) {
	return ensureBreachStopWithReads(bed.root, id, revision, now, bed.reads)
}

func (bed *goalMutationBed) close(job string) (string, error) {
	return critiqueRegisterCloseWithReads(bed.root, job, bed.reads)
}

func (bed *goalMutationBed) parsedAcceptedGoal(t *testing.T, id string) *goal.GoalFile {
	t.Helper()
	tip, present, err := bed.repository.Accepted()
	if err != nil || !present {
		t.Fatalf("accepted goal tip: %q present=%t err=%v", tip, present, err)
	}
	files, err := bed.repository.Files(tip, "plans/goals/"+id+".md")
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(files["plans/goals/"+id+".md"])
	if file == nil || len(problems) != 0 {
		t.Fatalf("accepted goal file does not parse: %+v %v", file, problems)
	}
	return file
}
