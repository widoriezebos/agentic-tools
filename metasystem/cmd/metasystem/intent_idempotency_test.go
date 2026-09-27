package main

// Acts are idempotent (R-129-ui, Wido 2026-09-27: "We should have idempotent
// actions. So if we pause an already paused goal, that should be fine. That's
// a principle that should go for all verbs."). Unit U-idem.
//
// Every public (object, action) has exactly one idempotency row, registered
// by the area file that owns its object (intent_idempotency_*_test.go):
//
//   - read: the action reads or waits and changes nothing, so a repeat is
//     idempotent by nature;
//   - creation: every call makes a new thing (a receipt, a build attempt, a
//     review round), so a second call is not a repeat; the row says why;
//   - stateful: the action sets a state, and its repeat, when that state
//     already holds, is success with no second record. The row carries a
//     witness that runs the action twice, through the router or through the
//     owner where the router needs a real terminal, and asserts exit 0 and
//     that the second run recorded nothing.
//
// The ratchet: a public action without a row, a row for no public action, a
// stateful row without a witness, or a creation without its reason fails.

import (
	"sort"
	"strings"
	"testing"
)

type idempotencyKind string

const (
	idemRead     idempotencyKind = "read"
	idemCreation idempotencyKind = "creation"
	idemStateful idempotencyKind = "stateful"
)

// idempotencyRow is one public action's idempotency: its kind, why (for a
// creation, why a second call is not a repeat; for a stateful action, what
// its repeat answers), and, for a stateful action, its witness.
type idempotencyRow struct {
	kind    idempotencyKind
	why     string
	witness func(*testing.T)
}

var idempotencyRows = map[string]idempotencyRow{}

// registerIdempotency records one action's row; the area files call it from
// init. A second registration of one action is a test failure, found by the
// ratchet.
func registerIdempotency(name string, kind idempotencyKind, why string, witness func(*testing.T)) {
	if _, twice := idempotencyRows[name]; twice {
		idempotencyDuplicates = append(idempotencyDuplicates, name)
	}
	idempotencyRows[name] = idempotencyRow{kind: kind, why: why, witness: witness}
}

var idempotencyDuplicates []string

func TestEveryPublicActionHasAnIdempotencyRow(t *testing.T) {
	t.Parallel()
	if len(idempotencyDuplicates) > 0 {
		t.Errorf("actions registered twice: %v", idempotencyDuplicates)
	}
	public := map[string]bool{}
	for _, command := range publicIntentCommands() {
		public[command.name] = true
		row, found := idempotencyRows[command.name]
		switch {
		case !found:
			t.Errorf("%q has no idempotency row: register it as read, creation (with why a second call is not a repeat) or stateful (with a witness that runs it twice) in an intent_idempotency_*_test.go file", command.name)
		case row.kind == idemStateful && row.witness == nil:
			t.Errorf("%q is stateful but has no witness that runs it twice", command.name)
		case row.kind == idemCreation && strings.TrimSpace(row.why) == "":
			t.Errorf("%q is a creation without the reason a second call is not a repeat", command.name)
		case row.kind != idemRead && row.kind != idemCreation && row.kind != idemStateful:
			t.Errorf("%q has an unknown idempotency kind %q", command.name, row.kind)
		}
	}
	var stale []string
	for name := range idempotencyRows {
		if !public[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("idempotency rows for no public action: %v", stale)
	}
}

// TestEveryStatefulActionRepeatsAsSuccess runs every stateful action's witness.
func TestEveryStatefulActionRepeatsAsSuccess(t *testing.T) {
	t.Parallel()
	var names []string
	for name, row := range idempotencyRows {
		if row.kind == idemStateful && row.witness != nil {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		witness := idempotencyRows[name].witness
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			t.Parallel()
			witness(t)
		})
	}
}
