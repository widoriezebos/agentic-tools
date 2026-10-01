package batchowner

import (
	"errors"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

// The join reads the goal's bound budget for the branch reader: a tier-1
// budget of zero review rounds (R-54-m1) lets its units join without a
// critic read; a tier-2 budget does not, whatever the caller asked.
func TestJoinPassesTheBudgetReadWaiverToTheBranchReader(t *testing.T) {
	t.Parallel()
	stop := errors.New("branch reader reached")
	for _, tc := range []struct {
		name   string
		rounds int64
		want   bool
	}{{"tier 1", 0, true}, {"tier 2", 2, false}} {
		var seen []bool
		dependencies := BatchJoinDependencies{
			Binding: func(string, string, time.Time) (dispatchcore.GoalBinding, error) {
				return dispatchcore.GoalBinding{File: &goal.GoalFile{Budget: &goal.Budget{ReviewRoundLimit: tc.rounds}}}, nil
			},
			member: func(request BatchJoinRequest) (batch.BranchMember, []byte, error) {
				seen = append(seen, request.ReadsWaived)
				return batch.BranchMember{}, nil, stop
			},
		}
		request := BatchJoinRequest{SeatRoot: "seat", LandingRoot: "lane", GoalID: "goal-tier", Last: true, ReadsWaived: !tc.want}
		if _, err := ExecuteBatchJoin(request, dependencies); !errors.Is(err, stop) {
			t.Fatalf("%s: join = %v", tc.name, err)
		}
		if len(seen) != 1 || seen[0] != tc.want {
			t.Fatalf("%s: the branch reader saw reads waived %v, want %v", tc.name, seen, tc.want)
		}
	}
}
