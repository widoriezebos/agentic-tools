package dispatch

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// The non-mission cap chain and the unsigned-mission-cap refusal
// (relocated from dispatch.sh). The rule and origin
// vocabularies are wire: the benchmark kit's extractor reads them from the
// cap-resolution record, so every spelling here is a contract.

var capMinPattern = regexp.MustCompile(`^[1-9][0-9]*$`)

// builtInCapMin is the last rung of the chain: no explicit argument, no
// configured key anywhere.
const builtInCapMin = "120"

func capGet(confPath, key string) (string, error) {
	value, _, err := config.Get(config.GetParams{
		Key: key, ConfPath: confPath, Default: missingSentinel, DefaultSet: true,
	})
	if err != nil {
		return "", err
	}
	return value, nil
}

// ResolveCap walks the non-mission cap chain — the explicit argument, then
// cap.min.<role>.<runtime>.<model>, cap.min.<runtime>.<model>, the same two
// alias-source rows when present, dispatch.cap-min, then the built-in floor —
// returning the cap with the rule and origin the resolution record carries.
func ResolveCap(confPath, role, runtime, model, source, requested string) (int64, string, string, error) {
	capText, rule, origin := requested, "argument", "argument"
	if requested == "" {
		steps := []struct{ key, rule string }{
			{"cap.min." + role + "." + runtime + "." + model, "config-role-pair"},
			{"cap.min." + runtime + "." + model, "config-pair"},
		}
		if source != "" {
			steps = append(steps,
				struct{ key, rule string }{"cap.min." + role + "." + runtime + "." + source, "config-role-pair-alias-source"},
				struct{ key, rule string }{"cap.min." + runtime + "." + source, "config-pair-alias-source"},
			)
		}
		steps = append(steps, struct{ key, rule string }{"dispatch.cap-min", "config-general"})
		capText, rule, origin = builtInCapMin, "built-in", "default"
		for _, step := range steps {
			value, err := capGet(confPath, step.key)
			if err != nil {
				return 0, "", "", err
			}
			if value == missingSentinel {
				continue
			}
			keyOrigin, err := config.KeyOrigin(config.GetParams{Key: step.key, ConfPath: confPath})
			if err != nil {
				return 0, "", "", err
			}
			capText, rule, origin = value, step.rule, keyOrigin
			break
		}
	}
	if !capMinPattern.MatchString(capText) {
		return 0, "", "", fmt.Errorf("dispatch cap must be a positive integer")
	}
	capMin, err := strconv.ParseInt(capText, 10, 64)
	if err != nil {
		return 0, "", "", fmt.Errorf("dispatch cap must be a positive integer")
	}
	return capMin, rule, origin, nil
}

// RefuseUnsignedMissionCap is the fence-authority decision: inside a
// mission, a cap key set through an unsigned surface (environment or the
// uncommitted .local file) must not set a mission cap — the signed mission
// fence is cap authority.
func RefuseUnsignedMissionCap(confPath, role, runtime, model, source string) error {
	keys := []string{
		"cap.min." + role + "." + runtime + "." + model,
		"cap.min." + runtime + "." + model,
	}
	if source != "" {
		keys = append(keys,
			"cap.min."+role+"."+runtime+"."+source,
			"cap.min."+runtime+"."+source,
		)
	}
	for _, key := range keys {
		origin, err := config.KeyOrigin(config.GetParams{Key: key, ConfPath: confPath})
		if err != nil {
			return err
		}
		if origin == "conf-local" || origin == "env" {
			return fmt.Errorf("mission dispatch refused: the mission fence is cap authority; unsigned %s key %s cannot set a mission cap", origin, key)
		}
	}
	return nil
}
