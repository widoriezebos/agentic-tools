package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func prepublicationJoinBed(t *testing.T) (batchJoinRequest, batchJoinDependencies, *int) {
	t.Helper()
	landing, admissionCalls := t.TempDir(), 0
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
		protectedTests: func(string, string, string) error { return nil },
		admissionRun: func(string, string, batch.Unit) (batch.JoinAdmission, error) {
			admissionCalls++
			return batch.JoinAdmission{Tree: "candidate-tree", Status: "verified"}, nil
		},
	}
	return request, dependencies, &admissionCalls
}

func assertJoinRefusedBeforeQueue(t *testing.T, request batchJoinRequest, dependencies batchJoinDependencies, code string, admissionCalls *int, wantAdmissionCalls int) error {
	t.Helper()
	_, err := executeBatchJoin(request, dependencies)
	if err == nil || !strings.Contains(err.Error(), code) || *admissionCalls != wantAdmissionCalls {
		t.Fatalf("join refusal=%v admission calls=%d want code=%s calls=%d", err, *admissionCalls, code, wantAdmissionCalls)
	}
	records, readErr := batch.NewStore(request.LandingRoot, nil).Records()
	if readErr != nil || len(records) != 0 {
		t.Fatalf("refused join changed queue: records=%+v err=%v", records, readErr)
	}
	return err
}

func TestLandingBatchJoinRefusesZeroAccountingRevision(t *testing.T) {
	t.Parallel()
	request, dependencies, admissionCalls := prepublicationJoinBed(t)
	dependencies.binding = func(string, string, time.Time) (dispatchcore.GoalBinding, error) {
		return dispatchcore.GoalBinding{Revision: 2, Machine: "seat", Lineage: "lineage", File: &goal.GoalFile{Claimed: &goal.ClaimRecord{}}, Capability: goal.StopCapability{ClaimEpoch: 1}}, nil
	}
	dependencies.publishAdmission = func(batch.Store, string, batch.Unit, string, time.Time, func(string, string, string) (testpolicy.Plan, error), func() error, batch.JoinAdmissionRun) error {
		return errors.New("join reached publication without an accounting revision")
	}
	assertJoinRefusedBeforeQueue(t, request, dependencies, "BATCH_JOIN_REVISION_MOVED", admissionCalls, 0)
}

func TestBatchTreePlanCommandUsesNestedModuleRoot(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	module := filepath.Join(repository, "metasystem")
	if err := os.Mkdir(module, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	command := batchTreePlanCommand("metasystem", repository, "goal-a", "tree-a", testpolicy.ModeAuto)
	if command.Dir != module {
		t.Fatalf("batch tree plan directory=%s, want nested module %s", command.Dir, module)
	}
	want := []string{"metasystem", "test", "plan", "--root", module, "--goal", "goal-a", "--tree", "tree-a", "--mode", "auto", "--purpose", "delivery", "--json"}
	if !reflect.DeepEqual(command.Args, want) {
		t.Fatalf("batch tree plan argv=%v, want %v", command.Args, want)
	}
}

func TestDirectoryTreesOverlapRejectsAncestorsOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cases := []struct {
		name        string
		left, right string
		want        bool
	}{
		{name: "equal", left: root, right: root, want: true},
		{name: "left ancestor", left: root, right: filepath.Join(root, "child"), want: true},
		{name: "right ancestor", left: filepath.Join(root, "child"), right: root, want: true},
		{name: "siblings", left: filepath.Join(root, "left"), right: filepath.Join(root, "right")},
		{name: "prefix sibling", left: root, right: root + "-neighbor"},
	}
	for _, test := range cases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := directoryTreesOverlap(test.left, test.right); got != test.want {
				t.Fatalf("directoryTreesOverlap(%q, %q)=%t, want %t", test.left, test.right, got, test.want)
			}
		})
	}
}

