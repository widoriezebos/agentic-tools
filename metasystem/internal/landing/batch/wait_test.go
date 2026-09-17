package batch

import (
	"strings"
	"testing"
	"time"
)

type waitFakeClock struct {
	now    time.Time
	store  Store
	id     string
	finish string
	ticks  int
}

func (clock *waitFakeClock) Now() time.Time { return clock.now }
func (clock *waitFakeClock) After(duration time.Duration) <-chan time.Time {
	clock.now = clock.now.Add(duration)
	clock.ticks++
	if clock.finish != "" && clock.ticks == 2 {
		_ = clock.store.Update(clock.id, func(record *Record) error {
			record.Transition(clock.finish, clock.now, "fixture", "test", "")
			return nil
		})
	}
	ready := make(chan time.Time, 1)
	ready <- clock.now
	return ready
}

func waitRecord(t *testing.T, state string) Store {
	t.Helper()
	store := NewStore(t.TempDir(), nil)
	must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, State: state}))
	return store
}

func TestBatchWaitReturnsOnEveryTerminalOrHeldState(t *testing.T) {
	for _, state := range []string{StateLanded, StateDissolved, StateHeldTrunkRed, StateHeldUnclassified} {
		t.Run(state, func(t *testing.T) {
			store := waitRecord(t, StateOpen)
			clock := &waitFakeClock{now: time.Unix(1, 0), store: store, id: testBatchID, finish: state}
			if state == StateHeldTrunkRed {
				must(t, store.Update(testBatchID, func(record *Record) error {
					record.TrunkRed = &TrunkRedHold{Opid: "op", Opids: []string{"op"}, Red: TrunkRed{Groups: []RedGroup{{ID: "g"}}}}
					return nil
				}))
			}
			record, err := Wait(store, testBatchID, 10*time.Minute, WaitClock{Now: clock.Now, After: clock.After})
			if err != nil || record.State != state || clock.ticks != 2 {
				t.Fatalf("record=%+v ticks=%d error=%v", record, clock.ticks, err)
			}
		})
	}
}

func TestBatchWaitUsesInjectedBound(t *testing.T) {
	store := waitRecord(t, StateOpen)
	clock := &waitFakeClock{now: time.Unix(1, 0), store: store, id: testBatchID}
	_, err := Wait(store, testBatchID, 10*time.Minute, WaitClock{Now: clock.Now, After: clock.After})
	if err == nil || !strings.Contains(err.Error(), "BATCH_WAIT_BOUND") || clock.ticks != 10 {
		t.Fatalf("ticks=%d error=%v", clock.ticks, err)
	}
}
