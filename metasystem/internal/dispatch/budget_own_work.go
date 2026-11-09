package dispatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

// ownWorkElapsed counts execution and seat occupancy outside the existing pauses.
// Optional evidence failures retain the complete wall-time clock at the caller.
func ownWorkElapsed(repoRoot, home, goalID string, start, now time.Time, work []clockDependency, pauses, slotWaits []outage.Span, environment map[string]bool) (elapsed time.Duration, reason string, openSince time.Time, problems []string) {
	var spans, excluded, seated []outage.Span
	for _, d := range work {
		if !d.executed {
			continue
		}
		if d.at.IsZero() {
			return 0, fmt.Sprintf("job %s (%s) has no readable start", d.id, d.runtime), openSince, problems
		}
		until := d.until
		if until.IsZero() {
			until = now
		}
		span := outage.Span{Start: d.at, End: until}
		if environment[d.id] {
			excluded = append(excluded, span)
		} else {
			spans = append(spans, span)
		}
	}
	seats := filepath.Join(repoRoot, "artifacts", "agents", "steward", "seats")
	entries, err := os.ReadDir(seats)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return 0, "seat directory is unreadable: " + err.Error(), openSince, problems
	}
	store := launch.Store{}
	if home != "" {
		store.Root = filepath.Join(home, "launch")
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var seat struct {
			LaunchID  string `json:"launchId"`
			Goal      string `json:"goal"`
			StartedAt string `json:"startedAt"`
			ReapedAt  string `json:"reapedAt"`
		}
		data, err := os.ReadFile(filepath.Join(seats, entry.Name()))
		if err != nil || json.Unmarshal(data, &seat) != nil {
			return 0, "seat record " + entry.Name() + " is unreadable", openSince, problems
		}
		if seat.Goal != goalID {
			continue
		}
		from, err := time.Parse(time.RFC3339Nano, seat.StartedAt)
		if err != nil {
			return 0, "seat record " + entry.Name() + " has no readable startedAt", openSince, problems
		}
		until := now
		if seat.ReapedAt != "" {
			until, err = time.Parse(time.RFC3339Nano, seat.ReapedAt)
			if err != nil {
				return 0, "seat record " + entry.Name() + " has no readable reapedAt", openSince, problems
			}
		}
		record, err := store.Read(seat.LaunchID)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return 0, fmt.Sprintf("seat record %s launch %s is unreadable: %v", entry.Name(), seat.LaunchID, err), openSince, problems
		}
		if err == nil && record.State.Terminal() {
			until, err = time.Parse(time.RFC3339Nano, record.FinishedAt)
			if err != nil {
				return 0, fmt.Sprintf("seat record %s launch %s has no readable finishedAt", entry.Name(), seat.LaunchID), openSince, problems
			}
		}
		credited, since, issues := creditedSlotWaits(store, seat.LaunchID, from, until, seat.ReapedAt == "" && !record.State.Terminal(), now)
		slotWaits, problems = append(slotWaits, credited...), append(problems, issues...)
		openSince = earliestSlotWait(openSince, since)
		seated = append(seated, outage.Span{Start: from, End: until})
	}
	spans = append(spans, cutWorkSpans(seated, append(excluded, slotWaits...))...)
	spans = clipOwnWork(spans, start, now)
	pauses = clipOwnWork(pauses, start, now)
	return max(0, outage.Paused(append(spans, pauses...))-outage.Paused(pauses)), "", openSince, problems
}

func clipOwnWork(spans []outage.Span, start, now time.Time) []outage.Span {
	var clipped []outage.Span
	for _, span := range spans {
		if span.Start.Before(start) {
			span.Start = start
		}
		if span.End.After(now) {
			span.End = now
		}
		if span.End.After(span.Start) {
			clipped = append(clipped, span)
		}
	}
	return clipped
}
