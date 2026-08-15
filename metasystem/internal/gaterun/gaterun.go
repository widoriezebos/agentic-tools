// Package gaterun records that a gate is running and answers whether one
// still is. A gate run is work in flight that no
// job record describes, so the turn-end report needs to know about it. It
// cannot be found by matching command lines — that answers for the whole
// machine and matches wait-loops and greps — so a gate writes a marker naming
// its process by the two kernel facts that identify it, and a reader believes
// a marker only while that exact process is alive, pruning the rest.
package gaterun

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// clock is the time source, overridable in tests.
var clock = time.Now

func markerDir(root string) string {
	return filepath.Join(root, "artifacts/agents/supervision/gate-runs")
}

// processStart returns a live pid's start second from the kernel, or ok=false
// when the pid is not a live, readable process.
func processStart(pid int64) (int64, bool) {
	exact, state, err := identity.KernelProber{}.Probe(pid)
	if err != nil || state != identity.Alive {
		return 0, false
	}
	return exact.StartedAt.Unix(), true
}

// Register records that the given process is a gate run. NONEMPTY-OR-ERROR
// (goal-system GOAL-17): an unreadable process identity is an error the
// caller must surface, never a silent success — a gate that cannot record
// its liveness must not run invisibly. The marker is written atomically
// (temp+rename) so a concurrent scan can never read, and prune, a
// half-written record.
func Register(root string, pid int64, gate string) (string, error) {
	start, ok := processStart(pid)
	if !ok {
		return "", fmt.Errorf("gate register: process identity for pid %d is unreadable; refusing an unverifiable marker", pid)
	}
	dir := markerDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%d.json", pid))
	data, err := json.Marshal(map[string]any{
		"pid":          pid,
		"pidStartedAt": start,
		"gate":         gate,
		"startedAt":    clock().UTC().Format("2006-01-02T15:04:05Z"),
	})
	if err != nil {
		return "", err
	}
	if err := atomicfile.WriteVolatile(path, string(data)+"\n"); err != nil {
		return "", err
	}
	return path, nil
}

// liveMarker is one surviving gate-run marker: a live registered process and
// the gate name it recorded.
type liveMarker struct {
	Pid          int64
	PidStartedAt int64
	Gate         string
}

// Survey is the lossless classification (goal-system GOAL-17): live
// well-formed markers return as facts; every marker the scan cannot vouch
// for surfaces in Unreadable instead of vanishing. Deletion remains
// correct ONLY for a provably dead process — an unparsable or unreadable
// marker whose process is alive (or unknowable) is a live gate the scan
// must not erase.
type SurveyResult struct {
	Live       []Marker
	Unreadable []string
}

// Marker is one live gate run, exported for the scanner.
type Marker struct {
	Pid  int64
	Gate string
}

// Survey classifies every marker in the checkout.
func Survey(root string) SurveyResult {
	var result SurveyResult
	dir := markerDir(root)
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		result.Unreadable = append(result.Unreadable, dir+": "+err.Error())
		return result
	}
	sort.Strings(paths)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			result.Unreadable = append(result.Unreadable, path+": "+err.Error())
			continue
		}
		var marker struct {
			Pid          *int64 `json:"pid"`
			PidStartedAt *int64 `json:"pidStartedAt"`
			Gate         string `json:"gate"`
		}
		if json.Unmarshal(data, &marker) != nil || marker.Pid == nil || marker.PidStartedAt == nil {
			// Register always writes <pid>.json atomically, so a file
			// whose name is not a pid was never a gate marker — prune it.
			// A <pid>.json with unparsable content is deleted only when
			// that process is provably gone; a live or unknowable owner
			// surfaces instead of vanishing.
			pidFromName, parseErr := strconv.ParseInt(strings.TrimSuffix(filepath.Base(path), ".json"), 10, 64)
			if parseErr != nil || provablyGone(pidFromName) {
				_ = os.Remove(path)
				continue
			}
			result.Unreadable = append(result.Unreadable, path+": unparsable marker for a live or unknowable process")
			continue
		}
		switch livenessOf(*marker.Pid, *marker.PidStartedAt) {
		case identity.Alive:
			result.Live = append(result.Live, Marker{Pid: *marker.Pid, Gate: marker.Gate})
		case identity.Dead:
			_ = os.Remove(path)
		default:
			result.Unreadable = append(result.Unreadable, path+": gate liveness unknown")
		}
	}
	return result
}

// livenessOf is the three-way verdict on a recorded identity.
func livenessOf(pid, start int64) identity.Liveness {
	switch err := unix.Kill(int(pid), 0); err {
	case nil, unix.EPERM:
	default:
		return identity.Dead
	}
	actual, ok := processStart(pid)
	if !ok {
		// The process exists but its start is unreadable: unknown, and
		// unknown never authorizes deletion.
		return identity.Unknown
	}
	if actual == start {
		return identity.Alive
	}
	return identity.Dead
}

// provablyGone is true only on a definitive absence.
func provablyGone(pid int64) bool {
	return unix.Kill(int(pid), 0) == unix.ESRCH
}

// liveMarkers keeps the historical live-only view Running and Fence build
// on, now riding the lossless classification.
func liveMarkers(root string) []liveMarker {
	var out []liveMarker
	for _, marker := range Survey(root).Live {
		start, ok := processStart(marker.Pid)
		if !ok {
			continue
		}
		out = append(out, liveMarker{Pid: marker.Pid, PidStartedAt: start, Gate: marker.Gate})
	}
	return out
}

// Running reports whether any gate is still running in this checkout,
// pruning markers whose processes are provably gone.
func Running(root string) bool {
	return len(liveMarkers(root)) > 0
}
