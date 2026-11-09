package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func creditedSlotWaits(store launch.Store, id string, from, until time.Time, running bool, now time.Time) (credited []outage.Span, openSince time.Time, problems []string) {
	dir, err := store.StateDir(id)
	var data []byte
	if err == nil {
		data, err = os.ReadFile(filepath.Join(dir, "waits.jsonl"))
	}
	if os.IsNotExist(err) || (err == nil && len(data) == 0) {
		return
	}
	if err != nil {
		return nil, time.Time{}, []string{"test slot wait records of launch " + id + " are unreadable; their time counts"}
	}
	type wait struct {
		line           proofrun.SlotWaitLine
		start, end     time.Time
		ended, invalid bool
	}
	waits := map[string]wait{}
	var markers []string
	unreadable := false
	for _, raw := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		var line proofrun.SlotWaitLine
		err := json.Unmarshal([]byte(raw), &line)
		at, timeErr := time.Parse(time.RFC3339Nano, line.At)
		if err != nil || timeErr != nil || line.Marker == "" || (line.Wait != "start" && line.Wait != "end") {
			unreadable = true
			continue
		}
		w, exists := waits[line.Marker]
		if !exists {
			markers = append(markers, line.Marker)
		}
		if line.Wait == "start" && !exists {
			w.line, w.start = line, at
		} else if line.Wait == "end" && exists && !w.ended && !at.Before(w.start) {
			w.end, w.ended = at, true
		} else {
			unreadable, w.invalid = true, true
		}
		waits[line.Marker] = w
	}
	if unreadable {
		problems = append(problems, "test slot wait records of launch "+id+" have an unreadable line; its time counts")
	}
	for _, marker := range markers {
		w := waits[marker]
		if w.invalid {
			continue
		}
		if !w.ended {
			held, err := proofrun.SlotWaitHeld(w.line.Directory, marker)
			if !running || err != nil || !held {
				problems = append(problems, "test slot wait "+marker+" of launch "+id+" has no end and no held marker for a running launch; its time counts")
				continue
			}
			w.end = now
			openSince = earliestSlotWait(openSince, w.start)
		}
		credited = append(credited, clipOwnWork([]outage.Span{{Start: w.start, End: w.end}}, from, until)...)
	}
	return
}

func earliestSlotWait(a, b time.Time) time.Time {
	if a.IsZero() || (!b.IsZero() && b.Before(a)) {
		return b
	}
	return a
}

func netSlotWaits(home string, deps []clockDependency, now time.Time) (net []clockDependency, waits []outage.Span, since time.Time, problems []string) {
	store := launch.Store{}
	if home != "" {
		store.Root = filepath.Join(home, "launch")
	}
	for _, d := range deps {
		if !d.launch {
			net = append(net, d)
			continue
		}
		credited, open, issues := creditedSlotWaits(store, d.id, d.at, d.until, d.running, now)
		waits, problems = append(waits, credited...), append(problems, issues...)
		since = earliestSlotWait(since, open)
		for _, span := range cutWorkSpans([]outage.Span{{Start: d.at, End: d.until}}, credited) {
			piece := d
			piece.at, piece.until = span.Start, span.End
			net = append(net, piece)
		}
	}
	return
}

func cutWorkSpans(spans, cuts []outage.Span) []outage.Span {
	for _, cut := range cuts {
		spans = cutPersonWork(spans, cut.End, []clockDependency{{at: cut.Start, until: cut.End, executed: true}})
	}
	return spans
}
