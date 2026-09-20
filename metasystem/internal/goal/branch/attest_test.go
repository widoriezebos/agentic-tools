package branch_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

func unitTree(t *testing.T, f *branchFixture, commit string) string {
	t.Helper()
	return git(t, f.root, "rev-parse", commit+"^{tree}")
}

func readerRecord(t *testing.T, f *branchFixture, commit string) string {
	t.Helper()
	digest, err := branch.UnitDigest(f.root, commit)
	if err != nil {
		t.Fatal(err)
	}
	path := "metasystem/records/misc/goal-a-u1-read.md"
	write(t, f.root, path, "Read commit "+commit+" with unit digest "+digest+" and found it clean.\n")
	return path
}

func TestAttestationReaderRecordIntegrity(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	record := readerRecord(t, f, unit)
	_, att, err := branch.CommitRead(branch.CommitReadRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "read-integrity", CheckClaim: claimAllowed,
		ReaderRecord: record, GateRunID: "fast-a", GateTree: unitTree(t, f, unit),
	})
	if err != nil {
		t.Fatal(err)
	}
	if att.Source.Kind != "reader-record" || att.Source.RecordSHA256 == "" {
		t.Fatalf("source = %+v", att.Source)
	}
	if _, err := branch.ValidateAttestation(f.root, f.base, "goal-a", "u1", unit); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.root, "metasystem", "records", "reads", "goal-a", unit+".json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := []byte(strings.Replace(string(original), "fast-a", "fast-b", 1))
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := branch.ValidateAttestation(f.root, f.base, "goal-a", "u1", unit); err == nil || !strings.Contains(err.Error(), branch.ReadInvalidCode) {
		t.Fatalf("edited attestation = %v", err)
	}
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	second := commitUnit(t, f, "u2", "metasystem/code.go", "two")
	copyPath := filepath.Join(f.root, "metasystem", "records", "reads", "goal-a", second+".json")
	if err := os.WriteFile(copyPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := branch.ValidateAttestation(f.root, f.base, "goal-a", "u2", second); err == nil || !strings.Contains(err.Error(), branch.ReadInvalidCode) {
		t.Fatalf("copied attestation = %v", err)
	}
}

func TestGLENestedReadCommitUsesProjectPaths(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	record := readerRecord(t, f, unit)
	installation := filepath.Join(f.root, "metasystem")
	read, att, err := branch.CommitRead(branch.CommitReadRequest{
		Repo: installation, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1",
		OpID: "nested-read", CheckClaim: claimAllowed, ReaderRecord: record,
		GateRunID: "fast-nested", GateTree: unitTree(t, f, unit),
	})
	if err != nil {
		t.Fatal(err)
	}
	if att.Source.ReaderRecord != record || read == "" {
		t.Fatalf("read=%q source=%+v", read, att.Source)
	}
	if _, err := os.Stat(filepath.Join(f.root, "metasystem", "records", "reads", "goal-a", unit+".json")); err != nil {
		t.Fatalf("project-relative attestation missing: %v", err)
	}
	if _, err := branch.ValidateAttestation(installation, f.base, "goal-a", "u1", unit); err != nil {
		t.Fatalf("nested read validation: %v", err)
	}
}

