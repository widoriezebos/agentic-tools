package branch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// publishReadRepository resolves the branch-read record store only; the
// publication never reads a subject, range or detached tree.
type publishReadRepository struct{ common string }

func (r publishReadRepository) CommonDir(string) (string, error) { return r.common, nil }
func (publishReadRepository) Range(string, string, string, string) ([]Commit, error) {
	return nil, errors.New("unexpected range read")
}
func (publishReadRepository) Subject(string, string) (AttestationSubject, error) {
	return AttestationSubject{}, errors.New("unexpected subject read")
}
func (publishReadRepository) Entries(string, string) ([]Entry, error) {
	return nil, errors.New("unexpected entries read")
}
func (publishReadRepository) Detached(string, string) (string, func() error, error) {
	return "", nil, errors.New("unexpected detached tree")
}

func TestPublishCollectedReadReconcilesTheSamePush(t *testing.T) {
	t.Parallel()
	f := newPushPolicyFixture(t)
	f.refs[policyGoalRef] = policyFirst
	repository := publishReadRepository{common: t.TempDir()}
	request := PublishReadRequest{Repo: f.repo, Remote: "origin", EndpointTip: f.endpoint, GoalID: "goal-a", UnitCommit: policySecond,
		CheckClaim: func() error { f.take(pushEvent{operation: "claim", repo: f.repo}); return nil }, Transport: f, Repository: repository}

	if _, err := publishCollectedReadWith(request, f.repository()); err == nil || !strings.Contains(err.Error(), "no collected read") {
		t.Fatalf("publishing before collection = %v", err)
	}
	common, recordPath, _, err := branchReadPathsWithRepository(repository, f.repo, "goal-a", policySecond)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveBranchReadRecord(common, recordPath, branchReadRecord{SchemaVersion: 1, Goal: "goal-a", Tree: policySecond, UnitCommit: policySecond, GateRunID: "gate1", RootJob: "crit1", AttestationCommit: policyFirst}); err != nil {
		t.Fatal(err)
	}
	op := PublishOperationID("gate1")
	txn := pushTxn{SchemaVersion: 1, Goal: "goal-a", Remote: "origin", Ref: policyGoalRef, New: policyFirst}
	f.expect(pe("claim"), pe("txn-refs"), pushExpectation{call: pushEvent{operation: "remote", remote: "origin", ref: policyGoalRef}},
		pushExpectation{call: pushEvent{operation: "tip", ref: policyGoalRef}}, pushExpectation{call: pushEvent{operation: "range", endpoint: pushPolicyBase, tip: policyFirst, goal: "goal-a"}},
		pushExpectation{call: pushEvent{operation: "tip", ref: originTipRef("goal-a")}}, pushExpectation{call: pushEvent{operation: "write-txn", opid: op, txn: txn}},
		pushExpectation{call: pushEvent{operation: "push", remote: "origin", ref: policyGoalRef, tip: policyFirst}, outcome: CASLanded})
	request.Hooks.AfterPush = func() error { return errors.New("simulated process death") }
	if result, err := publishCollectedReadWith(request, f.repository()); err == nil || result.OpID != op || result.RemoteTip != "" {
		t.Fatalf("a lost push response must not report publication: %+v %v", result, err)
	}
	request.Hooks.AfterPush = nil
	f.expect(pe("claim"), pe("txn-refs"), pushExpectation{call: pushEvent{operation: "txn", ref: txnRef(op)}},
		pushExpectation{call: pushEvent{operation: "remote", remote: "origin", ref: policyGoalRef}},
		pushExpectation{call: pushEvent{operation: "origin", goal: "goal-a", tip: policyFirst}},
		pushExpectation{call: pushEvent{operation: "clear", ref: txnRef(op)}}, pe("claim"),
		pushExpectation{call: pushEvent{operation: "remote", remote: "origin", ref: policyGoalRef}})
	result, err := publishCollectedReadWith(request, f.repository())
	if err != nil || result.State != "reconciled" || result.RemoteTip != policyFirst || result.Attestation != policyFirst || f.remote[policyGoalRef] != policyFirst {
		t.Fatalf("the repeat reconciles the same push and verifies the remote: %+v %v", result, err)
	}
	f.done()
	if _, err := os.Stat(filepath.Join(common, "metasystem-goal-reads")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

// A published read is reported only when the remote goal branch, read after
// the push, still contains the attestation: a descendant carrying it is
// published, while a replaced or deleted remote branch, or an ancestry that
// cannot be read, is refused with the attestation named.
func TestPublishCollectedReadRequiresTheRemoteToContainTheAttestation(t *testing.T) {
	t.Parallel()
	ancestryErr := errors.New("declared ancestry failure")
	for _, row := range []struct {
		name, remote, want string
		ancestry           *pushExpectation
	}{
		{name: "descendant", remote: policySecond,
			ancestry: &pushExpectation{call: pushEvent{operation: "ancestor", older: policyFirst, newer: policySecond}}},
		{name: "replaced", remote: policyMoved, want: "at " + policyMoved + " does not contain attestation " + policyFirst,
			ancestry: &pushExpectation{call: pushEvent{operation: "ancestor", older: policyFirst, newer: policyMoved}}},
		{name: "deleted", want: "origin has no goal/goal-a after publishing attestation " + policyFirst},
		{name: "ancestry-unknown", remote: policySecond, want: ancestryErr.Error(),
			ancestry: &pushExpectation{call: pushEvent{operation: "ancestor", older: policyFirst, newer: policySecond}, err: ancestryErr}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			f := newPushPolicyFixture(t)
			f.refs[policyGoalRef] = policySecond
			f.ancestors[[2]string{policyFirst, policyMoved}] = false
			f.afterOrigin = func(string) {
				if row.remote == "" {
					delete(f.remote, policyGoalRef)
				} else {
					f.remote[policyGoalRef] = row.remote
				}
			}
			repository := publishReadRepository{common: t.TempDir()}
			common, recordPath, _, err := branchReadPathsWithRepository(repository, f.repo, "goal-a", policyFirst)
			if err != nil {
				t.Fatal(err)
			}
			if err := saveBranchReadRecord(common, recordPath, branchReadRecord{SchemaVersion: 1, Goal: "goal-a", Tree: policyFirst, UnitCommit: policyFirst, GateRunID: "gate1", RootJob: "crit1", AttestationCommit: policyFirst}); err != nil {
				t.Fatal(err)
			}
			op := PublishOperationID("gate1")
			txn := pushTxn{SchemaVersion: 1, Goal: "goal-a", Remote: "origin", Ref: policyGoalRef, New: policySecond}
			f.expect(pe("claim"), pe("txn-refs"), pushExpectation{call: pushEvent{operation: "remote", remote: "origin", ref: policyGoalRef}},
				pushExpectation{call: pushEvent{operation: "tip", ref: policyGoalRef}}, pushExpectation{call: pushEvent{operation: "range", endpoint: pushPolicyBase, tip: policySecond, goal: "goal-a"}},
				pushExpectation{call: pushEvent{operation: "tip", ref: originTipRef("goal-a")}}, pushExpectation{call: pushEvent{operation: "write-txn", opid: op, txn: txn}},
				pushExpectation{call: pushEvent{operation: "push", remote: "origin", ref: policyGoalRef, tip: policySecond}, outcome: CASLanded},
				pushExpectation{call: pushEvent{operation: "origin", goal: "goal-a", tip: policySecond}},
				pushExpectation{call: pushEvent{operation: "clear", ref: txnRef(op)}}, pe("claim"),
				pushExpectation{call: pushEvent{operation: "remote", remote: "origin", ref: policyGoalRef}})
			if row.ancestry != nil {
				f.expect(*row.ancestry)
			}
			result, err := publishCollectedReadWith(PublishReadRequest{Repo: f.repo, Remote: "origin", EndpointTip: f.endpoint, GoalID: "goal-a", UnitCommit: policyFirst,
				CheckClaim: func() error { f.take(pushEvent{operation: "claim", repo: f.repo}); return nil }, Transport: f, Repository: repository}, f.repository())
			f.done()
			if result.Attestation != policyFirst || result.OpID != op || result.State != "pushed" || result.RemoteTip != row.remote {
				t.Fatalf("result=%+v", result)
			}
			if row.want == "" && err != nil || row.want != "" && (err == nil || !strings.Contains(err.Error(), row.want)) {
				t.Fatalf("err=%v, want %q", err, row.want)
			}
		})
	}
}
