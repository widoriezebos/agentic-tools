package cadence

import (
	"errors"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginecause"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

// A lane engine behind the fetched trunk is re-armed at that trunk, as a
// batch proof re-arms its base, and the preparation is asked again; a
// preparation that still refuses is a preparation refusal, never an
// unreadable ledger.
func TestCadencePreparationRearmsAnEngineBehindTheTrunk(t *testing.T) {
	t.Parallel()
	behind := enginecause.Refuse("engine-behind-tip", []enginecause.Fact{enginecause.Path("checkout", "/lane"), enginecause.Value("destination", "abc")},
		"this checkout's engine is behind the landing branch")
	prepares, rearmed := 0, []string{}
	owner := Owner{
		Prepare: func(testrun.SelectionRequest) (testrun.Preparation, error) {
			prepares++
			if prepares == 1 {
				return testrun.Preparation{}, behind
			}
			return testrun.Preparation{CandidateTree: "tree"}, nil
		},
		Rearm: func(root, commit string) error { rearmed = append(rearmed, root+"@"+commit); return nil },
	}
	prepared, err := prepareCadence("/lane", "commit-1", "tree", owner)
	if err != nil || prepared.CandidateTree != "tree" || prepares != 2 || len(rearmed) != 1 || rearmed[0] != "/lane@commit-1" {
		t.Fatalf("prepared=%+v err=%v prepares=%d rearmed=%v", prepared, err, prepares, rearmed)
	}

	childFailed := enginecause.Refuse("child-failed", []enginecause.Fact{enginecause.Path("engine", "/pin"), enginecause.Value("command", "x")}, "the pinned engine refused")
	owner.Prepare = func(testrun.SelectionRequest) (testrun.Preparation, error) { return testrun.Preparation{}, childFailed }
	rearmed = nil
	_, err = prepareCadence("/lane", "commit-1", "tree", owner)
	var refusal Refusal
	if !errors.As(err, &refusal) || refusal.Code != cadencePreparationRefused || !strings.Contains(refusal.Detail, "cause=child-failed") || len(rearmed) != 0 {
		t.Fatalf("an engine refusal that no re-arm fixes = %v (rearmed %v)", err, rearmed)
	}

	owner.Prepare = func(testrun.SelectionRequest) (testrun.Preparation, error) { return testrun.Preparation{}, behind }
	owner.Rearm = func(string, string) error { return errors.New("the lane checkout is busy") }
	_, err = prepareCadence("/lane", "commit-1", "tree", owner)
	if !errors.As(err, &refusal) || refusal.Code != cadencePreparationRefused || !strings.Contains(refusal.Detail, "the lane checkout is busy") {
		t.Fatalf("a failed re-arm = %v", err)
	}
}