func TestAttestationRequiresFastGateAndNamesChangedTests(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	write(t, f.root, "metasystem/code_test.go", "package fixture\n")
	git(t, f.root, "add", ".")
	git(t, f.root, "commit", "-qm", "base test")
	git(t, f.root, "push", "-q", "origin", "HEAD:main")
	f.base = git(t, f.root, "rev-parse", "HEAD")
	unit := commitUnit(t, f, "u1", "metasystem/code_test.go", "package fixture\n// changed\n")
	record := readerRecord(t, f, unit)
	base := branch.CommitReadRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "read-gate", CheckClaim: claimAllowed,
		ReaderRecord: record,
	}
	if _, _, err := branch.CommitRead(base); err == nil || !strings.Contains(err.Error(), branch.ReadUngatedCode) {
		t.Fatalf("ungated read = %v", err)
	}
	base.GateRunID, base.GateTree = "fast-a", f.base
	if _, _, err := branch.CommitRead(base); err == nil || !strings.Contains(err.Error(), branch.ReadUngatedCode) {
		t.Fatalf("wrong-tree gate = %v", err)
	}
	base.GateTree = unitTree(t, f, unit)
	if _, _, err := branch.CommitRead(base); err == nil || !strings.Contains(err.Error(), branch.ReadTestsUnnamedCode) {
		t.Fatalf("unnamed changed test = %v", err)
	}
	base.TestsChanged = []branch.TestChange{{Path: "metasystem/code_test.go", ReaderWord: "assertions still prove the intended behavior"}}
	if _, _, err := branch.CommitRead(base); err != nil {
		t.Fatal(err)
	}
}

func writeJSONFixture(t *testing.T, root, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, path, string(append(data, '\n')))
}

func TestAttestationCriticRootSourceValidates(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	subject, present, err := dispatch.ComputeReadSubject(dispatch.ReadSubjectRequest{
		RepoRoot: f.root, Role: "code-critic", Reviews: "commit:" + unit,
	})
	if err != nil || !present {
		t.Fatalf("subject present=%v err=%v", present, err)
	}
	root := map[string]any{
		"jobId": "critic-a", "role": "code-critic", "round": 1, "status": "completed", "chainClosed": true,
		"findingRegister": []any{}, "findingRegisterRound": 1, "findingRegisterSubjectDigest": subject.Digest(),
		"closure": map[string]any{"criticRoot": "critic-a", "round": 1, "subject": subject, "mechanism": "clean"},
	}
	writeJSONFixture(t, f.root, "artifacts/agents/jobs/critic-a.json", root)
	writeJSONFixture(t, f.root, "artifacts/agents/critic-a/rounds/1/subject.json", subject)
	writeJSONFixture(t, f.root, "artifacts/agents/critic-a/rounds/1/return.json", map[string]any{
		"jobId": "critic-a", "round": 1, "reviewedTree": subject.Tree,
	})
	_, att, err := branch.CommitRead(branch.CommitReadRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "read-critic", CheckClaim: claimAllowed,
		RootJob: "critic-a", GateRunID: "fast-a", GateTree: subject.Tree,
	})
	if err != nil {
		t.Fatal(err)
	}
	if att.Source.Kind != "critic-root" || att.Source.Round != 1 {
		t.Fatalf("source = %+v", att.Source)
	}
}

type carryFixture struct {
	fixture *branchFixture
	unit    string
}

func newCarryFixture(t *testing.T) carryFixture {
	t.Helper()
	f := newBranchFixture(t)
	write(t, f.root, "metasystem/code.go", "base")
	write(t, f.root, "metasystem/plans/goal-a.md", "base")
	git(t, f.root, "add", ".")
	git(t, f.root, "commit", "-qm", "branch fixture base")
	git(t, f.root, "push", "-q", "origin", "HEAD:main")
	f.base = git(t, f.root, "rev-parse", "HEAD")
	stage(t, f, "metasystem/plans/goal-a.md", "plan")
	if _, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", OpID: "carry-plan", Kind: branch.Plan, CheckClaim: claimAllowed,
	}); err != nil {
		t.Fatal(err)
	}
	unit := commitUnit(t, f, "u1", "metasystem/code.go", "unit")
	record := readerRecord(t, f, unit)
	if _, _, err := branch.CommitRead(branch.CommitReadRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "carry-old", CheckClaim: claimAllowed,
		ReaderRecord: record, GateRunID: "fast-old", GateTree: unitTree(t, f, unit),
	}); err != nil {
		t.Fatal(err)
	}
	return carryFixture{fixture: f, unit: unit}
}

func rebaseCarryFixture(t *testing.T, c carryFixture, changedPath string) (string, string) {
	return rebaseCarryUnit(t, c, changedPath, "u1")
}

