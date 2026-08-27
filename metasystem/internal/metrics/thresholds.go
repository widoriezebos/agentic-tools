package metrics

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

type floatBand struct {
	MinKey, MaxKey string
	MinRaw, MaxRaw string
	Min, Max       float64
	Invalid        string
}

type intMaximum struct {
	Key     string
	Raw     string
	Value   int64
	Invalid string
}

type shareLimit struct {
	Key     string
	Raw     string
	Value   float64
	Minimum bool
	Invalid string
}

type thresholds struct {
	Spend      floatBand
	Density    floatBand
	Stale      intMaximum
	ReworkItem intMaximum
	ReworkRate shareLimit
	Waiting    shareLimit
	Debt       intMaximum
	Delegates  shareLimit
	Collisions intMaximum
}

func configured(root, key, fallback string) (string, string) {
	value, _, err := config.Get(config.GetParams{
		Key: key, Default: fallback, DefaultSet: true,
		ConfPath: filepath.Join(root, "metasystem.conf"),
	})
	if err != nil {
		return "<unreadable>", err.Error()
	}
	return value, ""
}

func loadThresholds(root string) thresholds {
	band := func(minKey, minDefault, maxKey, maxDefault string) floatBand {
		b := floatBand{MinKey: minKey, MaxKey: maxKey}
		var minErr, maxErr string
		b.MinRaw, minErr = configured(root, minKey, minDefault)
		b.MaxRaw, maxErr = configured(root, maxKey, maxDefault)
		var minParseErr, maxParseErr error
		b.Min, minParseErr = strconv.ParseFloat(b.MinRaw, 64)
		b.Max, maxParseErr = strconv.ParseFloat(b.MaxRaw, 64)
		switch {
		case minErr != "":
			b.Invalid = fmt.Sprintf("%s=%s (%s)", minKey, b.MinRaw, minErr)
		case maxErr != "":
			b.Invalid = fmt.Sprintf("%s=%s (%s)", maxKey, b.MaxRaw, maxErr)
		case minParseErr != nil:
			b.Invalid = minKey + "=" + b.MinRaw
		case maxParseErr != nil:
			b.Invalid = maxKey + "=" + b.MaxRaw
		case !finiteNonnegative(b.Min):
			b.Invalid = minKey + "=" + b.MinRaw
		case !finiteNonnegative(b.Max):
			b.Invalid = maxKey + "=" + b.MaxRaw
		case b.Min > b.Max:
			b.Invalid = minKey + "=" + b.MinRaw + "," + maxKey + "=" + b.MaxRaw
		}
		return b
	}
	maximum := func(key, fallback string) intMaximum {
		m := intMaximum{Key: key}
		var getErr string
		m.Raw, getErr = configured(root, key, fallback)
		var parseErr error
		m.Value, parseErr = strconv.ParseInt(m.Raw, 10, 64)
		if getErr != "" || parseErr != nil || m.Value < 0 {
			m.Invalid = key + "=" + m.Raw
			if getErr != "" {
				m.Invalid += " (" + getErr + ")"
			}
		}
		return m
	}
	share := func(key, fallback string, minimum bool) shareLimit {
		s := shareLimit{Key: key, Minimum: minimum}
		var getErr string
		s.Raw, getErr = configured(root, key, fallback)
		var parseErr error
		s.Value, parseErr = strconv.ParseFloat(s.Raw, 64)
		if getErr != "" || parseErr != nil || math.IsNaN(s.Value) || math.IsInf(s.Value, 0) || s.Value < 0 || s.Value > 1 {
			s.Invalid = key + "=" + s.Raw
			if getErr != "" {
				s.Invalid += " (" + getErr + ")"
			}
		}
		return s
	}
	return thresholds{
		Spend:      band("metrics.overhead.spend-min", "0.25", "metrics.overhead.spend-max", "3.0"),
		Density:    band("metrics.overhead.density-min", "0.5", "metrics.overhead.density-max", "10"),
		Stale:      maximum("metrics.stale-checks.max-days", "7"),
		ReworkItem: maximum("metrics.rework.max-per-item", "3"),
		ReworkRate: share("metrics.rework.max-share", "0.5", false),
		Waiting:    share("metrics.waiting.max-share", "0.5", false),
		Debt:       maximum("metrics.debt-age.max-days", "30"),
		Delegates:  share("metrics.delegates.min-share", "0.5", true),
		Collisions: maximum("metrics.collisions.max-per-period", "3"),
	}
}

func finiteNonnegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func invalidThreshold(invalid string) string {
	return "threshold invalid: " + invalid + "; threshold disabled"
}

func (b floatBand) judgment(label string, value *float64) string {
	if b.Invalid != "" {
		return invalidThreshold(b.Invalid)
	}
	result := fmt.Sprintf("%s range=[%s,%s]", label, b.MinRaw, b.MaxRaw)
	if value == nil {
		return result + "; not evaluated"
	}
	if *value < b.Min || *value > b.Max {
		return result + "; crossed"
	}
	return result + "; within"
}

func (m intMaximum) judgment(label string, crossed bool, available bool) string {
	if m.Invalid != "" {
		return invalidThreshold(m.Invalid)
	}
	result := fmt.Sprintf("%s max=%s", label, m.Raw)
	if !available {
		return result + "; not evaluated"
	}
	if crossed {
		return result + "; crossed"
	}
	return result + "; within"
}

func (s shareLimit) judgment(label string, value *float64) string {
	if s.Invalid != "" {
		return invalidThreshold(s.Invalid)
	}
	word := "max"
	if s.Minimum {
		word = "min"
	}
	result := fmt.Sprintf("%s %s=%s", label, word, s.Raw)
	if value == nil {
		return result + "; not evaluated"
	}
	crossed := *value > s.Value
	if s.Minimum {
		crossed = *value < s.Value
	}
	if crossed {
		return result + "; crossed"
	}
	return result + "; within"
}
