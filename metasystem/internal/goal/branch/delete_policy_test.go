package branch_test

import (
	"errors"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

type deleteTransportCall struct {
	method, repo, remote, ref string
	expected, newTip          string
	readTip                   string
	present                   bool
	beforeTip                 string
	outcome                   branch.CASOutcome
	err                       error
	apply                     bool
	effectTip                 string
}

type deleteTransport struct {
	t     *testing.T
	calls []deleteTransportCall
	next  int
	tips  map[string]string
}

func newDeleteTransport(t *testing.T, ref, initialTip string, calls ...deleteTransportCall) *deleteTransport {
	t.Helper()
	transport := &deleteTransport{t: t, calls: calls, tips: map[string]string{ref: initialTip}}
	t.Cleanup(func() {
		if transport.next != len(transport.calls) {
			t.Errorf("delete transport consumed %d of %d calls", transport.next, len(transport.calls))
		}
	})
	return transport
}

func (d *deleteTransport) take(method, repo, remote, ref string) deleteTransportCall {
	d.t.Helper()
	if d.next == len(d.calls) {
		d.t.Fatalf("unexpected %s(%q, %q, %q)", method, repo, remote, ref)
	}
	call := d.calls[d.next]
	d.next++
	if call.method != method || call.repo != repo || call.remote != remote || call.ref != ref {
		d.t.Fatalf("delete transport call %d = %s(%q, %q, %q), want %s(%q, %q, %q)",
			d.next, method, repo, remote, ref, call.method, call.repo, call.remote, call.ref)
	}
	return call
}

func (d *deleteTransport) RemoteTip(repo, remote, ref string) (string, bool, error) {
	call := d.take("RemoteTip", repo, remote, ref)
	actual := d.tips[ref]
	if call.readTip != actual || call.present != (actual != "") {
		d.t.Fatalf("declared remote read %d = (%q, %t), state holds %q", d.next, call.readTip, call.present, actual)
	}
	return call.readTip, call.present, call.err
}

func (d *deleteTransport) Fetch(repo, remote, ref, destination string) error {
	d.t.Fatalf("unexpected Fetch(%q, %q, %q, %q)", repo, remote, ref, destination)
	return nil
}

func (d *deleteTransport) Push(repo, remote, ref, expected, tip string) (branch.CASOutcome, error) {
	call := d.take("Push", repo, remote, ref)
	if call.expected != expected || call.newTip != tip || call.beforeTip != d.tips[ref] {
		d.t.Fatalf("delete push %d = expected %q, new %q, remote %q; want expected %q, new %q, remote %q",
			d.next, expected, tip, d.tips[ref], call.expected, call.newTip, call.beforeTip)
	}
	if call.apply {
		d.tips[ref] = call.effectTip
	}
	return call.outcome, call.err
}

func (d *deleteTransport) move(ref, from, to string) {
	d.t.Helper()
	if d.tips[ref] != from || d.next != 1 {
		d.t.Fatalf("competing move before successful read: tip %q, consumed calls %d", d.tips[ref], d.next)
	}
	d.tips[ref] = to
}

func TestDeleteLeasedTipMovesAfterRead(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	const ref = "refs/heads/goal/goal-a"
	const intruder = "1111111111111111111111111111111111111111"
	const other = "2222222222222222222222222222222222222222"
	transport := newDeleteTransport(t, ref, intruder,
		deleteTransportCall{method: "RemoteTip", repo: repo, remote: "transport", ref: ref, readTip: intruder, present: true},
		deleteTransportCall{method: "Push", repo: repo, remote: "transport", ref: ref, expected: intruder, newTip: "", beforeTip: other,
			outcome: branch.CASRefused, err: errors.New("remote lease refused")},
	)
	err := branch.Delete(branch.DeleteRequest{Repo: repo, Remote: "transport", GoalID: "goal-a", Expected: intruder,
		CheckClaim: claimAllowed, PushTransport: transport, AfterRemoteRead: func() error {
			transport.move(ref, intruder, other)
			return nil
		}})
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != branch.LeaseMovedCode || transport.tips[ref] != other {
		t.Fatalf("moved leased delete = %v, remote = %q", err, transport.tips[ref])
	}
}

func TestDeleteUnknownOutcomeCompleted(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	const ref = "refs/heads/goal/goal-a"
	const tip = "3333333333333333333333333333333333333333"
	transport := newDeleteTransport(t, ref, tip,
		deleteTransportCall{method: "RemoteTip", repo: repo, remote: "origin", ref: ref, readTip: tip, present: true},
		deleteTransportCall{method: "Push", repo: repo, remote: "origin", ref: ref, expected: tip, newTip: "", beforeTip: tip,
			outcome: branch.CASUnknown, err: errors.New("push response lost"), apply: true, effectTip: ""},
		deleteTransportCall{method: "RemoteTip", repo: repo, remote: "origin", ref: ref, readTip: "", present: false},
	)
	err := branch.Delete(branch.DeleteRequest{Repo: repo, Remote: "origin", GoalID: "goal-a", Expected: tip,
		CheckClaim: claimAllowed, PushTransport: transport})
	if err != nil || transport.tips[ref] != "" {
		t.Fatalf("completed unknown delete = %v, remote = %q", err, transport.tips[ref])
	}
}

func TestDeleteUnknownOutcomeNotCompleted(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	const ref = "refs/heads/goal/goal-a"
	const tip = "4444444444444444444444444444444444444444"
	transport := newDeleteTransport(t, ref, tip,
		deleteTransportCall{method: "RemoteTip", repo: repo, remote: "origin", ref: ref, readTip: tip, present: true},
		deleteTransportCall{method: "Push", repo: repo, remote: "origin", ref: ref, expected: tip, newTip: "", beforeTip: tip,
			outcome: branch.CASUnknown, err: errors.New("push response lost")},
		deleteTransportCall{method: "RemoteTip", repo: repo, remote: "origin", ref: ref, readTip: tip, present: true},
	)
	err := branch.Delete(branch.DeleteRequest{Repo: repo, Remote: "origin", GoalID: "goal-a", Expected: tip,
		CheckClaim: claimAllowed, PushTransport: transport})
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != branch.PushUnknownCode || transport.tips[ref] != tip {
		t.Fatalf("incomplete unknown delete = %v, remote = %q", err, transport.tips[ref])
	}
}

func TestDeleteLandingRemovesOrphanRef(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	const ref = "refs/heads/landing/goal-a"
	const tip = "5555555555555555555555555555555555555555"
	transport := newDeleteTransport(t, ref, tip,
		deleteTransportCall{method: "RemoteTip", repo: repo, remote: "origin", ref: ref, readTip: tip, present: true},
		deleteTransportCall{method: "Push", repo: repo, remote: "origin", ref: ref, expected: tip, newTip: "", beforeTip: tip,
			outcome: branch.CASLanded, apply: true, effectTip: ""},
	)
	if err := branch.DeleteLanding(branch.DeleteLandingRequest{Repo: repo, Remote: "origin", GoalID: "goal-a",
		CheckClaim: claimAllowed, PushTransport: transport}); err != nil {
		t.Fatal(err)
	}
	if refs := transport.tips[ref]; refs != "" {
		t.Fatalf("orphan landing survived: %s", refs)
	}
}
