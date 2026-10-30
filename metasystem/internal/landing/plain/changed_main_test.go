package plain

import (
	"bytes"
	"encoding/json"
	"runtime"

	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

// The local main reference has no batch commits ahead of itself.
func changedMainBed(t *testing.T) *scopeBed {
	t.Helper()
	b := newScopeBed(t)
	b.git.batches = map[string]string{}
	b.seams.Trunk = true
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if len(args) == 4 && args[0] == "merge-base" && args[1] == "--is-ancestor" {
			if args[2] == "candidate" {
				return "", exec.Command("/usr/bin/false").Run()
			}
			return "", nil
		}
		return b.git.run(dir, args...)
	}
	b.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil }
	b.seams.Executable = func() (string, error) { return "fixture-engine", nil }
	b.seams.Launch = func([]string, string, string) (int64, error) { return int64(os.Getpid()), nil }
	b.seams.Alive = func(Running) bool { return true }
	// The unaffected group is a prerequisite, so three checks execute for two selections.
	b.contract.Groups[0].Requires = []string{"unrelated"}
	b.setContract(t)
	b.seams.Command = func(cmd *exec.Cmd) error {
		b.calls = append(b.calls, cmd)
		fmt.Fprintln(cmd.Stdout, "landing environment image toolchain")
		ids := strings.Fields(commandEnv(cmd, "LANDING_PROOF_GROUPS"))
		if commandEnv(cmd, "LANDING_PROOF_SCOPE") == "full" {
			ids = nil
			for _, group := range b.contract.Groups {
				ids = append(ids, group.ID)
			}
		} else if !slices.Contains(ids, "unrelated") {
			ids = append([]string{"unrelated"}, ids...)
		}
		for _, id := range ids {
			fmt.Fprintf(cmd.Stdout, "landing group %s passed 1\n", id)
		}
		fmt.Fprintln(cmd.Stdout, "LANDING-CHECKED\t0")
		return nil
	}
	return b
}

func TestChangedMainAdmissionPinsAffectedChecks(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"changed", "fresh", "cadence"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := changedMainBed(t)
			b.seams.Fresh = mode == "fresh"
			var running Running
			var already bool
			var err error
			if mode == "cadence" {
				if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("testing.contract=testing.json\nproof.trunk-every=5m\n"), 0600); err != nil {
					t.Fatal(err)
				}
				b.base.Trunk = true
				b.save(t, b.base)
				_, err = KeeperClock(lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}, func(lane.Record) error { return nil }, b.seams)
				if err == nil {
					running, _, _, err = ReadRunning(b.install, b.seams)
				}
			} else {
				running, already, err = Start(b.install, b.checkout, b.seams)
			}
			want := "scoped"
			if mode != "changed" {
				want = "full"
			}
			if err != nil || already || running.Admission.Scope != want {
				t.Fatalf("admission: %+v %v", running, err)
			}
			if mode == "changed" && (!reflect.DeepEqual(running.Admission.Groups, []string{"plans", "shared"}) || !strings.Contains(running.Admission.Reason, "3 checks run")) {
				t.Fatalf("affected checks or prerequisite count: %+v", running.Admission)
			}
			// Detached execution must use the admitted decision even if selection facts become unreadable.
			b.git.diffErr = errors.New("selection facts changed after admission")
			log, err := os.Create(running.Log)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = log.Close() })
			result, err := Run(b.install, b.checkout, "fixture-proof", running.Attempt, log, b.seams)
			if err != nil || result.Result != Green || result.Scope != want || len(b.calls) != 1 || commandEnv(b.calls[0], "LANDING_PROOF_SCOPE") != want || len(result.Ran) != 3 {
				t.Fatalf("detached run: %+v %v calls=%d", result, err, len(b.calls))
			}
			if want == "scoped" && (result.FullAt != b.base.FullAt || result.FullTree != b.base.FullTree || result.ScopeReason != running.Admission.Reason) {
				t.Fatalf("lost original credit or reason: %+v", result)
			}
		})
	}
}