func rebaseCarryUnit(t *testing.T, c carryFixture, changedPath, unitName string) (string, string) {
	t.Helper()
	f := c.fixture
	other := filepath.Join(t.TempDir(), "endpoint")
	git(t, filepath.Dir(other), "clone", "-q", f.origin, other)
	git(t, other, "config", "user.name", "fixture")
	git(t, other, "config", "user.email", "fixture@example.invalid")
	write(t, other, changedPath, "endpoint")
	git(t, other, "add", ".")
	git(t, other, "commit", "-qm", "endpoint advances")
	git(t, other, "push", "-q", "origin", "HEAD:main")
	endpoint := git(t, other, "rev-parse", "HEAD")
	git(t, f.root, "fetch", "-q", "origin")
	git(t, f.root, "rebase", "-q", "-X", "theirs", "--onto", endpoint, f.base, "goal/goal-a")
	tip := git(t, f.root, "rev-parse", "HEAD")
	commits, err := branch.ValidateRange(f.root, endpoint, tip, "goal-a")
	if err != nil {
		t.Fatal(err)
	}
	for _, commit := range commits {
		if commit.Kind == branch.Unit && commit.Unit == unitName {
			return endpoint, commit.ID
		}
	}
	t.Fatal("rebased range has no unit")
	return "", ""
}

func newTwoUnitCarryFixture(t *testing.T) carryFixture {
	t.Helper()
	f := newBranchFixture(t)
	for _, path := range []string{"metasystem/plans/p1.md", "metasystem/plans/p2.md", "metasystem/code1.go", "metasystem/code2.go"} {
		write(t, f.root, path, "base")
	}
	git(t, f.root, "add", ".")
	git(t, f.root, "commit", "-qm", "two-unit base")
	git(t, f.root, "push", "-q", "origin", "HEAD:main")
	f.base = git(t, f.root, "rev-parse", "HEAD")
	for i, step := range []struct {
		kind branch.Kind
		unit string
		path string
		body string
	}{
		{branch.Plan, "", "metasystem/plans/p1.md", "plan one"},
		{branch.Unit, "u1", "metasystem/code1.go", "unit one"},
		{branch.Plan, "", "metasystem/plans/p2.md", "plan two"},
		{branch.Unit, "u2", "metasystem/code2.go", "unit two"},
	} {
		stage(t, f, step.path, step.body)
		if _, err := branch.CommitStaged(branch.CommitRequest{
			Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: step.unit,
			OpID: fmt.Sprintf("two-unit-%d", i), Kind: step.kind, CheckClaim: claimAllowed,
		}); err != nil {
			t.Fatal(err)
		}
	}
	tip := git(t, f.root, "rev-parse", "refs/heads/goal/goal-a")
	commits, err := branch.ValidateRange(f.root, f.base, tip, "goal-a")
	if err != nil {
		t.Fatal(err)
	}
	unit := ""
	for _, commit := range commits {
		if commit.Kind == branch.Unit && commit.Unit == "u2" {
			unit = commit.ID
		}
	}
	record := readerRecord(t, f, unit)
	if _, _, err := branch.CommitRead(branch.CommitReadRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u2", OpID: "two-unit-read",
		CheckClaim: claimAllowed, ReaderRecord: record, GateRunID: "fast-old", GateTree: unitTree(t, f, unit),
	}); err != nil {
		t.Fatal(err)
	}
	return carryFixture{fixture: f, unit: unit}
}

