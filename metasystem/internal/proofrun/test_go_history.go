package proofrun

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
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

type goLaunchHistory struct {
	path        string
	scope       string
	packageKeys []string
	packages    map[string]string
	keys        []string
	durations   map[string]int64
}

type goLaunchRecord struct {
	Schema    int               `json:"schema"`
	Scope     string            `json:"scope"`
	Packages  map[string]string `json:"packages"`
	Written   time.Time         `json:"written"`
	Durations map[string]int64  `json:"durations"`
}

func goHistoryDigest(value any) string {
	data, _ := json.Marshal(value)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func loadGoLaunchHistory(installation roots.Installation, group testpolicy.Group, partitions []goTestPartition, environments ...[]string) *goLaunchHistory {
	h := &goLaunchHistory{durations: map[string]int64{}, packages: map[string]string{}}
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
	data, err := os.ReadFile(h.path)
	var record goLaunchRecord
	if err != nil || json.Unmarshal(data, &record) != nil || record.Schema != 2 || record.Scope != h.scope || record.Written.IsZero() || time.Since(record.Written) < 0 || time.Since(record.Written) > 7*24*time.Hour {
		return h
	}
	for key, duration := range record.Durations {
		if duration <= 0 || record.Packages[key] == "" {
			return h
		}
	}
	if record.Durations != nil {
		h.durations = record.Durations
		h.packages = record.Packages
	}
	return h
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
	packageDurations := map[string][]int64{}
	for key, duration := range h.durations {
		largest = max(largest, duration)
		pkg := h.packages[key]
		packageDurations[pkg] = append(packageDurations[pkg], duration)
	}
	for i, key := range h.keys {
		if duration := h.durations[key]; duration > 0 {
			weights[i] = duration
		} else if durations := packageDurations[h.packageKeys[i]]; len(durations) > 0 {
			// Divide before summing to keep even large valid observations in range.
			count := int64(len(durations))
			var mean, remainder int64
			for _, duration := range durations {
				mean += duration / count
				remainder += duration % count
			}
			weights[i] = mean + remainder/count
		} else if largest > 0 {
			weights[i] = largest + min(max(1, fallback[i]), math.MaxInt64-largest)
		}
	}
	return weights
}

// Only complete, executed launches replace observations. Cached and missing
// terminals keep the prior observation. This record never supplies a result.
func (h *goLaunchHistory) save(partitions []goTestPartition, executions []PackageExecution) {
	if h == nil || h.path == "" {
		return
	}
	durations := map[string]int64{}
	packages := map[string]string{}
	for i, key := range h.keys {
		if old := h.durations[key]; old > 0 {
			durations[key] = old
		}
		var elapsed int64
		complete := true
		for _, pkg := range partitions[i].Packages {
			found := false
			for _, execution := range executions {
				if execution.Shard != i+1 || execution.Package != pkg {
					continue
				}
				if execution.Mode == PackageExecuted && execution.ElapsedMS != nil && *execution.ElapsedMS > 0 && (execution.Status == "ok" || execution.Status == "fail") {
					elapsed += *execution.ElapsedMS
					found = true
				}
			}
			complete = complete && found
		}
		if complete && elapsed > 0 {
			durations[key] = elapsed
		}
		if durations[key] > 0 {
			packages[key] = h.packageKeys[i]
		}
	}
	data, err := json.Marshal(goLaunchRecord{Schema: 2, Scope: h.scope, Written: time.Now().UTC(), Durations: durations, Packages: packages})
	if err == nil {
		_ = atomicfile.WriteVolatile(h.path, string(data)+"\n")
	}
}
