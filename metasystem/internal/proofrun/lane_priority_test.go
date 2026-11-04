package proofrun

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type priorityResult struct {
	lease *HostResourceLease
	err   error
}

type priorityParty struct {
	checks chan chan struct{}
	waited chan struct{}
	result chan priorityResult
}

// Each permission releases one check hook: loop top, then admission guard.
func startPriorityParty(t *testing.T, ctx context.Context, readers loadReaders, directory, conf string) *priorityParty {
	t.Helper()
	ctx, cancel := context.WithCancel(ctx)
	p := &priorityParty{make(chan chan struct{}), make(chan struct{}), make(chan priorityResult, 1)}
	done := make(chan struct{})
	go func() {
		defer close(done)
		calls := 0
		lease, err := acquireHostResourcesWith(WithHostResourceWaitObserver(ctx, func() { close(p.waited) }), readers, directory, directory, conf, "heavy", nil, func() error {
			calls++
			if calls == 10 {
				return errors.New("ten check hooks without acquiring")
			}
			permit := make(chan struct{})
			select {
			case p.checks <- permit:
			case <-ctx.Done():
				return ctx.Err()
			}
			select {
			case <-permit:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		p.result <- priorityResult{lease, err}
	}()
	t.Cleanup(func() {
		cancel()
		<-done
		select {
		case result := <-p.result:
			_ = result.lease.Close()
		default:
		}
	})
	return p
}

func (p *priorityParty) nextHook(t *testing.T) chan struct{} {
	t.Helper()
	select {
	case permit := <-p.checks:
		return permit
	case result := <-p.result:
		_ = result.lease.Close()
		t.Fatalf("acquisition returned before the next loop hook: %v", result.err)
		return nil
	}
}

func (p *priorityParty) firstWait(t *testing.T) {
	t.Helper()
	close(p.nextHook(t))
	close(p.nextHook(t))
	select {
	case <-p.waited:
	case result := <-p.result:
		_ = result.lease.Close()
		t.Fatalf("acquisition returned before its failed-scan observer: %v", result.err)
	}
}

func (p *priorityParty) finish(t *testing.T) *HostResourceLease {
	t.Helper()
	for {
		select {
		case permit := <-p.checks:
			close(permit)
		case result := <-p.result:
			if result.err != nil {
				t.Fatal(result.err)
			}
			t.Cleanup(func() { _ = result.lease.Close() })
			return result.lease
		}
	}
}

func TestTheLanesCheckTakesTheFreedSlotBeforeAWaitingRun(t *testing.T) {
	t.Parallel()
	directory, conf := isolatedHostResources(t)
	writeTestFile(t, conf, []byte(AdmissionCapKey+"=2\n"), 0o600)
	a, err := acquireHostResourcesIn(t.Context(), directory, directory, conf, "heavy", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := acquireHostResourcesIn(t.Context(), directory, directory, conf, "heavy", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	lane := startPriorityParty(t, WithLaneCheckPriority(t.Context()), loadSeams, directory, conf)
	run := startPriorityParty(t, t.Context(), loadSeams, directory, conf)
	lane.firstWait(t)
	run.firstWait(t)
	markers, _ := filepath.Glob(filepath.Join(directory, "wait-lane-*"))
	if len(markers) != 1 {
		t.Fatalf("lane did not publish one wait marker: %v", markers)
	}
	held, err := heldByAnother(markers[0])
	if err != nil || !held {
		t.Fatalf("lane wait marker is not locked: held=%t err=%v", held, err)
	}
	laneParked := lane.nextHook(t)
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	close(run.nextHook(t))       // Loop top after A's release.
	close(run.nextHook(t))       // Guard: C must still find no room.
	runParked := run.nextHook(t) // Next loop top proves that scan returned no lease.
	close(laneParked)
	lane.finish(t)
	if markers, _ := filepath.Glob(filepath.Join(directory, "wait-lane-*")); len(markers) != 0 {
		t.Fatalf("acquired lane left its wait marker: %v", markers)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	close(runParked)
	run.finish(t)
	if markers, _ := filepath.Glob(filepath.Join(directory, "wait-*")); len(markers) != 0 {
		t.Fatalf("acquired run left wait markers: %v", markers)
	}
}

func TestAWaitingTestRunNeverHoldsTheLaneOut(t *testing.T) {
	t.Parallel()
	directory, conf := isolatedHostResources(t)
	census := &admissionCensus{}
	process := admissionProcess(5151, 0, time.Unix(100, 123000), "/x/bin/metasystem", "test", "run")
	exact := reclaimExact(5151, 100123000)
	exact.Argv, exact.ArgvKnown = process.exact.Argv, process.exact.ArgvKnown
	process.exact = exact
	census.replace(process)
	lane := startPriorityParty(t, WithLaneCheckPriority(t.Context()), censusLoadReaders(census, 2), directory, conf)
	lane.firstWait(t)
	markers, err := filepath.Glob(filepath.Join(directory, "wait-lane-*"))
	if err != nil || len(markers) != 1 {
		t.Fatalf("waiting lane did not publish one wait marker: markers=%v err=%v", markers, err)
	}
	if held, err := heldByAnother(markers[0]); err != nil || !held {
		t.Fatalf("waiting lane did not hold its wait marker: held=%t err=%v", held, err)
	}
	admission, err := hostAdmissionDirectory()
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(admission, "managed-5151.json")
	data, err := json.Marshal(processIdentity(process.exact, 5151))
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, marker, data, 0o600)
	defer os.Remove(marker)
	lane.finish(t)
}

func TestANestedRunUnderALeasedParentNeverWaitsOnItself(t *testing.T) {
	t.Parallel()
	for _, row := range []string{"ancestor", "resource held", "released", "different identity", "self", "unknown parent", "parent cycle", "no slot"} {
		t.Run(row, func(t *testing.T) {
			directory, conf := isolatedHostResources(t)
			parent := admissionProcess(4242, 0, time.Unix(100, 123000), "check")
			census := &admissionCensus{}
			self := admissionProcess(int64(os.Getpid()), 4242, time.Unix(200, 123000), "test")
			parent.exact = reclaimExact(4242, 100123000)
			self.exact = reclaimExact(self.exact.Pid, 200123000)
			if row == "self" {
				parent = self
			}
			if row == "unknown parent" {
				self.parent = -1
			}
			if row == "parent cycle" {
				self.parent = self.exact.Pid
			}
			census.replace(self, parent)
			record := hostLeaseRecord{Schema: 1, Owner: processIdentity(parent.exact, 4242), Class: "heavy", Slot: "slot-00", Cleared: true}
			if row == "different identity" {
				record.Owner = processIdentity(reclaimExact(parent.exact.Pid, 300123000), 4242)
			}
			if row == "no slot" {
				record.Slot = ""
			}
			path := filepath.Join(directory, "lease-heavy-"+strings.Repeat("a", 32))
			marker, acquired, err := tryHostFile(path)
			if err != nil || !acquired {
				t.Fatalf("lease marker: acquired=%t err=%v", acquired, err)
			}
			defer marker.Close()
			if err := json.NewEncoder(marker).Encode(record); err != nil {
				t.Fatal(err)
			}
			if row == "released" {
				if err := releaseHostProbe(marker); err != nil {
					t.Fatal(err)
				}
			}
			slot, _, err := tryHostFile(filepath.Join(directory, "slot-00"))
			if err != nil {
				t.Fatal(err)
			}
			defer slot.Close()
			wait, err := holdWaitMarker(directory, HostPriorityLane)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = os.Remove(wait.Name()); _ = releaseHostProbe(wait) }()
			var resources []string
			var resource *os.File
			if row == "resource held" {
				resources = []string{"fixture-db"}
				resource, _, err = tryHostFile(hostResourcePath(directory, resources[0]))
				if err != nil {
					t.Fatal(err)
				}
				defer resource.Close()
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			waited := false
			ctx = WithHostResourceWaitObserver(ctx, func() {
				waited = true
				markers, _ := filepath.Glob(filepath.Join(directory, "wait-run-*"))
				if row == "resource held" {
					if len(markers) != 0 {
						t.Error("nested resource wait published a capacity waiter")
					}
					_ = releaseHostProbe(resource)
				} else {
					cancel()
				}
			})
			checks := 0
			lease, err := acquireHostResourcesWith(ctx, censusLoadReaders(census, 2), directory, directory, conf, "heavy", resources, func() error {
				checks++
				if checks == 10 {
					return errors.New("ten check hooks without acquiring")
				}
				return nil
			})
			nested := row == "ancestor" || row == "resource held"
			if !nested {
				if lease != nil {
					_ = lease.Close()
				}
				if !waited || !errors.Is(err, context.Canceled) {
					t.Fatalf("non-ancestor did not wait: waited=%t lease=%v err=%v", waited, lease, err)
				}
			} else {
				if err != nil {
					t.Fatalf("nested acquisition waited on capacity: %v", err)
				}
				defer lease.Close()
				for _, file := range lease.Files() {
					if strings.HasPrefix(filepath.Base(file.Name()), "slot-") {
						t.Fatal("nested acquisition took another slot")
					}
				}
				if waited != (row == "resource held") {
					t.Fatalf("nested wait observer = %t", waited)
				}
			}
			if markers, _ := filepath.Glob(filepath.Join(directory, "wait-run-*")); len(markers) != 0 {
				t.Fatalf("return left wait markers: %v", markers)
			}
		})
	}
}

func TestLeaseAncestryScanToleratesClaimsInFlight(t *testing.T) {
	t.Parallel()
	t.Run("unfinished claim waits for admission guard", func(t *testing.T) {
		directory, conf := isolatedHostResources(t)
		guard, err := acquireHostGuard(t.Context(), directory, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer releaseHostProbe(guard)
		marker, acquired, err := tryHostFile(filepath.Join(directory, "lease-heavy-"+strings.Repeat("b", 32)))
		if err != nil || !acquired {
			t.Fatalf("unfinished claim: acquired=%t err=%v", acquired, err)
		}
		defer releaseHostProbe(marker)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		checks := 0
		lease, err := acquireHostResourcesWith(ctx, loadSeams, directory, directory, conf, "heavy", nil, func() error {
			checks++
			// Loop top, first guard attempt, then retry after the held guard.
			if checks == 3 {
				cancel()
				return ctx.Err()
			}
			return nil
		})
		if lease != nil {
			_ = lease.Close()
			t.Fatal("acquisition passed a held admission guard")
		}
		if checks != 3 || !errors.Is(err, context.Canceled) {
			t.Fatalf("unfinished claim did not wait for the guard: checks=%d err=%v", checks, err)
		}
	})
	t.Run("lease removed after held probe", func(t *testing.T) {
		directory, _ := isolatedHostResources(t)
		path := filepath.Join(directory, "lease-heavy-"+strings.Repeat("c", 32))
		marker, acquired, err := tryHostFile(path)
		if err != nil || !acquired {
			t.Fatalf("lease marker: acquired=%t err=%v", acquired, err)
		}
		defer releaseHostProbe(marker)
		owner, err := CurrentProcessIdentity(nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.NewEncoder(marker).Encode(hostLeaseRecord{Schema: 1, Owner: owner, Class: "heavy", Slot: "slot-00", Cleared: true}); err != nil {
			t.Fatal(err)
		}
		probed := false
		nested, err := nestedUnderLeaseHolder(loadSeams, directory, func(candidate string) (bool, error) {
			if candidate != path {
				t.Fatalf("unexpected lease probe: %s", candidate)
			}
			held, err := heldByAnother(candidate)
			if err != nil || !held {
				t.Fatalf("lease was not held before removal: held=%t err=%v", held, err)
			}
			if err := os.Remove(candidate); err != nil {
				t.Fatal(err)
			}
			probed = true
			return held, nil
		})
		if !probed || nested || err != nil {
			t.Fatalf("removed lease was treated as an ancestor or an error: probed=%t nested=%t err=%v", probed, nested, err)
		}
	})
}

func TestLaneWaitScanRemovesStaleMarkersWithoutCreatingThem(t *testing.T) {
	t.Parallel()
	directory, _ := isolatedHostResources(t)
	missing := filepath.Join(directory, "wait-lane-missing")
	if held, err := heldByAnother(missing); held || err != nil {
		t.Fatalf("missing marker: held=%t err=%v", held, err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("probe created a missing marker: %v", err)
	}
	for _, priority := range []string{HostPriorityLane, hostPriorityRun} {
		marker, err := holdWaitMarker(directory, priority)
		if err != nil {
			t.Fatal(err)
		}
		if err := releaseHostProbe(marker); err != nil {
			t.Fatal(err)
		}
	}
	waiting, err := laneCheckWaiting(directory, nil)
	if err != nil || waiting {
		t.Fatalf("stale markers held admission: waiting=%t err=%v", waiting, err)
	}
	if markers, _ := filepath.Glob(filepath.Join(directory, "wait-*")); len(markers) != 0 {
		t.Fatalf("stale markers remain: %v", markers)
	}
}
