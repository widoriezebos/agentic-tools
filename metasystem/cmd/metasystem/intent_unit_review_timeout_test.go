package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

type reviewTimeoutGit struct{ workGit }

func (g reviewTimeoutGit) Run(dir string, env []string, args ...string) ([]byte, error) {
	if slices.Contains(args, "--raw") {
		return []byte(":100644 100644 " + strings.Repeat("a", 40) + " " + strings.Repeat("b", 40) + " M\x00unit.go\x00"), nil
	}
	return g.workGit.Run(dir, env, args...)
}

func TestUnitReviewReadTimeoutBoundaries(t *testing.T) {
	t.Parallel()
	for _, boundary := range []string{"entry", "owner", "created"} {
		for name, cause := range map[string]error{
			"projection": &goal.ReadTimeout{Limit: 4 * time.Second},
			"process":    boundedexec.ErrTimedOut,
			"transport":  context.DeadlineExceeded,
			"authority":  errors.New("claim belongs to another session"),
			"canceled":   context.Canceled,
		} {
			boundary, cause := boundary, cause
			t.Run(boundary+"/"+name, func(t *testing.T) {
				t.Parallel()
				b, owners, _ := rebaseIntentBed(t)
				git := reviewTimeoutGit{workGit{b}}
				runner := owners.work.units(stateroot.Layout{})
				runner.Git = git
				owners.work.units = func(stateroot.Layout) *launch.UnitRunner { return runner }
				originalGit := owners.work.git
				owners.work.git = func(dir string, args ...string) ([]byte, error) {
					if slices.Contains(args, "--raw") || slices.Contains(args, "add") || args[0] == "write-tree" || slices.Contains(args, "--cached") {
						return git.Run(dir, nil, args...)
					}
					return originalGit(dir, args...)
				}
				brief := b.brief("timeout.md", "Build the unit.\n")
				code, built := b.runJSON(owners, append([]string{"work", "build", b.id, "u1", "--brief", brief, "--lines", "5", "--reason", "exercise ledger expiry", "--by", "Wido"}, workCheck...)...)
				if code != 0 {
					t.Fatalf("build = %+v, code=%d", built, code)
				}
				run := resultData(t, built)["run"].(string)
				command := []string{"work", "review", "run:" + run}
				operation := &branch.OpError{Code: branch.NotHolderCode, Message: cause.Error(), Cause: cause}
				if boundary == "created" {
					operation.CreatedCommit = strings.Repeat("c", 40)
				}
				owners.connection.claimCheck = func(string, string, goal.Endpoint) func() error {
					return func() error {
						if boundary == "entry" {
							return cause
						}
						return nil
					}
				}
				called := false
				owners.connection.commitToken = func(_ string, commit func() error) error { return commit() }
				owners.connection.commit = func(branch.CommitRequest) (string, error) { called = true; return "", operation }
				code, result := b.runJSON(owners, command...)
				want := fmt.Sprintf("the branch commit owner did not commit unit u1: %v", operation)
				if boundary == "entry" {
					want = fmt.Sprintf("this session can't commit to goal %s's branch, so nothing was committed", b.id)
				}
				if goal.IsReadTimeout(cause) {
					seconds := 3
					if _, ok := cause.(*goal.ReadTimeout); ok {
						seconds = 4
					}
					effect := "nothing was committed"
					if boundary == "created" {
						effect = "commit " + operation.CreatedCommit + " was created and its publication is not confirmed"
					}
					want = fmt.Sprintf("the goal ledger could not be fetched within %d s (the remote or this host is slow), so %s", seconds, effect)
					if result.Next == nil || !slices.Equal(result.Next.Argv, append([]string{"metasystem"}, append(slices.Clone(command), "--json")...)) || result.Next.Reason != "the bound subject is kept; the same command reconciles or commits it once" {
						t.Fatalf("timeout retry = %+v", result.Next)
					}
				}
				if code != 1 || result.Summary != want || called != (boundary != "entry") {
					t.Fatalf("review = %+v, code=%d, commit called=%t; want %q", result, code, called, want)
				}
				t.Log(result.Summary)
			})
		}
	}
}
