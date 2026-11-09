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
func ownWorkElapsed(repoRoot, home, goalID string, start, now time.Time, work []clockDependency, pauses []outage.Span, environment map[string]bool) (time.Duration, string) {
	var spans, excluded []outage.Span
	for _, d := range work {
		if !d.executed {
			continue
		}
		if d.at.IsZero() {
			return 0, fmt.Sprintf("job %s (%s) has no readable start", d.id, d.runtime)
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
		return 0, "seat directory is unreadable: " + err.Error()
	}
	store := launch.Store{}
	if home != "" {
		store.Root = filepath.Join(home, "launch")
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
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
			return 0, "seat record " + entry.Name() + " is unreadable"
		}
		if seat.Goal != goalID {
			continue
		}
		from, err := time.Parse(time.RFC3339Nano, seat.StartedAt)
		if err != nil {
			return 0, "seat record " + entry.Name() + " has no readable startedAt"
		}
		until := now
		if seat.ReapedAt != "" {
			until, err = time.Parse(time.RFC3339Nano, seat.ReapedAt)
			if err != nil {
				return 0, "seat record " + entry.Name() + " has no readable reapedAt"
			}
		}
		record, err := store.Read(seat.LaunchID)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return 0, fmt.Sprintf("seat record %s launch %s is unreadable: %v", entry.Name(), seat.LaunchID, err)
		}
		if err == nil && record.State.Terminal() {
			until, err = time.Parse(time.RFC3339Nano, record.FinishedAt)
			if err != nil {
				return 0, fmt.Sprintf("seat record %s launch %s has no readable finishedAt", entry.Name(), seat.LaunchID)
			}
		}
		occupancy := []outage.Span{{Start: from, End: until}}
		for _, cut := range excluded {
			var remaining []outage.Span
			for _, span := range occupancy {
				if !cut.End.After(cut.Start) || !cut.End.After(span.Start) || !cut.Start.Before(span.End) {
					remaining = append(remaining, span)
					continue
				}
				if cut.Start.After(span.Start) {
					remaining = append(remaining, outage.Span{Start: span.Start, End: cut.Start})
				}
				if cut.End.Before(span.End) {
					remaining = append(remaining, outage.Span{Start: cut.End, End: span.End})
				}
			}
			occupancy = remaining
		}
		spans = append(spans, occupancy...)
	}
	spans = clipOwnWork(spans, start, now)
	pauses = clipOwnWork(pauses, start, now)
	return max(0, outage.Paused(append(spans, pauses...))-outage.Paused(pauses)), ""
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
