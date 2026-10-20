package up

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/up/testdata/startfixture"
)

func currentRunnerRepair(string) (steward.RunnerRepairOutcome, error) {
	return steward.RunnerRepairOutcome{Status: "CURRENT", Generation: 148}, nil
}

func TestStartVerifiesTheFourProcessesAfterReconciliation(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"ordinary", "recovery"} {
		for _, fault := range []string{"current", "owner", "watcher", "reaper", "runner", "unknown"} {
			t.Run(mode+"/"+fault, func(t *testing.T) {
				t.Parallel()
				options := seatUpOptions(t)
				binary, digest := startfixture.Stage(t, options.Root)
				options.Binary = binary
				changes := map[string]string{}
				if fault != "current" {
					changes[fault] = "obsolete"
				}
				if fault == "unknown" {
					changes = map[string]string{"watcher": "unknown"}
				}
				options.Executions = startfixture.Executions(t, options.Root, binary, changes)
				home := t.TempDir()
				options.LaneHome = func() (string, error) { return home, nil }
				options.RepairStewardRunner = currentRunnerRepair
				options.RecoverOnly = true
				options.IfDown = true
				result := Result{}
				if mode == "ordinary" {
					result, _ = ordinaryBody(options)
				} else {
					result = recovery(options)
				}
				generation := component(result, "engine-generation")
				if fault == "current" {
					want := "armed"
					if mode == "recovery" {
						want = "recovery-not-needed"
					}
					if result.Outcome != want || result.ExitCode() != 0 || generation.Outcome != "verified" || !strings.Contains(generation.Detail, digest) {
						t.Fatalf("current: %+v", result)
					}
				} else {
					want := "failed"
					if mode == "recovery" {
						want = "recovery-partial"
					}
					role := fault
					if fault == "unknown" {
						role = "watcher"
					}
					if result.Outcome != want || result.ExitCode() != 1 || !strings.Contains(generation.Detail, role) {
						t.Fatalf("stale callback accepted: %+v", result)
					}
					if fault == "unknown" {
						if !strings.Contains(generation.Detail, "unverifiable") || !strings.Contains(generation.Detail, "executable identity is unknown") {
							t.Fatalf("unknown cause lost: %+v", result)
						}
					} else {
						oldDigest := options.Executions(options.Root)[fault].Digest
						if !strings.Contains(generation.Detail, oldDigest) {
							t.Fatalf("digest lost: %+v", result)
						}
					}
				}
				if _, err := os.Stat(plain.Dir(options.Root)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("non-lane created landing directory: %v", err)
				}
			})
		}
	}
	for _, registered := range []bool{false, true} {
		t.Run(map[bool]string{false: "excluded-non-lane", true: "excluded-lane"}[registered], func(t *testing.T) {
			t.Parallel()
			options := seatUpOptions(t)
			binary, _ := startfixture.Stage(t, options.Root)
			options.Binary = binary
			options.Executions = startfixture.Executions(t, options.Root, binary, map[string]string{"runner": "absent"})
			home := t.TempDir()
			options.LaneHome = func() (string, error) { return home, nil }
			if registered {
				startfixture.Register(t, home, options.Root)
			}
			options.EnsureStewardRunner = func(string, *steward.EnrolledBinary) (steward.EnsureRunnerResult, error) {
				return steward.EnsureRunnerResult{Action: "excluded"}, nil
			}
			result, _ := ordinaryBody(options)
			line := component(result, "engine-generation")
			if registered {
				if result.Outcome != "armed" || result.ExitCode() != 0 || line.Outcome != "verified" || !strings.Contains(line.Detail, "roles=owner,watcher,reaper") || strings.Contains(line.Detail, "runner") {
					t.Fatalf("excluded lane: %+v", result)
				}
			} else if result.Outcome != "armed" || result.ExitCode() != 0 || strings.Contains(line.Detail, "runner") {
				t.Fatalf("excluded non-lane: %+v", result)
			}
		})
	}
}

func TestProofCannotStartBetweenTheGuardAndTheReplacement(t *testing.T) {
	t.Parallel()
	root := canonicalRuntimePath(t.TempDir())
	home := t.TempDir()
	startfixture.Stage(t, root)
	startfixture.Register(t, home, root)
	options := Options{Root: root, LaneHome: func() (string, error) { return home, nil }, ProofAlive: func(plain.Running) bool { return true }}
	nonLane := canonicalRuntimePath(t.TempDir())
	nonLaneOptions := options
	nonLaneOptions.Root = nonLane
	directStop := false
	if attempt, err := supervisorProofGate(nonLaneOptions)(func() error { directStop = true; return nil }); err != nil || attempt != "" || !directStop {
		t.Fatalf("non-lane gate: %q %v stopped=%t", attempt, err, directStop)
	}
	if _, err := os.Stat(plain.Dir(nonLane)); !os.IsNotExist(err) {
		t.Fatalf("non-lane gate created landing directory: %v", err)
	}
	gate := supervisorProofGate(options)
	called := false
	attempt, err := gate(func() error {
		called = true
		held, err := lock.File(filepath.Join(plain.Dir(root), "lane.lock"), 0600, lock.TryExclusive)
		if held != nil {
			_ = held.Release()
		}
		if !lock.Busy(err) {
			t.Fatalf("proof admission can race the stop: %v", err)
		}
		return nil
	})
	if err != nil || attempt != "" || !called {
		t.Fatalf("guard: %q %v called=%t", attempt, err, called)
	}
	held, err := lock.File(filepath.Join(plain.Dir(root), "lane.lock"), 0600, lock.TryExclusive)
	if err != nil {
		t.Fatal(err)
	}
	_ = held.Release()
	startfixture.JSON(t, filepath.Join(plain.Dir(root), "running.json"), plain.Running{Attempt: "proof-148"})
	called = false
	attempt, err = gate(func() error { called = true; return nil })
	if err != nil || attempt != "proof-148" || called {
		t.Fatalf("live proof: %q %v called=%t", attempt, err, called)
	}
}
