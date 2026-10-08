package processmeasure

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
)

type Drift struct {
	Stops   []loopstop.Stop
	Bands   map[string]string
	Unknown []string
}

// Decide compares recorded cost with declared allowances; missing evidence is unknown.
func Decide(subject string, m Measures, checkMinutes *float64) Drift {
	d := Drift{Bands: map[string]string{}}
	var elapsed, checks *float64
	if m.ElapsedHours != nil {
		minutes := *m.ElapsedHours * 60
		elapsed = &minutes
	}
	if m.Hours["attest"] != nil {
		minutes := *m.Hours["attest"] * 60
		checks = &minutes
	}
	var elapsedLimit *float64
	if m.EstimateMinutes != nil {
		limit := 2 * *m.EstimateMinutes
		elapsedLimit = &limit
	}
	zero := 0.0
	for _, band := range []struct {
		name       string
		now, limit *float64
	}{
		{"unit elapsed minutes", elapsed, elapsedLimit},
		{"unit check minutes", checks, checkMinutes},
		{"unit full-suite minutes", m.SuiteMinutes, &zero},
	} {
		d.Bands[band.name] = "unknown"
		if band.now == nil || band.limit == nil {
			continue
		}
		d.Bands[band.name] = "in band"
		if *band.now <= *band.limit {
			continue
		}
		d.Bands[band.name] = "above band"
		s := loopstop.Stop{Loop: "process", Subject: subject + "/" + band.name, Attempt: 1, Budget: 1, Class: band.name + " above allowance", Cause: &loopstop.Cause{Kind: "unclassified"}, At: m.ObservedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")}
		s.Measure.Name, s.Measure.Previous, s.Measure.Now = band.name, []string{fmt.Sprint(*band.limit)}, []string{fmt.Sprint(*band.now)}
		d.Stops = append(d.Stops, loopstop.Decide(loopstop.Input{Stop: s}))
	}
	return d
}
