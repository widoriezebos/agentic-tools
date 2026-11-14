package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"golang.org/x/sys/unix"
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
				locked := false
				checks := 0
				projectionBed := newGoalCLIBed(t, goalCLISeed{})
				owners.connection.claimCheck = func(_ string, _ string, endpoint goal.Endpoint) func() error {
					return func() error {
						checks++
						if endpoint.UnlockedCommandRead != !locked {
							t.Fatalf("command allowance under lock=%t: endpoint=%+v", locked, endpoint)
						}
						// Drive the production legacy reader with this call's property.
						endpoint.Repository = projectionBed.repo
						endpoint.Root = projectionBed.root
						if locked {
							reviewReadWithInjectedExpiry(t, endpoint)
						} else {
							_, err := goal.ProjectWithDeadline(endpoint, true, goalCLISeedNow, func(wait time.Duration) <-chan time.Time {
								if wait != 30*time.Second {
									t.Errorf("preflight bound = %s, want 30s", wait)
								}
								return make(chan time.Time)
							})
							if err != nil {
								t.Fatal(err)
							}
						}
						if boundary == "entry" {
							return cause
						}
						return nil
					}
				}
				called := false
				lockRoot := t.TempDir()
				lockPath := lease.LockPath(lockRoot)
				owners.connection.commitToken = func(_ string, commit func() error) error {
					release, err := lease.LockBounded(lockPath, "review")
					if err != nil {
						return err
					}
					locked = true
					defer func() { locked = false; release() }()
					return commit()
				}
				renewalDone := make(chan error, 1)
				owners.connection.commit = func(req branch.CommitRequest) (string, error) {
					called = true
					// The renewal uses this same flock. It is excluded until the
					// inner read finishes, then acquires as soon as review releases it.
					file, err := os.OpenFile(lockPath, os.O_RDWR, 0600)
					if err != nil {
						t.Fatal(err)
					}
					if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != unix.EWOULDBLOCK {
						t.Fatalf("review did not hold renewal lock: %v", err)
					}
					go func() {
						err := unix.Flock(int(file.Fd()), unix.LOCK_EX)
						if err == nil {
							err = unix.Flock(int(file.Fd()), unix.LOCK_UN)
						}
						file.Close()
						renewalDone <- err
					}()
					if err := req.CheckClaim(); err != nil {
						t.Fatal(err)
					}
					if boundary == "created" {
						if err := req.CheckClaim(); err != nil {
							t.Fatal(err)
						}
					}
					select {
					case err := <-renewalDone:
						t.Fatalf("renewal passed review's lock early: %v", err)
					default:
					}
					return "", operation
				}
				code, result := b.runJSON(owners, command...)
				if boundary != "entry" {
					if err := <-renewalDone; err != nil {
						t.Fatalf("renewal lock acquisition: %v", err)
					}
				}
				wantChecks := 1
				if boundary == "owner" {
					wantChecks = 2
				}
				if boundary == "created" {
					wantChecks = 3
				}
				if checks != wantChecks {
					t.Fatalf("checks=%d, want %d", checks, wantChecks)
				}
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

// No timer advances: the read holds capture until its own deadline is fired.
func reviewReadWithInjectedExpiry(t *testing.T, endpoint goal.Endpoint) {
	t.Helper()
	captured, release, completed := make(chan struct{}), make(chan struct{}), make(chan struct{})
	repository := endpoint.Repository
	endpoint.Repository = freshCommandRepository{Repository: repository,
		capture: func(_ context.Context, opid string) (string, error) {
			close(captured)
			<-release
			return repository.Capture(opid)
		},
		cleaned: func(context.Context) { close(completed) },
	}
	expired := make(chan time.Time)
	result := make(chan error, 1)
	go func() {
		_, err := goal.ProjectWithDeadline(endpoint, true, goalCLISeedNow, func(wait time.Duration) <-chan time.Time {
			if wait != 4*time.Second {
				t.Errorf("inner bound = %s, want 4s", wait)
			}
			return expired
		})
		result <- err
	}()
	<-captured
	close(expired)
	err := <-result
	close(release)
	<-completed
	if !goal.IsReadTimeout(err) || !strings.Contains(goal.ReadTimeoutMessage(err), "within 4 s") {
		t.Fatalf("inner expiry = %v", err)
	}
}
