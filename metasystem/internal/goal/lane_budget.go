package goal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

// LaneBudgetRecord keeps closed stays and projects the current lane stay.
type LaneBudgetRecord struct {
	PausedAt      time.Time     `json:"lane_paused_at,omitempty,omitzero"`
	PausedMinutes float64       `json:"lane_paused_minutes,omitempty"`
	Spans         []outage.Span `json:"-"`
	ClosedSpans   []outage.Span `json:"closed_spans,omitempty"`
	Problems      []string      `json:"problems,omitempty"`
}

func (record LaneBudgetRecord) Words() string {
	if record.PausedAt.IsZero() {
		return ""
	}
	return fmt.Sprintf("in the lane since %s, clock paused (%gh so far)", record.PausedAt.Local().Format("15:04"), math.Round(record.PausedMinutes/6)/10)
}

// ReadLaneBudget clips each lane stay to this budget episode. Repeated
// hand-ins do not restart a stay; a return resumes it even if main stays red.
func ReadLaneBudget(install, id string, start, now time.Time, repositories ...Repository) (LaneBudgetRecord, error) {
	type member struct{ Goal, SHA string }
	type event struct {
		Goal, SHA, Outcome, At, Commit string
		Members                        []member `json:"batch-members"`
	}
	var events []event
	var record LaneBudgetRecord
	for _, name := range []string{"queue.jsonl", "pushes.jsonl"} {
		path := filepath.Join(install, "artifacts", "agents", "landing", name)
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			record.Problems = append(record.Problems, fmt.Sprintf("read %s: %v", path, err))
			continue
		}
		// Queue appends are independent records; a torn append cannot hide
		// complete records after it or invalidate another goal's clock.
		for _, line := range bytes.Split(data, []byte("\n")) {
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			var row event
			if err := json.Unmarshal(line, &row); err != nil {
				record.Problems = append(record.Problems, fmt.Sprintf("skipped unreadable line in %s", path))
				continue
			}
			if name == "queue.jsonl" && row.Goal == id {
				events = append(events, row)
			}
			if name == "pushes.jsonl" {
				if len(row.Members) == 0 {
					events = append(events, event{Commit: row.Commit, Outcome: "pushed", At: row.At})
				}
				for _, one := range row.Members {
					if one.Goal == id {
						events = append(events, event{Goal: id, SHA: one.SHA, Outcome: "landed", At: row.At})
					}
				}
			}
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].At < events[j].At })
	var began time.Time
	var sha string
	var repository Repository = gitRepository{endpoint: Endpoint{Root: install}}
	if len(repositories) > 0 {
		repository = repositories[0]
	}
	closeStay := func(end time.Time) {
		if !began.IsZero() {
			from, until := maxTime(began, start), minTime(end, now)
			if until.After(from) {
				record.Spans = append(record.Spans, outage.Span{Start: from, End: until})
			}
		}
		began, sha = time.Time{}, ""
	}
	for _, row := range events {
		if row.Outcome == "pushed" && sha == "" {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, row.At)
		if err != nil {
			return LaneBudgetRecord{}, fmt.Errorf("goal %s has an unreadable lane timestamp: %w", id, err)
		}
		if at.After(now) {
			continue
		}
		// Older push records name only the published commit. The existing
		// repository reader supplies the missing membership fact.
		if row.Outcome == "pushed" {
			if sha == "" {
				continue
			}
			landed, err := repository.IsAncestor(sha, row.Commit)
			if err != nil {
				return LaneBudgetRecord{}, err
			}
			if landed {
				closeStay(at)
			}
			continue
		}
		if row.Outcome == "" {
			if sha == row.SHA {
				continue
			}
			if began.IsZero() {
				began = at
			}
			sha = row.SHA
		} else if sha == row.SHA && (row.Outcome == "returned" || row.Outcome == "landed") {
			closeStay(at)
		}
	}
	record.ClosedSpans = append([]outage.Span(nil), record.Spans...)
	record.PausedAt = began
	closeStay(now)
	record.PausedMinutes = outage.Paused(record.Spans).Minutes()
	return record, nil
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// ReadClosedLaneBudget reads the goal's closed stays independently of the
// current registration, clipping them to the budget episode.
func ReadClosedLaneBudget(home, id string, start, now time.Time) (LaneBudgetRecord, error) {
	var record LaneBudgetRecord
	data, err := os.ReadFile(laneBudgetPath(home, id))
	if errors.Is(err, os.ErrNotExist) {
		return record, nil
	}
	if err != nil {
		return record, err
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return record, err
	}
	for _, span := range record.ClosedSpans {
		if span.Start.IsZero() || !span.End.After(span.Start) {
			return LaneBudgetRecord{}, fmt.Errorf("goal %s has an invalid closed lane stay", id)
		}
		from, until := maxTime(span.Start, start), minTime(span.End, now)
		if until.After(from) {
			record.Spans = append(record.Spans, outage.Span{Start: from, End: until})
		}
	}
	record.PausedMinutes = outage.Paused(record.Spans).Minutes()
	return record, nil
}

func laneBudgetPath(home, id string) string {
	return filepath.Join(home, "host", "goal-budgets", id, "lane.json")
}

// SaveClosedLaneBudget joins closed stays under the goal's own lock. Exact
// span identity makes recovery after a recorded return or push idempotent.
func SaveClosedLaneBudget(home, install, id string, now time.Time) error {
	current, err := ReadLaneBudget(install, id, time.Time{}, now)
	if err != nil {
		return err
	}
	if len(current.ClosedSpans) == 0 {
		return nil
	}
	path := laneBudgetPath(home, id)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	held, err := lock.File(path+".lock", 0600, lock.Exclusive)
	if err != nil {
		return err
	}
	defer held.Release()
	record, err := ReadClosedLaneBudget(home, id, time.Time{}, now)
	if err != nil {
		return err
	}
	for _, span := range current.ClosedSpans {
		found := false
		for _, old := range record.ClosedSpans {
			if old.Start.Equal(span.Start) && old.End.Equal(span.End) {
				found = true
				break
			}
		}
		if !found {
			record.ClosedSpans = append(record.ClosedSpans, span)
		}
	}
	record.PausedMinutes = outage.Paused(record.ClosedSpans).Minutes()
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteFile(path, append(data, '\n'), 0600, home)
	if err == nil && !durable {
		return fmt.Errorf("goal %s closed lane stays were written but durability is unconfirmed", id)
	}
	return err
}