func TestChangedMainFullFallbacks(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"unreadable results", "unreadable paths", "damaged history", "unreadable ancestry", "unreadable contract", "code", "contract", "environment", "uncovered", "newer red"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			b := changedMainBed(t)
			switch failure {
			case "unreadable results":
				if err := os.Remove(resultsPath(b.install)); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(resultsPath(b.install), 0700); err != nil {
					t.Fatal(err)
				}
			case "damaged history":
				f, err := os.OpenFile(resultsPath(b.install), os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				_, err = f.WriteString("broken\n")
				if err != nil {
					t.Fatal(err)
				}
				_ = f.Close()
			case "unreadable ancestry":
				git := b.seams.Git
				b.seams.Git = func(dir string, args ...string) (string, error) {
					if args[0] == "merge-base" {
						return "", errors.New("cannot read ancestry")
					}
					return git(dir, args...)
				}
			case "unreadable paths":
				b.git.diffErr = errors.New("unreadable paths")
			case "unreadable contract":
				b.git.showErr = errors.New("unreadable contract")
			case "code":
				b.git.changed[[2]string{b.base.Tree, b.git.tree}] = "metasystem/internal/code.go"
			case "contract":
				b.git.changed[[2]string{b.base.Tree, b.git.tree}] = "metasystem/testing.json"
			case "environment":
				b.base.Environment = "old environment"
				b.save(t, b.base)
			case "uncovered":
				for i := range b.contract.Groups {
					b.contract.Groups[i].Inputs = []string{"metasystem/records/**"}
				}
				b.setContract(t)
			case "newer red":
				outside := b.contract.Groups[1]
				outside.ID, outside.Inputs = "outside", []string{"metasystem/outside/**"}
				b.contract.Groups = append(b.contract.Groups, outside)
				b.setContract(t)
				red := b.base
				red.Result = Red
				red.Commit = "later-red"
				red.Tree = "red-tree"
				red.At = bedNow.Add(-time.Minute).Format(time.RFC3339)
				red.Failed = []FailedUnit{{Unit: "outside"}}
				b.save(t, red)
			}
			result, err := Run(b.install, b.checkout, "fixture-proof", "", b.output, b.seams)
			if failure == "unreadable results" {
				if err == nil {
					t.Fatal("unreadable history admitted green")
				}
				return
			}
			if err != nil || result.Result != Green || result.Scope != "full" || result.FullTree != result.Tree {
				t.Fatalf("fallback: %+v %v", result, err)
			}
			if failure == "environment" && len(b.calls) != 2 {
				t.Fatal("changed environment did not execute full fallback")
			}
		})
	}
}

// Main keeps an inherited full green past the hour: the check of main runs
// only the groups whose inputs changed since that green's full tree.
func TestChangedMainKeepsAnExpiredInheritedFull(t *testing.T) {
	t.Parallel()
	b := changedMainBed(t)
	if err := os.Remove(resultsPath(b.install)); err != nil {
		t.Fatal(err)
	}
	b.base.At = bedNow.Add(-time.Minute).Format(time.RFC3339)
	b.base.FullAt = bedNow.Add(-61 * time.Minute).Format(time.RFC3339)
	b.base.FullTree = "original"
	b.git.changed[[2]string{b.base.FullTree, b.git.tree}] = "metasystem/plans/page.md"
	b.base.Reason = "inherits green from tree original"
	b.save(t, b.base)
	b.seams.Closure = func(string, string, string) (adapter.Closure, error) {
		return adapter.Closure{Unowned: []string{"metasystem/plans/page.md"}}, nil
	}
	result, err := Run(b.install, b.checkout, "fixture-proof", "", b.output, b.seams)
	if err != nil || result.Result != Green || result.Scope != "scoped" || result.Base != "original" || result.FullTree != "original" || result.FullAt != b.base.FullAt {
		t.Fatalf("expired inherited full: %+v %v", result, err)
	}
}

func TestScopedTrunkClockCreditAndOriginalDebt(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"scoped green", "scoped red", "full red", "full nonrun", "candidate", "candidate red", "inherited full push"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newScopeBed(t)
			b.base.Trunk = true
			b.save(t, b.base)
			r := Result{Trunk: true, Commit: "main", Tree: "main-tree", Scope: "scoped", Result: Green, At: bedNow.Format(time.RFC3339), FullTree: b.base.Tree, FullAt: b.base.At}
			want := b.base.At
			switch kind {
			case "scoped red":
				r.Result = Red
				r.CountedFull = true
			case "full red":
				r.Scope = "full"
				r.Result = Red
				r.CountedFull = true
				want = r.At
			case "full nonrun":
				r.Scope = "full"
				r.Result = Red
				r.Cause = &Cause{Kind: "environment"}
			case "candidate", "candidate red":
				if kind == "candidate red" {
					r.Result = Red
					r.CountedFull = true
				}
				r.Trunk = false
				r.Scope = "full"
				r.FullTree = r.Tree
				r.FullAt = r.At
			case "inherited full push":
				r.Trunk = false
				r.Scope = "full"
				r.Reason = "inherits green from tree original"
				writePushRecords(t, b.install, Pushed{Tree: r.Tree, At: r.At})
			}
			b.save(t, r)
			at, _, err := fullProofClock(b.install)
			if err != nil || at.Format(time.RFC3339) != want {
				t.Fatalf("clock=%s want=%s %v", at, want, err)
			}
			if kind == "scoped green" || kind == "scoped red" {
				for _, offset := range []time.Duration{49 * time.Minute, 51 * time.Minute, 90 * time.Minute} {
					next := r
					next.At = bedNow.Add(offset).Format(time.RFC3339)
					next.Tree = fmt.Sprint(offset)
					b.save(t, next)
					due, err := scopedProofDue(b.install, bedNow.Add(offset))
					if err != nil || due != (offset > 50*time.Minute) {
						t.Fatalf("original debt at %s=%v %v", offset, due, err)
					}
					nonrun := next
					nonrun.Scope = "full"
					nonrun.Result = Red
					nonrun.CountedFull = false
					nonrun.Cause = &Cause{Kind: "environment"}
					b.save(t, nonrun)
					if after, err := scopedProofDue(b.install, bedNow.Add(offset)); err != nil || after != due {
						t.Fatalf("non-run cleared debt: %v %v", after, err)
					}
				}
			}
		})
	}
}

