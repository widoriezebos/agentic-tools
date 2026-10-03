package config_test

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/protocol"
)

// TestEveryRoleAndLaunchKindHasARow: every role the engine can dispatch and
// every launch kind that starts an agent works from a roster row, so no work
// starts on an agent nobody chose. An empty source fails, so a walk that
// finds nothing cannot pass.
func TestEveryRoleAndLaunchKindHasARow(t *testing.T) {
	t.Parallel()
	entries, err := fs.ReadDir(protocol.Files(), "roles")
	if err != nil {
		t.Fatal(err)
	}
	var roles []string
	for _, entry := range entries {
		role, ok := strings.CutSuffix(entry.Name(), ".md")
		if ok && protocol.Dispatchable(role) {
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		t.Fatal("the embedded protocol has no dispatchable role")
	}
	if len(launch.AgentKinds) == 0 {
		t.Fatal("launch.AgentKinds names no launch kind")
	}
	for tier := uint8(1); tier <= 3; tier++ {
		for _, role := range roles {
			for _, mode := range []string{"design", "build"} {
				if _, _, ok := config.RoleRow(role, mode, tier); !ok {
					t.Errorf("role %s in mode %s at tier %d works from no roster row", role, mode, tier)
				}
			}
		}
		for _, kind := range launch.AgentKinds {
			if _, _, ok := config.LaunchRow(kind, tier); !ok {
				t.Errorf("launch kind %s at tier %d works from no roster row", kind, tier)
			}
		}
	}
}

func TestRolesAndLaunchKindsAnswerTheirRow(t *testing.T) {
	t.Parallel()
	type answer struct{ roster, row string }
	tierWork := func(row string) func(uint8) answer {
		return func(tier uint8) answer { return answer{fmt.Sprintf("tier-%d", tier), row} }
	}
	fixed := func(roster, row string) func(uint8) answer {
		return func(uint8) answer { return answer{roster, row} }
	}
	roles := []struct {
		role, mode string
		want       func(uint8) answer
	}{
		{"implementer", "design", tierWork("design")},
		{"implementer", "build", tierWork("build")},
		{"implementer", "", tierWork("build")},
		{"design-critic", "design", tierWork("design-critique")},
		{"code-critic", "build", tierWork("code-critique")},
		{"verifier", "build", tierWork("verify")},
		{"investigator", "build", tierWork("investigate")},
		{"warden", "build", tierWork("warden")},
		{"behavior-judge", "build", tierWork("behavior-judge")},
		{"steward-continuation", "build", fixed("seat", "steward")},
	}
	kinds := []struct {
		kind string
		want func(uint8) answer
	}{
		{"design", tierWork("design")},
		{"critique", tierWork("design-critique")},
		{"build", tierWork("build")},
		{"read", tierWork("code-critique")},
		{"seat", fixed("seat", "seat")},
		{"landing", fixed("landing", "landing")},
	}
	for tier := uint8(1); tier <= 3; tier++ {
		for _, c := range roles {
			roster, row, ok := config.RoleRow(c.role, c.mode, tier)
			if got := (answer{roster, row}); !ok || got != c.want(tier) {
				t.Errorf("RoleRow(%q, %q, %d) = %v, %v; want %v", c.role, c.mode, tier, got, ok, c.want(tier))
			}
		}
		for _, c := range kinds {
			roster, row, ok := config.LaunchRow(c.kind, tier)
			if got := (answer{roster, row}); !ok || got != c.want(tier) {
				t.Errorf("LaunchRow(%q, %d) = %v, %v; want %v", c.kind, tier, got, ok, c.want(tier))
			}
		}
	}
	// Work outside the tier rosters reads the same roster whatever the tier.
	for _, tier := range []uint8{0, 1, 2, 3, 9} {
		if roster, row, _ := config.RoleRow("steward-continuation", "build", tier); roster != "seat" || row != "steward" {
			t.Errorf("steward-continuation at tier %d reads %s %s; want seat steward", tier, roster, row)
		}
		for _, kind := range []string{"seat", "landing"} {
			if roster, row, _ := config.LaunchRow(kind, tier); roster != kind || row != kind {
				t.Errorf("launch kind %s at tier %d reads %s %s; want %s %s", kind, tier, roster, row, kind, kind)
			}
		}
	}
	for _, role := range []string{"orchestrator", "nobody", ""} {
		if roster, row, ok := config.RoleRow(role, "build", 2); ok || roster != "" || row != "" {
			t.Errorf("RoleRow(%q) = %q, %q, %v; want not found", role, roster, row, ok)
		}
	}
	for _, kind := range []string{"proof", "nothing", ""} {
		if roster, row, ok := config.LaunchRow(kind, 2); ok || roster != "" || row != "" {
			t.Errorf("LaunchRow(%q) = %q, %q, %v; want not found", kind, roster, row, ok)
		}
	}
}

func TestTierRosterReadsTierThreeOutsideOneToThree(t *testing.T) {
	t.Parallel()
	for tier, want := range map[uint8]string{0: "tier-3", 1: "tier-1", 2: "tier-2", 3: "tier-3", 9: "tier-3"} {
		if got := config.TierRoster(tier); got != want {
			t.Errorf("TierRoster(%d) = %s; want %s", tier, got, want)
		}
	}
}
