package proofrun

import (
	"cmp"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/roots"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// NativeHistoryRootEnvironment carries the installation that owns advisory
// proof state into an extracted lane candidate. It grants no proof authority.
const NativeHistoryRootEnvironment = "METASYSTEM_NATIVE_HISTORY_ROOT"

const goHistoryLimit = 4096
const goHistoryMaxAge = 7 * 24 * time.Hour

type goLaunchHistory struct {
	path              string
	scope             string
	packageKeys       []string
	keys              []string
	entries           map[string]goLaunchObservation
	means             map[string]goLaunchObservation
	completeSelection bool
}

type goLaunchObservation struct {
	Duration int64     `json:"duration"`
	Package  string    `json:"package,omitempty"`
	Observed time.Time `json:"observed"`
}

type goLaunchRecord struct {
	Schema  int                            `json:"schema"`
	Scope   string                         `json:"scope"`
	Entries map[string]goLaunchObservation `json:"entries"`
	// Means are a snapshot of the latest complete selection's measured launches.
	// Narrow replays update exact entries without changing this snapshot.
	Means map[string]goLaunchObservation `json:"means"`
}

func goHistoryDigest(value any) string {
	data, _ := json.Marshal(value)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func loadGoLaunchHistory(installation roots.Installation, group testpolicy.Group, partitions []goTestPartition, environments ...[]string) *goLaunchHistory {
	h := &goLaunchHistory{}
	if installation == "" {
		return h
	}
	tags := slices.Clone(group.BuildTags)
	slices.Sort(tags)
	buildEnvironment := map[string]string{}
	for _, environment := range environments {
		for _, entry := range environment {
			name, value, _ := strings.Cut(entry, "=")
			switch name {
			case "GOOS", "GOARCH", "GOFLAGS", "GOEXPERIMENT", "CGO_ENABLED", "GOTOOLCHAIN", "GOENV", "GOAMD64", "GOARM64", "GOARM", "GOMIPS", "GOMIPS64", "GOPPC64", "GORISCV64", "GOWASM", "CC", "CXX", "CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_LDFLAGS":
				buildEnvironment[name] = value
			}
		}
	}
	variant := goHistoryDigest(struct {
		Tags           []string
		Race, Coverage bool
		OS, Arch, Go   string
		Environment    map[string]string
	}{tags, group.Race, group.Coverage, runtime.GOOS, runtime.GOARCH, runtime.Version(), buildEnvironment})
	h.scope = goHistoryDigest([]string{filepath.Clean(string(installation)), variant})
	// Keep variants apart: the tagged batch pass must not erase the suite.
	h.path = installation.Path("artifacts", "agents", "proof-runs", "advisory", "native-"+variant+".json")
	for _, partition := range partitions {
		packages, names := slices.Clone(partition.Packages), slices.Clone(partition.Names)
		slices.Sort(packages)
		slices.Sort(names)
		h.packageKeys = append(h.packageKeys, goHistoryDigest(packages))
		h.keys = append(h.keys, goHistoryDigest(struct {
			Packages, Names []string
			Variant         string
		}{packages, names, variant}))
	}
	record := h.read(time.Now())
	h.entries, h.means = record.Entries, record.Means
	return h
}

func (h *goLaunchHistory) read(now time.Time) goLaunchRecord {
	empty := goLaunchRecord{Schema: 3, Scope: h.scope, Entries: map[string]goLaunchObservation{}, Means: map[string]goLaunchObservation{}}
	file, err := os.Open(h.path)
	if err != nil {
		return empty
	}
	defer file.Close()
	// Advisory input is bounded as well as output; oversized or malformed records miss.
	data, err := io.ReadAll(io.LimitReader(file, 4<<20))
	var record goLaunchRecord
	if err != nil || json.Unmarshal(data, &record) != nil || record.Schema != 3 || record.Scope != h.scope {
		return empty
	}
	if record.Entries == nil {
		record.Entries = empty.Entries
	}
	if record.Means == nil {
		record.Means = empty.Means
	}
	for key, entry := range record.Entries {
		if entry.Package == "" {
			delete(record.Entries, key)
		}
	}
	trimGoObservations(record.Entries, now)
	trimGoObservations(record.Means, now)
	return record
}

func trimGoObservations(entries map[string]goLaunchObservation, now time.Time) {
	keys := make([]string, 0, len(entries))
	for key, entry := range entries {
		age := now.Sub(entry.Observed)
		if entry.Duration <= 0 || entry.Observed.IsZero() || age < 0 || age > goHistoryMaxAge {
			delete(entries, key)
		} else {
			keys = append(keys, key)
		}
	}
	if len(keys) <= goHistoryLimit {
		return
	}
	slices.SortFunc(keys, func(a, b string) int {
		if order := entries[b].Observed.Compare(entries[a].Observed); order != 0 {
			return order
		}
		return cmp.Compare(a, b)
	})
	for _, key := range keys[goHistoryLimit:] {
		delete(entries, key)
	}
}

func goHistoryMean(durations []int64) int64 {
	count := int64(len(durations))
	var mean, remainder int64
	for _, duration := range durations {
		mean += duration / count
		remainder += duration % count
	}
	return mean + remainder/count
}

func (h *goLaunchHistory) weights(fallback []int64) []int64 {
	weights := slices.Clone(fallback)
	if h == nil {
		return weights
	}
	// Unknown costs lead measured work, retaining the relative static order.
	// Durations and package means are milliseconds; add the static weight to
	// the largest observation so test counts cannot masquerade as durations.
	var largest int64
	for _, entry := range h.entries {
		largest = max(largest, entry.Duration)
	}
	for _, mean := range h.means {
		largest = max(largest, mean.Duration)
	}
	for i, key := range h.keys {
		if entry := h.entries[key]; entry.Duration > 0 {
			weights[i] = entry.Duration
		} else if mean := h.means[h.packageKeys[i]]; mean.Duration > 0 {
			weights[i] = mean.Duration
		} else if largest > 0 {
			weights[i] = largest + min(max(1, fallback[i]), math.MaxInt64-largest)
		}
	}
	return weights
}

// Successful executed launches replace observations. Red or cut launches keep
// their previous cost and age; cached and missing terminals do not refresh it.
// This record never supplies a result.
func (h *goLaunchHistory) save(partitions []goTestPartition, executions []PackageExecution) {
	if h == nil || h.path == "" {
		return
	}
	now := time.Now().UTC()
	// Re-read at publication so an intervening narrow run's entries survive.
	// Atomic replacement allows lost concurrent updates, never a partial record.
	record := h.read(now)
	measured := map[string][]int64{}
	incomplete := map[string]bool{}
	for i, key := range h.keys {
		var elapsed int64
		complete, green := true, true
		for _, pkg := range partitions[i].Packages {
			found := false
			for _, execution := range executions {
				if execution.Shard != i+1 || execution.Package != pkg {
					continue
				}
				if execution.Mode == PackageExecuted && execution.ElapsedMS != nil && *execution.ElapsedMS > 0 && (execution.Status == "ok" || execution.Status == "fail") {
					elapsed += min(*execution.ElapsedMS, math.MaxInt64-elapsed)
					found = true
					green = green && execution.Status == "ok"
				}
			}
			complete = complete && found
		}
		incomplete[h.packageKeys[i]] = incomplete[h.packageKeys[i]] || !complete || !green
		if !complete || elapsed <= 0 {
			continue
		}
		if !green && record.Entries[key].Duration > 0 {
			continue
		}
		record.Entries[key] = goLaunchObservation{Duration: elapsed, Package: h.packageKeys[i], Observed: now}
		measured[h.packageKeys[i]] = append(measured[h.packageKeys[i]], elapsed)
	}
	if h.completeSelection {
		// Each package uses one complete set of measured shards. An unmeasured
		// or red package retains its prior snapshot without blocking other packages.
		means := map[string]goLaunchObservation{}
		for _, pkg := range h.packageKeys {
			if incomplete[pkg] {
				if previous, ok := record.Means[pkg]; ok {
					means[pkg] = previous
				}
			} else if durations := measured[pkg]; len(durations) > 0 {
				means[pkg] = goLaunchObservation{Duration: goHistoryMean(durations), Observed: now}
			}
		}
		record.Means = means
	}
	trimGoObservations(record.Entries, now)
	trimGoObservations(record.Means, now)
	data, err := json.Marshal(record)
	if err == nil {
		_ = atomicfile.WriteVolatile(h.path, string(data)+"\n")
	}
}