func TestChangedMainRecoveryFetchesEmptyQueue(t *testing.T) {
	t.Parallel()
	for _, scenario := range mainProofReuseCases {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b := changedMainBed(t)
			wantStart := seedMainProofReuse(t, b, scenario)
			git := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				if args[0] == "fetch" {
					return "", nil
				}
				return git(dir, args...)
			}
			launches := 0
			b.seams.Launch = func([]string, string, string) (int64, error) { launches++; return int64(os.Getpid()), nil }
			before, err := os.ReadFile(resultsPath(b.install))
			if err != nil {
				t.Fatal(err)
			}
			if err := RecoverMovedMain(b.install, b.checkout, b.seams); err != nil {
				t.Fatal(err)
			}
			_, recorded, _, err := ReadRunning(b.install, b.seams)
			after, readErr := os.ReadFile(resultsPath(b.install))
			if err != nil || readErr != nil || recorded != wantStart || (launches == 1) != wantStart || string(before) != string(after) {
				t.Fatalf("recovery: recorded=%v launches=%d wantStart=%v errors=%v/%v", recorded, launches, wantStart, err, readErr)
			}
		})
	}

	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			t.Parallel()
			b := changedMainBed(t)
			b.base.Trunk = true
			b.save(t, b.base)
			local := b.base.Commit
			fetches, launches := 0, 0
			git := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				if args[0] == "fetch" {
					fetches++
					if failure {
						return "", errors.New("remote unavailable")
					}
					local = b.git.commit
					return "", nil
				}
				if strings.Contains(strings.Join(args, " "), "origin/main^{commit}") {
					return local, nil
				}
				return git(dir, args...)
			}
			b.seams.Launch = func([]string, string, string) (int64, error) { launches++; return int64(os.Getpid()), nil }
			err := RecoverMovedMain(b.install, b.checkout, b.seams)
			if failure {
				if err == nil || !strings.Contains(err.Error(), "remote unavailable") || launches != 0 {
					t.Fatalf("failed fetch: %v launches=%d", err, launches)
				}
				return
			}
			running, recorded, _, readErr := ReadRunning(b.install, b.seams)
			if err != nil || readErr != nil || !recorded || fetches != 1 || launches != 1 || running.Commit != b.git.commit || running.Admission.Scope != "scoped" {
				t.Fatalf("empty queue recovery: %+v %v %v fetch=%d launch=%d", running, err, readErr, fetches, launches)
			}
		})
	}
}

func TestChangedMainPreservesCandidateRepairCheckpoint(t *testing.T) {
	t.Parallel()
	b := changedMainBed(t)
	checkpoint := Running{Checkpoint: true, Attempt: "candidate", Commit: "candidate", Tree: "candidate-tree"}
	red := Result{Attempt: checkpoint.Attempt, Commit: checkpoint.Commit, Tree: checkpoint.Tree, Result: Red}
	b.save(t, red)
	if err := writeRunning(b.install, checkpoint); err != nil {
		t.Fatal(err)
	}
	b.seams.Alive = func(Running) bool { return false }
	git := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "fetch" {
			return "", nil
		}
		return git(dir, args...)
	}
	if err := RecoverMovedMain(b.install, b.checkout, b.seams); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Start(b.install, b.checkout, b.seams); err == nil || err.Error() != "main's check waits for the candidate's repair of tree candidate-tr to finish" {
		t.Fatalf("repair refusal: %v", err)
	}
	stored, _, _, err := ReadRunning(b.install, b.seams)
	if err != nil || !reflect.DeepEqual(stored, checkpoint) || !RecordedFixRed(b.install, stored) {
		t.Fatalf("checkpoint changed or repair inadmissible: %+v %v", stored, err)
	}
	if err := removeRunning(b.install); err != nil {
		t.Fatal(err)
	}
	if err := RecoverMovedMain(b.install, b.checkout, b.seams); err != nil {
		t.Fatal(err)
	}
	stored, _, _, err = ReadRunning(b.install, b.seams)
	if err != nil || !stored.Trunk || stored.Commit != b.git.commit {
		t.Fatalf("pending main did not start: %+v %v", stored, err)
	}
}

func TestChangedInputsDuringProofWaitForAffectedChecks(t *testing.T) {
	t.Parallel()
	b := newScopeBed(t)
	command := b.seams.Command
	b.seams.Command = func(cmd *exec.Cmd) error {
		err := command(cmd)
		b.git.tree, b.git.commit = "advanced-tree", "advanced-commit"
		return err
	}
	b.seams.Git = func(dir string, args ...string) (string, error) { return b.git.run(dir, args...) }
	result, err := Run(b.install, b.checkout, "fixture-proof", "", b.output, b.seams)
	if err != nil || result.Result != Green {
		t.Fatalf("initial proof: %+v %v", result, err)
	}
	if _, ok, err := ResultFor(b.install, b.git.tree); err != nil || ok {
		t.Fatalf("changed inputs got green without execution: %v %v", ok, err)
	}
	b.base = result
	b.git.batches[b.git.commit] = "batch-a\nbatch-b"
	b.git.changed[[2]string{result.Tree, b.git.tree}] = "metasystem/plans/designs/page.md"
	b.setContract(t)
	b.seams.NewID = func() string { return "advanced" }
	b.seams.Command = command
	next := b.run(t)
	if next.Result != Green || next.Scope != "scoped" || len(b.calls) != 2 || !reflect.DeepEqual(next.Ran, []string{"plans", "shared"}) {
		t.Fatalf("advanced input proof: %+v calls=%d", next, len(b.calls))
	}
}

