package registry

import (
	"testing"
	"time"
)

var compactNow = time.Date(2026, 8, 9, 21, 0, 0, 0, time.UTC)

const compactGrace = 10 * time.Minute

func compactOrFail(t *testing.T, rows ...map[string]any) []Frame {
	t.Helper()
	kept, err := CompactFrames(frames(rows...), compactNow, compactGrace)
	if err != nil {
		t.Fatalf("CompactFrames: %v", err)
	}
	return kept
}

func events(kept []Frame) []string {
	var out []string
	for _, frame := range kept {
		out = append(out, frame.Record["event"].(string))
	}
	return out
}

// SLC-R10-001, verified by round 11: a bound unreleased custody keeps
// its claim's FULL skeleton — including the terminal — even when the
// claim closed clean, so re-reducing the compacted file shows the
// claim CLOSED and the custody BOUND. No phantom open claim, no slot.
func TestBindingAndClosureSurviveCompaction(t *testing.T) {
	custody := raw(EventCustody, "", map[string]any{
		"custodianPid": 10.0, "custodianPidStartedAt": 20.0,
	})
	custody["custodyId"] = "cust-a"
	kept := compactOrFail(t,
		custody,
		raw(EventArming, "tag-a", map[string]any{"custodyId": "cust-a"}),
		armedRow("tag-a", 41),
		raw(EventExited, "tag-a", map[string]any{"reason": "shutdown", "teardownComplete": true}),
	)
	reduced, err := Reduce(kept)
	if err != nil {
		t.Fatal(err)
	}
	claim := reduced.Claims["tag-a"]
	if claim == nil || !claim.Closed || claim.Open() {
		t.Fatalf("compaction reopened a phantom claim: %+v", claim)
	}
	bound := reduced.Custodies["cust-a"]
	if bound == nil || bound.BoundOwnerTag != "tag-a" || bound.Released {
		t.Fatalf("compaction lost the binding: %+v", bound)
	}
}

// SLC-R8-002 + SLC-R9-003: generations at or below the watermark are
// dropped; everything above it survives — and since the watermark is
// contiguous by construction, an unverified older generation can only
// exist when the watermark sits below it, which retains it.
func TestWatermarkGovernsGenerationRetention(t *testing.T) {
	kept := compactOrFail(t,
		raw(EventArming, "tag-a", nil),
		armedRow("tag-a", 41),
		raw(EventRelaunched, "tag-a", map[string]any{
			"generation": 1.0, "watcherTag": "w1", "reaperTag": "r1", "retiredThrough": 0.0,
		}),
		raw(EventLaunched, "tag-a", map[string]any{
			"generation": 1.0, "component": "watcher", "pid": 42.0, "pidStartedAt": 200.0,
		}),
		raw(EventRelaunched, "tag-a", map[string]any{
			"generation": 2.0, "watcherTag": "w2", "reaperTag": "r2", "retiredThrough": 1.0,
		}),
	)
	reduced, err := Reduce(kept)
	if err != nil {
		t.Fatal(err)
	}
	claim := reduced.Claims["tag-a"]
	if claim.Generations[1] != nil {
		t.Fatal("a retired generation survived compaction")
	}
	if claim.Generations[2] == nil {
		t.Fatal("the current generation was dropped")
	}
}

func TestCleanClosedClaimsAreDropped(t *testing.T) {
	kept := compactOrFail(t,
		raw(EventArming, "tag-a", nil),
		armedRow("tag-a", 41),
		raw(EventExited, "tag-a", map[string]any{"reason": "purpose-gone", "teardownComplete": true}),
		raw(EventArming, "tag-live", nil),
	)
	reduced, err := Reduce(kept)
	if err != nil {
		t.Fatal(err)
	}
	if reduced.Claims["tag-a"] != nil {
		t.Fatal("a clean unbound closed claim survived compaction")
	}
	if reduced.Claims["tag-live"] == nil {
		t.Fatal("a live reservation was dropped")
	}
}

func TestSweepableClaimSurvivesUntilSwept(t *testing.T) {
	base := []map[string]any{
		raw(EventArming, "tag-a", nil),
		armedRow("tag-a", 41),
		raw(EventReaped, "tag-a", map[string]any{"reason": "owner-dead", "sweepPending": true}),
	}
	kept := compactOrFail(t, base...)
	reduced, _ := Reduce(kept)
	if claim := reduced.Claims["tag-a"]; claim == nil || !claim.Sweepable() {
		t.Fatal("a sweepable claim must survive compaction")
	}
	kept = compactOrFail(t, append(base, raw(EventSwept, "tag-a", nil))...)
	reduced, _ = Reduce(kept)
	if reduced.Claims["tag-a"] != nil {
		t.Fatal("a swept claim must compact away")
	}
}

