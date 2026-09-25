package branch

import (
	"errors"
	"strings"
	"testing"
)

const (
	mirrorGoalID  = "goal-a"
	mirrorGoalRef = "refs/heads/goal/goal-a"
)

type mirrorPolicyArgs struct {
	method, repo, remote, ref, destination string
	endpointTip, tip, goalID               string
	expected, newTip                       string
}

type mirrorPolicyCall struct {
	args      mirrorPolicyArgs
	readTip   string
	present   bool
	beforeTip string
	outcome   CASOutcome
	err       error
}

type mirrorPolicyRig struct {
	t     *testing.T
	calls []mirrorPolicyCall
	next  int
	tips  map[string]string
}

func newMirrorPolicyRig(t *testing.T, originTip string, calls ...mirrorPolicyCall) *mirrorPolicyRig {
	t.Helper()
	rig := &mirrorPolicyRig{t: t, calls: calls, tips: map[string]string{"origin": originTip, "transport": ""}}
	t.Cleanup(func() {
		if rig.next != len(rig.calls) {
			t.Errorf("mirror consumed %d of %d calls; next = %+v", rig.next, len(rig.calls), rig.calls[rig.next:])
		}
	})
	return rig
}

func (m *mirrorPolicyRig) take(args mirrorPolicyArgs) mirrorPolicyCall {
	m.t.Helper()
	if m.next == len(m.calls) {
		m.t.Fatalf("unexpected mirror call: %+v", args)
	}
	call := m.calls[m.next]
	m.next++
	if call.args != args {
		m.t.Fatalf("mirror call %d = %+v, want %+v", m.next, args, call.args)
	}
	return call
}

func (m *mirrorPolicyRig) RemoteTip(repo, remote, ref string) (string, bool, error) {
	call := m.take(mirrorPolicyArgs{method: "RemoteTip", repo: repo, remote: remote, ref: ref})
	if current := m.tips[remote]; current != call.readTip || (current != "") != call.present {
		m.t.Fatalf("remote %q read = (%q, %t), state holds %q", remote, call.readTip, call.present, current)
	}
	return call.readTip, call.present, call.err
}

func (m *mirrorPolicyRig) Fetch(repo, remote, ref, destination string) error {
	return m.take(mirrorPolicyArgs{method: "Fetch", repo: repo, remote: remote, ref: ref, destination: destination}).err
}

func (m *mirrorPolicyRig) Push(repo, remote, ref, expected, tip string) (CASOutcome, error) {
	call := m.take(mirrorPolicyArgs{method: "Push", repo: repo, remote: remote, ref: ref, expected: expected, newTip: tip})
	if current := m.tips[remote]; current != call.beforeTip {
		m.t.Fatalf("remote %q at push = %q, want %q", remote, current, call.beforeTip)
	}
	if call.outcome == CASLanded {
		if call.beforeTip != expected {
			m.t.Fatalf("landed push lease = %q, remote holds %q", expected, call.beforeTip)
		}
		m.tips[remote] = tip
	}
	return call.outcome, call.err
}

func (m *mirrorPolicyRig) move(remote, from, to string) {
	m.t.Helper()
	if current := m.tips[remote]; current != from {
		m.t.Fatalf("remote %q before movement = %q, want %q", remote, current, from)
	}
	m.tips[remote] = to
}

func (m *mirrorPolicyRig) dependencies() fetchValidationDependencies {
	return fetchValidationDependencies{
		validateRange: func(repo, endpointTip, tip, goalID string) ([]Commit, error) {
			call := m.take(mirrorPolicyArgs{method: "ValidateRange", repo: repo, endpointTip: endpointTip, tip: tip, goalID: goalID})
			return nil, call.err
		},
		clearRef: func(repo, ref string) error {
			return m.take(mirrorPolicyArgs{method: "Cleanup", repo: repo, ref: ref}).err
		},
	}
}

func (m *mirrorPolicyRig) request(repo, endpointTip, opID string) MirrorRequest {
	return MirrorRequest{Repo: repo, Origin: "origin", Transport: "transport", EndpointTip: endpointTip,
		GoalID: mirrorGoalID, OpID: opID, PushTransport: m, CheckClaim: func() error {
			return m.take(mirrorPolicyArgs{method: "CheckClaim"}).err
		}}
}

func mirrorClaim() mirrorPolicyCall {
	return mirrorPolicyCall{args: mirrorPolicyArgs{method: "CheckClaim"}}
}

func mirrorRead(repo, remote, tip string) mirrorPolicyCall {
	return mirrorPolicyCall{args: mirrorPolicyArgs{method: "RemoteTip", repo: repo, remote: remote, ref: mirrorGoalRef},
		readTip: tip, present: tip != ""}
}

func mirrorFetch(repo, opID string, err error) mirrorPolicyCall {
	return mirrorPolicyCall{args: mirrorPolicyArgs{method: "Fetch", repo: repo, remote: "origin", ref: mirrorGoalRef,
		destination: "refs/metasystem/goals/fetch/" + opID}, err: err}
}

func mirrorValidate(repo, endpointTip, tip string, err error) mirrorPolicyCall {
	return mirrorPolicyCall{args: mirrorPolicyArgs{method: "ValidateRange", repo: repo, endpointTip: endpointTip,
		tip: tip, goalID: mirrorGoalID}, err: err}
}

func mirrorCleanup(repo, opID string, err error) mirrorPolicyCall {
	return mirrorPolicyCall{args: mirrorPolicyArgs{method: "Cleanup", repo: repo,
		ref: "refs/metasystem/goals/fetch/" + opID}, err: err}
}