func TestChangedMainStewardHealthRowRunsRecordsChecks(t *testing.T) {
	t.Parallel()
	for _, scenario := range mainProofReuseCases {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b, scoped := changedMainStewardFixture(t)
			wantStart := seedMainProofReuse(t, scoped, scenario)
			if scenario == "damaged history" || scenario == "trunk red" {
				_, err := stewardTrunk(b.install, b.checkout, scoped.seams)
				_, recorded, _, readErr := ReadRunning(b.install, scoped.seams)
				if err != nil || readErr != nil || recorded != wantStart {
					t.Fatalf("steward entrypoint: recorded=%v want=%v errors=%v/%v", recorded, wantStart, err, readErr)
				}
				return
			}
			status := ObserveStatus(b.home, b.record, scoped.seams, nil)
			if healthRow(status.Health, "unproven-incidents") == nil {
				t.Fatalf("missing health row: %+v", status)
			}
			run, err := StewardStep(b.home, bedNow, scoped.seams)
			_, recorded, _, readErr := ReadRunning(b.install, scoped.seams)
			if err != nil || readErr != nil || recorded != wantStart {
				t.Fatalf("steward: %+v recorded=%v wantStart=%v errors=%v/%v", run, recorded, wantStart, err, readErr)
			}
		})
	}

	for _, baseline := range []bool{true, false} {
		t.Run(fmt.Sprint(baseline), func(t *testing.T) {
			t.Parallel()
			b, scoped := changedMainStewardFixture(t)
			if !baseline {
				if err := os.Remove(resultsPath(b.install)); err != nil {
					t.Fatal(err)
				}
			}
			status := ObserveStatus(b.home, b.record, scoped.seams, nil)
			if healthRow(status.Health, "unproven-incidents") == nil {
				t.Fatalf("record move suppressed the health row: %+v", status)
			}
			run, err := StewardStep(b.home, bedNow, scoped.seams)
			if err != nil {
				t.Fatalf("production steward: %+v %v", run, err)
			}
			running, recorded, _, err := ReadRunning(b.install, scoped.seams)
			want := "full"
			if baseline {
				want = "scoped"
			}
			if err != nil || !recorded || running.Admission.Scope != want {
				t.Fatalf("steward admission: %+v %v", running, err)
			}
			log, err := os.Create(running.Log)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = log.Close() })
			result, err := Run(b.install, b.checkout, "fixture-proof", running.Attempt, log, scoped.seams)
			if err != nil || result.Result != Green || result.Scope != want || len(scoped.calls) != 1 {
				t.Fatalf("steward execution: %+v %v", result, err)
			}
		})
	}
}

func TestChangedMainRecoveryIncidentDepth(t *testing.T) {
	t.Parallel()
	for _, baseline := range []bool{true, false} {
		t.Run(fmt.Sprint(baseline), func(t *testing.T) {
			t.Parallel()
			b := changedMainBed(t)
			b.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
				return []goal.TrunkRedEntry{{ID: "incident", Class: goal.TrunkRedClassTrunkRed, Sightings: []goal.TrunkRedSighting{{BaseCommit: b.base.Commit}}}}, nil
			}
			if !baseline {
				if err := os.Remove(resultsPath(b.install)); err != nil {
					t.Fatal(err)
				}
			}
			git := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				if args[0] == "fetch" {
					return "", nil
				}
				return git(dir, args...)
			}
			if err := RecoverMovedMain(b.install, b.checkout, b.seams); err != nil {
				t.Fatal(err)
			}
			running, _, _, err := ReadRunning(b.install, b.seams)
			if err != nil {
				t.Fatal(err)
			}
			log, err := os.Create(running.Log)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = log.Close() })
			result, err := Run(b.install, b.checkout, "fixture-proof", running.Attempt, log, b.seams)
			want := "full"
			if baseline {
				want = "scoped"
			}
			if err != nil || result.Result != Green || result.Scope != want || len(b.calls) != 1 {
				t.Fatalf("recovery depth: %+v %v", result, err)
			}
		})
	}
}

