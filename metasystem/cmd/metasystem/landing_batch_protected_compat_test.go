package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gopackages"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// Identical selected group IDs do not make a rebased policy equal.
func TestBatchRebasedVerifyUsesIdentityComposition(t *testing.T) {
	t.Parallel()
	claim := batch.Claim{Revision: 1, AccountingRevision: 1}
	units := []batch.Unit{{GoalID: "goal-a", Claim: claim, SelectedGroups: []string{"protected"}}}
	seal := map[string]batch.Claim{"goal-a": claim}
	decision := func(contractDigest string) batch.PrefixDecision {
		t.Helper()
		got, err := planPrefixDecisionWith("", units, "rebased-tree", func(_, _, tree string, _ testpolicy.Mode, _ []string) (testingPlanOutput, error) {
			return testingPlanOutput{PolicyBaseCommit: "trusted-base", CandidateTree: tree, ContractDigest: contractDigest,
				BaseContractDigest: "base-policy", Plan: testpolicy.Plan{RequiredGroups: []string{"protected"}, SelectedGroups: []string{"protected"}}}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	first, moved := decision("policy-one"), decision("policy-two")
	if !slices.Equal(first.Groups, moved.Groups) || first.PolicyContext == moved.PolicyContext {
		t.Fatalf("rebased policy context did not bind contract identity: first=%+v moved=%+v", first, moved)
	}
	firstID, err := batch.PrefixDecisionID("base-tree", "rebased-tree", units, seal, first)
	if err != nil {
		t.Fatal(err)
	}
	movedID, err := batch.PrefixDecisionID("base-tree", "rebased-tree", units, seal, moved)
	if err != nil || firstID == movedID {
		t.Fatalf("rebased proof accepted changed policy context: before=%s after=%s err=%v", firstID, movedID, err)
	}
}

// The required static group's typed refusal must reach the join command.
func TestLandingBatchJoinRefusesRedFastStaticGate(t *testing.T) {
	t.Parallel()
	contract, err := testpolicy.Load(filepath.Join("..", "..", "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := testpolicy.Select(contract, testpolicy.SelectionRequest{ChangedPaths: []string{"metasystem/cmd/metasystem/landing_batch_join.go"}, RequestedMode: testpolicy.ModeDeep, Purpose: testpolicy.PurposeDelivery})
	if err != nil || !slices.Contains(plan.RequiredGroups, "fast-static-build") {
		t.Fatalf("static admission group is not required: groups=%v err=%v", plan.RequiredGroups, err)
	}
	request, dependencies, _ := prepublicationJoinBed(t)
	called := 0
	dependencies.admissionRun = func(_, _ string, _ batch.Unit) (batch.JoinAdmission, error) {
		called++
		return batch.JoinAdmission{}, &batch.JoinAdmissionRed{Reason: "BATCH_JOIN_ADMISSION_RED: fast-static-build staticcheck: unused assignment"}
	}
	dependencies.publishAdmission = func(_ batch.Store, id string, unit batch.Unit, _ string, _ time.Time,
		_ func(string, string, string) (testpolicy.Plan, error), _ func() error, run batch.JoinAdmissionRun) error {
		_, err := run(id, unit)
		return err
	}
	_, err = executeBatchJoin(request, dependencies)
	var red *batch.JoinAdmissionRed
	if !errors.As(err, &red) || called != 1 || !strings.Contains(err.Error(), "fast-static-build") || !strings.Contains(err.Error(), "staticcheck") {
		t.Fatalf("red static admission was not preserved: calls=%d err=%v", called, err)
	}
	records, err := batch.NewStore(request.LandingRoot, nil).Records()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if len(record.Units) != 0 {
			t.Fatalf("red static admission published batch membership: %+v", record.Units)
		}
	}
}

// A nested landing module, rather than the caller's project root, owns Go
// package discovery. A base change must include its transitive consumer.
func TestProductionJoinGateSelectsPackagesAgainstTheLandingRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	module := filepath.Join(root, "metasystem")
	for path, contents := range map[string]string{
		"metasystem/go.mod":                    "module example.invalid/protected\n\ngo 1.27\n",
		"metasystem/base/base.go":              "package base\nconst Value = 1\n",
		"metasystem/base/base_test.go":         "package base\nimport \"testing\"\nfunc TestBase(t *testing.T) {}\n",
		"metasystem/consumer/consumer.go":      "package consumer\nimport \"example.invalid/protected/base\"\nvar Value = base.Value\n",
		"metasystem/consumer/consumer_test.go": "package consumer\nimport \"testing\"\nfunc TestConsumer(t *testing.T) {}\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	testingFixtureGit(t, root, "init", "-q")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "base")
	base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD^{tree}"))
	if err := os.WriteFile(filepath.Join(module, "base", "base.go"), []byte("package base\nconst Value = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testingFixtureGit(t, root, "add", ".")
	tree := strings.TrimSpace(testingFixtureGit(t, root, "write-tree"))
	selection, err := gopackages.Select(module, base, tree)
	if err != nil {
		t.Fatal(err)
	}
	if selection.Tree == "" || !slices.Equal(selection.Packages, []string{"./base", "./consumer"}) ||
		!slices.Contains(selection.InputDirs["./consumer"], "./base") {
		t.Fatalf("nested landing module lost changed package or transitive consumer: %+v", selection)
	}
}
