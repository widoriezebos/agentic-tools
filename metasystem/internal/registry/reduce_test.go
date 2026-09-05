package registry

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// The reduction tests replay REG-3's rules as event sequences. Each
// helper builds one raw record the way a writer would frame it.

func raw(event, tag string, extra map[string]any) map[string]any {
	record := map[string]any{
		"schemaVersion": 1.0,
		"event":         event,
		"checkoutPath":  "/repo",
		"at":            "2026-08-09T20:00:00Z",
	}
	if tag != "" {
		record["ownerTag"] = tag
	}
	for key, value := range extra {
		record[key] = value
	}
	return record
}

func frames(records ...map[string]any) []Frame {
	var out []Frame
	for i, record := range records {
		out = append(out, Frame{Line: i + 1, Record: record})
	}
	return out
}

func reduceOrFail(t *testing.T, rows ...map[string]any) *Reduction {
	t.Helper()
	reduction, err := Reduce(frames(rows...))
	if err != nil {
		t.Fatalf("Reduce: %v", err)
	}
	return reduction
}

func armedRow(tag string, pid int64) map[string]any {
	return raw(EventArmed, tag, map[string]any{
		"ownerPid": float64(pid), "ownerPidStartedAt": 100.0, "generation": 1.0,
	})
}

func TestClaimLifecycle(t *testing.T) {
	reduction := reduceOrFail(t,
		raw(EventArming, "tag-a", nil),
		armedRow("tag-a", 41),
		raw(EventRelaunched, "tag-a", map[string]any{
			"generation": 1.0, "watcherTag": "w1", "reaperTag": "r1", "retiredThrough": 0.0,
		}),
		raw(EventLaunched, "tag-a", map[string]any{
			"generation": 1.0, "component": "watcher", "pid": 42.0, "pidStartedAt": 200.0,
		}),
		raw(EventExited, "tag-a", map[string]any{"reason": "purpose-gone", "teardownComplete": true}),
	)
	claim := reduction.Claims["tag-a"]
	if claim == nil || !claim.Armed || !claim.Closed {
		t.Fatalf("lifecycle did not reduce: %+v", claim)
	}
	if claim.Open() || claim.Sweepable() {
		t.Fatalf("a cleanly exited claim is neither open nor sweepable: %+v", claim)
	}
	if claim.Owner.Pid != 41 || claim.Reason != "purpose-gone" || claim.ClosedBy != EventExited {
		t.Fatalf("wrong reduced fields: %+v", claim)
	}
	if got := claim.CurrentGeneration(); got == nil || got.WatcherTag != "w1" || got.Identities["watcher"].Pid != 42 {
		t.Fatalf("generation set wrong: %+v", got)
	}
}

func TestTerminalsAreAbsorbing(t *testing.T) {
	reduction := reduceOrFail(t,
		raw(EventArming, "tag-a", nil),
		raw(EventExited, "tag-a", map[string]any{"reason": "terminated", "teardownComplete": true}),
		raw(EventReaped, "tag-a", map[string]any{"reason": "owner-dead", "sweepPending": true}),
	)
	claim := reduction.Claims["tag-a"]
	if claim.ClosedBy != EventExited || claim.Reason != "terminated" || claim.SweepPending {
		t.Fatalf("a later reaped overrode an absorbing exited: %+v", claim)
	}
}

// SLC-R5-004: after a reap, a delayed armed does not reopen the claim
// — refused at the door by the guard, dropped at reduction if present.
func TestLateArmedDoesNotReopen(t *testing.T) {
	reduction := reduceOrFail(t,
		raw(EventArming, "tag-a", nil),
		raw(EventReaped, "tag-a", map[string]any{"reason": "establishment-orphan", "sweepPending": false}),
		armedRow("tag-a", 77),
	)
	claim := reduction.Claims["tag-a"]
	if claim.Armed || !claim.Closed || claim.Owner.Pid != 0 {
		t.Fatalf("late armed reopened a closed claim: %+v", claim)
	}
	if !reduction.TagSeen("tag-a") {
		t.Fatal("a closed claim's tag must stay seen (SLC-R7-008)")
	}
}

