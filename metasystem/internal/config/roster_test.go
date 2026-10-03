package config_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/protocol"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

// A refused launch records the roster refusal's code without parsing text.
var _ refusal.Coder = (*config.RosterRefusal)(nil)

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

var installed = []string{"claude", "codex"}

// rosterHome is a registry home holding content as its rosters file, if any.
func rosterHome(t *testing.T, content string) string {
	t.Helper()
	home := t.TempDir()
	if content != "" {
		if err := errors.Join(os.MkdirAll(filepath.Join(home, "host"), 0o700), os.WriteFile(config.RostersPath(home), []byte(content), 0o600)); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func rostersJSON(rosters string) string { return `{"version": 1, "rosters": {` + rosters + `}}` }

const tierOneDesign = `"tier-1": {"type": "tier", "rows": {"design": {"runtime": "claude", "model": "claude-opus-5-5", "effort": "xhigh"}`

func wantRosterRefusal(t *testing.T, home, roster, row, code, message string, argv ...string) {
	t.Helper()
	_, err := config.RosterRow(home, roster, row, installed)
	var refused *config.RosterRefusal
	if !errors.As(err, &refused) || refused.Code != code || refused.Message != message || !slices.Equal(refused.Argv, argv) {
		t.Errorf("reading %s %s got %#v;\nwant %s %q %q", roster, row, err, code, message, argv)
	}
}

// TestRosterRowRefusesAMissingRowNamingTheCommand: an unset row is never answered empty.
func TestRosterRowRefusesAMissingRowNamingTheCommand(t *testing.T) {
	t.Parallel()
	homes := map[string]string{
		"no file":           "",
		"no such roster":    rostersJSON(`"seat": {"type": "seat", "rows": {}}`),
		"no such row":       rostersJSON(`"tier-2": {"type": "tier", "rows": {"design": {"runtime": "claude", "model": "claude-opus-5-5", "effort": "xhigh"}}}`),
		"no rosters at all": `{"version": 1}`,
		"rows left out":     rostersJSON(`"tier-2": {"type": "tier"}`),
	}
	for name, content := range homes {
		t.Run(name, func(t *testing.T) {
			home := rosterHome(t, content)
			wantRosterRefusal(t, home, "tier-2", "build", config.RosterUnset, "No agent is set for build in roster tier-2 on this computer. A person sets it with: metasystem roster set tier-2 build RUNTIME:MODEL:EFFORT. Nothing was started.", "metasystem", "roster", "set", "tier-2", "build", "RUNTIME:MODEL:EFFORT")
			wantRosterRefusal(t, home, "partner", "partner", config.RosterUnset, "No agent is set for partner in roster partner on this computer. A person sets it with: metasystem roster set partner partner RUNTIME:MODEL. Nothing was started.", "metasystem", "roster", "set", "partner", "partner", "RUNTIME:MODEL")
		})
	}
	home := rosterHome(t, rostersJSON(`"tier-2": {"type": "tier", "rows": {"build": {"runtime": "codex", "model": "gpt-5.5", "effort": "high"}}}`))
	if agent, err := config.RosterRow(home, "tier-2", "build", installed); err != nil || agent != (config.RosterAgent{Runtime: "codex", Model: "gpt-5.5", Effort: "high"}) {
		t.Errorf("RosterRow(tier-2, build) = %+v, %v; want codex gpt-5.5 high", agent, err)
	}
}

// TestRosterRowRefusesEveryReadOfAnUnusableFile: even of a sound row, naming the problem.
func TestRosterRowRefusesEveryReadOfAnUnusableFile(t *testing.T) {
	t.Parallel()
	withRow := func(row string) string { return rostersJSON(tierOneDesign + `, ` + row + `}}`) }
	cases := []struct{ name, content, reason string }{
		{"a directory, which can't be read", "", "read "},
		{"unreadable bytes", "{\"version\": 1, \xff\xfe", "it is not valid rosters JSON ("},
		{"null", " null", "it is not one JSON object"},
		{"an array", `[{"version": 1}]`, "it is not one JSON object"},
		{"a second JSON value", rostersJSON(tierOneDesign+`}}`) + ` {"version": 1}`, "it holds more after its JSON object"},
		{"trailing bytes", rostersJSON(tierOneDesign+`}}`) + "\n}", "it holds more after its JSON object"},
		{"an unknown field", `{"version": 1, "colour": "red"}`, `it is not valid rosters JSON (json: unknown field "colour")`},
		{"a key not in lower case", `{"Version": 1}`, "it writes the key Version, and keys are written in lower case: version"},
		{"a key that only folds to lower case", `{"verſion": 1}`, "it writes the key verſion, and keys are written in lower case: version"},
		{"a model not in lower case after a model", withRow(`"build": {"runtime": "claude", "model": "claude-opus-5-5", "Model": "gpt-5.5", "effort": "high"}`), "it writes the key Model, and keys are written in lower case: model"},
		{"a model twice", withRow(`"build": {"runtime": "claude", "model": "claude-opus-5-5", "model": "gpt-5.5", "effort": "high"}`), "it writes the key model twice in one object"},
		{"a missing version", `{"rosters": {` + tierOneDesign + `}}}}`, "it names no version, and this engine reads version 1"},
		{"version 2", `{"version": 2}`, "its version is 2, and this engine reads version 1"},
		{"an unknown roster", rostersJSON(tierOneDesign + `}}, "tier-4": {"type": "tier", "rows": {}}`), "tier-4 is not a roster"},
		{"an unknown row", withRow(`"deploy": {"runtime": "claude", "model": "claude-opus-5-5", "effort": "high"}`), "deploy is not a row of tier-1"},
		{"a wrong type", rostersJSON(tierOneDesign + `}}, "seat": {"type": "tier", "rows": {}}`), `roster seat has type "tier", and its type is seat`},
		{"a row without effort", withRow(`"build": {"runtime": "claude", "model": "claude-opus-5-5"}`), "tier-1 build: a row is RUNTIME:MODEL:EFFORT, and effort is low, medium, high, xhigh or max"},
		{"effort huge", withRow(`"build": {"runtime": "claude", "model": "claude-opus-5-5", "effort": "huge"}`), "tier-1 build: a row is RUNTIME:MODEL:EFFORT, and effort is low, medium, high, xhigh or max"},
		{"runtime auto", withRow(`"build": {"runtime": "auto", "model": "claude-opus-5-5", "effort": "high"}`), "tier-1 build: auto is not a runtime; a row names this computer's agent"},
		{"main in build", withRow(`"build": {"runtime": "main"}`), "tier-1 build: main is only for verify, investigate, warden and behavior-judge"},
		{"main with a model", withRow(`"verify": {"runtime": "main", "model": "claude-opus-5-5"}`), "tier-1 verify: main takes no model or effort"},
		{"a partner row with an effort", rostersJSON(tierOneDesign + `}}, "partner": {"type": "partner", "rows": {"partner": {"runtime": "claude", "model": "claude-opus-5-5", "effort": "high"}}}`), "partner partner: the Partner takes no effort; write RUNTIME:MODEL"},
		{"a partner row without a model", rostersJSON(tierOneDesign + `}}, "partner": {"type": "partner", "rows": {"partner": {"runtime": "claude"}}}`), "partner partner: the Partner's row is RUNTIME:MODEL"},
		{"a placeholder model", withRow(`"build": {"runtime": "codex", "model": "<model>", "effort": "high"}`), "tier-1 build: <model> is a template placeholder, not a model"},
		{"a broken byte in a model", withRow(`"build": {"runtime": "claude", "model": "` + "\xff" + `", "effort": "high"}`), "it is not valid UTF-8"},
	}
	for _, c := range cases {
		home := rosterHome(t, c.content)
		if c.content == "" && os.MkdirAll(config.RostersPath(home), 0o700) != nil {
			t.Fatal("the rosters file could not be made a directory")
		}
		for _, read := range [][2]string{{"tier-1", "design"}, {"seat", "seat"}} {
			_, err := config.RosterRow(home, read[0], read[1], installed)
			var refused *config.RosterRefusal
			if !errors.As(err, &refused) || refused.Code != config.RosterUnreadable || !slices.Equal(refused.Argv, []string{"metasystem", "roster", "list"}) ||
				!strings.HasPrefix(refused.Message, "The rosters file "+config.RostersPath(home)+" can't be used: "+c.reason) || !strings.HasSuffix(refused.Message, ". metasystem roster list shows what is wrong. Nothing was started.") {
				t.Errorf("%s: reading %s %s got %#v; want %s naming metasystem roster list and the reason %q", c.name, read[0], read[1], err, config.RosterUnreadable, c.reason)
			}
		}
	}
}

func TestRosterRowChecksTheRuntimeAndAnswersMainAndThePartner(t *testing.T) {
	t.Parallel()
	home := rosterHome(t, rostersJSON(`"tier-2": {"type": "tier", "rows": {"build": {"runtime": "gemini", "model": "gemini-3-pro", "effort": "high"}, "verify": {"runtime": "main"}}}, "partner": {"type": "partner", "rows": {"partner": {"runtime": "claude", "model": "claude-opus-5-5"}}}`))
	wantRosterRefusal(t, home, "tier-2", "build", config.RosterRuntime, "Roster tier-2 runs build on gemini, and this installation runs claude and codex. A person sets it with: metasystem roster set tier-2 build RUNTIME:MODEL:EFFORT. Nothing was started.", "metasystem", "roster", "set", "tier-2", "build", "RUNTIME:MODEL:EFFORT")
	for read, want := range map[[2]string]config.RosterAgent{{"tier-2", "verify"}: {Runtime: "main"}, {"partner", "partner"}: {Runtime: "claude", Model: "claude-opus-5-5"}} {
		if agent, err := config.RosterRow(home, read[0], read[1], installed); err != nil || agent != want {
			t.Errorf("RosterRow(%s, %s) = %+v, %v; want %+v", read[0], read[1], agent, err, want)
		}
	}
}