func TestUnboundCustodyGraceWindow(t *testing.T) {
	fresh := raw(EventCustody, "", map[string]any{
		"custodianPid": 10.0, "custodianPidStartedAt": 20.0,
	})
	fresh["custodyId"] = "cust-fresh"
	fresh["at"] = compactNow.Add(-time.Minute).Format(time.RFC3339)
	stale := raw(EventCustody, "", map[string]any{
		"custodianPid": 11.0, "custodianPidStartedAt": 21.0,
	})
	stale["custodyId"] = "cust-stale"
	stale["at"] = compactNow.Add(-time.Hour).Format(time.RFC3339)

	kept := compactOrFail(t, fresh, stale)
	names := map[string]bool{}
	for _, frame := range kept {
		names[frame.Record["custodyId"].(string)] = true
	}
	if !names["cust-fresh"] || names["cust-stale"] {
		t.Fatalf("grace window misapplied: kept %v", names)
	}
}

func TestTornMarkersAndFragmentsCompactAway(t *testing.T) {
	torn := map[string]any{
		"schemaVersion": 1.0, "event": TornEvent, "checkoutPath": "", "at": "x",
	}
	input := frames(raw(EventArming, "tag-a", nil), torn)
	input = append(input, Frame{Line: 99, Raw: []byte("{\"frag")})
	kept, err := CompactFrames(input, compactNow, compactGrace)
	if err != nil {
		t.Fatal(err)
	}
	if got := events(kept); len(got) != 1 || got[0] != EventArming {
		t.Fatalf("compaction kept debris: %v", got)
	}
}

func TestCompactionFailsClosedOnCorruption(t *testing.T) {
	bad := raw(EventExited, "tag-a", map[string]any{"reason": "nope", "teardownComplete": true})
	if _, err := CompactFrames(frames(raw(EventArming, "tag-a", nil), bad), compactNow, compactGrace); err == nil {
		t.Fatal("compaction must refuse a corrupt registry (REG-5)")
	}
}

// End to end: growth stays bounded because separated relaunch cycles
// retire their predecessors — the healthy-operation shape of
// SLC-R5-018/SLC-R8-002.
func TestHealthyOperationCompactsToOneGeneration(t *testing.T) {
	rows := []map[string]any{raw(EventArming, "tag-a", nil), armedRow("tag-a", 41)}
	for generation := 1; generation <= 40; generation++ {
		rows = append(rows,
			raw(EventRelaunched, "tag-a", map[string]any{
				"generation": float64(generation),
				"watcherTag": "w", "reaperTag": "r",
				"retiredThrough": float64(generation - 1),
			}),
			raw(EventLaunched, "tag-a", map[string]any{
				"generation": float64(generation), "component": "watcher",
				"pid": 42.0, "pidStartedAt": 200.0,
			}),
		)
	}
	kept := compactOrFail(t, rows...)
	if len(kept) != 4 { // arming, armed, one relaunched, one launched
		t.Fatalf("healthy operation should compact to one generation, kept %d records", len(kept))
	}
}

func TestWriteCompactedRewritesAtomically(t *testing.T) {
	path := t.TempDir() + "/registry.jsonl"
	appendOrFail(t, path, `{"schemaVersion":1,"event":"arming","checkoutPath":"/r","at":"2026-08-09T20:00:00Z","ownerTag":"t"}`)
	appendOrFail(t, path, `{"schemaVersion":1,"event":"exited","checkoutPath":"/r","at":"2026-08-09T20:00:01Z","ownerTag":"t","reason":"purpose-gone","teardownComplete":true}`)
	read, err := ReadFrames(path)
	if err != nil {
		t.Fatal(err)
	}
	kept, err := CompactFrames(read, compactNow, compactGrace)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteCompacted(path, kept); err != nil {
		t.Fatal(err)
	}
	after, err := ReadFrames(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 0 {
		t.Fatalf("a clean closed claim survived the rewrite: %v", after)
	}
	appendOrFail(t, path, `{"schemaVersion":1,"event":"arming","checkoutPath":"/r","at":"2026-08-09T20:00:02Z","ownerTag":"t2"}`)
}