func TestAttestationCarryPreservesUnitAndFoldBytes(t *testing.T) {
	t.Parallel()
	c := newCarryFixture(t)
	endpoint, unit := rebaseCarryFixture(t, c, "unrelated.txt")
	_, att, err := branch.CommitRead(branch.CommitReadRequest{
		Repo: c.fixture.root, Remote: "origin", EndpointTip: endpoint, GoalID: "goal-a", Unit: "u1", OpID: "carry-new", CheckClaim: claimAllowed,
		Carry: c.unit, GateRunID: "fast-new", GateTree: unitTree(t, c.fixture, unit),
	})
	if err != nil {
		t.Fatal(err)
	}
	if att.Carry == nil || att.Carry.FromCommit != c.unit || att.Carry.ToCommit != unit {
		t.Fatalf("carry = %+v", att.Carry)
	}
	if want := git(t, c.fixture.root, "rev-parse", c.unit+"^{tree}"); att.Carry.FromTree != want {
		t.Fatalf("carry from tree = %s, want %s", att.Carry.FromTree, want)
	}
	if want := git(t, c.fixture.root, "rev-parse", unit+"^{tree}"); att.Carry.ToTree != want {
		t.Fatalf("carry to tree = %s, want %s", att.Carry.ToTree, want)
	}
}

func TestReadInstallRefusalLeavesNothingStaged(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	record := readerRecord(t, f, unit)
	lockPath := filepath.Join(f.root, ".git", "refs", "heads", "goal", "goal-a.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := branch.CommitRead(branch.CommitReadRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "read-install-refusal",
		CheckClaim: claimAllowed, ReaderRecord: record, GateRunID: "fast", GateTree: unitTree(t, f, unit),
	})
	if err == nil || !strings.Contains(err.Error(), "cannot lock ref") {
		t.Fatalf("locked read install = %v", err)
	}
	attestation := filepath.Join(f.root, "metasystem", "records", "reads", "goal-a", unit+".json")
	if _, statErr := os.Stat(attestation); !os.IsNotExist(statErr) {
		t.Fatalf("refused read left attestation: %v", statErr)
	}
	if staged := git(t, f.root, "diff", "--cached", "--name-only"); staged != "" {
		t.Fatalf("refused read left staged paths: %q", staged)
	}
	if status := git(t, f.root, "status", "--porcelain=v1", "--untracked-files=all"); status != "?? "+record {
		t.Fatalf("refused read status = %q", status)
	}
}

func TestReadCommitKeepsDirtyLedgerWithoutAdoption(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	write(t, f.root, "metasystem/memory/receipts.log", "seed\n")
	git(t, f.root, "add", ".")
	git(t, f.root, "commit", "-qm", "tracked ledger")
	git(t, f.root, "push", "-q", "origin", "HEAD:main")
	f.base = git(t, f.root, "rev-parse", "HEAD")
	unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	record := readerRecord(t, f, unit)
	write(t, f.root, "metasystem/memory/receipts.log", "dirty ledger\n")
	_, _, err := branch.CommitRead(branch.CommitReadRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "read-dirty-ledger",
		CheckClaim: claimAllowed, ReaderRecord: record, GateRunID: "fast", GateTree: unitTree(t, f, unit),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, readErr := os.ReadFile(filepath.Join(f.root, "metasystem/memory/receipts.log")); readErr != nil || string(got) != "dirty ledger\n" {
		t.Fatalf("dirty ledger = %q, %v", got, readErr)
	}
}

func TestAttestationCarryRefusesChangedUnitOrFold(t *testing.T) {
	t.Parallel()
	for _, changed := range []string{"metasystem/code.go", "metasystem/plans/goal-a.md"} {
		changed := changed
		t.Run(filepath.Base(changed), func(t *testing.T) {
			t.Parallel()
			c := newCarryFixture(t)
			endpoint, unit := rebaseCarryFixture(t, c, changed)
			_, _, err := branch.CommitRead(branch.CommitReadRequest{
				Repo: c.fixture.root, Remote: "origin", EndpointTip: endpoint, GoalID: "goal-a", Unit: "u1", OpID: "carry-refuse", CheckClaim: claimAllowed,
				Carry: c.unit, GateRunID: "fast-new", GateTree: unitTree(t, c.fixture, unit),
			})
			var refusal *branch.OpError
			if !errors.As(err, &refusal) || refusal.Code != branch.ReadStaleCode {
				t.Fatalf("changed %s carry = %v", changed, err)
			}
		})
	}
}

func TestAttestationCarryChecksEveryEarlierFold(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, changedPath string
		wantStale         bool
	}{
		{name: "first plan changes", changedPath: "metasystem/plans/p1.md", wantStale: true},
		{name: "unrelated endpoint change", changedPath: "unrelated.txt"},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			c := newTwoUnitCarryFixture(t)
			endpoint, unit := rebaseCarryUnit(t, c, test.changedPath, "u2")
			_, _, err := branch.CommitRead(branch.CommitReadRequest{
				Repo: c.fixture.root, Remote: "origin", EndpointTip: endpoint, GoalID: "goal-a", Unit: "u2", OpID: "carry-all-folds",
				CheckClaim: claimAllowed, Carry: c.unit, GateRunID: "fast-new", GateTree: unitTree(t, c.fixture, unit),
			})
			var refusal *branch.OpError
			if test.wantStale {
				if !errors.As(err, &refusal) || refusal.Code != branch.ReadStaleCode {
					t.Fatalf("changed first fold carry = %v", err)
				}
			} else if err != nil {
				t.Fatalf("unchanged folds carry = %v", err)
			}
		})
	}
}

