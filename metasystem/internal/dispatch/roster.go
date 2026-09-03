package dispatch

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// Roster resolution, tier ranking, and escalation classification
// (relocated from dispatch.sh): which runtime:model
// pair a role's roster names, which pair the caller effectively requested,
// and whether honoring the request needs escalation approval. The shell
// keeps only the approval ladder (TTY confirm, signed envelope, refusal
// texts) over this verb's result.

// RosterParams is one resolution request.
type RosterParams struct {
	ConfPath        string
	Role            string
	Mode            string
	RuntimeOverride string
	ModelOverride   string
}

// RosterResolution is the decision dispatch consumes. Field order is the
// wire order: `job resolve-roster` marshals this struct directly.
type RosterResolution struct {
	AliasedFrom        string `json:"aliasedFrom"`
	CostDirection      string `json:"costDirection"`
	EscalationRequired bool   `json:"escalationRequired"`
	Model              string `json:"model"`
	Overridden         bool   `json:"overridden"`
	RequestedPair      string `json:"requestedPair"`
	RosterAliasedFrom  string `json:"rosterAliasedFrom"`
	RosterModel        string `json:"rosterModel"`
	RosterPair         string `json:"rosterPair"`
	RosterRuntime      string `json:"rosterRuntime"`
	Runtime            string `json:"runtime"`
	TiersPresent       bool   `json:"tiersPresent"`
}

// missingSentinel mirrors the shell's `--default __missing__` probe: the
// resolver cannot distinguish "unset" from "set to the default" any other
// way, and the sentinel's survival into later key names (a roster-less role
// dispatched with --runtime) is inherited behavior kept deliberately.
const missingSentinel = "__missing__"

func rosterGet(conf, key, mode string) (string, error) {
	value, _, err := config.Get(config.GetParams{
		Key: key, Mode: mode, ConfPath: conf,
		Default: missingSentinel, DefaultSet: true,
	})
	if err != nil {
		return "", err
	}
	return value, nil
}

func splitCSV(value string) []string {
	var out []string
	for _, entry := range strings.Split(value, ",") {
		out = append(out, strings.TrimSpace(entry))
	}
	return out
}

// modelTier ranks one runtime:model pair: the index of the single tier
// naming it, 999999 when absent or ambiguous. Enumeration walks from 1 and
// stops at the first missing index — validation guarantees contiguity, and
// ResolveRoster re-checks it before ranking.
func modelTier(conf, wanted string) (int, error) {
	rank, found := 999999, 0
	for index := 1; ; index++ {
		value, err := rosterGet(conf, fmt.Sprintf("model.tier.%d", index), "")
		if err != nil {
			return 0, err
		}
		if value == missingSentinel || value == "" {
			break
		}
		for _, entry := range splitCSV(value) {
			if entry == wanted {
				found++
				rank = index
			}
		}
	}
	if found == 1 {
		return rank, nil
	}
	return 999999, nil
}

// tierIndices enumerates the configured model.tier.N indices from the
// merged config the resolver actually reads, sorted ascending.
func tierIndices(conf string) []int {
	var indices []int
	for _, key := range config.Keys(conf, "model.tier.", os.Environ()) {
		suffix := strings.TrimPrefix(key, "model.tier.")
		n, err := strconv.Atoi(suffix)
		if err == nil && n >= 1 && suffix == strconv.Itoa(n) {
			indices = append(indices, n)
		}
	}
	sort.Ints(indices)
	return indices
}