func changedMainStewardFixture(t *testing.T) (*stewardBed, *scopeBed) {
	t.Helper()
	scoped := changedMainBed(t)
	b := newStewardBed(t)
	b.install, b.checkout = scoped.install, scoped.checkout
	b.record.Root, b.record.Install = b.checkout, b.install
	shadowRegister(t, b.home, b.record)
	scoped.seams.Machine = b.seams.Machine
	scoped.seams.Load = b.seams.Load
	scoped.seams.Steward = b.seams.Steward
	scoped.seams.Steward.Trunk = nil // Exercise the health row's production admission.
	fallback := b.seams.Git
	git := scoped.seams.Git
	scoped.seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "fetch" {
			return "", nil
		}
		if args[0] == "symbolic-ref" {
			return "main", nil
		}
		output, err := git(dir, args...)
		if err != nil && strings.Contains(err.Error(), "is not stubbed") {
			return fallback(dir, args...)
		}
		return output, err
	}
	scoped.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
		return []goal.TrunkRedEntry{{ID: "incident", Class: goal.TrunkRedClassTrunkRed, Sightings: []goal.TrunkRedSighting{{BaseCommit: scoped.base.Commit}}}}, nil
	}
	return b, scoped
}

func TestChangedMainStewardWakePreservesRepair(t *testing.T) {
	t.Parallel()
	b, scoped := changedMainStewardFixture(t)
	checkpoint := Running{Checkpoint: true, Attempt: "candidate", Commit: "candidate", Tree: "candidate-tree"}
	scoped.save(t, Result{Attempt: checkpoint.Attempt, Commit: checkpoint.Commit, Tree: checkpoint.Tree, Result: Red})
	if err := writeRunning(b.install, checkpoint); err != nil {
		t.Fatal(err)
	}
	scoped.seams.Alive = func(Running) bool { return false }
	if err := RecoverMovedMain(b.install, b.checkout, scoped.seams); err != nil {
		t.Fatal(err)
	}
	_, err := StewardStep(b.home, bedNow, scoped.seams)
	var repair *RepairPending
	if !errors.As(err, &repair) || !strings.Contains(err.Error(), "candidate's repair of tree candidate-tr to finish") {
		t.Fatalf("steward did not defer to repair: %v", err)
	}
	stored, _, _, err := ReadRunning(b.install, scoped.seams)
	if err != nil || !reflect.DeepEqual(stored, checkpoint) || !RecordedFixRed(b.install, stored) {
		t.Fatalf("steward replaced repair: %+v %v", stored, err)
	}
	if err := removeRunning(b.install); err != nil {
		t.Fatal(err)
	}
	if err := RecoverMovedMain(b.install, b.checkout, scoped.seams); err != nil {
		t.Fatal(err)
	}
	stored, _, _, err = ReadRunning(b.install, scoped.seams)
	if err != nil || !stored.Trunk || stored.Admission.Scope != "scoped" {
		t.Fatalf("pending main after repair: %+v %v", stored, err)
	}
}

func TestChangedMainUsesOriginalFullExecutionTree(t *testing.T) {
	t.Parallel()
	b := changedMainBed(t)
	if err := os.Remove(resultsPath(b.install)); err != nil {
		t.Fatal(err)
	}
	b.base.Tree = "inherited-tree"
	b.base.FullTree = "original-tree"
	b.base.At = bedNow.Add(-time.Minute).Format(time.RFC3339)
	b.base.Reason = "inherits green from tree original-tree"
	b.git.changed[[2]string{b.base.Tree, b.git.tree}] = "metasystem/plans/page.md"
	b.git.changed[[2]string{b.base.FullTree, b.git.tree}] = "metasystem/plans/page.md\nmetasystem/records/old-record.md"
	b.seams.Closure = func(string, string, string) (adapter.Closure, error) { return adapter.Closure{}, nil }
	b.save(t, b.base)
	result, err := Run(b.install, b.checkout, "fixture-proof", "", b.output, b.seams)
	if err != nil || result.Result != Green || result.Scope != "scoped" || result.Base != b.base.FullTree || result.FullAt != b.base.FullAt || commandEnv(b.calls[0], "LANDING_PROOF_GROUPS") != "plans unrelated shared" {
		t.Fatalf("original execution inputs not checked: %+v %v", result, err)
	}
}

func TestFoldedCommitsSurviveAnUnchangedProvenTree(t *testing.T) {
	t.Parallel()
	b := newScopeBed(t)
	batch := &Batch{ID: "folded", State: BatchRunning, Lane: lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}, FoldedCommits: []string{"record-commit"}}
	if err := withLock(b.install, func() error {
		return completePush(b.install, b.checkout, batch, "old-main", "folded-commit", b.base.Tree, bedNow, b.seams)
	}); err != nil {
		t.Fatal(err)
	}
	pushed, found, err := LastPush(b.install)
	if err != nil || !found || !reflect.DeepEqual(pushed.FoldedCommits, batch.FoldedCommits) {
		t.Fatalf("fold metadata lost for unchanged tree: %+v %v", pushed, err)
	}
	source, _, err := ResultFor(b.install, b.base.Tree)
	if err != nil || len(source.FoldedCommits) != 0 {
		t.Fatalf("publication rewrote the original execution: %+v %v", source, err)
	}
}

var mainProofReuseCases = []string{"landed batch", "scoped batch", "inherited batch", "expired scoped", "expired inherited", "impact", "missing environment", "changed environment", "batch red", "trunk red", "newer red", "damaged history", "pending", "nonrun"}

