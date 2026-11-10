package plain

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestALaterPersonExceptionAdmitsOneAgentProve(t *testing.T) {
	t.Parallel()
	for _, later := range []bool{false, true} {
		t.Run(map[bool]string{false: "older exception", true: "later exception"}[later], func(t *testing.T) {
			b := newRepeatBed(t)
			if err := os.MkdirAll(Dir(b.install), 0o755); err != nil {
				t.Fatal(err)
			}
			at := bedNow.Add(-2 * time.Hour)
			checked := at.Add(-time.Hour)
			if later {
				checked = at.Add(time.Hour)
			}
			member := GoalSHA{Goal: "a", SHA: "member"}
			if err := appendLine(queuePath(b.install), Line{Goal: member.Goal, SHA: member.SHA, Exception: &Exception{Person: &ActProvenance{Person: "Wido", CheckedAt: checked}}}); err != nil {
				t.Fatal(err)
			}
			if err := writeBatch(b.install, &Batch{ID: "batch", Base: "parent", State: BatchRunning, Lane: lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}, Members: []GoalSHA{member}}); err != nil {
				t.Fatal(err)
			}
			if err := appendLine(resultsPath(b.install), Result{Tree: "tree", Commit: "commit", Result: Red, At: at.Format(time.RFC3339), Repeat: "used"}); err != nil {
				t.Fatal(err)
			}
			git := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				switch {
				case args[0] == "cat-file":
					return "", nil
				case strings.Join(args, " ") == "rev-list --first-parent --reverse --parents parent..commit":
					return "commit parent member", nil
				case args[0] == "merge-base":
					if args[3] == "commit" || args[2] == args[3] {
						return "", nil
					}
					return "", exec.Command("/usr/bin/false").Run()
				default:
					return git(dir, args...)
				}
			}
			launches := 0
			b.seams.Launch = func([]string, string, string) (int64, error) { launches++; return int64(os.Getpid()), nil }
			b.seams.Alive = func(Running) bool { return true }
			running, _, err := Start(b.install, b.checkout, b.seams)
			var refused *NoRepeat
			if !later {
				if !errors.As(err, &refused) || launches != 0 {
					t.Fatalf("old exception admitted a retry: %v, launches=%d", err, launches)
				}
				return
			}
			if err != nil || launches != 1 {
				t.Fatalf("later exception did not admit one retry: %v, launches=%d", err, launches)
			}
			result, err := Run(b.install, b.checkout, failedReport, running.Attempt, io.Discard, b.seams)
			if err != nil || result.Result != Red {
				t.Fatalf("retry result: %+v %v", result, err)
			}
			_, _, err = Start(b.install, b.checkout, b.seams)
			if !errors.As(err, &refused) || launches != 1 {
				t.Fatalf("exception admitted a second retry: %v, launches=%d", err, launches)
			}
		})
	}
}

func TestRecoveryDoesNotReproveAnExaminedTree(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"examined", "environment", "pending", "unrecorded main red"} {
		t.Run(scenario, func(t *testing.T) {
			b, launches, _, _ := trunkAdmissionBed(t)
			r := Result{Trunk: true, Commit: b.git.commit, Tree: b.git.tree, Result: Green, Scope: "scoped", At: bedNow.Format(time.RFC3339), Environment: "image toolchain"}
			if scenario == "pending" {
				r.ClassificationPending = true
			}
			if scenario == "unrecorded main red" {
				r.Result, r.Cause, r.Failed = Red, &Cause{Kind: "main"}, []FailedUnit{{Unit: "plans"}}
			}
			b.save(t, r)
			if scenario == "environment" {
				b.save(t, Result{Tree: "elsewhere", Result: Green, Environment: "new image"})
			}
			for _, elapsed := range []time.Duration{time.Minute, time.Hour, 3 * time.Hour} {
				b.seams.Now = func() time.Time { return bedNow.Add(elapsed) }
				if err := RecoverMovedMain(b.install, b.checkout, b.seams); err != nil {
					t.Fatal(err)
				}
			}
			want := 1
			if scenario == "examined" {
				want = 0
			}
			if *launches != want {
				t.Fatalf("recovery launches=%d, want %d", *launches, want)
			}
			if scenario == "examined" {
				coverage := b.seams
				coverage.Person = &ActProvenance{Person: "fixture"}
				if _, settled, err := Settled(b.install, b.checkout, coverage); err != nil || settled {
					t.Fatalf("examined became reusable coverage: settled=%v %v", settled, err)
				}
			}
		})
	}
}