// ResolveRoster resolves a role's roster pair and the effective request,
// classifying whether the request escalates. Refusal texts are the wire the
// shell relays verbatim.
func ResolveRoster(p RosterParams) (RosterResolution, error) {
	get := func(key string) (string, error) { return rosterGet(p.ConfPath, key, p.Mode) }

	rosterRuntime, err := get("role." + p.Role + ".runtime")
	if err != nil {
		return RosterResolution{}, err
	}
	if rosterRuntime == missingSentinel {
		if rosterRuntime, err = get("role.default.runtime"); err != nil {
			return RosterResolution{}, err
		}
	}
	runtime := p.RuntimeOverride
	if runtime == "" {
		runtime = rosterRuntime
	}
	if runtime == missingSentinel || runtime == "" {
		return RosterResolution{}, fmt.Errorf("role %s has neither a runtime entry nor role.default.runtime", p.Role)
	}
	if runtime == "main" {
		return RosterResolution{}, fmt.Errorf("role %s is assigned to main and cannot be dispatched", p.Role)
	}
	registered, err := rosterGet(p.ConfPath, "metasystem.runtimes", "")
	if err != nil {
		return RosterResolution{}, err
	}
	if registered == missingSentinel {
		registered = ""
	}
	inRoster := false
	for _, entry := range splitCSV(registered) {
		if entry != "" && entry == runtime {
			inRoster = true
		}
	}
	if !inRoster {
		return RosterResolution{}, fmt.Errorf("runtime %s is outside metasystem.runtimes", runtime)
	}

	var rosterModel, rosterInput string
	var rosterAliased bool
	if rosterRuntime == "main" {
		rosterModel = "<current-session>"
	} else {
		if rosterModel, err = get("role." + p.Role + ".model." + rosterRuntime); err != nil {
			return RosterResolution{}, err
		}
		if rosterModel == missingSentinel {
			if rosterModel, err = get("role.default.model." + rosterRuntime); err != nil {
				return RosterResolution{}, err
			}
		}
		if rosterModel == missingSentinel {
			return RosterResolution{}, fmt.Errorf("role %s resolves to %s but has no model.%s value", p.Role, rosterRuntime, rosterRuntime)
		}
		rosterInput = rosterModel
		rosterModel, rosterAliased, err = config.ResolveModelAlias(p.ConfPath, rosterRuntime, rosterModel)
		if err != nil {
			return RosterResolution{}, err
		}
	}

	requestedModel, err := get("role." + p.Role + ".model." + runtime)
	if err != nil {
		return RosterResolution{}, err
	}
	if requestedModel == missingSentinel {
		if requestedModel, err = get("role.default.model." + runtime); err != nil {
			return RosterResolution{}, err
		}
	}
	if requestedModel == missingSentinel {
		return RosterResolution{}, fmt.Errorf("role %s resolves to %s but has no model.%s value", p.Role, runtime, runtime)
	}
	requestedInput := requestedModel
	requestedModel, requestedAliased, err := config.ResolveModelAlias(p.ConfPath, runtime, requestedModel)
	if err != nil {
		return RosterResolution{}, err
	}
	effectiveInput := requestedInput
	model := requestedModel
	effectiveAliased := requestedAliased
	if p.ModelOverride != "" {
		effectiveInput = p.ModelOverride
		model, effectiveAliased, err = config.ResolveModelAlias(p.ConfPath, runtime, p.ModelOverride)
		if err != nil {
			return RosterResolution{}, err
		}
	}
	rosterAliasedFrom := ""
	if rosterAliased {
		rosterAliasedFrom = rosterInput
	}
	aliasedFrom := ""
	if effectiveAliased {
		aliasedFrom = effectiveInput
	}

	out := RosterResolution{
		AliasedFrom:       aliasedFrom,
		Model:             model,
		Overridden:        p.RuntimeOverride != "" || p.ModelOverride != "",
		RequestedPair:     runtime + ":" + model,
		RosterAliasedFrom: rosterAliasedFrom,
		RosterModel:       rosterModel,
		RosterPair:        rosterRuntime + ":" + rosterModel,
		RosterRuntime:     rosterRuntime,
		Runtime:           runtime,
	}
	if !out.Overridden || out.RequestedPair == out.RosterPair {
		return out, nil
	}
	indices := tierIndices(p.ConfPath)
	if len(indices) == 0 {
		out.EscalationRequired = true
		out.CostDirection = "unranked (model tiers absent; overrides always escalate)"
		return out, nil
	}
	// A gap is a config error, not a truncation: ranking stops at the first
	// missing index, so a gap would silently drop every tier above it.
	expected := 1
	for _, index := range indices {
		if index != expected {
			return RosterResolution{}, fmt.Errorf("model tiers must be contiguous from 1: found index %d where %d was expected (a gap would be silently ignored during ranking)", index, expected)
		}
		expected++
	}
	out.TiersPresent = true
	rosterTier, err := modelTier(p.ConfPath, out.RosterPair)
	if err != nil {
		return RosterResolution{}, err
	}
	requestedTier, err := modelTier(p.ConfPath, out.RequestedPair)
	if err != nil {
		return RosterResolution{}, err
	}
	switch {
	case rosterTier == 999999 || requestedTier == 999999:
		out.EscalationRequired = true
		out.CostDirection = "unranked (one or both resolved pairs are absent from model.tier.*)"
	case requestedTier > rosterTier:
		out.EscalationRequired = true
		out.CostDirection = fmt.Sprintf("higher (tier %d -> tier %d)", rosterTier, requestedTier)
	}
	return out, nil
}
