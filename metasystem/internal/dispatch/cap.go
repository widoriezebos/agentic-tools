package dispatch

import (
	"errors"
	"fmt"
	"os"
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
const dispatchCapMaxKey = "dispatch.cap-max"

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
// cap.min.<role>.<runtime>.<model>, cap.min.<runtime>.<model>,
// dispatch.cap-min, then the built-in floor — returning the cap with the
// rule and origin the resolution record carries.
func ResolveCap(confPath, role, runtime, model, requested string) (int64, string, string, error) {
	capText, rule, origin := requested, "argument", "argument"
	if requested == "" {
		steps := []struct{ key, rule string }{
			{"cap.min." + role + "." + runtime + "." + model, "config-role-pair"},
			{"cap.min." + runtime + "." + model, "config-pair"},
			{"dispatch.cap-min", "config-general"},
		}
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
	maxText, _, err := config.Get(config.GetParams{Key: dispatchCapMaxKey, ConfPath: confPath, Default: builtInCapMin, DefaultSet: true})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			maxText = builtInCapMin
		} else {
			return 0, "", "", err
		}
	}
	if !capMinPattern.MatchString(maxText) {
		return 0, "", "", fmt.Errorf("%s must be a positive integer", dispatchCapMaxKey)
	}
	maximum, err := strconv.ParseInt(maxText, 10, 64)
	if err != nil {
		return 0, "", "", fmt.Errorf("%s must be a positive integer", dispatchCapMaxKey)
	}
	if capMin > maximum {
		return 0, "", "", fmt.Errorf("dispatch cap %d minutes exceeds %s=%d", capMin, dispatchCapMaxKey, maximum)
	}
	return capMin, rule, origin, nil
}

// RefuseUnsignedMissionCap is the fence-authority decision: inside a
// mission, a cap key set through an unsigned surface (environment or the
// uncommitted .local file) must not set a mission cap — the signed mission
// fence is cap authority.
func RefuseUnsignedMissionCap(confPath, role, runtime, model string) error {
	for _, key := range []string{
		"cap.min." + role + "." + runtime + "." + model,
		"cap.min." + runtime + "." + model,
	} {
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