func TestReductionDropsIdentityKeysAcrossCheckoutPaths(t *testing.T) {
	assertLoggedPaths := func(t *testing.T, reduction *Reduction) {
		t.Helper()
		output := strings.Join(reduction.Dropped, "\n")
		if !strings.Contains(output, "sequence-illegal") ||
			!strings.Contains(output, "/checkout/a") || !strings.Contains(output, "/checkout/b") {
			t.Fatalf("the dropped record was not logged with both checkout paths: %q", output)
		}
	}

	t.Run("owner tag", func(t *testing.T) {
		arming := raw(EventArming, "tag-a", nil)
		arming["checkoutPath"] = "/checkout/a"
		armed := armedRow("tag-a", 41)
		armed["checkoutPath"] = "/checkout/b"
		reduction, err := Reduce(frames(arming, armed))
		if err != nil {
			t.Fatalf("a sequence-illegal owner row corrupted the registry: %v", err)
		}
		claim := reduction.Claims["tag-a"]
		if claim == nil || claim.CheckoutPath != "/checkout/a" || claim.Armed {
			t.Fatalf("the conflicting armed row changed the earlier claim: %+v", claim)
		}
		assertLoggedPaths(t, reduction)
	})

	t.Run("production owner selection survives compaction", func(t *testing.T) {
		relaunched := raw(EventRelaunched, "tag-compacted", map[string]any{
			"generation": 1.0, "watcherTag": "w1", "reaperTag": "r1", "retiredThrough": 0.0,
		})
		relaunched["checkoutPath"] = "/checkout/a"
		launched := raw(EventLaunched, "tag-compacted", map[string]any{
			"generation": 1.0, "component": "watcher", "pid": 42.0, "pidStartedAt": 200.0,
		})
		launched["checkoutPath"] = "/checkout/a"
		conflicting := raw(EventLaunched, "tag-compacted", map[string]any{
			"generation": 1.0, "component": "reaper", "pid": 43.0, "pidStartedAt": 200.0,
		})
		conflicting["checkoutPath"] = "/checkout/b"
		input := frames(relaunched, launched, conflicting)
		reduction, err := Reduce(input)
		if err != nil {
			t.Fatalf("reduce production owner rows: %v", err)
		}
		if checkout, found := reduction.OwnerCheckoutPath("tag-compacted"); !found || checkout != "/checkout/a" {
			t.Fatalf("production owner checkout selection = %q, %v", checkout, found)
		}
		if owner := reduction.PublishedOwners["tag-compacted"]; owner == nil || owner.Generations[1].Identities["watcher"].Pid != 42 {
			t.Fatalf("launched identity did not bind to the write-ahead generation: %+v", owner)
		}
		assertLoggedPaths(t, reduction)
		kept, err := CompactFrames(input, time.Now(), time.Hour)
		if err != nil {
			t.Fatalf("compact production owner rows: %v", err)
		}
		compacted, err := Reduce(kept)
		if err != nil {
			t.Fatalf("reduce compacted production owner rows: %v", err)
		}
		if checkout, found := compacted.OwnerCheckoutPath("tag-compacted"); !found || checkout != "/checkout/a" {
			t.Fatalf("compacted owner checkout selection = %q, %v; want the production ledger path", checkout, found)
		}
	})

	t.Run("claim rows never select an owner checkout", func(t *testing.T) {
		reduction := reduceOrFail(t,
			raw(EventArming, "claim-only", nil),
			armedRow("claim-only", 41),
		)
		if checkout, found := reduction.OwnerCheckoutPath("claim-only"); found {
			t.Fatalf("claim-only rows selected checkout %q", checkout)
		}
	})

	t.Run("custody binding", func(t *testing.T) {
		custody := raw(EventCustody, "", map[string]any{
			"custodyId": "custody-a", "custodianPid": 51.0, "custodianPidStartedAt": 100.0,
		})
		custody["checkoutPath"] = "/checkout/a"
		arming := raw(EventArming, "tag-b", map[string]any{"custodyId": "custody-a"})
		arming["checkoutPath"] = "/checkout/b"
		reduction, err := Reduce(frames(custody, arming))
		if err != nil {
			t.Fatalf("a sequence-illegal custody binding corrupted the registry: %v", err)
		}
		if reduction.Claims["tag-b"] != nil || reduction.Custodies["custody-a"].BoundOwnerTag != "" {
			t.Fatalf("the conflicting arming row bound custody across checkouts: %+v", reduction)
		}
		assertLoggedPaths(t, reduction)
	})

	t.Run("custody row after owner binding", func(t *testing.T) {
		arming := raw(EventArming, "tag-c", map[string]any{"custodyId": "custody-b"})
		arming["checkoutPath"] = "/checkout/a"
		custody := raw(EventCustody, "", map[string]any{
			"custodyId": "custody-b", "custodianPid": 52.0, "custodianPidStartedAt": 100.0,
		})
		custody["checkoutPath"] = "/checkout/b"
		reduction, err := Reduce(frames(arming, custody))
		if err != nil {
			t.Fatalf("a later sequence-illegal custody row corrupted the registry: %v", err)
		}
		if reduction.Claims["tag-c"] == nil || reduction.Custodies["custody-b"] != nil {
			t.Fatalf("the conflicting custody row replaced the earlier checkout binding: %+v", reduction)
		}
		assertLoggedPaths(t, reduction)
	})
}