func seedMainProofReuse(t *testing.T, b *scopeBed, scenario string) bool {
	t.Helper()
	if err := os.Remove(resultsPath(b.install)); err != nil {
		t.Fatal(err)
	}
	r := b.base
	r.Commit, r.Tree, r.FullTree = b.git.commit, b.git.tree, b.git.tree
	r.At = bedNow.Add(-time.Minute).Format(time.RFC3339)
	r.FullAt = r.At
	want := false
	switch scenario {
	case "landed batch":
	case "scoped batch", "inherited batch", "expired scoped", "expired inherited":
		r.Scope, r.FullTree = "scoped", "original-full"
		r.FullAt = bedNow.Add(-10 * time.Minute).Format(time.RFC3339)
		if scenario == "inherited batch" || scenario == "expired inherited" {
			r.Scope, r.Reason = "full", "inherits green from tree previous"
		}
		if scenario == "expired scoped" || scenario == "expired inherited" {
			r.FullAt = bedNow.Add(-61 * time.Minute).Format(time.RFC3339)
			want = true
		}
	case "impact":
		r.Scope = "impact"
		r.FullTree, r.FullAt = "", ""
		want = true
	case "missing environment":
		r.Environment = ""
		want = true
	case "changed environment", "newer red", "damaged history":
		want = true
	case "batch red", "trunk red":
		r.Result = Red
		r.Trunk = scenario == "trunk red"
		want = !r.Trunk
	case "pending":
		r.ClassificationPending = true
		want = true
	case "nonrun":
		r.Result = Red
		r.Cause = &Cause{Kind: "environment"}
		r.Repeat = "allowed"
		want = true
	default:
		t.Fatalf("unknown proof scenario %s", scenario)
	}
	b.save(t, r)
	if scenario == "changed environment" || scenario == "newer red" {
		newer := r
		newer.Attempt, newer.At = "newer", bedNow.Format(time.RFC3339)
		if scenario == "newer red" {
			newer.Result = Red
		} else {
			newer.Tree, newer.Environment = "another-tree", "another environment"
		}
		b.save(t, newer)
	}
	if scenario == "damaged history" {
		f, err := os.OpenFile(resultsPath(b.install), os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.WriteString("broken\n")
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return want
}

func TestChangedMainPersonSettlesOnlyApplicableGreen(t *testing.T) {
	t.Parallel()
	for _, scenario := range append(append([]string{}, mainProofReuseCases...), "fresh") {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b := changedMainBed(t)
			kind := scenario
			if kind == "fresh" {
				kind = "landed batch"
				b.seams.Fresh = true
			}
			wantStart := seedMainProofReuse(t, b, kind) || scenario == "trunk red" || scenario == "fresh"
			b.seams.Person = &ActProvenance{Person: "Wido"}
			result, settled, err := Settled(b.install, b.checkout, b.seams)
			if err != nil || settled == wantStart || settled && result.Attempt != b.base.Attempt {
				t.Fatalf("settled=%v result=%+v err=%v", settled, result, err)
			}
			if !settled {
				running, already, err := Start(b.install, b.checkout, b.seams)
				if err != nil || already || running.Admission.Scope != "full" {
					t.Fatalf("person admission: %+v already=%v err=%v", running, already, err)
				}
			}
		})
	}
}

func TestChangedMainPersonOverridesRepairBeforeReplacement(t *testing.T) {
	t.Parallel()
	b := changedMainBed(t)
	checkpoint := Running{Checkpoint: true, Attempt: "candidate", Commit: "candidate", Tree: "candidate-tree"}
	b.save(t, Result{Attempt: checkpoint.Attempt, Tree: checkpoint.Tree, Commit: checkpoint.Commit, Result: Red})
	if err := writeRunning(b.install, checkpoint); err != nil {
		t.Fatal(err)
	}
	fix := &Fix{Attempt: checkpoint.Attempt, Goal: "repair", Units: []string{"unit"}, Job: "job", Commit: "candidate", State: "building"}
	if err := WriteFix(b.install, fix); err != nil {
		t.Fatal(err)
	}
	person := &ActProvenance{Person: "Wido", Kind: "proof", Root: b.checkout, CheckedAt: bedNow}
	b.seams.Person = person
	notified := false
	b.seams.NotifyRepairOverride = func(impact string) error {
		notified = true
		rows, err := readLines[Intervention](interventionsPath(b.install))
		stored, _, _, readErr := ReadRunning(b.install, b.seams)
		open, fixErr := ReadFix(b.install)
		if err != nil || readErr != nil || fixErr != nil || open == nil || open.State != "building" || !reflect.DeepEqual(stored, checkpoint) || len(rows) != 1 || !reflect.DeepEqual(rows[0].Person, person) || rows[0].What != impact || !strings.Contains(impact, "tree candidate-tr (fix repair)") || !strings.Contains(impact, "Wido's request") {
			t.Fatalf("override boundary: impact=%q rows=%+v checkpoint=%+v fix=%+v errors=%v/%v/%v", impact, rows, stored, open, err, readErr, fixErr)
		}
		return nil
	}
	b.seams.Launch = func([]string, string, string) (int64, error) {
		stored, _, _, err := ReadRunning(b.install, b.seams)
		old, fixErr := FixForAttempt(b.install, checkpoint.Attempt)
		if !notified || err != nil || fixErr != nil || old == nil || old.State != "closed" || !stored.Trunk || stored.Attempt == checkpoint.Attempt {
			t.Fatalf("launch before replacement: %+v fix=%+v notified=%v errors=%v/%v", stored, old, notified, err, fixErr)
		}
		return int64(os.Getpid()), nil
	}
	if _, _, err := Start(b.install, b.checkout, b.seams); err != nil {
		t.Fatal(err)
	}
}