func TestReadRefusalsPreserveCheckout(t *testing.T) {
	t.Parallel()
	t.Run("non-holder", func(t *testing.T) {
		t.Parallel()
		f := newBranchFixture(t)
		unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
		record := readerRecord(t, f, unit)
		before := snapshotCheckout(t, f.root)
		_, _, err := branch.CommitRead(branch.CommitReadRequest{
			Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "read-non-holder",
			CheckClaim: func() error { return errors.New("claim moved") }, ReaderRecord: record,
			GateRunID: "fast", GateTree: unitTree(t, f, unit),
		})
		var refusal *branch.OpError
		if !errors.As(err, &refusal) || refusal.Code != branch.NotHolderCode {
			t.Fatalf("non-holder read = %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(f.root, "metasystem", "records", "reads", "goal-a", unit+".json")); !os.IsNotExist(statErr) {
			t.Fatalf("non-holder attestation exists: %v", statErr)
		}
		requireCheckoutUnchanged(t, f.root, before)
	})

	t.Run("class", func(t *testing.T) {
		t.Parallel()
		f := newBranchFixture(t)
		unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
		record := readerRecord(t, f, unit)
		stage(t, f, "metasystem/extra.go", "wrong class")
		before := snapshotCheckout(t, f.root)
		_, _, err := branch.CommitRead(branch.CommitReadRequest{
			Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "read-class",
			CheckClaim: claimAllowed, ReaderRecord: record, GateRunID: "fast", GateTree: unitTree(t, f, unit),
		})
		var refusal *branch.OpError
		if !errors.As(err, &refusal) || refusal.Code != branch.RangeCode {
			t.Fatalf("read class refusal = %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(f.root, "metasystem", "records", "reads", "goal-a", unit+".json")); !os.IsNotExist(statErr) {
			t.Fatalf("class-refused attestation exists: %v", statErr)
		}
		requireCheckoutUnchanged(t, f.root, before)
	})
}

func TestReadAdoptionRefusalsLeaveCheckoutUntouched(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, wantCode string
		remoteRead     bool
		claimMoves     bool
	}{
		{name: "staged change conflicts with remote", wantCode: branch.StaleCode, remoteRead: true},
		{name: "claim moves during preparation", wantCode: branch.NotHolderCode, claimMoves: true},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			f := newBranchFixture(t)
			unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
			if _, err := branch.Push(pushRequest(f, "first-push")); err != nil {
				t.Fatal(err)
			}

			other := cloneBranchFixture(t, f)
			if _, err := branch.Push(pushRequest(other, "other-adopt")); err != nil {
				t.Fatal(err)
			}
			if test.remoteRead {
				record := readerRecord(t, other, unit)
				if _, _, err := branch.CommitRead(branch.CommitReadRequest{
					Repo: other.root, Remote: "origin", EndpointTip: other.base, GoalID: "goal-a", Unit: "u1", OpID: "other-read",
					CheckClaim: claimAllowed, ReaderRecord: record, GateRunID: "remote-fast", GateTree: unitTree(t, other, unit),
				}); err != nil {
					t.Fatal(err)
				}
			} else {
				commitUnit(t, other, "u2", "metasystem/other.go", "two")
			}
			if _, err := branch.Push(pushRequest(other, "other-push")); err != nil {
				t.Fatal(err)
			}
			remote := remoteGoalTip(t, f)

			record := readerRecord(t, f, unit)
			if test.remoteRead {
				write(t, f.root, record, "Independent read of commit "+unit+" with unit digest "+mustUnitDigest(t, f.root, unit)+".\n")
			}
			before := snapshotCheckout(t, f.root)
			checkClaim := claimAllowed
			if test.claimMoves {
				checks := 0
				checkClaim = func() error {
					checks++
					if checks > 1 {
						return errors.New("claim moved")
					}
					return nil
				}
			}
			_, _, err := branch.CommitRead(branch.CommitReadRequest{
				Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "local-read",
				CheckClaim: checkClaim, ReaderRecord: record, GateRunID: "local-fast", GateTree: unitTree(t, f, unit),
			})
			var refusal *branch.OpError
			if !errors.As(err, &refusal) || refusal.Code != test.wantCode {
				t.Fatalf("read adoption refusal = %v", err)
			}
			attestation := filepath.Join(f.root, "metasystem", "records", "reads", "goal-a", unit+".json")
			if _, statErr := os.Stat(attestation); !os.IsNotExist(statErr) {
				t.Fatalf("refused read left attestation: %v", statErr)
			}
			requireCheckoutUnchanged(t, f.root, before)

			if err := os.Remove(filepath.Join(f.root, filepath.FromSlash(record))); err != nil {
				t.Fatal(err)
			}
			stage(t, f, "metasystem/next.go", "next")
			next, err := branch.CommitStaged(branch.CommitRequest{
				Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "next", OpID: "next-unit",
				Kind: branch.Unit, CheckClaim: claimAllowed,
			})
			if err != nil || git(t, f.root, "rev-parse", next+"^") != remote {
				t.Fatalf("unit commit after refused read = %s, %v", next, err)
			}
		})
	}
}