func TestLandingBatchJoinRefusesDroppedListedTest(t *testing.T) {
	t.Parallel()
	production := productionBatchJoinDependencies()
	if reflect.ValueOf(production.protectedTests).Pointer() != reflect.ValueOf(productionBatchProtectedTests).Pointer() {
		t.Fatal("production join does not use the protected-test gate")
	}
	request, dependencies, calls := prepublicationJoinBed(t)
	dependencies.protectedTests = func(_, _, _ string) error {
		return fmt.Errorf("BATCH_JOIN_TEST_DROPPED: group landing-command-standard package cmd/metasystem test TestProtectedJoin is absent from the candidate tree")
	}
	assertJoinRefusedBeforeQueue(t, request, dependencies, "BATCH_JOIN_TEST_DROPPED", calls, 0)
}

func TestLandingBatchProtectedTestsUseConfiguredContractAndProjectCWD(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	portableGitAt(t, project, "init", "-q")
	portableGitAt(t, project, "config", "user.name", "Fixture")
	portableGitAt(t, project, "config", "user.email", "fixture@example.invalid")
	installation := filepath.Join(project, "metasystem")
	contract := testpolicy.Contract{
		SchemaVersion: 1,
		ProjectRisk:   testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:      []testpolicy.Surface{{ID: "app", Paths: []string{"app/**"}, Standard: []string{"protected"}, Critical: []string{"protected"}}},
		Groups: []testpolicy.Group{{ID: "protected", Kind: "unit", Adapter: "go", CWD: "app", Inputs: []string{"pkg/**"},
			Obligations: []string{"protected"}, Platforms: []string{"any"}, TargetMS: 1, Packages: []string{"./pkg"}, Tests: json.RawMessage(`["TestProtected"]`)}},
		Always: testpolicy.Always{Canary: []string{"protected"}}, Unknown: []string{"protected"}, Cadence: []string{},
	}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string][]byte{
		"metasystem/go.mod":                []byte("module fixture\n"),
		"metasystem/metasystem.conf":       []byte("testing.contract=contracts/custom.json\n"),
		"metasystem/contracts/custom.json": data,
		"app/pkg/value_test.go":            []byte("package pkg\nimport \"testing\"\nfunc TestProtected(t *testing.T) {}\n"),
	} {
		absolute := filepath.Join(project, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	portableGitAt(t, project, "add", ".")
	portableGitAt(t, project, "commit", "-q", "-m", "base")
	base := portableGitAt(t, project, "rev-parse", "HEAD^{tree}")
	if err := productionBatchProtectedTests(installation, base, base); err != nil {
		t.Fatalf("configured base contract and project cwd rejected: %v", err)
	}
	if err := productionBatchProtectedTests(project, base, base); err != nil {
		t.Fatalf("project-root batch join could not resolve nested installation: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, "app", "pkg", "value_test.go"),
		[]byte("package pkg\nimport \"testing\"\nfunc TestRenamed(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	contract.Groups[0].Tests = json.RawMessage(`["TestRenamed"]`)
	data, err = json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "contracts", "custom.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	portableGitAt(t, project, "add", ".")
	candidate := portableGitAt(t, project, "write-tree")
	if err := productionBatchProtectedTests(installation, base, candidate); err == nil ||
		!strings.Contains(err.Error(), "BATCH_JOIN_TEST_DROPPED") || !strings.Contains(err.Error(), "TestProtected") {
		t.Fatalf("configured candidate dropped-test refusal=%v", err)
	}
	request, dependencies, admissionCalls := prepublicationJoinBed(t)
	request.LandingRoot = project
	dependencies.base = func(string) (string, error) { return base, nil }
	dependencies.assemble = func(string, string, []batch.Unit) ([]string, error) { return []string{candidate}, nil }
	dependencies.protectedTests = productionBatchProtectedTests
	assertJoinRefusedBeforeQueue(t, request, dependencies, "BATCH_JOIN_TEST_DROPPED", admissionCalls, 0)
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"),
		[]byte("testing.contract=contracts/other.json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := productionBatchProtectedTests(project, base, base); err == nil ||
		!strings.Contains(err.Error(), "base metasystem.conf differs") {
		t.Fatalf("uncommitted contract-path replacement refusal=%v", err)
	}
}