func TestChangedMainPendingFixOnlyHasTrueReason(t *testing.T) {
	t.Parallel()
	b := changedMainBed(t)
	if err := WriteFix(b.install, &Fix{Attempt: "candidate", Goal: "repair", Units: []string{"unit"}, Job: "job", Commit: "candidate", State: "building"}); err != nil {
		t.Fatal(err)
	}
	_, _, err := Start(b.install, b.checkout, b.seams)
	var repair *RepairPending
	if !errors.As(err, &repair) || err.Error() != "main's check waits for the pending fix of repair" {
		t.Fatalf("wrong fix-only reason: %v", err)
	}
	b.seams.Person = &ActProvenance{Person: "Wido"}
	notified := false
	b.seams.NotifyRepairOverride = func(impact string) error {
		notified = true
		if impact != "this check supersedes the pending repair of repair; its record closes at the next lifecycle refresh" {
			t.Fatalf("fix-only impact names a nonexistent checkpoint: %q", impact)
		}
		return nil
	}
	if _, _, err := Start(b.install, b.checkout, b.seams); err != nil {
		t.Fatal(err)
	}
	if !notified {
		t.Fatal("fix-only override was not explained")
	}
}

// Admission fixtures give the launch owner and child distinct exact identities.
func trunkAdmissionBed(t *testing.T) (*scopeBed, *int, string, string) {
	t.Helper()
	b := changedMainBed(t)
	launches := 0
	const child = int64(1 << 29)
	ownerRef, childRef := syntheticTrunkProcess(t, int64(os.Getpid())), syntheticTrunkProcess(t, child)
	b.seams.ProcessRef = func(pid int64) string {
		switch pid {
		case int64(os.Getpid()):
			return ownerRef
		case child:
			return childRef
		default:
			t.Fatalf("unexpected identity read: %d", pid)
			return ""
		}
	}
	b.seams.Alive = func(r Running) bool { return r.Process != "" && !r.Checkpoint }
	b.seams.Launch = func([]string, string, string) (int64, error) { launches++; return child, nil }
	git := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "fetch" {
			return "", nil
		}
		return git(dir, args...)
	}
	return b, &launches, ownerRef, childRef
}

