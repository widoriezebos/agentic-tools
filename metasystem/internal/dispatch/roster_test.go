package dispatch

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
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
			// The verifier's compiled default is main; a role with no entry
			// takes the default roster.
			params: RosterParams{Role: "ghost"},
			want: RosterResolution{
				Model: "sonnet", RequestedPair: "claude:sonnet",
				RosterModel: "sonnet", RosterPair: "claude:sonnet",
				RosterRuntime: "claude", Runtime: "claude",
			},
		},
		{
			// Every role's runtime defaults to auto; a seat that empties the
			// default roster has none.
			name:    "no roster anywhere refuses",
			conf:    []string{"metasystem.runtimes=codex", "role.default.runtime="},
			params:  RosterParams{Role: "ghost"},
			refusal: "role ghost has neither a runtime entry nor role.default.runtime",
		},
		{
			name: "a template placeholder from the default model refuses and names the key and the command",
			conf: []string{rosterBase,
				"role.default.runtime=codex", "role.default.model.codex=<model>"},
			params:  RosterParams{Role: "steward-continuation"},
			refusal: "role steward-continuation resolves to codex:<model>, a template placeholder from role.default.model.codex; set it with: metasystem settings set role.default.model.codex <the codex model this seat runs>, which writes ",
		},
		{
			name: "a template placeholder from the role's own model refuses by that key",
			conf: []string{rosterBase,
				"role.default.runtime=codex", "role.default.model.codex=gpt-5.6",
				"role.implementer.runtime=claude", "role.implementer.model.claude=<model>"},
			params:  RosterParams{Role: "implementer"},
			refusal: "a template placeholder from role.implementer.model.claude; set it with: metasystem settings set role.implementer.model.claude <the claude model this seat runs>, which writes ",
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
			conf:    []string{"metasystem.runtimes=claude,codex,fake", "role.implementer.runtime=fake"},
			params:  RosterParams{Role: "implementer"},
			refusal: "role implementer resolves to fake but has no model.fake value",
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

// TestResolveRosterReadsItsConfigurationLookupNotTheProcess: a request's
// configuration (a selected installation's roster) reaches the roster
// through the params' lookup, never through process environment.
func TestResolveRosterReadsItsConfigurationLookupNotTheProcess(t *testing.T) {
	t.Parallel()
	conf := writeConf(t, rosterBase, "role.code-critic.runtime=claude", "role.code-critic.model.claude=sonnet")
	overlay := map[string]string{
		"METASYSTEM_ROLE_CODE_CRITIC_RUNTIME":     "codex",
		"METASYSTEM_ROLE_CODE_CRITIC_MODEL_CODEX": "gpt-5.6-sol",
	}
	lookup := func(key string) (string, bool) { value, ok := overlay[key]; return value, ok }
	got, err := ResolveRoster(RosterParams{ConfPath: conf, Role: "code-critic", LookupEnv: lookup})
	if err != nil || got.Runtime != "codex" || got.Model != "gpt-5.6-sol" {
		t.Fatalf("roster with a configuration lookup = %+v, %v; want codex:gpt-5.6-sol", got, err)
	}
	if err := ValidateRuntimeHazardConfigurationWith(lookup, t.TempDir(), "claude", "opus", HazardDestructiveReach); err == nil {
		t.Fatal("a claude builder without maximal models passed the destructive-reach check")
	}
	overlay["METASYSTEM_RUNTIME_CLAUDE_MAXIMAL_MODELS"] = "opus"
	if err := ValidateRuntimeHazardConfigurationWith(lookup, t.TempDir(), "claude", "opus", HazardDestructiveReach); err != nil {
		t.Fatalf("the lookup's maximal models were not read: %v", err)
	}
}

// A role on auto takes the first listed runtime on the invocation's PATH and
// that runtime's own model.
func TestResolveRosterTakesAutoFromThePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := testexec.WriteFile(filepath.Join(dir, "devin"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if err := os.WriteFile(conf, []byte("# overrides only\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lookup := func(key string) (string, bool) {
		if key == "PATH" {
			return dir, true
		}
		return "", false
	}
	for role, model := range map[string]string{"implementer": "claude-opus-5-5-xhigh", "code-critic": "gpt-6-astra-xhigh", "design-critic": "gpt-6-astra-xhigh"} {
		got, err := ResolveRoster(RosterParams{ConfPath: conf, Role: role, LookupEnv: lookup})
		if err != nil || got.RosterPair != "devin:"+model || got.Runtime != "devin" {
			t.Fatalf("%s: %+v err=%v", role, got, err)
		}
	}
}
