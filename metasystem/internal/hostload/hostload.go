// Package hostload reads the machine's load averages and core count, the
// facts a proof attempt records at its start and end so a failure under a
// crowded box is attributable in the record rather than argued afterwards.
package hostload

import (
	"runtime"
	"time"
)

// Sample is one reading of the host: the three load averages, the core
// count they are measured against, and whether the reading was available.
type Sample struct {
	At        string  `json:"at"`
	Load1m    float64 `json:"load1m"`
	Load5m    float64 `json:"load5m"`
	Load15m   float64 `json:"load15m"`
	Cores     int     `json:"cores"`
	Available bool    `json:"available"`
	// Detail says why a reading is unavailable; empty when it is.
	Detail string `json:"detail,omitempty"`
}

// Read samples the host at now. An unreadable platform yields a sample
// that says so instead of an error: the record must always be written.
func Read(now time.Time) Sample {
	sample := Sample{At: now.UTC().Format(time.RFC3339Nano), Cores: runtime.NumCPU()}
	one, five, fifteen, err := readLoad()
	if err != nil {
		sample.Detail = err.Error()
		return sample
	}
	sample.Load1m, sample.Load5m, sample.Load15m, sample.Available = one, five, fifteen, true
	return sample
}

// Saturated reports whether the one-minute load meets or exceeds the core
// count: every core has at least one runnable thread waiting on it.
func (s Sample) Saturated() bool {
	return s.Available && s.Cores > 0 && s.Load1m >= float64(s.Cores)
}
