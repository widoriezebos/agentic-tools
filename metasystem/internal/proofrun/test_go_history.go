package proofrun

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
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
	path      string
	tree      string
	keys      []string
	durations map[string]int64
}

type goLaunchRecord struct {
	Schema    int              `json:"schema"`
	Tree      string           `json:"tree"`
	Written   time.Time        `json:"written"`
	Durations map[string]int64 `json:"durations"`
}

func goHistoryDigest(value any) string {
	data, _ := json.Marshal(value)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// The source closure is known from discovery, without Git or a completed
// launch. An unreadable input disables this optimization, never the proof.
func goHistoryTree(root string, discovery goDiscovery) string {
	hash := sha256.New()
	for _, name := range discovery.Inputs {
		// Local configuration is neither a source input nor advisory evidence.
		if strings.HasSuffix(name, ".local") {
			return ""
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if os.IsNotExist(err) && name == "go.sum" {
			fmt.Fprintln(hash, "go.sum absent")
			continue
		}
		if err != nil {
			return ""
		}
		fmt.Fprintf(hash, "%s\x00%x\n", name, sha256.Sum256(data))
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func loadGoLaunchHistory(installation roots.Installation, tree string, group testpolicy.Group, partitions []goTestPartition, environments ...[]string) *goLaunchHistory {
	h := &goLaunchHistory{tree: tree, durations: map[string]int64{}}
	if installation == "" || tree == "" {
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
	// Keep variants apart: the tagged batch pass must not erase the suite.
	h.path = installation.Path("artifacts", "agents", "proof-runs", "advisory", "native-"+variant+".json")
	for _, partition := range partitions {
		packages, names := slices.Clone(partition.Packages), slices.Clone(partition.Names)
		slices.Sort(packages)
		slices.Sort(names)
		h.keys = append(h.keys, goHistoryDigest(struct {
			Packages, Names []string
			Variant         string
		}{packages, names, variant}))
	}
	data, err := os.ReadFile(h.path)
	var record goLaunchRecord
	if err != nil || json.Unmarshal(data, &record) != nil || record.Schema != 1 || record.Tree != tree || record.Written.IsZero() || time.Since(record.Written) < 0 || time.Since(record.Written) > 7*24*time.Hour {
		return h
	}
	for _, duration := range record.Durations {
		if duration <= 0 {
			return h
		}
	}
	if record.Durations != nil {
		h.durations = record.Durations
	}
	return h
}

func (h *goLaunchHistory) weights(fallback []int64) []int64 {
	weights := slices.Clone(fallback)
	if h == nil {
		return weights
	}
	for i, key := range h.keys {
		if duration := h.durations[key]; duration > 0 {
			weights[i] = duration
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
	}
	data, err := json.Marshal(goLaunchRecord{Schema: 1, Tree: h.tree, Written: time.Now().UTC(), Durations: durations})
	if err == nil {
		_ = atomicfile.WriteVolatile(h.path, string(data)+"\n")
	}
}
