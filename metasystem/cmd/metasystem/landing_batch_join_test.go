package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

func prepublicationJoinBed(t *testing.T) (batchJoinRequest, batchJoinDependencies, *int) {
	t.Helper()
	landing, gateCalls := t.TempDir(), 0
	request := batchJoinRequest{SeatRoot: t.TempDir(), LandingRoot: landing, GoalID: "goal-a", ChainID: "chain-a", At: time.Unix(1, 0)}
	dependencies := batchJoinDependencies{
		binding: func(string, string, time.Time) (dispatchcore.GoalBinding, error) {
			return dispatchcore.GoalBinding{Revision: 2, Machine: "seat", Lineage: "lineage", File: &goal.GoalFile{Claimed: &goal.ClaimRecord{AccountingRevision: 1}}, Capability: goal.StopCapability{ClaimEpoch: 1}}, nil
		},
		chain: func(string, string, string, uint64) (batch.CertifiedChain, error) {
			return batch.CertifiedChain{ID: "chain-a", Patch: []byte("patch")}, nil
		},
		base:           func(string) (string, error) { return "base-tree", nil },
		mint:           func() (string, error) { return "01j5x00000000000000000ba99", nil },
		transport:      func(string, batch.CertifiedChain) error { return nil },
		assemble:       func(string, string, []batch.Unit) ([]string, error) { return []string{"candidate-tree"}, nil },
		fixtures:       func(string) ([]byte, error) { return nil, nil },
		protectedTests: func(string, string, string) error { return nil },
		gate: func(string, string, []byte, []byte, *batch.Unit) error {
			gateCalls++
			return nil
		},
	}
	return request, dependencies, &gateCalls
}

func assertJoinRefusedBeforeQueue(t *testing.T, request batchJoinRequest, dependencies batchJoinDependencies, code string, gateCalls *int, wantGateCalls int) error {
	t.Helper()
	_, err := executeBatchJoin(request, dependencies)
	if err == nil || !strings.Contains(err.Error(), code) || *gateCalls != wantGateCalls {
		t.Fatalf("join refusal=%v gate calls=%d want code=%s calls=%d", err, *gateCalls, code, wantGateCalls)
	}
	records, readErr := batch.NewStore(request.LandingRoot, nil).Records()
	if readErr != nil || len(records) != 0 {
		t.Fatalf("refused join changed queue: records=%+v err=%v", records, readErr)
	}
	return err
}

func TestLandingBatchJoinRefusesRedFastStaticGate(t *testing.T) {
	repository := t.TempDir()
	for _, arguments := range [][]string{{"init", "-q"}, {"config", "user.name", "Fixture"}, {"config", "user.email", "fixture@example.invalid"}} {
		command := exec.Command("git", arguments...)
		command.Dir = repository
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
	}
	if err := os.MkdirAll(filepath.Join(repository, "metasystem"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "metasystem", "go.mod"), []byte("module fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"add", "."}, {"commit", "-q", "-m", "seed"}} {
		command := exec.Command("git", arguments...)
		command.Dir = repository
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
	}
	command := exec.Command("git", "rev-parse", "HEAD^{tree}")
	command.Dir = repository
	treeBytes, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	fakeBin := t.TempDir()
	fakeBash := "#!/bin/sh\n[ -f go.mod ] || { echo 'wrong module root' >&2; exit 9; }\necho 'staticcheck: unused assignment' >&2\nexit 7\n"
	if err := os.WriteFile(filepath.Join(fakeBin, "bash"), []byte(fakeBash), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	result := productionJoinGate(repository)(strings.TrimSpace(string(treeBytes)), batch.GateStep{Name: "fast gate", Args: []string{"bash", "scripts/agents/go-gate.sh", "--fast"}})
	if result.ExitCode != 7 || !strings.Contains(result.Detail, "staticcheck") {
		t.Fatalf("production static result=%+v", result)
	}

	request, dependencies, calls := prepublicationJoinBed(t)
	dependencies.gate = func(_ string, tree string, patch, fixtures []byte, unit *batch.Unit) error {
		*calls++
		return batch.RunJoinGate(tree, patch, fixtures, unit, func(string, batch.GateStep) batch.GateStepResult {
			return batch.GateStepResult{RunID: "red-static", ExitCode: 1, Detail: "staticcheck: unused assignment"}
		})
	}
	joinErr := assertJoinRefusedBeforeQueue(t, request, dependencies, "BATCH_JOIN_GATE_RED", calls, 1)
	if !strings.Contains(joinErr.Error(), "fast gate") || !strings.Contains(joinErr.Error(), "staticcheck") {
		t.Fatalf("static gate refusal did not name the failing step: %v", joinErr)
	}
}

func TestProductionJoinGateSelectsPackagesAgainstTheLandingRoot(t *testing.T) {
	repository := t.TempDir()
	git := func(arguments ...string) string {
		command := exec.Command("git", arguments...)
		command.Dir = repository
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "-q")
	git("config", "user.name", "Fixture")
	git("config", "user.email", "fixture@example.invalid")
	files := map[string]string{
		"metasystem/go.mod":                       "module fixture\n",
		"metasystem/outer/keep.go":                "package outer\n",
		"metasystem/outer/missing/inner/value.go": "package inner\n",
		"metasystem/scripts/agents/go-gate.sh":    "#!/bin/sh\nexit 0\n",
	}
	for path, content := range files {
		absolute := filepath.Join(repository, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(path, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(absolute, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("commit", "-q", "-m", "base")
	if err := os.Remove(filepath.Join(repository, "metasystem", "outer", "missing", "inner", "value.go")); err != nil {
		t.Fatal(err)
	}
	git("add", "-u")
	tree := git("write-tree")
	patch := []byte(git("diff", "--cached", "--binary", "HEAD"))
	unit := batch.Unit{}
	if err := productionBatchJoinDependencies().gate(repository, tree, patch, nil, &unit); err != nil {
		t.Fatalf("production join gate: %v", err)
	}
	if len(unit.Gate) != 2 {
		t.Fatalf("production join gate runs=%v, want fast gate and parent package", unit.Gate)
	}
}

func TestLandingBatchJoinRefusesDroppedListedTest(t *testing.T) {
	production := productionBatchJoinDependencies()
	if reflect.ValueOf(production.protectedTests).Pointer() != reflect.ValueOf(batch.CheckProtectedTests).Pointer() {
		t.Fatal("production join does not use the protected-test gate")
	}
	request, dependencies, calls := prepublicationJoinBed(t)
	dependencies.protectedTests = func(_, _, _ string) error {
		return fmt.Errorf("BATCH_JOIN_TEST_DROPPED: group landing-command-standard package cmd/metasystem test TestProtectedJoin is absent from the candidate tree")
	}
	assertJoinRefusedBeforeQueue(t, request, dependencies, "BATCH_JOIN_TEST_DROPPED", calls, 1)
}
