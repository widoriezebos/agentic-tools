package plain

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestPendingAdmissionDependsOnParentAndPersonCanRetry(t *testing.T) {
	t.Parallel()
	for _, live := range []bool{false, true} {
		t.Run(map[bool]string{false: "dead parent", true: "live parent"}[live], func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			pid := int64(os.Getpid())
			ref := processRef(pid)
			if !live {
				child := exec.Command("/usr/bin/true")
				if err := child.Run(); err != nil {
					t.Fatal(err)
				}
				pid, ref = int64(child.Process.Pid), "dead-parent"
			}
			admission := ExecutionAdmission{Attempt: "pending", Commit: "commit", Tree: "tree", Scope: "full", State: "pending"}
			running := Running{Attempt: "pending", Commit: "commit", Tree: "tree", Pid: pid, Process: ref, Admission: &admission, Executions: []ExecutionAdmission{admission}}
			if err := withLock(b.install, func() error { return writeRunning(b.install, running) }); err != nil {
				t.Fatal(err)
			}
			if _, recorded, alive, err := ReadRunning(b.install, b.seams); err != nil || !recorded || alive != live {
				t.Fatalf("pending parent: recorded=%v alive=%v err=%v", recorded, alive, err)
			}
			if live {
				return
			}
			if err := withLock(b.install, func() error {
				_, recorded, alive, err := checkState(b.install, b.checkout, "", b.seams)
				if err == nil && (recorded || alive) {
					t.Errorf("dead pending blocks a retry: recorded=%v alive=%v", recorded, alive)
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
			failed, recorded, alive, err := ReadRunning(b.install, b.seams)
			if err != nil || !recorded || alive || failed.Admission.State != "failed" || failed.Executions[0].State != "failed" {
				t.Fatalf("pending not reconciled as failed: %+v %v", failed, err)
			}
			if lines, err := Results(b.install); err != nil || len(lines) != 0 {
				t.Fatalf("unlaunched admission wrote a result: %+v %v", lines, err)
			}
			b.seams.Person = &ActProvenance{Kind: "proof", Person: "Wido"}
			result, err := Run(b.install, b.checkout, "printf 'LANDING-CHECKED\\t0\\n'", "", io.Discard, b.seams)
			if err != nil || result.Result != Green || !result.CountedFull {
				t.Fatalf("person retry: %+v %v", result, err)
			}
		})
	}
}

func TestHeldFullCheckAllowsLaterAutomaticAdmission(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	held := Result{Attempt: "scoped", Commit: "commit", Tree: "tree", Result: "held", Scope: "scoped", ScopeReason: "full check pending: the proof environment changed"}
	if err := withLock(b.install, func() error { return appendLine(resultsPath(b.install), held) }); err != nil {
		t.Fatal(err)
	}
	result, err := Run(b.install, b.checkout, "printf 'LANDING-CHECKED\\t0\\n'", "", io.Discard, b.seams)
	if err != nil || result.Result != Green || result.Scope != "full" || !result.CountedFull || result.Person != nil {
		t.Fatalf("automatic full admission: %+v %v", result, err)
	}
	lines, err := Results(b.install)
	if err != nil || len(lines) != 2 || lines[0].Result != "held" || lines[0].Cause != nil || strings.Contains(lines[0].Repeat, "started") {
		t.Fatalf("held check became red or a repeat: %+v %v", lines, err)
	}
}

func TestDetachedAdmissionRequiresChildIdentityAndFence(t *testing.T) {
	t.Parallel()
	for _, boundary := range []string{"child identity", "execution fence"} {
		t.Run(boundary, func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			if boundary == "execution fence" {
				b.seams.Trunk = true
				git := b.seams.Git
				b.seams.Git = func(dir string, args ...string) (string, error) {
					if strings.Join(args, " ") == "rev-parse --verify origin/main^{commit}" {
						return "commit", nil
					}
					return git(dir, args...)
				}
			}
			b.seams.Launch = func([]string, string, string) (int64, error) { return int64(os.Getpid()), nil }
			running, _, err := Start(b.install, b.checkout, b.seams)
			if err != nil {
				t.Fatal(err)
			}
			if boundary == "child identity" {
				running.Process = "foreign-process"
				b.seams.Alive = func(Running) bool { return true }
				if err := withLock(b.install, func() error { return writeRunning(b.install, running) }); err != nil {
					t.Fatal(err)
				}
			} else {
				b.seams.FenceCheck = func() error { return errors.New("execution fence held") }
			}
			executions := 0
			b.seams.Command = func(*exec.Cmd) error { executions++; return nil }
			expected := "LANE_PROOF_ADMISSION"
			if boundary == "execution fence" {
				expected = "execution fence held"
			}
			if result, err := Run(b.install, b.checkout, "check", running.Attempt, io.Discard, b.seams); err == nil || !strings.Contains(err.Error(), expected) || executions != 0 {
				t.Fatalf("%s bypass: %+v err=%v executions=%d", boundary, result, err, executions)
			}
			current, recorded, _, err := ReadRunning(b.install, b.seams)
			if err != nil || !recorded || current.Admission.State != "launched" || current.Attempt != running.Attempt {
				t.Fatalf("refusal claimed admission: %+v %v", current, err)
			}
		})
	}
}

func TestTrunkAdmissionClosesMainSubjectAcrossTrees(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	now := bedNow
	stops := []Stop{
		{Loop: "lane-proof", Trunk: true, Scope: "full", Subject: "main", Tree: "old-tree", Decision: "stop", Required: []string{"metasystem", "landing", "prove", "--trunk"}, At: now.Format(time.RFC3339)},
		{Loop: "lane-proof", Trunk: true, Scope: "scoped", Subject: "main", Tree: "old-tree", Decision: "stop", Required: []string{"metasystem", "landing", "prove", "--trunk"}, At: now.Add(time.Second).Format(time.RFC3339)},
		{Loop: "lane-proof", Trunk: true, Scope: "full", Subject: "other-goal", Tree: "old-tree", Decision: "stop", Required: []string{"metasystem", "landing", "prove"}, At: now.Format(time.RFC3339)},
	}
	if err := withLock(install, func() error {
		for _, stop := range stops {
			if err := appendLine(stopsPath(install), stop); err != nil {
				return err
			}
		}
		return closeProofAdmissionStopsLocked(install, Running{Trunk: true, Tree: "new-tree", Attempt: "new-attempt", Person: &ActProvenance{Kind: "proof", Person: "Wido"}}, now)
	}); err != nil {
		t.Fatal(err)
	}
	lines, err := readLines[Stop](stopsPath(install))
	if err != nil || len(lines) != 4 || lines[3].Decision != "close" || lines[3].Subject != "main" || lines[3].Scope != "full" {
		t.Fatalf("main subject closure: %+v %v", lines, err)
	}
}

func TestBatchAdmissionClosesMatchingBatchAcrossScopes(t *testing.T) {
	t.Parallel()
	for _, match := range []string{"batch", "subject"} {
		t.Run(match, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			stop := Stop{Loop: "lane-proof", Subject: "a", BatchID: "old-batch", Tree: "old-tree", Scope: "scoped", Decision: "stop", At: bedNow.Format(time.RFC3339), Required: []string{"metasystem", "landing", "prove"}}
			running := Running{Attempt: "new-check", Tree: "new-tree", BatchID: "batch", BatchMembers: []GoalSHA{{Goal: "a", SHA: "sha"}}, Person: &ActProvenance{Kind: "proof", Person: "Wido"}}
			if match == "batch" {
				stop.BatchID, stop.Subject = running.BatchID, "batch subject"
			}
			if err := withLock(install, func() error {
				if err := appendLine(stopsPath(install), stop); err != nil {
					return err
				}
				return closeProofAdmissionStopsLocked(install, running, bedNow)
			}); err != nil {
				t.Fatal(err)
			}
			if current, err := NewestStop(install); err != nil || current != nil {
				t.Fatalf("matching %s request remained open: %+v %v", match, current, err)
			}
			lines, err := readLines[Stop](stopsPath(install))
			if err != nil || len(lines) != 2 || lines[0].Decision != "stop" || lines[1].Decision != "close" {
				t.Fatalf("closure did not preserve the request: %+v %v", lines, err)
			}
		})
	}
}

func TestDepthDecisionPreservesImpactScopeError(t *testing.T) {
	t.Parallel()
	b := newScopeBed(t)
	b.seams.DepthScope = "full"
	b.seams.DepthReason = "tier 3: full"
	b.seams.Impact = true
	decision := proofScope(b.install, b.checkout, Running{}, b.seams)
	if decision.Base != "" || !strings.Contains(decision.ScopeReason, "impact check error: no batch base is recorded") || !strings.Contains(decision.ScopeReason, "tier 3: full") {
		t.Fatalf("impact error was replaced by depth reason: %+v", decision)
	}
}
