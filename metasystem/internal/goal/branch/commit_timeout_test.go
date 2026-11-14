package branch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestCommitAccessReadTimeoutEffects(t *testing.T) {
	t.Parallel()
	for _, afterCommit := range []bool{false, true} {
		for name, cause := range map[string]error{
			"projection": &goal.ReadTimeout{Limit: 4 * time.Second},
			"process":    boundedexec.ErrTimedOut,
			"transport":  context.DeadlineExceeded,
			"authority":  errors.New("claim belongs to machine-b+lineage-b"),
			"canceled":   context.Canceled,
		} {
			name, cause, afterCommit := name, cause, afterCommit
			if afterCommit {
				name += "-after-commit"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				f := newPolicyFixture(t)
				req := f.req("read-timeout")
				if afterCommit {
					f.expect("tip-goal", "remote", "head-root", "tip-origin", "staged", "patch", "open", "apply", "head-scratch", "head-scratch", "commit", "head-scratch", "range-new", "close")
				}
				checks := 0
				req.CheckClaim = func() error {
					checks++
					if afterCommit && checks == 1 {
						return nil
					}
					return cause
				}
				tip, err := commitStaged(req, f.repository())
				var operation *OpError
				wantCommit := ""
				if afterCommit {
					wantCommit = policyNew
				}
				if tip != "" || !errors.Is(err, cause) || !errors.As(err, &operation) || operation.CreatedCommit != wantCommit || operation.Code != NotHolderCode || err.Error() != cause.Error() || goal.IsReadTimeout(err) != goal.IsReadTimeout(cause) {
					t.Fatalf("commit = %s, error=%+v, operation=%+v", tip, err, operation)
				}
				f.assertState(policyState{headRef: "refs/heads/main", headCommit: policyBase, index: policyIndex, status: "staged:metasystem/code.go"})
			})
		}
	}
}
