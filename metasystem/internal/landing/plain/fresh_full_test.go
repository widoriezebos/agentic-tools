package plain

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTrunkClockCountsOnlyCompletedTrunkChecksAndFreshFullPushes(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"never", "green trunk", "red trunk", "expired trunk", "full push", "expired full push", "unpublished full", "scoped push", "inherited push", "legacy push", "newer red at push", "old push newer full", "custom interval", "invalid interval", "zero interval"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			interval := "4h"
			if name == "custom interval" {
				interval = "30m"
			}
			if name == "invalid interval" {
				interval = "bad"
			}
			if name == "zero interval" {
				interval = "0h"
			}
			if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("proof.trunk-every="+interval+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			at := bedNow.Add(-time.Hour)
			proof := Result{Tree: "tree", Commit: "main", Result: Green, Scope: "full", At: at.Format(time.RFC3339), FullTree: "tree", FullAt: at.Format(time.RFC3339)}
			due := true
			pushed := false
			switch name {
			case "green trunk", "red trunk", "expired trunk", "custom interval":
				proof.Trunk = true
				if name == "red trunk" {
					proof.Result = Red
				}
				if name == "expired trunk" {
					proof.At = bedNow.Add(-4 * time.Hour).Format(time.RFC3339)
				}
				due = name == "expired trunk" || name == "custom interval"
			case "full push", "expired full push", "scoped push", "inherited push", "legacy push", "newer red at push", "old push newer full":
				pushed = true
				if name == "expired full push" {
					proof.At = bedNow.Add(-4 * time.Hour).Format(time.RFC3339)
					proof.FullAt = proof.At
				}
				if name == "scoped push" {
					proof.Scope = "scoped"
				}
				if name == "inherited push" {
					proof.Reason = "inherits green from tree older"
				}
				if name == "legacy push" {
					proof.Scope = ""
				}
				due = name != "full push"
			}
			var records []string
			if name != "never" {
				data, err := json.Marshal(proof)
				if err != nil {
					t.Fatal(err)
				}
				records = append(records, string(data))
			}
			if name == "newer red at push" {
				red := proof
				red.Result = Red
				data, _ := json.Marshal(red)
				records = append(records, string(data))
			}
			writeProofRecords(t, install, records, "")
			if pushed {
				at := proof.At
				if name == "old push newer full" {
					at = bedNow.Add(-5 * time.Hour).Format(time.RFC3339)
				}
				writePushRecords(t, install, Pushed{Tree: proof.Tree, At: at})
			}
			got, err := fullProofDue(install, bedNow)
			if name == "invalid interval" || name == "zero interval" {
				if err == nil || !strings.Contains(err.Error(), "proof.trunk-every") {
					t.Fatalf("invalid duration: %v", err)
				}
				return
			}
			if err != nil || got != due {
				t.Fatalf("due=%v want=%v err=%v", got, due, err)
			}
		})
	}
}

