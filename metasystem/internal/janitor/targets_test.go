package janitor

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// fakeJanitorWorld programs the environment per checkout path and tag.
type fakeJanitorWorld struct {
	checkouts map[string]supervise.FileState
	liveness  map[int64]identity.Liveness
	tagged    map[string]bool
	announced map[string]bool
	expired   map[string]bool
}

func (w *fakeJanitorWorld) CheckoutState(path string) supervise.FileState {
	if state, ok := w.checkouts[path]; ok {
		return state
	}
	return supervise.Present
}
func (w *fakeJanitorWorld) OwnerLiveness(ref registry.ProcessRef) identity.Liveness {
	if state, ok := w.liveness[ref.Pid]; ok {
		return state
	}
	return identity.Alive
}
func (w *fakeJanitorWorld) LiveTagged(claim *registry.Claim) bool { return w.tagged[claim.OwnerTag] }
func (w *fakeJanitorWorld) LiveAnnouncedSession(path string) bool { return w.announced[path] }
func (w *fakeJanitorWorld) ReservationExpired(tag string) bool    { return w.expired[tag] }

func claimRows(tag, path string, pid int64, closedBy string, extra map[string]any) []map[string]any {
	rows := []map[string]any{}
	arming := raw2janitor(registry.EventArming, tag, path, nil)
	rows = append(rows, arming)
	if pid > 0 {
		rows = append(rows, raw2janitor(registry.EventArmed, tag, path, map[string]any{
			"ownerPid": float64(pid), "ownerPidStartedAt": 100.0, "generation": 1.0,
		}))
	}
	if closedBy != "" {
		rows = append(rows, raw2janitor(closedBy, tag, path, extra))
	}
	return rows
}

func raw2janitor(event, tag, path string, extra map[string]any) map[string]any {
	row := map[string]any{
		"schemaVersion": 1.0, "event": event, "checkoutPath": path,
		"at": "2026-08-10T00:00:00Z", "ownerTag": tag,
	}
	for key, value := range extra {
		row[key] = value
	}
	return row
}

func reduceRows(t *testing.T, rows ...[]map[string]any) *registry.Reduction {
	t.Helper()
	var frames []registry.Frame
	line := 0
	for _, group := range rows {
		for _, row := range group {
			line++
			frames = append(frames, registry.Frame{Line: line, Record: row})
		}
	}
	reduction, err := registry.Reduce(frames)
	if err != nil {
		t.Fatal(err)
	}
	return reduction
}

// The full D-4 order in one table: checkout-gone first, then
// owner-dead on live checkouts, orphans, sweepables, custodian-dead.
func TestSelectTargetsOrderAndGuards(t *testing.T) {
	custody := map[string]any{
		"schemaVersion": 1.0, "event": registry.EventCustody, "checkoutPath": "/live-custody",
		"at": "2026-08-10T00:00:00Z", "custodyId": "cust-a",
		"custodianPid": 900.0, "custodianPidStartedAt": 50.0,
	}
	custodiedArming := raw2janitor(registry.EventArming, "tag-custodied", "/live-custody",
		map[string]any{"custodyId": "cust-a"})

	reduction := reduceRows(t,
		claimRows("tag-gone", "/deleted", 10, "", nil),
		claimRows("tag-dead-owner", "/live", 20, "", nil),
		claimRows("tag-live", "/live", 30, "", nil),
		claimRows("tag-orphan", "/live", 0, "", nil),
		claimRows("tag-guarded-orphan", "/announced", 0, "", nil),
		claimRows("tag-sweep", "/live", 40, registry.EventReaped,
			map[string]any{"reason": "owner-dead", "sweepPending": true}),
		[]map[string]any{custody, custodiedArming},
	)
	world := &fakeJanitorWorld{
		checkouts: map[string]supervise.FileState{"/deleted": supervise.Absent},
		liveness:  map[int64]identity.Liveness{20: identity.Dead, 900: identity.Dead},
		tagged:    map[string]bool{"tag-orphan": true, "tag-guarded-orphan": true},
		announced: map[string]bool{"/announced": true},
		expired:   map[string]bool{"tag-orphan": true, "tag-guarded-orphan": true},
	}
	targets := SelectTargets(reduction, world)

	var kinds []TargetKind
	var tags []string
	for _, target := range targets {
		kinds = append(kinds, target.Kind)
		tags = append(tags, target.Claim.OwnerTag)
	}
	wantTags := []string{"tag-gone", "tag-dead-owner", "tag-orphan", "tag-sweep", "tag-custodied"}
	wantKinds := []TargetKind{CheckoutGone, OwnerDead, EstablishmentOrphan, SweepableClaim, CustodianDead}
	if len(targets) != len(wantTags) {
		t.Fatalf("selected %v (%v), want %v", tags, kinds, wantTags)
	}
	for i := range wantTags {
		if tags[i] != wantTags[i] || kinds[i] != wantKinds[i] {
			t.Fatalf("position %d: got %s/%v, want %s/%v", i, tags[i], kinds[i], wantTags[i], wantKinds[i])
		}
	}
}

// Indeterminacy never selects: unreadable checkouts and unproven
// owners are reported, not acted on (D-1's discipline in D-4).
func TestIndeterminacyNeverSelects(t *testing.T) {
	reduction := reduceRows(t,
		claimRows("tag-blind-checkout", "/unreadable", 10, "", nil),
		claimRows("tag-unknown-owner", "/live", 20, "", nil),
	)
	world := &fakeJanitorWorld{
		checkouts: map[string]supervise.FileState{"/unreadable": supervise.Indeterminate},
		liveness:  map[int64]identity.Liveness{20: identity.Unknown},
	}
	if targets := SelectTargets(reduction, world); len(targets) != 0 {
		t.Fatalf("indeterminacy selected targets: %v", targets)
	}
}

// A live announced session guards BOTH the orphan reap (SLC-R13-003)
// and the custodian-dead reap (SLC-R5-006); a dead custodian on a
// GONE checkout is the checkout-gone path's business, not custody's.
func TestCustodyGuards(t *testing.T) {
	custody := map[string]any{
		"schemaVersion": 1.0, "event": registry.EventCustody, "checkoutPath": "/gone",
		"at": "2026-08-10T00:00:00Z", "custodyId": "cust-gone",
		"custodianPid": 900.0, "custodianPidStartedAt": 50.0,
	}
	arming := raw2janitor(registry.EventArming, "tag-c", "/gone", map[string]any{"custodyId": "cust-gone"})
	reduction := reduceRows(t, []map[string]any{custody, arming})
	world := &fakeJanitorWorld{
		checkouts: map[string]supervise.FileState{"/gone": supervise.Absent},
		liveness:  map[int64]identity.Liveness{900: identity.Dead},
	}
	targets := SelectTargets(reduction, world)
	if len(targets) != 1 || targets[0].Kind != CheckoutGone {
		t.Fatalf("a dead custodian on a gone checkout must be the checkout-gone path: %v", targets)
	}
}