func TestReadAdoptionKeepsUntrackedScratchFile(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	if _, err := branch.Push(pushRequest(f, "scratch-first")); err != nil {
		t.Fatal(err)
	}
	other := cloneBranchFixture(t, f)
	if _, err := branch.Push(pushRequest(other, "scratch-adopt")); err != nil {
		t.Fatal(err)
	}
	commitUnit(t, other, "u2", "metasystem/other.go", "two")
	if _, err := branch.Push(pushRequest(other, "scratch-remote")); err != nil {
		t.Fatal(err)
	}
	remote := remoteGoalTip(t, f)
	record := readerRecord(t, f, unit)
	write(t, f.root, "metasystem/scratch-notes.txt", "operator scratch")
	tip, _, err := branch.CommitRead(branch.CommitReadRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "scratch-read",
		CheckClaim: claimAllowed, ReaderRecord: record, GateRunID: "scratch-fast", GateTree: unitTree(t, f, unit),
	})
	if err != nil || git(t, f.root, "rev-parse", tip+"^") != remote {
		t.Fatalf("read adoption=%s err=%v", tip, err)
	}
	if got := git(t, f.root, "status", "--porcelain=v1", "--untracked-files=all"); got != "?? metasystem/scratch-notes.txt" {
		t.Fatalf("scratch status=%q", got)
	}
}

func mustUnitDigest(t *testing.T, root, commit string) string {
	t.Helper()
	digest, err := branch.UnitDigest(root, commit)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