func TestTrunkStartPreservesSubjectAndRefusesConcurrentBatch(t *testing.T) {
	t.Parallel()
	b := newScopeBed(t)
	original := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --verify origin/main^{commit}":
			return "main", nil
		case "rev-parse --verify main^{tree}":
			return b.base.Tree, nil
		}
		return original(dir, args...)
	}
	b.seams.Trunk = true
	b.seams.Alive = func(Running) bool { return true }
	b.seams.Executable = func() (string, error) { return "engine", nil }
	var argv []string
	b.seams.Launch = func(args []string, _, _ string) (int64, error) { argv = append([]string{}, args...); return 0, nil }
	running, already, err := Start(b.install, b.checkout, b.seams)
	if err != nil || already || !running.Trunk || running.Commit != "main" || !strings.Contains(strings.Join(argv, " "), "--trunk") {
		t.Fatalf("start: %+v %v %v argv=%v", running, already, err, argv)
	}
	if _, already, err := Start(b.install, b.checkout, b.seams); err != nil || !already {
		t.Fatalf("duplicate start: %v %v", already, err)
	}
	// A batch targeting the same tree cannot take over the running trunk check.
	b.seams.Trunk = false
	b.seams.Git = func(_ string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --verify HEAD^{commit}":
			return "main", nil
		case "rev-parse --verify HEAD^{tree}":
			return b.base.Tree, nil
		}
		return "", nil
	}
	if _, _, err := Start(b.install, b.checkout, b.seams); err == nil {
		t.Fatal("batch stole the trunk check")
	} else {
		var busy *Busy
		if !errors.As(err, &busy) {
			t.Fatalf("expected the running check to refuse the batch: %v", err)
		}
	}
	// The saved attempt keeps its subject even when the caller sees a newer tip.
	b.seams.Trunk = true
	b.seams.Git = func(dir string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --verify origin/main^{commit}":
			return "new-main", nil
		case "rev-parse --verify new-main^{tree}":
			return "new-main-tree", nil
		}
		return original(dir, args...)
	}
	b.seams.CommandForCommit = func(commit string) (string, error) {
		if commit != "main" {
			t.Fatalf("detached attempt changed its command's commit to %s", commit)
		}
		return "main-command", nil
	}
	result, err := Run(b.install, b.checkout, "", running.Attempt, b.output, b.seams)
	if err != nil || !result.Trunk || result.Commit != "main" || result.Tree != running.Tree || result.Scope != "full" || len(b.calls) != 1 || commandEnv(b.calls[0], "LANDING_COMMIT") != "main" {
		t.Fatalf("detached check lost its original subject: %+v %v", result, err)
	}
}

func TestExecutableChangeSinceFullCannotHideBehindScopedGreen(t *testing.T) {
	t.Parallel()
	b := newScopeBed(t)
	b.base.Scope = "scoped"
	b.base.Tree = "scoped-tree"
	b.base.Commit = "scoped-commit"
	b.git.batches[b.base.Commit] = "batch-a\nbatch-b"
	b.git.changed[[2]string{b.base.Tree, b.git.tree}] = "metasystem/plans/page.md"
	b.git.changed[[2]string{b.base.FullTree, b.git.tree}] = "scripts/check.sh\nmetasystem/plans/page.md"
	b.save(t, b.base)
	result := b.run(t)
	if result.Scope != "full" || result.ScopeReason != "an executable input changed: scripts/check.sh" || len(b.calls) != 1 || commandEnv(b.calls[0], "LANDING_PROOF_SCOPE") != "full" {
		t.Fatalf("hidden executable change: %+v", result)
	}
}

func TestTrunkChecksUseTheirClockInsteadOfTheBatchBudget(t *testing.T) {
	t.Parallel()
	b := newScopeBed(t)
	if _, _, err := HandIn(b.install, Line{Goal: "g", SHA: "sha"}); err != nil {
		t.Fatal(err)
	}
	original := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "merge-base" {
			return "", nil
		}
		return original(dir, args...)
	}
	for _, attempt := range []string{"one", "two"} {
		proof := Result{Trunk: true, CountedFull: true, Attempt: attempt, Result: Red, Cause: &Cause{Kind: "main"}, Goals: []GoalSHA{{Goal: "g", SHA: "sha"}}}
		b.save(t, proof)
		if err := recordProofStop(b.install, proof); err != nil {
			t.Fatal(err)
		}
	}
	if err := checkProofBudget(b.install, b.checkout, b.git.commit, b.seams); err != nil {
		t.Fatalf("trunk checks spent the batch allowance: %v", err)
	}
	if stop, err := NewestStop(b.install); err != nil || stop != nil {
		t.Fatalf("trunk check stopped the lane: %+v %v", stop, err)
	}
}

