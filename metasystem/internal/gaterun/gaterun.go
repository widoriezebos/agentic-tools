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
	"time"

	"golang.org/x/sys/unix"

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

// alive reports whether the process identified by (pid, start) is still
// running. An unreadable start time cannot prove the recorded process died, so
// the safe answer is alive; a readable mismatch proves pid reuse.
func alive(pid, start int64) bool {
	switch err := unix.Kill(int(pid), 0); err {
	case nil:
	case unix.EPERM:
		return true
	default:
		return false
	}
	actual, ok := processStart(pid)
	if !ok {
		return true
	}
	return actual == start
}

// Register records that the given process is a gate run. It returns the marker
// path, or an empty path when the pid's start time is unreadable: a marker
// that cannot be verified later would be believed forever, so recording
// nothing is safer.
func Register(root string, pid int64, gate string) (string, error) {
	start, ok := processStart(pid)
	if !ok {
		return "", nil
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
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
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

// liveMarkers scans the checkout's gate-run markers and is the one home of
// the prune policy: a marker whose record is unparsable or whose process is
// gone is deleted; an unreadable file is left alone. Only live, well-formed
// markers return. Running and Fence both build on this scan.
func liveMarkers(root string) []liveMarker {
	paths, _ := filepath.Glob(filepath.Join(markerDir(root), "*.json"))
	sort.Strings(paths)
	var out []liveMarker
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var marker struct {
			Pid          *int64 `json:"pid"`
			PidStartedAt *int64 `json:"pidStartedAt"`
			Gate         string `json:"gate"`
		}
		if json.Unmarshal(data, &marker) != nil || marker.Pid == nil || marker.PidStartedAt == nil {
			_ = os.Remove(path)
			continue
		}
		if !alive(*marker.Pid, *marker.PidStartedAt) {
			_ = os.Remove(path)
			continue
		}
		out = append(out, liveMarker{Pid: *marker.Pid, PidStartedAt: *marker.PidStartedAt, Gate: marker.Gate})
	}
	return out
}

// Running reports whether any gate is still running in this checkout, pruning
// every marker whose process is gone or whose record is unparsable.
func Running(root string) bool {
	return len(liveMarkers(root)) > 0
}
