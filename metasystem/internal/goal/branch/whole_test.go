package branch

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

func TestUnitWholeTrailerAndStatus(t *testing.T) {
	t.Parallel()
	_, trailers, err := commitMessage(CommitRequest{GoalID: "goal-a", Unit: "u2", Kind: Unit, Whole: true}, "")
	if err != nil || trailers != "Goal-Unit: goal-a/u2\nGoal-Whole: goal-a" {
		t.Fatalf("last unit trailers=%q err=%v", trailers, err)
	}
	raw := rawTree("metasystem/a.go")
	calls := append(singleCommitTranscript(), rangeOutput(trailers+"\n", "show", "-s", "--format=%(trailers:only,unfold=true)", "tip"))
	calls = append(calls, treeCalls("tip", "base", raw)...)
	calls = append(calls, treeCalls("tip", "base", raw)...)
	commits, err := validateWithTranscript(t, "base", "tip", calls...)
	if err != nil || len(commits) != 1 || !commits[0].Whole || commits[0].Kind != Unit {
		t.Fatalf("marked commit no longer reads as unit: %+v %v", commits, err)
	}
	deps := defaultStatusDependencies()
	deps.validatedRange = func(string, string, string, string) ([]Commit, error) { return commits, nil }
	status, err := inspectStatus(rangeRepo, "base", "tip", "goal-a", deps)
	if err != nil || len(status.Units) != 1 || !status.Units[0].Whole {
		t.Fatalf("status lost goal end: %+v %v", status, err)
	}
	_, trailers, err = commitMessage(CommitRequest{GoalID: "goal-a", Unit: "u1", Kind: Unit}, "")
	if err != nil || strings.Contains(trailers, "Goal-Whole") {
		t.Fatalf("ordinary unit declares end: %q %v", trailers, err)
	}
}

func TestUnitWholeOnlyForItsGoal(t *testing.T) {
	t.Parallel()
	for _, suffix := range []string{"", "Goal-Whole: another-goal\n"} {
		t.Run(strings.TrimSpace(suffix), func(t *testing.T) {
			t.Parallel()
			stub := testgit.New(t, rangeOutput("Goal-Unit: goal-a/u1\n"+suffix, "show", "-s", "--format=%(trailers:only,unfold=true)", "tip"))
			info, err := kindOfWithGit(rangeRepo, "tip", "goal-a", rangeReader(stub))
			if err != nil || info.Whole || info.Kind != Unit {
				t.Fatalf("unmarked unit status: %+v %v", info, err)
			}
		})
	}
}
