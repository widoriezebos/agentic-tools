package batch

import (
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const testBatchID = "01j5x00000000000000000ba01"

type scriptedProber map[int64]identity.Liveness

func (p scriptedProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{Pid: pid, StartedAt: time.Unix(pid, 0)}, p[pid], nil
}
func must(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}
func load(t *testing.T, s Store) Record      { r, e := s.Load(testBatchID); must(t, e); return r }
func contents(t *testing.T, p string) []byte { b, e := os.ReadFile(p); must(t, e); return b }
func TestBatchJoinsSerializeUnderTheLock(t *testing.T) {
	store := NewStore(t.TempDir(), scriptedProber{})
	must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, TipTree: "base", State: StateOpen}))
	held, contended := make(chan struct{}, 1), make(chan struct{})
	store.seams.flock = func(_ int, operation int) error {
		if operation == unix.LOCK_UN {
			<-held
			return nil
		}
		select {
		case held <- struct{}{}:
		default:
			close(contended)
			held <- struct{}{}
		}
		return nil
	}
	first, release, second := make(chan struct{}), make(chan struct{}), make(chan struct{})
	done := make(chan error, 2)
	update := func(change func(*Record) error) { done <- store.Update(testBatchID, change) }
	go update(func(record *Record) error { record.TipTree += "+goal-1"; close(first); <-release; return nil })
	<-first
	go update(func(record *Record) error { record.TipTree += "+goal-2"; close(second); return nil })
	select {
	case <-contended:
	case <-second:
		t.Error("second join entered the batch mutation while the first held it")
	}
	close(release)
	must(t, <-done)
	must(t, <-done)
	if <-second; load(t, store).TipTree != "base+goal-1+goal-2" {
		t.Fatalf("serialized tip=%q", load(t, store).TipTree)
	}
}
func TestBatchHistoryIsAppendOnly(t *testing.T) {
	store := NewStore(t.TempDir(), scriptedProber{})
	must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, State: StateOpen, Units: []Unit{{GoalID: "g1", Chain: "j1", Claim: Claim{Machine: "m1", Lineage: "l1", Epoch: 1, Revision: 1, AccountingRevision: 1}, State: UnitJoined}}}))
	states := []string{StateSealed, StateProving, StateDiagnosing, StateProving, StateLanding, StateLanded, StateOpen, StateDissolved}
	for index, state := range states {
		must(t, store.Update(testBatchID, func(record *Record) error {
			record.Transition(state, time.Unix(int64(index), 0), "tick", "m1l+landing-m1l", "")
			return nil
		}))
	}
	if got := len(load(t, store).History); got != len(states) {
		t.Fatalf("history has %d entries, want %d", got, len(states))
	}
	path, _ := store.recordPath(testBatchID)
	before := contents(t, path)
	for name, mutate := range map[string]func(*Record){
		"truncate history": func(r *Record) { r.History = r.History[:1] },
		"edit history":     func(r *Record) { r.History[0].Verb = "rewrite" },
		"edit revision":    func(r *Record) { r.Units[0].Claim.Revision++ },
		"remove unit":      func(r *Record) { r.Units = nil },
	} {
		if err := store.Update(testBatchID, func(r *Record) error { mutate(r); return nil }); err == nil {
			t.Fatalf("%s mutation was accepted", name)
		}
		if string(contents(t, path)) != string(before) {
			t.Fatalf("%s refusal changed the record", name)
		}
	}
}
func TestBatchLivenessUsesInjectedProber(t *testing.T) {
	prober := scriptedProber{101: identity.Alive, 102: identity.Dead, 103: identity.Unknown}
	for pid, want := range prober {
		if got := NewStore("", prober).Liveness(identity.Ref{Pid: pid, StartedAtSec: pid}); got != want {
			t.Fatalf("pid %d liveness=%s, want %s", pid, got, want)
		}
	}
}
