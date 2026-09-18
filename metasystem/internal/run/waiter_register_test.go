package run

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type registeredWaitProber struct {
	exact map[int64]identity.Exact
	state map[int64]identity.Liveness
}

func (p *registeredWaitProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	state, ok := p.state[pid]
	if !ok {
		return identity.Exact{}, identity.Dead, nil
	}
	if state == identity.Unknown {
		return identity.Exact{}, state, errors.New("identity unavailable")
	}
	return p.exact[pid], state, nil
}

func registeredWaitOwnerFixture() Caller {
	epoch := int64(7)
	return Caller{Class: "MAIN", MainId: "main-register", OwnerLineage: "lineage-register", ClaimEpoch: &epoch, SessionId: "session-register"}
}

func TestWaitEndAndSweep(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	prober := &registeredWaitProber{
		exact: map[int64]identity.Exact{
			81: {Pid: 81, StartedAt: time.Unix(800, 0), StartTicks: 810, BootID: "process-boot"},
			82: {Pid: 82, StartedAt: time.Unix(900, 0), StartTicks: 820, BootID: "process-boot"},
			83: {Pid: 83, StartedAt: time.Unix(1000, 0), StartTicks: 830, BootID: "process-boot"},
		},
		state: map[int64]identity.Liveness{81: identity.Alive, 82: identity.Alive, 83: identity.Alive},
	}
	store := &Store{Root: root, Prober: prober}
	owner := registeredWaitOwnerFixture()
	options := WaitOptions{
		Now:       func() time.Time { return now },
		BootClock: func() (string, time.Duration, error) { return "system-boot", time.Hour, nil },
	}
	registerLocal := func(pid int64, label string) Waiter {
		t.Helper()
		row, err := store.RegisterDetachedWait(RegisterWaitRequest{
			Kind: "local", Pid: pid, Label: label, Owner: owner, RuntimeSession: owner.SessionId, Timeout: time.Hour,
		}, options)
		if err != nil {
			t.Fatal(err)
		}
		return row
	}

	ended := registerLocal(81, "first build")
	row, err := store.EndDetachedWait(ended.WaitID, owner, owner.SessionId, options)
	if err != nil || row.State != WaiterStateInterrupted || row.Result == nil ||
		row.Result.Reason != "registered wait ended by "+owner.MainId || row.InterruptedBy != owner.MainId {
		t.Fatalf("ended row=%+v result=%+v err=%v", row, row.Result, err)
	}
	again, err := store.EndDetachedWait(ended.WaitID, owner, owner.SessionId, options)
	if err != nil || again.State != WaiterStateInterrupted || again.Result == nil || again.Result.Reason != row.Result.Reason {
		t.Fatalf("second end changed the result: row=%+v err=%v", again, err)
	}
	foreign := owner
	foreign.MainId = "main-foreign"
	foreign.OwnerLineage = "lineage-foreign"
	if _, err := store.EndDetachedWait(ended.WaitID, foreign, foreign.SessionId, options); err == nil || WaiterExitCode(err) != ExitWaiterBusy {
		t.Fatalf("foreign end error=%v code=%d", err, WaiterExitCode(err))
	}

	dead := registerLocal(82, "dead build")
	human, err := store.RegisterDetachedWait(RegisterWaitRequest{
		Kind: "human", Question: "Ship this change?", Owner: owner, RuntimeSession: owner.SessionId, Timeout: time.Minute,
	}, options)
	if err != nil {
		t.Fatal(err)
	}
	prober.state[82] = identity.Dead
	now = now.Add(2 * time.Minute)
	_ = registerLocal(83, "next build")
	for _, waitID := range []string{dead.WaitID, human.WaitID} {
		stored, _, err := FindWaiterByID(root, waitID)
		if err != nil || stored.State != WaiterStateInterrupted || stored.Result == nil || stored.InterruptedBy != owner.MainId {
			t.Fatalf("swept row %s=%+v err=%v", waitID, stored, err)
		}
	}
}

func TestWaiterRowsFromBeforeLocalWaitsStillRead(t *testing.T) {
	old := `{"schemaVersion":2,"waitId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","nonce":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","kind":"run","targetId":"run-a","ownerDigest":"owner","pid":41,"pidStartedAt":100,"session":"session-a","mainId":"main-a","selector":{"kind":"run","targetId":"run-a"},"target":{"generation":1,"launchNonce":"nonce-a"},"state":"pending","delivery":"blocking"}`
	decoder := json.NewDecoder(strings.NewReader(old))
	decoder.DisallowUnknownFields()
	var row Waiter
	if err := decoder.Decode(&row); err != nil {
		t.Fatal(err)
	}
	if row.Label != "" || row.Question != "" || row.JobID != "" || row.Kind != "run" {
		t.Fatalf("old row changed shape: %+v", row)
	}
	if err := ValidateWaitSelector(row.Selector); err != nil {
		t.Fatalf("old selector no longer reads: %v", err)
	}
	if err := ValidateWaitSelector(WaitSelector{Kind: "local", TargetID: strings.Repeat("a", 32)}); err != nil {
		t.Fatalf("local selector: %v", err)
	}
	if err := ValidateWaitSelector(WaitSelector{Kind: "human", TargetID: strings.Repeat("b", 32), Question: "Proceed?"}); err != nil {
		t.Fatalf("human selector: %v", err)
	}
	for _, selector := range []WaitSelector{
		{Kind: "human", TargetID: strings.Repeat("b", 32)},
		{Kind: "local", TargetID: strings.Repeat("a", 32), Question: "not local"},
		{Kind: "run", TargetID: "run-a", Question: "not a run field"},
	} {
		if err := ValidateWaitSelector(selector); err == nil {
			t.Fatalf("invalid selector accepted: %+v", selector)
		}
	}
}
