package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConf(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "metasystem.conf")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const rosterBase = "metasystem.runtimes=claude,codex"

func TestResolveRosterDecisions(t *testing.T) {
	cases := []struct {
		name    string
		conf    []string
		params  RosterParams
		want    RosterResolution
		refusal string
	}{
		{
			name: "role entry wins over default",
			conf: []string{rosterBase,
				"role.implementer.runtime=codex", "role.implementer.model.codex=gpt-5.6",
				"role.default.runtime=claude", "role.default.model.claude=sonnet"},
			params: RosterParams{Role: "implementer"},
			want: RosterResolution{
				Model: "gpt-5.6", RequestedPair: "codex:gpt-5.6",
				RosterModel: "gpt-5.6", RosterPair: "codex:gpt-5.6",
				RosterRuntime: "codex", Runtime: "codex",
			},
		},
		{
			name: "default roster fills an absent role entry",
			conf: []string{rosterBase,
				"role.default.runtime=claude", "role.default.model.claude=sonnet"},
			params: RosterParams{Role: "verifier"},
			want: RosterResolution{
				Model: "sonnet", RequestedPair: "claude:sonnet",
				RosterModel: "sonnet", RosterPair: "claude:sonnet",
				RosterRuntime: "claude", Runtime: "claude",
			},
		},
		{
			name:    "no roster anywhere refuses",
			conf:    []string{rosterBase},
			params:  RosterParams{Role: "ghost"},
			refusal: "role ghost has neither a runtime entry nor role.default.runtime",
		},
		{
			name: "main roster cannot be dispatched",
			conf: []string{rosterBase, "role.retro.runtime=main"},
			params: RosterParams{
				Role: "retro",
			},
			refusal: "role retro is assigned to main and cannot be dispatched",
		},
		{
			name: "main roster with an override dispatches against current-session",
			conf: []string{rosterBase, "role.retro.runtime=main",
				"role.retro.model.claude=sonnet"},
			params: RosterParams{Role: "retro", RuntimeOverride: "claude"},
			want: RosterResolution{
				CostDirection:      "unranked (model tiers absent; overrides always escalate)",
				EscalationRequired: true,
				Model:              "sonnet", Overridden: true,
				RequestedPair: "claude:sonnet",
				RosterModel:   "<current-session>", RosterPair: "main:<current-session>",
				RosterRuntime: "main", Runtime: "claude",
			},
		},
		{
			name: "unregistered runtime refuses",
			conf: []string{"metasystem.runtimes=claude",
				"role.implementer.runtime=codex", "role.implementer.model.codex=gpt-5.6"},
			params:  RosterParams{Role: "implementer"},
			refusal: "runtime codex is outside metasystem.runtimes",
		},
		{
			name:    "missing model refuses naming the runtime",
			conf:    []string{rosterBase, "role.implementer.runtime=codex"},
			params:  RosterParams{Role: "implementer"},
			refusal: "role implementer resolves to codex but has no model.codex value",
		},
		{
			name: "override to the same pair does not escalate",
			conf: []string{rosterBase,
				"role.implementer.runtime=codex", "role.implementer.model.codex=gpt-5.6"},
			params: RosterParams{Role: "implementer", ModelOverride: "gpt-5.6"},
			want: RosterResolution{
				Model: "gpt-5.6", Overridden: true, RequestedPair: "codex:gpt-5.6",
				RosterModel: "gpt-5.6", RosterPair: "codex:gpt-5.6",
				RosterRuntime: "codex", Runtime: "codex",
			},
		},
		{
			name: "roster source aliases both resolved inputs",
			conf: []string{rosterBase,
				"role.implementer.runtime=claude", "role.implementer.model.claude=claude-fable-5",
				"runtime.claude.model-alias.claude-fable-5=claude-fable-5-1"},
			params: RosterParams{Role: "implementer"},
			want: RosterResolution{
				AliasedFrom: "claude-fable-5", Model: "claude-fable-5-1",
				RequestedPair:     "claude:claude-fable-5-1",
				RosterAliasedFrom: "claude-fable-5", RosterModel: "claude-fable-5-1",
				RosterPair: "claude:claude-fable-5-1", RosterRuntime: "claude", Runtime: "claude",
			},
		},
		{
			name: "source override aliases to the roster target without escalation",
			conf: []string{rosterBase,
				"role.implementer.runtime=claude", "role.implementer.model.claude=claude-fable-5-1",
				"runtime.claude.model-alias.claude-fable-5=claude-fable-5-1"},
			params: RosterParams{Role: "implementer", ModelOverride: "claude-fable-5"},
			want: RosterResolution{
				AliasedFrom: "claude-fable-5", Model: "claude-fable-5-1", Overridden: true,
				RequestedPair: "claude:claude-fable-5-1", RosterModel: "claude-fable-5-1",
				RosterPair: "claude:claude-fable-5-1", RosterRuntime: "claude", Runtime: "claude",
			},
		},
		{
			name: "roster and override preserve distinct alias sources",
			conf: []string{rosterBase,
				"role.implementer.runtime=claude", "role.implementer.model.claude=family-a",
				"runtime.claude.model-alias.family-a=target", "runtime.claude.model-alias.family-b=target"},
			params: RosterParams{Role: "implementer", ModelOverride: "family-b"},
			want: RosterResolution{
				AliasedFrom: "family-b", Model: "target", Overridden: true,
				RequestedPair: "claude:target", RosterAliasedFrom: "family-a", RosterModel: "target",
				RosterPair: "claude:target", RosterRuntime: "claude", Runtime: "claude",
			},
		},
		{
			name: "target override clears effective alias provenance but retains roster provenance",
			conf: []string{rosterBase,
				"role.implementer.runtime=claude", "role.implementer.model.claude=family-a",
				"runtime.claude.model-alias.family-a=target"},
			params: RosterParams{Role: "implementer", ModelOverride: "target"},
			want: RosterResolution{
				Model: "target", Overridden: true, RequestedPair: "claude:target",
				RosterAliasedFrom: "family-a", RosterModel: "target", RosterPair: "claude:target",
				RosterRuntime: "claude", Runtime: "claude",
			},
		},
		{
			name: "higher tier escalates with the wording",
			conf: []string{rosterBase,
				"role.implementer.runtime=claude", "role.implementer.model.claude=sonnet",
				"role.implementer.model.codex=gpt-5.6",
				"model.tier.1=claude:sonnet", "model.tier.2=codex:gpt-5.6"},
			params: RosterParams{Role: "implementer", RuntimeOverride: "codex"},
			want: RosterResolution{
				CostDirection:      "higher (tier 1 -> tier 2)",
				EscalationRequired: true,
				Model:              "gpt-5.6", Overridden: true,
				RequestedPair: "codex:gpt-5.6",
				RosterModel:   "sonnet", RosterPair: "claude:sonnet",
				RosterRuntime: "claude", Runtime: "codex",
				TiersPresent: true,
			},
		},
		{
			name: "lower tier passes without escalation",
			conf: []string{rosterBase,
				"role.implementer.runtime=codex", "role.implementer.model.codex=gpt-5.6",
				"role.implementer.model.claude=sonnet",
				"model.tier.1=claude:sonnet", "model.tier.2=codex:gpt-5.6"},
			params: RosterParams{Role: "implementer", RuntimeOverride: "claude"},
			want: RosterResolution{
				Model: "sonnet", Overridden: true, RequestedPair: "claude:sonnet",
				RosterModel: "gpt-5.6", RosterPair: "codex:gpt-5.6",
				RosterRuntime: "codex", Runtime: "claude",
				TiersPresent: true,
			},
		},
		{
			name: "unranked pair escalates with the wording",
			conf: []string{rosterBase,
				"role.implementer.runtime=claude", "role.implementer.model.claude=sonnet",
				"role.implementer.model.codex=gpt-5.6",
				"model.tier.1=claude:sonnet"},
			params: RosterParams{Role: "implementer", RuntimeOverride: "codex"},
			want: RosterResolution{
				CostDirection:      "unranked (one or both resolved pairs are absent from model.tier.*)",
				EscalationRequired: true,
				Model:              "gpt-5.6", Overridden: true,
				RequestedPair: "codex:gpt-5.6",
				RosterModel:   "sonnet", RosterPair: "claude:sonnet",
				RosterRuntime: "claude", Runtime: "codex",
				TiersPresent: true,
			},
		},
		{
			name: "ambiguous rank counts as unranked",
			conf: []string{rosterBase,
				"role.implementer.runtime=claude", "role.implementer.model.claude=sonnet",
				"role.implementer.model.codex=gpt-5.6",
				"model.tier.1=claude:sonnet,codex:gpt-5.6", "model.tier.2=codex:gpt-5.6"},
			params: RosterParams{Role: "implementer", RuntimeOverride: "codex"},
			want: RosterResolution{
				CostDirection:      "unranked (one or both resolved pairs are absent from model.tier.*)",
				EscalationRequired: true,
				Model:              "gpt-5.6", Overridden: true,
				RequestedPair: "codex:gpt-5.6",
				RosterModel:   "sonnet", RosterPair: "claude:sonnet",
				RosterRuntime: "claude", Runtime: "codex",
				TiersPresent: true,
			},
		},
		{
			name: "tier gap refuses by index",
			conf: []string{rosterBase,
				"role.implementer.runtime=claude", "role.implementer.model.claude=sonnet",
				"role.implementer.model.codex=gpt-5.6",
				"model.tier.1=claude:sonnet", "model.tier.3=codex:gpt-5.6"},
			params:  RosterParams{Role: "implementer", RuntimeOverride: "codex"},
			refusal: "model tiers must be contiguous from 1: found index 3 where 2 was expected",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.params.ConfPath = writeConf(t, c.conf...)
			got, err := ResolveRoster(c.params)
			if c.refusal != "" {
				if err == nil || !strings.Contains(err.Error(), c.refusal) {
					t.Fatalf("want refusal containing %q, got %+v err=%v", c.refusal, got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected refusal: %v", err)
			}
			if got != c.want {
				t.Fatalf("resolution mismatch:\n got %+v\nwant %+v", got, c.want)
			}
		})
	}
}
