package branch

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func readBeforePrefixLanding(t *testing.T) (*attestationPolicyFixture, Attestation, string, string) {
	t.Helper()
	f := newAttestationPolicyFixture(t, "metasystem/code.go", true)
	first, endpoint := policyID("1"), policyID("0")
	f.raw[first] = policyRaw("metasystem/records/reads/goal-a/earlier.json", policyID("2"), policyID("3"))
	commits := []Commit{{ID: first, Kind: Read}, {ID: f.plan, Kind: Read}, {ID: f.unit, Kind: Unit, Unit: "u1", Units: []string{"u1"}}}
	f.ranges[f.base+":"+f.unit] = commits
	f.writeReaderRecord()
	expectReadFacts(f, f.root, f.base, f.unit, f.unit, commits)
	expectReadPublication(f, f.root, f.unit, f.record, false)
	_, att, err := commitRead(f.request(false), f, f.effects())
	if err != nil {
		t.Fatal(err)
	}
	f.ranges[endpoint+":"+f.unit] = commits[1:]
	f.ancestors = map[string]bool{first + ":" + endpoint: true}
	return f, att, first, endpoint
}

func expectLandedFoldValidation(f *attestationPolicyFixture, endpoint, unit, first string, complete bool) {
	f.expect("TopLevel", f.root)
	f.expect("ReadSubject", f.root, unit)
	f.expect("RawEntries", f.root, unit)
	f.expect("Range", f.root, endpoint, unit, "goal-a")
	f.expect("RawEntries", f.root, f.plan)
	f.expect("IsAncestor", f.root, first, endpoint)
	if f.ancestorErr == nil {
		f.expect("IsAncestor", f.root, f.plan, endpoint)
	}
	if complete {
		f.expect("RawEntries", f.root, unit)
		f.expect("TopLevel", f.root)
	}
}

func TestAttestationDropsLandedFolds(t *testing.T) {
	t.Parallel()
	f, recorded, first, endpoint := readBeforePrefixLanding(t)
	expectLandedFoldValidation(f, endpoint, f.unit, first, true)
	att, err := validateAttestation(f, f.root, "", endpoint, "goal-a", "u1", f.unit, map[string]bool{})
	if err != nil || !reflect.DeepEqual(att.Folds, recorded.Folds) {
		t.Fatalf("read after prefix landing = %+v, %v", att, err)
	}
}

func TestAttestationRefusesMissingUnlandedFold(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
	}{{name: "not landed"}, {name: "ancestry unavailable", err: errors.New("repository unavailable")}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f, _, first, endpoint := readBeforePrefixLanding(t)
			f.ancestors[first+":"+endpoint] = false
			f.ancestorErr = tc.err
			expectLandedFoldValidation(f, endpoint, f.unit, first, false)
			_, err := validateAttestation(f, f.root, "", endpoint, "goal-a", "u1", f.unit, map[string]bool{})
			var refusal *OpError
			if !errors.As(err, &refusal) || refusal.Code != ReadInvalidCode || !strings.Contains(err.Error(), "no longer matches the goal branch below it") {
				t.Fatalf("missing fold refusal = %v", err)
			}
		})
	}
}

func TestAttestationMatchingFoldsSkipsAncestry(t *testing.T) {
	t.Parallel()
	f, _, _, _ := readBeforePrefixLanding(t)
	expectLocalReadValidation(f, f.root, f.base, f.unit, f.ranges[f.base+":"+f.unit], f.record)
	if _, err := validateAttestation(f, f.root, "", f.base, "goal-a", "u1", f.unit, map[string]bool{}); err != nil {
		t.Fatal(err)
	}
}

// expectCarryChanges is the change comparison a carry makes for each build.
func expectCarryChanges(f *attestationPolicyFixture, commits ...string) {
	for _, commit := range commits {
		f.expect("ChangePatch", f.root, commit)
		f.expect("Prefix", f.root)
		f.expect("TreeEntry", f.root, commit, "testing.json")
	}
}

func TestAttestationCarryDropsLandedFolds(t *testing.T) {
	t.Parallel()
	f, prior, first, endpoint := readBeforePrefixLanding(t)
	oldUnit := f.unit
	f.subjects[oldUnit] = f.subject
	f.unit, f.tree, f.tip = policyID("6"), policyID("7"), policyID("8")
	f.raw[f.unit] = append([]byte(nil), f.raw[oldUnit]...)
	f.patches[oldUnit] = []byte("diff --git a/metasystem/code.go b/metasystem/code.go\nindex old..new 100644\n--- a/metasystem/code.go\n+++ b/metasystem/code.go\n@@ -1 +1 @@\n-old\n+new\n")
	f.patches[f.unit] = append([]byte(nil), f.patches[oldUnit]...)
	f.subjects[f.unit] = readsubject.ReadSubject{Kind: readsubject.SubjectCommit, Commit: f.unit, Parent: f.plan, Tree: f.tree, DiffDigest: policyHash([]byte("rebased unit patch\n"))}
	commits := []Commit{{ID: f.plan, Kind: Read}, {ID: f.unit, Kind: Unit, Unit: "u1", Units: []string{"u1"}}}
	f.ranges[endpoint+":"+f.unit] = commits
	req := f.request(false)
	req.EndpointTip, req.Carry, req.ReaderRecord = endpoint, oldUnit, ""
	expectReadFacts(f, f.root, endpoint, f.unit, f.unit, commits)
	expectLandedFoldValidation(f, endpoint, oldUnit, first, true)
	expectCarryChanges(f, oldUnit, f.unit)
	expectReadPublication(f, f.root, f.unit, "", false)
	_, att, err := commitRead(req, f, f.effects())
	if err != nil || att.Carry == nil || att.Carry.FromCommit != oldUnit || !reflect.DeepEqual(att.Folds, prior.Folds[1:]) {
		t.Fatalf("carried read after prefix landing = %+v, %v", att, err)
	}
	expectLocalReadValidation(f, f.root, endpoint, f.unit, commits, "")
	f.expect("CommitExists", f.root, oldUnit)
	expectLandedFoldValidation(f, endpoint, oldUnit, first, true)
	expectCarryChanges(f, oldUnit, f.unit)
	if _, err := validateAttestation(f, f.root, "", endpoint, "goal-a", "u1", f.unit, map[string]bool{}); err != nil {
		t.Fatalf("carried read validation = %v", err)
	}
}
