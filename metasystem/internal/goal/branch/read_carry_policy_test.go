package branch

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func expectReadFacts(f *attestationPolicyFixture, repo, endpoint, branchTip, unit string, commits []Commit) {
	f.expect("Inspect")
	f.expect("Range", repo, endpoint, branchTip, "goal-a")
	f.expect("ReadSubject", repo, unit)
	f.expect("RawEntries", repo, unit)
	f.expect("RawEntries", repo, unit)
	f.expect("Range", repo, endpoint, unit, "goal-a")
	for _, commit := range commits {
		if commit.ID == unit {
			break
		}
		if commit.Kind != Unit {
			f.expect("RawEntries", repo, commit.ID)
		}
	}
}

func expectPriorRead(f *attestationPolicyFixture, repo, endpoint, unit string, commits []Commit) {
	f.expect("TopLevel", repo)
	f.expect("ReadSubject", repo, unit)
	f.expect("RawEntries", repo, unit)
	f.expect("Range", repo, endpoint, unit, "goal-a")
	for _, commit := range commits {
		if commit.ID == unit {
			break
		}
		if commit.Kind != Unit {
			f.expect("RawEntries", repo, commit.ID)
		}
	}
	f.expect("RawEntries", repo, unit)
	f.expect("TopLevel", repo)
}

func expectReadPublication(f *attestationPolicyFixture, repo, unit, reader string, critic bool) {
	if reader != "" {
		f.expect("TopLevel", repo)
	}
	f.expect("StagedPaths", repo)
	f.expect("Patch", repo, reader)
	f.expect("Build")
	f.expect("IndexTree", repo)
	f.expect("TopLevel", repo)
	paths := []string{repo, attestationPath("goal-a", unit)}
	if critic {
		paths = append(paths, closureBundlePath("goal-a", unit))
	} else if reader != "" {
		paths = append(paths, reader)
	}
	f.expect("Stage", paths...)
	f.expect("Install", f.tip)
}

func expectLocalReadValidation(f *attestationPolicyFixture, repo, endpoint, unit string, commits []Commit, sourcePath string) {
	f.expect("TopLevel", repo)
	f.expect("ReadSubject", repo, unit)
	f.expect("RawEntries", repo, unit)
	f.expect("Range", repo, endpoint, unit, "goal-a")
	for _, commit := range commits {
		if commit.ID == unit {
			break
		}
		if commit.Kind != Unit {
			f.expect("RawEntries", repo, commit.ID)
		}
	}
	f.expect("RawEntries", repo, unit)
	if sourcePath != "" {
		f.expect("TopLevel", repo)
	}
}