// SLC-R7-007: records pair by generation, never append order — a
// stale generation-1 launched retry landing after generation 2's
// records updates generation 1, and the CURRENT set stays generation 2.
func TestGenerationPairing(t *testing.T) {
	reduction := reduceOrFail(t,
		raw(EventArming, "tag-a", nil),
		armedRow("tag-a", 41),
		raw(EventRelaunched, "tag-a", map[string]any{
			"generation": 1.0, "watcherTag": "w1", "reaperTag": "r1", "retiredThrough": 0.0,
		}),
		raw(EventRelaunched, "tag-a", map[string]any{
			"generation": 2.0, "watcherTag": "w2", "reaperTag": "r2", "retiredThrough": 1.0,
		}),
		raw(EventLaunched, "tag-a", map[string]any{
			"generation": 1.0, "component": "watcher", "pid": 91.0, "pidStartedAt": 300.0,
		}),
	)
	claim := reduction.Claims["tag-a"]
	if current := claim.CurrentGeneration(); current.WatcherTag != "w2" {
		t.Fatalf("current set is not the highest generation: %+v", current)
	}
	if claim.Generations[1].Identities["watcher"].Pid != 91 {
		t.Fatal("stale retry did not land in its own generation")
	}
	if len(claim.Generations[2].Identities) != 0 {
		t.Fatal("stale retry paired old identities with new tags")
	}
	if claim.RetiredThrough != 1 {
		t.Fatalf("watermark not advanced: %d", claim.RetiredThrough)
	}
}

func TestSweepableStates(t *testing.T) {
	cases := []struct {
		name      string
		rows      []map[string]any
		sweepable bool
	}{
		{
			name: "exited incomplete teardown",
			rows: []map[string]any{
				raw(EventArming, "t", nil),
				raw(EventExited, "t", map[string]any{"reason": "giving-up", "teardownComplete": false}),
			},
			sweepable: true,
		},
		{
			name: "reaped with survivors",
			rows: []map[string]any{
				raw(EventArming, "t", nil),
				raw(EventReaped, "t", map[string]any{"reason": "owner-dead", "sweepPending": true}),
			},
			sweepable: true,
		},
		{
			name: "swept clears (SLC-R5-015)",
			rows: []map[string]any{
				raw(EventArming, "t", nil),
				raw(EventReaped, "t", map[string]any{"reason": "owner-dead", "sweepPending": true}),
				raw(EventSwept, "t", nil),
			},
			sweepable: false,
		},
		{
			name: "clean reap",
			rows: []map[string]any{
				raw(EventArming, "t", nil),
				raw(EventReaped, "t", map[string]any{"reason": "checkout-gone", "sweepPending": false}),
			},
			sweepable: false,
		},
	}
	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			reduction := reduceOrFail(t, row.rows...)
			if got := reduction.Claims["t"].Sweepable(); got != row.sweepable {
				t.Fatalf("sweepable=%v, want %v", got, row.sweepable)
			}
		})
	}
}