func TestMainReconciliationKeepsOriginalBatchExecution(t *testing.T) {
	t.Parallel()
	b, _, _, _ := trunkAdmissionBed(t)
	git := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "cat-file" {
			return "", nil
		}
		return git(dir, args...)
	}
	r := b.base
	r.At, r.FullAt = bedNow.Format(time.RFC3339), bedNow.Format(time.RFC3339)
	b.git.changed[[2]string{r.Commit, b.git.commit}] = "metasystem/records/notes.md"
	b.save(t, r)
	inherited := r
	inherited.Trunk, inherited.Scope, inherited.Commit, inherited.Tree = true, "inherited", b.git.commit, b.git.tree
	inherited.Reason = "inherits green from tree original"
	b.save(t, inherited)
	greens, err := MainGreenToReconcile(b.install, b.checkout, b.seams)
	if err != nil || len(greens) == 0 || greens[0].Commit != r.Commit || !greens[0].Executed() {
		t.Fatalf("lost original execution: %+v %v", greens, err)
	}
	b.git.changed[[2]string{r.Commit, b.git.commit}] = "metasystem/internal/code.go"
	greens, err = MainGreenToReconcile(b.install, b.checkout, b.seams)
	if err != nil || len(greens) != 0 {
		t.Fatalf("executable descendant kept stale execution: %+v %v", greens, err)
	}
}

func TestTrunkRecoveryRechecksAnExaminedProofUnderTheLock(t *testing.T) {
	t.Parallel()
	b, launches, _, _ := trunkAdmissionBed(t)
	if err := os.Remove(resultsPath(b.install)); err != nil {
		t.Fatal(err)
	}
	red := b.base
	red.Result, red.Cause, red.Failed = Red, &Cause{Kind: "main"}, []FailedUnit{{Unit: "plans"}}
	red.FullAt, red.FullTree = "", ""
	b.save(t, red)
	b.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
		return []goal.TrunkRedEntry{{ID: "red-plans", Identity: "red:plans", Group: "plans"}}, nil
	}
	b.seams.ProcessRef = processRef
	git := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "ls-tree" {
			return "metasystem/cmd/metasystem/main.go", nil
		}
		return git(dir, args...)
	}
	b.seams.Launch = func([]string, string, string) (int64, error) { *launches++; return int64(os.Getpid()), nil }
	first, _, err := Start(b.install, b.checkout, b.seams)
	if err != nil {
		t.Fatal(err)
	}
	paused, resume, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	recovery := b.seams
	enteringStart := false
	recovery.TimerHeld = func() bool { enteringStart = true; return false }
	recovery.Git = func(dir string, args ...string) (string, error) {
		// Start reads its subject before taking the admission lock. Finish
		// the proof here, after recovery's outside-lock decisions have run.
		if enteringStart && strings.Join(args, " ") == "rev-parse --verify origin/main^{commit}" {
			enteringStart = false
			close(paused)
			<-resume
		}
		return b.seams.Git(dir, args...)
	}
	callbacks := 0
	var recovered Running
	recovery.RecoveryStarted = func(r Running, _ bool) { callbacks++; recovered = r }
	go func() { done <- RecoverMovedMain(b.install, b.checkout, recovery) }()
	<-paused
	defer func() {
		if resume != nil {
			close(resume)
			<-done
		}
	}()
	log, err := os.Create(first.Log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.Close() })
	saved, err := Run(b.install, b.checkout, "fixture-proof", first.Attempt, log, b.seams)
	if err != nil || saved.Result != Green || saved.Scope != "scoped" || saved.FullAt != "" {
		t.Fatalf("scoped execution without full coverage: %+v %v", saved, err)
	}
	close(resume)
	resume = nil
	if err := <-done; err != nil || *launches != 1 || callbacks != 0 || recovered.Reused != nil {
		t.Fatalf("recovery relaunched or reused examined evidence: launches=%d callbacks=%d reused=%+v %v", *launches, callbacks, recovered.Reused, err)
	}
}
