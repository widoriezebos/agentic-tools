// Package pattern is the steward's behaviour patterns (design
// steward-acts-on-behaviour-patterns): each pattern is one registry row and
// one pure detector over typed signals the readers fill once per cycle. The
// lane's steward runs the pass; v1 only reports, through the steward's
// AlertEpisode lifecycle (internal/steward, pattern_episode.go).
//
// Readers and detectors decide on typed fields only: never a history
// entry's Detail, a commit subject, log text or last-tick-error
// (TestPatternReadersTypedOnly).
package pattern

import (
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// Observation, ObsKind and Evidence are the steward's lifecycle types under
// the design's names.
type (
	Observation = steward.PatternObservation
	ObsKind     = steward.ObsKind
	Evidence    = steward.AlertEvidence
)

// The four observation kinds.
const (
	Finding = steward.ObsFinding
	Clear   = steward.ObsClear
	Held    = steward.ObsHeld
	Unknown = steward.ObsUnknown
)

// Thresholds are one pattern's resolved threshold values by name.
type Thresholds map[string]string

func (t Thresholds) float(name string) float64 {
	value, err := strconv.ParseFloat(t[name], 64)
	if err != nil {
		return 0
	}
	return value
}

func (t Thresholds) int(name string) int {
	value, err := strconv.Atoi(t[name])
	if err != nil {
		return 0
	}
	return value
}

// Detector is pure: no I/O and no clock but the one it is given.
type Detector func(Signals, time.Time, Thresholds) []Observation

// Pattern is one registry row: its name, its thresholds' compiled defaults
// (config.defaults.go holds them; the row names them) and its detector.
type Pattern struct {
	Name     string
	Defaults map[string]string
	Detect   Detector
}

// ModeKey is a pattern's off|report switch.
func ModeKey(name string) string { return "steward.pattern." + name }

// ThresholdKey is one of a pattern's thresholds.
func ThresholdKey(name, threshold string) string { return ModeKey(name) + "." + threshold }

// The pass-wide settings.
const (
	ClearTicksKey = "steward.pattern.clear-ticks"
	MaxGapKey     = "steward.pattern.max-gap-sec"
)

// ModeOff and ModeReport are the v1 modes; v1 has no act mode.
const (
	ModeOff    = "off"
	ModeReport = "report"
)

func defaults(name string, thresholds ...string) map[string]string {
	values := map[string]string{}
	for _, threshold := range thresholds {
		values[threshold] = config.MustDefault(ThresholdKey(name, threshold))
	}
	return values
}

// Registry is every pattern v1 runs, in the order it runs them.
func Registry() []Pattern {
	return []Pattern{
		{Name: Stagnation, Defaults: defaults(Stagnation, "batch-hours", "repeat"), Detect: DetectStagnation},
	}
}