func TestSweptIsPostTerminalOnly(t *testing.T) {
	reduction := reduceOrFail(t,
		raw(EventArming, "t", nil),
		raw(EventSwept, "t", nil),
	)
	if reduction.Claims["t"].Swept {
		t.Fatal("swept applied to an open claim")
	}
}

// D-3, SLC-R4-007: custody binds through the claim's records, and a
// release names its custodyId so one release can never hide another.
func TestCustodyBindingAndRelease(t *testing.T) {
	custodyRow := func(id string) map[string]any {
		record := raw(EventCustody, "", map[string]any{
			"custodianPid": 10.0, "custodianPidStartedAt": 20.0,
		})
		record["custodyId"] = id
		return record
	}
	releaseRow := func(id string) map[string]any {
		record := raw(EventCustodyReleased, "", nil)
		record["custodyId"] = id
		return record
	}
	reduction := reduceOrFail(t,
		custodyRow("cust-a"),
		custodyRow("cust-b"),
		raw(EventArming, "tag-a", map[string]any{"custodyId": "cust-a"}),
		armedRow("tag-a", 41),
		releaseRow("cust-b"),
	)
	a, b := reduction.Custodies["cust-a"], reduction.Custodies["cust-b"]
	if a.BoundOwnerTag != "tag-a" || a.Released {
		t.Fatalf("custody a should be bound and unreleased: %+v", a)
	}
	if b.BoundOwnerTag != "" || !b.Released {
		t.Fatalf("custody b should be unbound and released: %+v", b)
	}
}

func TestReduceFailsClosedOnInvalidRecord(t *testing.T) {
	_, err := Reduce(frames(
		raw(EventArming, "tag-a", nil),
		raw(EventExited, "tag-a", map[string]any{"reason": "not-a-reason", "teardownComplete": true}),
	))
	if err == nil {
		t.Fatal("a schema-invalid record must mark the registry corrupt (REG-5)")
	}
}

func TestOrphanRecordsAreDroppedOrTracked(t *testing.T) {
	reduction := reduceOrFail(t,
		armedRow("no-reservation", 5),
		raw(EventLaunched, "no-claim", map[string]any{
			"generation": 1.0, "component": "reaper", "pid": 6.0, "pidStartedAt": 7.0,
		}),
		raw(EventReaped, "terminal-only", map[string]any{"reason": "owner-dead", "sweepPending": true}),
	)
	if reduction.Claims["no-reservation"] != nil || reduction.Claims["no-claim"] != nil {
		t.Fatal("identity records without a reservation must be dropped")
	}
	orphan := reduction.Claims["terminal-only"]
	if orphan == nil || !orphan.Sweepable() {
		t.Fatal("a bare terminal keeps its sweep bookkeeping")
	}
}

// The reduction consumes exactly what the framing layer produces: a
// round-trip through AppendFrame and ReadFrames, not hand-built frames.
func TestReduceRoundTripThroughFraming(t *testing.T) {
	path := t.TempDir() + "/armed-checkouts.jsonl"
	rows := []map[string]any{
		raw(EventArming, "tag-rt", nil),
		armedRow("tag-rt", 51),
		raw(EventExited, "tag-rt", map[string]any{"reason": "shutdown", "teardownComplete": true}),
	}
	for _, row := range rows {
		payload, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		if err := AppendFrame(path, payload); err != nil {
			t.Fatal(err)
		}
	}
	read, err := ReadFrames(path)
	if err != nil {
		t.Fatal(err)
	}
	reduction, err := Reduce(read)
	if err != nil {
		t.Fatal(err)
	}
	claim := reduction.Claims["tag-rt"]
	if claim == nil || !claim.Closed || claim.Reason != "shutdown" {
		t.Fatalf("round trip lost the claim: %+v", claim)
	}
}

func TestReductionOrderIsFirstAppearance(t *testing.T) {
	var rows []map[string]any
	for i := 0; i < 5; i++ {
		rows = append(rows, raw(EventArming, fmt.Sprintf("tag-%d", i), nil))
	}
	reduction := reduceOrFail(t, rows...)
	for i, tag := range reduction.SortedTags() {
		if tag != fmt.Sprintf("tag-%d", i) {
			t.Fatalf("order not first-appearance: %v", reduction.SortedTags())
		}
	}
}
