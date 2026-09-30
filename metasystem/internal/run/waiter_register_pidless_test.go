package run

import (
	"strings"
	"testing"
	"time"
)

// A pidless local wait covers work with no process of its own (in-process
// sub-agents): a label and a bounded timeout, at most MaxPidlessLocalWaitTimeout;
// it ends at its deadline or by EndDetachedWait.
func TestPidlessLocalWaitIsBoundedAndEndsAtItsDeadline(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	store := &Store{Root: root, Prober: &registeredWaitProber{}}
	owner := registeredWaitOwnerFixture()
	options := WaitOptions{
		Now:       func() time.Time { return now },
		BootClock: func() (string, time.Duration, error) { return "system-boot", time.Hour, nil },
	}
	request := RegisterWaitRequest{Kind: "local", Label: "in-session sub-agents", Owner: owner, RuntimeSession: owner.SessionId, Timeout: 90 * time.Minute}
	row, err := store.RegisterDetachedWait(request, options)
	if err != nil || row.Pid != 0 || row.RegisteredBootID != "system-boot" || row.Deadline != now.Add(90*time.Minute).Format(time.RFC3339Nano) {
		t.Fatalf("pidless wait row=%+v err=%v", row, err)
	}
	over := request
	over.Timeout = MaxPidlessLocalWaitTimeout + time.Minute
	if _, err := store.RegisterDetachedWait(over, options); err == nil || !strings.Contains(err.Error(), "2 hours") {
		t.Fatalf("an unbounded pidless wait registered: %v", err)
	}
	now = now.Add(2 * time.Hour)
	if _, err := store.RegisterDetachedWait(request, options); err != nil {
		t.Fatal(err)
	}
	stored, _, err := FindWaiterByID(root, row.WaitID)
	if err != nil || stored.State != WaiterStateInterrupted {
		t.Fatalf("a pidless wait past its deadline was not swept: %+v %v", stored, err)
	}
}
