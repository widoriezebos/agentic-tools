package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

func (p BudgetProjection) PersonWords() string {
	if p.PersonWaitSince.IsZero() {
		return ""
	}
	return fmt.Sprintf("waiting for a person since %s, clock paused (%.1fh so far)", p.PersonWaitSince.Local().Format("15:04"), p.PersonPaused.Hours())
}

// Dispatch reads the channel's record without importing its command layer,
// which depends on dispatch. Unbounded records leave time on the clock.
func personWaits(repoRoot, goalID string, from, now time.Time, work []clockDependency) (spans []outage.Span, problems []string) {
	dir := filepath.Join(repoRoot, "artifacts", "agents", "channel", "questions")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, []string{dir + ": " + err.Error()}
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		var q struct {
			ID, Goal, About, State        string
			OpenedAt, ClosedAt, RetiredAt time.Time
			Answer                        struct{ At time.Time }
			UnitStop, Recovery            *json.RawMessage
			Decision                      struct{ Kind, Subject string }
		}
		data, err := os.ReadFile(path)
		if err == nil {
			err = json.Unmarshal(data, &q)
		}
		if err != nil || q.ID == "" || q.OpenedAt.IsZero() || q.State == "" {
			problems = append(problems, path+": unreadable question; its wait counts")
			continue
		}
		var end time.Time
		switch {
		case q.Goal == "" && q.About == "machine" && q.Decision.Subject == goalID && (q.Decision.Kind == "steward-hold" || q.Decision.Kind == "uncollected-round"):
			end = q.RetiredAt
			if end.IsZero() {
				end = now
			}
		case q.Goal == goalID:
			if q.UnitStop != nil || q.Recovery != nil {
				end = q.ClosedAt
				if end.IsZero() {
					end = q.RetiredAt
				}
				if q.State != "closed" {
					end = now
				}
			} else {
				end = q.Answer.At
				if end.IsZero() {
					end = q.ClosedAt
				}
				if end.IsZero() {
					end = q.RetiredAt
				}
				if end.IsZero() && q.State == "open" {
					end = now
				}
			}
			if end.IsZero() {
				problems = append(problems, fmt.Sprintf("question %s has no close time; its wait counts", q.ID))
				continue
			}
		default:
			continue
		}
		start := q.OpenedAt
		if start.Before(from) {
			start = from
		}
		if end.After(now) {
			end = now
		}
		if end.After(start) {
			spans = append(spans, outage.Span{Start: start, End: end})
		}
	}
	return cutPersonWork(spans, now, work), problems
}

func cutPersonWork(spans []outage.Span, now time.Time, work []clockDependency) []outage.Span {
	for _, job := range work {
		if !job.executed {
			continue
		}
		until := job.until
		if until.IsZero() {
			until = now
		}
		var remaining []outage.Span
		for _, span := range spans {
			if !until.After(job.at) || !until.After(span.Start) || !job.at.Before(span.End) {
				remaining = append(remaining, span)
				continue
			}
			if job.at.After(span.Start) {
				remaining = append(remaining, outage.Span{Start: span.Start, End: job.at})
			}
			if until.Before(span.End) {
				remaining = append(remaining, outage.Span{Start: until, End: span.End})
			}
		}
		spans = remaining
	}
	return spans
}
