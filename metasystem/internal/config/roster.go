package config

// The rosters: each names, for every one of its rows, the runtime, model and
// effort that does one kind of agent work. The rosters are fixed: three for
// goal work, one per risk tier, and one each for the seat, the landing lane
// and the Project Partner. This file holds them, their types and their rows,
// and which roster and row each dispatched role and launch kind works from.

import (
	"fmt"
	"slices"
)

type rosterKind struct {
	id, kind string
	rows     []string
}

var tierRows = []string{"design", "design-critique", "build", "code-critique", "verify", "investigate", "warden", "behavior-judge"}

// rosterKinds is the one table of rosters, their types and their rows. The
// identifiers are fixed; no other roster exists.
var rosterKinds = []rosterKind{
	{"tier-1", "tier", tierRows}, {"tier-2", "tier", tierRows}, {"tier-3", "tier", tierRows},
	{"seat", "seat", []string{"seat", "steward"}},
	{"landing", "landing", []string{"landing"}},
	{"partner", "partner", []string{"partner"}},
}

// The rows of dispatched roles and of launch kinds; each must be a row of
// the table, or RoleRow and LaunchRow answer that it has none.
var (
	roleRows   = map[string]string{"design-critic": "design-critique", "implementer": "build", "code-critic": "code-critique", "verifier": "verify", "investigator": "investigate", "warden": "warden", "behavior-judge": "behavior-judge", "steward-continuation": "steward"}
	launchRows = map[string]string{"design": "design", "critique": "design-critique", "build": "build", "read": "code-critique", "seat": "seat", "landing": "landing"}
)

// TierRoster is the roster a goal of the given risk tier works from. Work
// with no goal, or a tier outside 1 to 3, reads tier-3.
func TierRoster(tier uint8) string {
	if tier == 1 || tier == 2 {
		return fmt.Sprintf("tier-%d", tier)
	}
	return "tier-3"
}

// RoleRow is the roster and row a dispatched role works from; the
// implementer designs in mode design and builds in every other mode.
func RoleRow(role, mode string, tier uint8) (roster, row string, ok bool) {
	if role == "implementer" && mode == "design" {
		return rowOf("design", tier)
	}
	return rowOf(roleRows[role], tier)
}

// LaunchRow is the roster and row a launch kind starts its agent from.
func LaunchRow(kind string, tier uint8) (roster, row string, ok bool) {
	return rowOf(launchRows[kind], tier)
}

func rowOf(row string, tier uint8) (string, string, bool) {
	for _, kind := range rosterKinds {
		if slices.Contains(kind.rows, row) && (kind.kind != "tier" || kind.id == TierRoster(tier)) {
			return kind.id, row, true
		}
	}
	return "", "", false
}
