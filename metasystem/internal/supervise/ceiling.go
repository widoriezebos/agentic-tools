package supervise

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// The watcher-ceiling derivation (review script-orchestration-04, relocated
// from arm-supervision.sh): the supervision contract's core number. Dispatch
// refuses caps at or above it and re-arm refuses ceilings below reserved
// caps — both refusals were already Go; the input they check now is too.

var ceilingValuePattern = regexp.MustCompile(`^[1-9][0-9]*$`)

// DeriveCeiling computes the watcher cap ceiling: the maximum of the
// 120-minute floor, the declared --max-cap, dispatch.cap-min,
// fence.job-cap-min, every configured cap.min.* key, and every raw
// METASYSTEM_CAP_MIN_* environment value — plus the 30-minute allowance.
// declared is 0 when absent (the verb validates its spelling). Every source
// refuses a non-positive-integer value by its own name.
func DeriveCeiling(confPath string, declared int64, environ []string) (int64, error) {
	maximum := int64(120)
	if declared > maximum {
		maximum = declared
	}
	take := func(key, value string) error {
		if !ceilingValuePattern.MatchString(value) {
			return fmt.Errorf("%s must be a positive integer", key)
		}
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("%s must be a positive integer", key)
		}
		if n > maximum {
			maximum = n
		}
		return nil
	}
	get := func(key, def string) (string, error) {
		value, _, err := config.Get(config.GetParams{
			Key: key, ConfPath: confPath, Default: def, DefaultSet: true,
		})
		return value, err
	}

	value, err := get("dispatch.cap-min", "120")
	if err != nil {
		return 0, err
	}
	if err := take("dispatch.cap-min", value); err != nil {
		return 0, err
	}
	value, err = get("fence.job-cap-min", "")
	if err != nil {
		return 0, err
	}
	if value != "" {
		if err := take("fence.job-cap-min", value); err != nil {
			return 0, err
		}
	}
	for _, key := range config.Keys(confPath, "cap.min.", environ) {
		value, err := get(key, "")
		if err != nil {
			return 0, err
		}
		if value == "" {
			continue
		}
		if err := take(key, value); err != nil {
			return 0, err
		}
	}
	for _, entry := range environ {
		name, value, ok := strings.Cut(entry, "=")
		if !ok || !strings.HasPrefix(name, "METASYSTEM_CAP_MIN_") {
			continue
		}
		if err := take(name, value); err != nil {
			return 0, err
		}
	}
	return maximum + 30, nil
}
