// Package audit holds the kill-shell program's mechanical fences
// (plans/kill-shell.md): decisions the gate consults between steps, never
// gate sequencing — the bootstrap owns that, because Go never launches the
// toolchain and no trustworthy binary exists before the first build.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// CoverageBaseline is the checked-in ratchet: per-package floors that only
// ever rise, by deliberate commit.
type CoverageBaseline struct {
	Note   string             `json:"note"`
	Exempt map[string]string  `json:"exempt"`
	Floors map[string]float64 `json:"floors"`
}

// ReadCoverageBaseline loads and validates the ratchet file.
func ReadCoverageBaseline(path string) (*CoverageBaseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("coverage ratchet baseline unreadable: %w", err)
	}
	var baseline CoverageBaseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("coverage ratchet baseline unparsable: %w", err)
	}
	if len(baseline.Floors) == 0 {
		return nil, fmt.Errorf("coverage ratchet baseline names no floors")
	}
	for pkg, floor := range baseline.Floors {
		if floor < 0 || floor > 100 {
			return nil, fmt.Errorf("coverage ratchet floor out of range: %s=%v", pkg, floor)
		}
	}
	return &baseline, nil
}

var coverageLineRe = regexp.MustCompile(`^(ok|---)?\s*(\S+)\s.*coverage:\s+([0-9.]+)% of statements`)

// ParseCoverage extracts per-package coverage from `go test -cover` output,
// stripping the module prefix so packages match the baseline's keys.
func ParseCoverage(output, modulePrefix string) map[string]float64 {
	results := map[string]float64{}
	for _, line := range strings.Split(output, "\n") {
		match := coverageLineRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		pct, err := strconv.ParseFloat(match[3], 64)
		if err != nil {
			continue
		}
		pkg := strings.TrimPrefix(match[2], modulePrefix)
		results[pkg] = pct
	}
	return results
}

// CheckCoverage judges measured coverage against the ratchet. Every measured
// package must be at or above its floor; every measured package must be
// KNOWN (in floors or exempt) — a new package must register a floor, or the
// ratchet would silently ignore it; every floored package must have been
// measured — losing sight of a package never counts as passing it; and
// every package in the independent inventory must be measured or exempt —
// `go test` prints no coverage value for a package with no test files, so
// without the inventory join a brand-new testless package is invisible
// (go-production-grade B8). A nil inventory skips only that last join, for
// callers that genuinely have no package list.
func CheckCoverage(baseline *CoverageBaseline, measured map[string]float64, inventory []string) []string {
	var violations []string
	for _, pkg := range inventory {
		if _, present := measured[pkg]; present {
			continue
		}
		if _, exempt := baseline.Exempt[pkg]; exempt {
			continue
		}
		violations = append(violations, fmt.Sprintf(
			"package %s is in the module but produced no coverage (no test files?); add tests and a floor, or record an exemption", pkg))
	}
	for _, pkg := range sortedPackageKeys(measured) {
		pct := measured[pkg]
		floor, known := baseline.Floors[pkg]
		if !known {
			if _, exempt := baseline.Exempt[pkg]; exempt {
				continue
			}
			violations = append(violations, fmt.Sprintf(
				"package %s (%.1f%%) has no ratchet floor; register it", pkg, pct))
			continue
		}
		if pct < floor {
			violations = append(violations, fmt.Sprintf(
				"package %s coverage %.1f%% is below its ratchet floor %.1f%%", pkg, pct, floor))
		}
	}
	for _, pkg := range sortedPackageKeys(baseline.Floors) {
		if _, present := measured[pkg]; !present {
			violations = append(violations, fmt.Sprintf(
				"floored package %s was not measured; losing sight never passes", pkg))
		}
	}
	return violations
}

func sortedPackageKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