func mirrorPush(repo, expected, tip, beforeTip string, outcome CASOutcome, err error) mirrorPolicyCall {
	return mirrorPolicyCall{args: mirrorPolicyArgs{method: "Push", repo: repo, remote: "transport",
		ref: mirrorGoalRef, expected: expected, newTip: tip}, beforeTip: beforeTip, outcome: outcome, err: err}
}

func TestTransportMirrorAndLeasedDelete(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	endpoint := strings.Repeat("1", 40)
	firstTip := strings.Repeat("2", 40)
	amendedTip := strings.Repeat("3", 40)
	intruderTip := strings.Repeat("4", 40)
	const opID = "mirror-policy"
	leaseErr := errors.New("transport lease refused")
	rig := newMirrorPolicyRig(t, firstTip,
		mirrorClaim(), mirrorRead(repo, "origin", firstTip), mirrorFetch(repo, opID, nil),
		mirrorValidate(repo, endpoint, firstTip, nil), mirrorCleanup(repo, opID, nil),
		mirrorRead(repo, "transport", ""), mirrorPush(repo, "", firstTip, "", CASLanded, nil),
		mirrorClaim(), mirrorRead(repo, "origin", amendedTip), mirrorFetch(repo, opID, nil),
		mirrorValidate(repo, endpoint, amendedTip, nil), mirrorCleanup(repo, opID, nil),
		mirrorRead(repo, "transport", firstTip), mirrorPush(repo, firstTip, amendedTip, firstTip, CASLanded, nil),
		mirrorClaim(), mirrorRead(repo, "origin", amendedTip), mirrorFetch(repo, opID, nil),
		mirrorValidate(repo, endpoint, amendedTip, nil), mirrorCleanup(repo, opID, nil),
		mirrorRead(repo, "transport", amendedTip),
		mirrorPush(repo, amendedTip, amendedTip, intruderTip, CASRefused, leaseErr),
	)
	request := rig.request(repo, endpoint, opID)
	first, err := mirror(request, rig.dependencies())
	if err != nil || first != (PushResult{State: "mirrored", Tip: firstTip}) || rig.tips["transport"] != firstTip {
		t.Fatalf("first mirror = %+v, err=%v, transport=%q", first, err, rig.tips["transport"])
	}
	rig.move("origin", firstTip, amendedTip)
	second, err := mirror(request, rig.dependencies())
	if err != nil || second != (PushResult{State: "mirrored", Tip: amendedTip}) || rig.tips["transport"] != amendedTip {
		t.Fatalf("amended mirror = %+v, err=%v, transport=%q", second, err, rig.tips["transport"])
	}
	request.Hooks.AfterTransportRead = func() error {
		rig.move("transport", amendedTip, intruderTip)
		return nil
	}
	result, err := mirror(request, rig.dependencies())
	var refusal *OpError
	if result != (PushResult{}) || !errors.As(err, &refusal) || refusal.Code != LeaseMovedCode || rig.tips["transport"] != intruderTip {
		t.Fatalf("moved transport mirror = %+v, err=%v, transport=%q", result, err, rig.tips["transport"])
	}
}

func TestMirrorUsesOperationFetchRef(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	endpoint := strings.Repeat("5", 40)
	tip := strings.Repeat("6", 40)
	const opID = "mirror-op"
	rig := newMirrorPolicyRig(t, tip,
		mirrorClaim(), mirrorRead(repo, "origin", tip), mirrorFetch(repo, opID, nil),
		mirrorValidate(repo, endpoint, tip, nil), mirrorCleanup(repo, opID, nil),
		mirrorRead(repo, "transport", ""), mirrorPush(repo, "", tip, "", CASLanded, nil),
	)
	result, err := mirror(rig.request(repo, endpoint, opID), rig.dependencies())
	if err != nil || result != (PushResult{State: "mirrored", Tip: tip}) || rig.tips["transport"] != tip {
		t.Fatalf("operation fetch ref mirror = %+v, err=%v, transport=%q", result, err, rig.tips["transport"])
	}
}

func TestMirrorFetchAndValidationFailures(t *testing.T) {
	t.Parallel()
	fetchErr := errors.New("fetch failed")
	rangeErr := errors.New("range rejected")
	cleanupErr := errors.New("cleanup failed")
	for _, test := range []struct {
		name, opID              string
		fetchErr, rangeErr      error
		cleanupErr, expectedErr error
	}{
		{name: "fetch failure plus cleanup", opID: "fetch-failure", fetchErr: fetchErr, cleanupErr: cleanupErr, expectedErr: fetchErr},
		{name: "range failure plus cleanup", opID: "range-failure", rangeErr: rangeErr, cleanupErr: cleanupErr, expectedErr: rangeErr},
		{name: "cleanup-only failure", opID: "cleanup-failure", cleanupErr: cleanupErr, expectedErr: cleanupErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			endpoint := strings.Repeat("7", 40)
			tip := strings.Repeat("8", 40)
			calls := []mirrorPolicyCall{mirrorClaim(), mirrorRead(repo, "origin", tip), mirrorFetch(repo, test.opID, test.fetchErr)}
			if test.fetchErr == nil {
				calls = append(calls, mirrorValidate(repo, endpoint, tip, test.rangeErr))
			}
			calls = append(calls, mirrorCleanup(repo, test.opID, test.cleanupErr))
			rig := newMirrorPolicyRig(t, tip, calls...)
			result, err := mirror(rig.request(repo, endpoint, test.opID), rig.dependencies())
			if result != (PushResult{}) || err != test.expectedErr || rig.tips["transport"] != "" || rig.tips["origin"] != tip {
				t.Fatalf("failed mirror = %+v, err=%v, origin=%q, transport=%q", result, err, rig.tips["origin"], rig.tips["transport"])
			}
		})
	}
}
