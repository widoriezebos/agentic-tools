package branch

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestCommitAccessReadTimeoutEffects(t *testing.T) {
	t.Parallel()
	if os.Getenv("RESUMED_DROP_TIMEOUT_CHILD") == "1" {
		resumedDropTimeoutEffects(t)
		return
	}
	t.Run("resumed-drop", func(t *testing.T) {
		t.Parallel()
		bin := t.TempDir()
		// Only the scratch-token adapter's installation-prefix read crosses Git.
		// The child has this stub as its entire PATH; all commit facts are injected.
		if err := testexec.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\ncase \" $* \" in *\" rev-parse --show-prefix \"*) exit 0;; *) exit 97;; esac\n"), 0700); err != nil {
			t.Fatal(err)
		}
		command := exec.Command(os.Args[0], "-test.run=^TestCommitAccessReadTimeoutEffects$", "-test.count=1")
		command.Env = append(os.Environ(), "PATH="+bin, "RESUMED_DROP_TIMEOUT_CHILD=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("resumed drop: %v\n%s", err, output)
		}
	})
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

func resumedDropTimeoutEffects(t *testing.T) {
	t.Helper()
	for name, cause := range map[string]error{
		"projection": &goal.ReadTimeout{Limit: 4 * time.Second},
		"process":    boundedexec.ErrTimedOut,
		"transport":  context.DeadlineExceeded,
		"authority":  errors.New("claim belongs to another holder"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newPolicyFixture(t)
			req := f.req("resumed-drop")
			req.Kind, req.ResumeWorktree = Drop, f.scratch
			req.KeepWorktree = func() bool { return true }
			req.CheckClaim = func() error { return cause }
			r := f.repository()
			r.facts.Suffix = func(string, string, string) ([]string, error) { return []string{policyNew}, nil }
			r.facts.Ancestor = func(string, string, string) (bool, error) { return true, nil }
			r.facts.Kind = func(string, string, string) (KindInfo, error) {
				return KindInfo{Kind: Drop, Unit: req.Unit, Operation: req.OpID}, nil
			}
			r.facts.Index = func(string) (string, error) { return policyNewTree, nil }
			r.facts.Tree = func(string) (string, error) { return policyNewTree, nil }
			f.expect("head-scratch", "head-scratch", "range-new")
			tip, err := r.buildCommitOnto(req, commitBranchState{baseTip: policyBase}, "unused", "unused", nil)
			var operation *OpError
			if tip != "" || !errors.Is(err, cause) || !errors.As(err, &operation) || operation.CreatedCommit != policyNew || goal.IsReadTimeout(err) != goal.IsReadTimeout(cause) {
				t.Fatalf("retained commit lost: tip=%s operation=%+v err=%v", tip, operation, err)
			}
		})
	}
}
