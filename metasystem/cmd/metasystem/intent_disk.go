package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// diskOwners are the seams of the disk verbs; the zero value is
// production: the user's cache dir, the state beside the machine registry,
// the wall clock.
type diskOwners struct {
	userCacheDir func() (string, error)
	stateDir     string
	now          func() time.Time
}

func diskIntentCommands() []intentCommand {
	return []intentCommand{{
		object: "disk", action: "clean", audience: "both", summary: "trim what MetaSystem keeps on this computer's disk to its caps, now",
		usage: []string{"metasystem disk clean [--go-cache]"},
		details: []string{
			"Runs the steward's cache trim now: the engine's Go build cache (Go's default cache, which every plain `go build` on this computer shares), the delegates' cache and both staticcheck caches, each to its cap (disk.go-cache-cap-gib, disk.delegate-go-cache-cap-gib, disk.staticcheck-cache-cap-gib), least recently used first.",
			"Nothing used within disk.go-cache-keep-hours is ever deleted, so a build that is running keeps what it reads; a pass ends within disk.cache-trim-budget-sec and the next resumes where it ended. Another trim running is reported and is success.",
			"--go-cache names the cache trim, which is the whole pass today. A repeat with nothing over its cap removes nothing.",
		},
		flags:    []intentFlag{{name: "go-cache", usage: "trim the Go and staticcheck caches to their caps"}},
		maxArgs:  0,
		examples: []string{"metasystem disk clean --go-cache"},
		run:      runIntentDiskClean,
	}}
}

func runIntentDiskClean(inv *intentInvocation) int {
	if len(inv.input.args) != 0 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "disk clean takes no argument: metasystem disk clean [--go-cache]; nothing was done"})
	}
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	owners := inv.owners.disk
	now := owners.now
	if now == nil {
		now = time.Now
	}
	started := now()
	reports, err := steward.TrimMachineCaches(context.Background(), inv.layout.InstallationRoot, owners.userCacheDir, owners.stateDir, started, now, nil)
	if err != nil {
		summary := "disk clean: " + err.Error()
		if strings.Contains(err.Error(), "disk.") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: summary + "; nothing was trimmed",
				Decision: "fix the disk setting with metasystem settings set, then run disk clean again"})
		}
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: summary})
	}
	var lines []string
	removed := 0
	var freed int64
	for _, report := range reports {
		removed += report.EntriesRemoved
		freed += report.BytesRemoved
		lines = append(lines, diskTrimLine(report))
	}
	summary := fmt.Sprintf("trimmed the machine caches: %d entries, %s freed", removed, diskBytes(freed))
	if removed == 0 {
		summary = "the machine caches are within their caps: nothing removed"
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: summary, text: lines, Data: map[string]any{"caches": reports}})
}

// diskTrimLine is one cache's line: its size against its cap, what went,
// and how the pass ended.
func diskTrimLine(report gocache.TrimReport) string {
	line := fmt.Sprintf("%s: %s", report.Cache, report.EndedBy)
	switch report.EndedBy {
	case "absent":
		return line + " (" + report.Root + " does not exist)"
	case "lock-held", "refused":
		return line + ": " + report.Reason
	}
	if report.Phase == "measure" {
		return line + fmt.Sprintf(", measuring (%s counted so far; the next pass resumes at shard %s)", diskBytes(report.Checkpoint.BytesSoFar), report.Checkpoint.Shard)
	}
	line += fmt.Sprintf(", %s of %s cap, %d removed (%s)", diskBytes(report.BytesAfter), diskBytes(report.CapBytes), report.EntriesRemoved, diskBytes(report.BytesRemoved))
	if report.BytesAfter > report.CapBytes && report.Phase == "idle" {
		line += fmt.Sprintf("; %s used within the keep window stays", diskBytes(report.KeepWindowBytes))
	}
	if report.UnknownCount > 0 {
		line += fmt.Sprintf("; %d entries not Go's layout were left untouched", report.UnknownCount)
	}
	return line
}

func diskBytes(size int64) string {
	const gib, mib = int64(1) << 30, int64(1) << 20
	switch {
	case size >= gib:
		return fmt.Sprintf("%.1f GiB", float64(size)/float64(gib))
	case size >= mib:
		return fmt.Sprintf("%.1f MiB", float64(size)/float64(mib))
	}
	return fmt.Sprintf("%d B", size)
}