func syntheticTrunkProcess(t *testing.T, pid int64) string {
	t.Helper()
	ref := identity.Ref{Pid: pid, StartedAtUnixMicro: bedNow.UnixMicro()}
	if runtime.GOOS == "linux" {
		ref.StartedAtUnixMicro, ref.StartTicks, ref.BootID = 0, 12345, "fixture-boot"
	}
	encoded, err := identity.EncodeRef(ref)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func savedTrunkProof(t *testing.T, b *scopeBed) Result {
	t.Helper()
	r := b.base
	r.Trunk, r.Commit, r.Tree = true, b.git.commit, b.git.tree
	r.FullTree, r.At = r.Tree, bedNow.Add(-34*time.Second).Format(time.RFC3339)
	r.FullAt, r.Attempt = r.At, "completed"
	b.save(t, r)
	return r
}

func assertTrunkAdmission(t *testing.T, b *scopeBed, launches *int, owner, child string) {
	t.Helper()
	running, already, err := Start(b.install, b.checkout, b.seams)
	if err != nil || already || running.Reused != nil || *launches != 1 || running.Launch == nil || running.Launch.Owner != owner || running.Process != child {
		t.Fatalf("admission: %+v already=%v launches=%d err=%v", running, already, *launches, err)
	}
	stored, recorded, _, err := ReadRunning(b.install, b.seams)
	if err != nil || !recorded || !reflect.DeepEqual(stored, running) {
		t.Fatalf("stored admission: %+v recorded=%v err=%v", stored, recorded, err)
	}
}

func TestTrunkAdmissionPersonStillLaunches(t *testing.T) {
	t.Parallel()
	b, launches, owner, child := trunkAdmissionBed(t)
	savedTrunkProof(t, b)
	b.seams.Person = &ActProvenance{Person: "Wido"}
	assertTrunkAdmission(t, b, launches, owner, child)
}

func TestTrunkAdmissionFreshStillLaunches(t *testing.T) {
	t.Parallel()
	b, launches, owner, child := trunkAdmissionBed(t)
	savedTrunkProof(t, b)
	b.seams.Fresh = true
	assertTrunkAdmission(t, b, launches, owner, child)
}

func TestTrunkAdmissionRecordsMoveStillLaunchesScoped(t *testing.T) {
	t.Parallel()
	b, launches, owner, child := trunkAdmissionBed(t)
	assertTrunkAdmission(t, b, launches, owner, child)
	r, _, _, err := ReadRunning(b.install, b.seams)
	if err != nil || r.Admission == nil || r.Admission.Scope != "scoped" || !reflect.DeepEqual(r.Admission.Groups, []string{"plans", "shared"}) {
		t.Fatalf("records admission: %+v %v", r, err)
	}
}

func TestTrunkAdmissionReusesCompletion34SecondsAgo(t *testing.T) {
	t.Parallel()
	b, launches, _, _ := trunkAdmissionBed(t)
	saved := savedTrunkProof(t, b)
	running, already, err := Start(b.install, b.checkout, b.seams)
	if err != nil || !already || *launches != 0 || running.Reused == nil || !reflect.DeepEqual(*running.Reused, saved) || running.Attempt != saved.Attempt || running.Tree != saved.Tree || running.Commit != saved.Commit || running.Log != saved.Log || running.Since != saved.At || !running.Trunk || running.Process != "" {
		t.Fatalf("completed reuse: %+v already=%v launches=%d err=%v", running, already, *launches, err)
	}
	if _, recorded, _, err := ReadRunning(b.install, b.seams); err != nil || recorded {
		t.Fatalf("reuse wrote a running record: recorded=%v err=%v", recorded, err)
	}
	encoded, err := json.Marshal(running)
	withoutMarker := running
	withoutMarker.Reused = nil
	want, _ := json.Marshal(withoutMarker)
	if err != nil || !bytes.Equal(encoded, want) {
		t.Fatalf("transient result serialized: %s %v", encoded, err)
	}
	var decoded Running
	if err := json.Unmarshal(encoded, &decoded); err != nil || decoded.Reused != nil {
		t.Fatalf("transient result round-tripped: %+v %v", decoded, err)
	}
}

func TestTrunkRecoveryRechecksAfterProofFinalizes(t *testing.T) {
	t.Parallel()
	b, launches, _, _ := trunkAdmissionBed(t)
	// Run authenticates its child against this process before claiming admission.
	b.seams.ProcessRef = processRef
	b.seams.Launch = func([]string, string, string) (int64, error) { *launches++; return int64(os.Getpid()), nil }
	first, already, err := Start(b.install, b.checkout, b.seams)
	if err != nil || already || *launches != 1 {
		t.Fatalf("first admission: %+v %v", first, err)
	}
	paused, resume, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	var recovered Running
	var joined bool
	recovery := b.seams
	recovery.TimerHeld = func() bool { close(paused); <-resume; return false }
	recovery.RecoveryStarted = func(r Running, already bool) { recovered, joined = r, already }
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
	if err != nil || saved.Result != Green || saved.Attempt != first.Attempt {
		t.Fatalf("first finalization: %+v %v", saved, err)
	}
	if _, recorded, _, err := ReadRunning(b.install, b.seams); err != nil || recorded {
		t.Fatalf("finalization retained running record: %v %v", recorded, err)
	}
	last, found, err := LastResult(b.install)
	if err != nil || !found || !reflect.DeepEqual(last, saved) {
		t.Fatalf("finalization lost result: %+v %v", last, err)
	}
	if _, ok := applicableMainProof(b.install, b.checkout, b.git.commit, b.git.tree, b.seams, true); !ok {
		t.Fatalf("finalized result is not applicable: %+v", saved)
	}
	close(resume)
	resume = nil
	err = <-done
	if err != nil || *launches != 1 || !joined || recovered.Reused == nil || !reflect.DeepEqual(*recovered.Reused, saved) {
		t.Fatalf("recovery admitted again: %+v already=%v launches=%d err=%v", recovered, joined, *launches, err)
	}
}

func TestTrunkAdmissionApplicabilityHoldsLaneLock(t *testing.T) {
	t.Parallel()
	b, launches, _, _ := trunkAdmissionBed(t)
	r := b.base
	r.Trunk, r.Commit, r.Tree, r.Attempt = true, b.git.commit, b.git.tree, "red"
	r.Result, r.Cause, r.Failed = Red, &Cause{Kind: "main"}, []FailedUnit{{Unit: "fixture-unit"}}
	b.save(t, r)
	calls := 0
	b.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
		calls++
		held, err := lock.File(lockPath(b.install), 0600, lock.TryExclusive)
		if held != nil {
			_ = held.Release()
		}
		if !lock.Busy(err) {
			t.Errorf("applicability read outside lock: %v", err)
		}
		return []goal.TrunkRedEntry{{ID: "incident", Identity: "red:fixture-unit"}}, nil
	}
	running, already, err := Start(b.install, b.checkout, b.seams)
	if err != nil || !already || running.Reused == nil || running.Reused.Attempt != r.Attempt || *launches != 0 || calls == 0 {
		t.Fatalf("locked red reuse: %+v already=%v launches=%d incidents=%d err=%v", running, already, *launches, calls, err)
	}
}