func TestAttestationReaderRecordIntegrity(t *testing.T) {
	t.Parallel()
	f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
	recordBytes := f.writeReaderRecord()
	req := f.request(false)
	req.OpID, req.GateRunID = "read-integrity", "fast-a"
	commits := f.rangeFacts()
	expectReadFacts(f, f.root, f.base, f.unit, f.unit, commits)
	expectReadPublication(f, f.root, f.unit, f.record, false)
	_, att, err := commitRead(req, f, f.effects())
	if err != nil {
		t.Fatal(err)
	}
	if att.Source.Kind != "reader-record" || att.Source.RecordSHA256 != policyHash(recordBytes) {
		t.Fatalf("source = %+v", att.Source)
	}
	expectLocalReadValidation(f, f.root, f.base, f.unit, commits, f.record)
	if _, err := validateAttestation(f, f.root, "", f.base, "goal-a", "u1", f.unit, map[string]bool{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.root, filepath.FromSlash(attestationPath("goal-a", f.unit)))
	original, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(original, f.snapshots[f.tip][attestationPath("goal-a", f.unit)]) {
		t.Fatalf("published attestation = %q, %v", original, err)
	}
	edited := []byte(strings.Replace(string(original), "fast-a", "fast-b", 1))
	if bytes.Equal(edited, original) {
		t.Fatal("attestation edit did not change bytes")
	}
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	f.expect("TopLevel", f.root)
	if _, err := validateAttestation(f, f.root, "", f.base, "goal-a", "u1", f.unit, map[string]bool{}); err == nil || !strings.Contains(err.Error(), ReadInvalidCode) {
		t.Fatalf("edited attestation = %v", err)
	}
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	second := policyID("6")
	copyPath := filepath.Join(f.root, filepath.FromSlash(attestationPath("goal-a", second)))
	if err := os.WriteFile(copyPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	f.expect("TopLevel", f.root)
	if _, err := validateAttestation(f, f.root, "", f.base, "goal-a", "u2", second, map[string]bool{}); err == nil || !strings.Contains(err.Error(), ReadInvalidCode) {
		t.Fatalf("copied attestation = %v", err)
	}
}

func TestGLENestedReadCommitUsesProjectPaths(t *testing.T) {
	t.Parallel()
	f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
	f.writeReaderRecord()
	installation := filepath.Join(f.root, "metasystem")
	req := f.request(false)
	req.Repo, req.OpID, req.GateRunID = installation, "nested-read", "fast-nested"
	commits := f.rangeFacts()
	expectReadFacts(f, installation, f.base, f.unit, f.unit, commits)
	expectReadPublication(f, installation, f.unit, f.record, false)
	read, att, err := commitRead(req, f, f.effects())
	if err != nil {
		t.Fatal(err)
	}
	if att.Source.ReaderRecord != f.record || read == "" {
		t.Fatalf("read=%q source=%+v", read, att.Source)
	}
	if _, err := os.Stat(filepath.Join(f.root, filepath.FromSlash(attestationPath("goal-a", f.unit)))); err != nil {
		t.Fatalf("project-relative attestation missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installation, filepath.FromSlash(attestationPath("goal-a", f.unit)))); !os.IsNotExist(err) {
		t.Fatalf("installation-relative attestation exists: %v", err)
	}
	expectLocalReadValidation(f, installation, f.base, f.unit, commits, f.record)
	if _, err := validateAttestation(f, installation, "", f.base, "goal-a", "u1", f.unit, map[string]bool{}); err != nil {
		t.Fatalf("nested read validation: %v", err)
	}
}

func TestAttestationCriticRootSourceValidates(t *testing.T) {
	t.Parallel()
	f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
	f.writeJob("completed", false)
	installation := filepath.Join(f.root, "metasystem")
	if err := os.MkdirAll(filepath.Join(installation, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(f.root, "artifacts", "agents"), filepath.Join(installation, "artifacts", "agents")); err != nil {
		t.Fatal(err)
	}
	req := f.request(true)
	req.Repo, req.OpID, req.GateRunID = installation, "read-critic", "fast-a"
	commits := f.rangeFacts()
	expectReadFacts(f, installation, f.base, f.unit, f.unit, commits)
	expectReadPublication(f, installation, f.unit, "", true)
	_, att, err := commitRead(req, f, f.effects())
	if err != nil {
		t.Fatal(err)
	}
	if att.Source.Kind != "critic-root" || att.Source.Round != 1 || att.Source.ClosureSHA256 == "" {
		t.Fatalf("source = %+v", att.Source)
	}
	if _, err := os.Stat(filepath.Join(f.root, filepath.FromSlash(closureBundlePath("goal-a", f.unit)))); err != nil {
		t.Fatalf("project-relative closure missing: %v", err)
	}
	expectLocalReadValidation(f, installation, f.base, f.unit, commits, closureBundlePath("goal-a", f.unit))
	if _, err := validateAttestation(f, installation, "", f.base, "goal-a", "u1", f.unit, map[string]bool{}); err != nil {
		t.Fatalf("critic attestation validation: %v", err)
	}
}

type readCarryFixture struct {
	f        *attestationPolicyFixture
	oldUnit  string
	oldTree  string
	oldRange []Commit
	newRange []Commit
	endpoint string
}

func newReadCarryFixture(t *testing.T, twoUnits bool) readCarryFixture {
	t.Helper()
	f := newAttestationPolicyFixture(t, "metasystem/code.go", true)
	oldRange := f.rangeFacts()
	if twoUnits {
		f.unitName = "u2"
		f.raw[f.unit] = policyRaw("metasystem/code2.go", policyID("3"), policyID("4"))
		f.raw[policyID("1")] = policyRaw("metasystem/code1.go", policyID("3"), policyID("4"))
		f.raw[policyID("2")] = policyRaw("metasystem/plans/p2.md", policyID("3"), policyID("4"))
		oldRange = []Commit{{ID: f.plan, Kind: Plan}, {ID: policyID("1"), Kind: Unit, Unit: "u1", Units: []string{"u1"}},
			{ID: policyID("2"), Kind: Plan}, {ID: f.unit, Kind: Unit, Unit: "u2", Units: []string{"u2"}}}
		f.subject.Parent = policyID("2")
	}
	f.ranges[f.base+":"+f.unit] = oldRange
	f.subjects[f.unit] = f.subject
	f.writeReaderRecord()
	req := f.request(false)
	req.Unit, req.OpID, req.GateRunID = f.unitName, "carry-old", "fast-old"
	expectReadFacts(f, f.root, f.base, f.unit, f.unit, oldRange)
	expectReadPublication(f, f.root, f.unit, f.record, false)
	if _, _, err := commitRead(req, f, f.effects()); err != nil {
		t.Fatal(err)
	}
	c := readCarryFixture{f: f, oldUnit: f.unit, oldTree: f.tree, oldRange: oldRange, endpoint: policyID("e")}
	return c
}

func (c *readCarryFixture) rebase(changedPath string) {
	f := c.f
	newUnit, newTree, newTip := policyID("6"), policyID("7"), policyID("8")
	newPlan := policyID("9")
	newRange := []Commit{{ID: newPlan, Kind: Plan}}
	f.raw[newPlan] = append([]byte(nil), f.raw[f.plan]...)
	parent := newPlan
	if f.unitName == "u2" {
		newFirstUnit, newSecondPlan := policyID("4"), policyID("5")
		f.raw[newFirstUnit] = append([]byte(nil), f.raw[policyID("1")]...)
		f.raw[newSecondPlan] = append([]byte(nil), f.raw[policyID("2")]...)
		newRange = append(newRange, Commit{ID: newFirstUnit, Kind: Unit, Unit: "u1", Units: []string{"u1"}}, Commit{ID: newSecondPlan, Kind: Plan})
		parent = newSecondPlan
	}
	if changedPath == "metasystem/plans/goal-a.md" || changedPath == "metasystem/plans/p1.md" {
		f.raw[newPlan] = policyRaw(changedPath, policyID("1"), policyID("0"))
	}
	f.raw[newUnit] = append([]byte(nil), f.raw[c.oldUnit]...)
	if changedPath == "metasystem/code.go" {
		f.raw[newUnit] = policyRaw(changedPath, policyID("3"), policyID("0"))
	}
	newRange = append(newRange, Commit{ID: newUnit, Kind: Unit, Unit: f.unitName, Units: []string{f.unitName}})
	f.ranges[c.endpoint+":"+c.oldUnit] = c.oldRange
	f.ranges[c.endpoint+":"+newUnit] = newRange
	f.subjects[newUnit] = readsubject.ReadSubject{Kind: readsubject.SubjectCommit, Commit: newUnit, Parent: parent,
		Tree: newTree, DiffDigest: policyHash([]byte("rebased unit patch\n"))}
	f.unit, f.tree, f.tip, f.branchTip = newUnit, newTree, newTip, newUnit
	c.newRange = newRange
}

func (c *readCarryFixture) request(opID string) CommitReadRequest {
	f := c.f
	return CommitReadRequest{Repo: f.root, Remote: "origin", EndpointTip: c.endpoint, GoalID: "goal-a", Unit: f.unitName,
		OpID: opID, CheckClaim: func() error { return nil }, Carry: c.oldUnit, GateRunID: "fast-new", GateTree: f.tree}
}

func (c *readCarryFixture) expectRead(publish bool) {
	f := c.f
	expectReadFacts(f, f.root, c.endpoint, f.unit, f.unit, c.newRange)
	expectPriorRead(f, f.root, c.endpoint, c.oldUnit, c.oldRange)
	if publish {
		expectReadPublication(f, f.root, f.unit, "", false)
	}
}

func TestAttestationCarryPreservesUnitAndFoldBytes(t *testing.T) {
	t.Parallel()
	c := newReadCarryFixture(t, false)
	c.rebase("unrelated.txt")
	c.expectRead(true)
	_, att, err := commitRead(c.request("carry-new"), c.f, c.f.effects())
	if err != nil {
		t.Fatal(err)
	}
	if att.Carry == nil || att.Carry.FromCommit != c.oldUnit || att.Carry.ToCommit != c.f.unit {
		t.Fatalf("carry = %+v", att.Carry)
	}
	if att.Carry.FromTree != c.oldTree || att.Carry.ToTree != c.f.tree {
		t.Fatalf("carry trees = %+v, want %s and %s", att.Carry, c.oldTree, c.f.tree)
	}
	if !bytes.Equal(c.f.snapshots[c.f.tip][attestationPath("goal-a", c.f.unit)], c.f.generated[attestationPath("goal-a", c.f.unit)]) {
		t.Fatal("carried attestation bytes differ from published bytes")
	}
}

func TestReadCommitKeepsDirtyLedgerWithoutAdoption(t *testing.T) {
	t.Parallel()
	f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
	f.writeReaderRecord()
	ledger := "metasystem/memory/receipts.log"
	f.write(ledger, []byte("dirty ledger\n"))
	expectReadFacts(f, f.root, f.base, f.unit, f.unit, f.rangeFacts())
	expectReadPublication(f, f.root, f.unit, f.record, false)
	if _, _, err := commitRead(f.request(false), f, f.effects()); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(ledger))); err != nil || string(got) != "dirty ledger\n" {
		t.Fatalf("dirty ledger = %q, %v", got, err)
	}
}

func TestAttestationCarryRefusesChangedUnitOrFold(t *testing.T) {
	t.Parallel()
	for _, changed := range []string{"metasystem/code.go", "metasystem/plans/goal-a.md"} {
		changed := changed
		t.Run(filepath.Base(changed), func(t *testing.T) {
			t.Parallel()
			c := newReadCarryFixture(t, false)
			c.rebase(changed)
			c.expectRead(false)
			_, _, err := commitRead(c.request("carry-refuse"), c.f, c.f.effects())
			var refusal *OpError
			if !errors.As(err, &refusal) || refusal.Code != ReadStaleCode {
				t.Fatalf("changed %s carry = %v", changed, err)
			}
			if _, err := os.Stat(filepath.Join(c.f.root, filepath.FromSlash(attestationPath("goal-a", c.f.unit)))); !os.IsNotExist(err) {
				t.Fatalf("stale carry published attestation: %v", err)
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
			c := newReadCarryFixture(t, true)
			c.rebase(test.changedPath)
			c.expectRead(!test.wantStale)
			_, att, err := commitRead(c.request("carry-all-folds"), c.f, c.f.effects())
			var refusal *OpError
			if test.wantStale {
				if !errors.As(err, &refusal) || refusal.Code != ReadStaleCode {
					t.Fatalf("changed first fold carry = %v", err)
				}
				if _, statErr := os.Stat(filepath.Join(c.f.root, filepath.FromSlash(attestationPath("goal-a", c.f.unit)))); !os.IsNotExist(statErr) {
					t.Fatalf("stale earlier fold published attestation: %v", statErr)
				}
			} else if err != nil || att.Carry == nil || att.Carry.FromCommit != c.oldUnit || att.Carry.ToCommit != c.f.unit || att.Carry.FromTree != c.oldTree || att.Carry.ToTree != c.f.tree {
				t.Fatalf("unchanged folds carry = %+v, %v", att.Carry, err)
			}
		})
	}
}

func TestReadRefusalsPreserveCheckout(t *testing.T) {
	t.Parallel()
	t.Run("non-holder", func(t *testing.T) {
		t.Parallel()
		f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
		record := f.writeReaderRecord()
		f.write("metasystem/memory/receipts.log", []byte("existing ledger\n"))
		req := f.request(false)
		req.CheckClaim = func() error { return errors.New("claim moved") }
		_, _, err := commitRead(req, f, f.effects())
		var refusal *OpError
		if !errors.As(err, &refusal) || refusal.Code != NotHolderCode {
			t.Fatalf("non-holder read = %v", err)
		}
		assertRefusedReadFiles(t, f, record)
	})
	t.Run("class", func(t *testing.T) {
		t.Parallel()
		f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
		record := f.writeReaderRecord()
		f.write("metasystem/extra.go", []byte("wrong class"))
		f.write("metasystem/memory/receipts.log", []byte("existing ledger\n"))
		f.staged = []string{"metasystem/extra.go"}
		expectReadFacts(f, f.root, f.base, f.unit, f.unit, f.rangeFacts())
		f.expect("TopLevel", f.root)
		f.expect("StagedPaths", f.root)
		_, _, err := commitRead(f.request(false), f, f.effects())
		var refusal *OpError
		if !errors.As(err, &refusal) || refusal.Code != RangeCode {
			t.Fatalf("read class refusal = %v", err)
		}
		assertRefusedReadFiles(t, f, record)
		if got, readErr := os.ReadFile(filepath.Join(f.root, "metasystem/extra.go")); readErr != nil || string(got) != "wrong class" {
			t.Fatalf("staged file = %q, %v", got, readErr)
		}
	})
}

func assertRefusedReadFiles(t *testing.T, f *attestationPolicyFixture, record []byte) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(f.root, filepath.FromSlash(attestationPath("goal-a", f.unit)))); !os.IsNotExist(err) {
		t.Fatalf("refused read published attestation: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(f.record))); err != nil || !bytes.Equal(got, record) {
		t.Fatalf("reader record = %q, %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(f.root, "metasystem/memory/receipts.log")); err != nil || string(got) != "existing ledger\n" {
		t.Fatalf("ledger = %q, %v", got, err)
	}
	if f.generated != nil || len(f.snapshots) != 0 {
		t.Fatalf("refused read generated publication: %v", f.generated)
	}
}