func TestProofModesRefuseAnotherRunningModeOnTheSameTree(t *testing.T) {
	t.Parallel()
	modes := []struct {
		name        string
		gate, trunk bool
	}{{name: "batch"}, {name: "gate", gate: true}, {name: "trunk", trunk: true}}
	for _, current := range modes {
		for _, requested := range modes {
			if current.name == requested.name {
				continue
			}
			t.Run(current.name+" to "+requested.name, func(t *testing.T) {
				t.Parallel()
				b := newScopeBed(t)
				b.seams.Gate, b.seams.Trunk = requested.gate, requested.trunk
				b.seams.Alive = func(Running) bool { return true }
				git := b.seams.Git
				b.seams.Git = func(dir string, args ...string) (string, error) {
					if strings.Join(args, " ") == "rev-parse --verify origin/main^{commit}" {
						return b.git.commit, nil
					}
					if strings.Join(args, " ") == "rev-parse --verify "+b.git.commit+"^{tree}" {
						return b.git.tree, nil
					}
					return git(dir, args...)
				}
				// A reusable green must not hide a running check in another mode.
				green := Result{Tree: b.git.tree, Commit: b.git.commit, Result: Green, Scope: "full", At: bedNow.Format(time.RFC3339)}
				if err := appendLine(b.seams.resultsPath(b.install), green); err != nil {
					t.Fatal(err)
				}
				running := Running{Gate: current.gate, Trunk: current.trunk, Attempt: "active", Tree: b.git.tree, Commit: b.git.commit}
				if err := writeRunning(b.install, running); err != nil {
					t.Fatal(err)
				}
				assertBusy := func(err error) {
					t.Helper()
					var busy *Busy
					if !errors.As(err, &busy) || !reflect.DeepEqual(busy.Running, running) {
						t.Fatalf("mode mismatch did not preserve the running check: %v", err)
					}
				}
				_, _, err := Start(b.install, b.checkout, b.seams)
				assertBusy(err)
				_, err = Run(b.install, b.checkout, "proof-command", running.Attempt, b.output, b.seams)
				assertBusy(err)
				if len(b.calls) != 0 {
					t.Fatalf("mode mismatch ran a command: %v", b.calls)
				}
			})
		}
	}
}

func TestProofStopExcludesOtherGateTreesAndOpenTrunkChecks(t *testing.T) {
	t.Parallel()
	for _, each := range []struct {
		name         string
		gate, closed bool
	}{{name: "open trunk"}, {name: "closed trunk", closed: true}, {name: "other gate tree", gate: true}} {
		t.Run(each.name, func(t *testing.T) {
			t.Parallel()
			gate := each.gate
			b := newScopeBed(t)
			mode := ProveSeams{Gate: gate}
			scope := "full"
			if gate {
				scope = "gate"
			}
			result := Result{Tree: "current-tree", Attempt: "current", Result: Red, Scope: scope,
				Goals: []GoalSHA{{Goal: "g", SHA: "sha"}}, Repeat: "allowed", At: bedNow.Format(time.RFC3339),
				Cause: &Cause{Kind: "environment", Tests: []string{"TestCurrent"}}}
			prior := result
			prior.Attempt, prior.Failed = "prior", []FailedUnit{{Unit: "unit", Tests: []string{"TestPrior"}}}
			if err := appendLine(mode.resultsPath(b.install), prior); err != nil {
				t.Fatal(err)
			}
			unrelated := result
			unrelated.Attempt, unrelated.CountedFull = "unrelated", true
			unrelated.Failed = []FailedUnit{{Unit: "unit", Tests: []string{"TestUnrelated"}}}
			if gate {
				unrelated.Tree = "other-gate-tree"
			} else {
				unrelated.Trunk, unrelated.LoopClosed = true, each.closed
			}
			if err := appendLine(mode.resultsPath(b.install), unrelated); err != nil {
				t.Fatal(err)
			}
			if err := recordProofStop(b.install, result); err != nil {
				t.Fatal(err)
			}
			stops, err := readLines[Stop](stopsPath(b.install))
			if err != nil || len(stops) != 1 {
				t.Fatalf("stop record: %+v %v", stops, err)
			}
			stop := stops[0]
			previous := []string{"unit TestPrior"}
			if each.closed {
				previous = nil
			}
			if stop.Attempt != 0 || !reflect.DeepEqual(stop.Measure.Previous, previous) {
				t.Fatalf("unrelated proof changed the stop: %+v", stop)
			}
		})
	}
}
