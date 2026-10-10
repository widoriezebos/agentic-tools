package launch

import (
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostcapacity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

func addCall(sum *Measurement, call Measurement) {
	sum.Calls += call.Calls
	sum.InputTokens += call.InputTokens
	sum.CacheReadTokens += call.CacheReadTokens
	sum.CacheCreationTokens += call.CacheCreationTokens
	sum.OutputTokens += call.OutputTokens
	sum.PeakContext = max(sum.PeakContext, call.PeakContext)
}

func tokenCounts(m Measurement) *hostcapacity.Tokens {
	return &hostcapacity.Tokens{Calls: m.Calls, Input: m.InputTokens, CacheRead: m.CacheReadTokens, CacheCreation: m.CacheCreationTokens, Output: m.OutputTokens, PeakContext: m.PeakContext}
}

// CapacityUsage reads each provider session once; nil counts mean unknown.
func (m *Manager) CapacityUsage(at time.Time) (hostcapacity.Usage, error) {
	u := hostcapacity.Usage{From: at.Add(-time.Hour).UTC().Format(time.RFC3339Nano), Until: at.UTC().Format(time.RFC3339Nano), Sessions: []hostcapacity.SessionUsage{}, Problems: []string{}}
	records, err := m.List()
	groups := map[string][]Record{}
	for _, record := range records {
		if record.Adapter == "plain-exec" || record.Adapter == "" {
			continue
		}
		key := outage.Provider(record.Adapter) + ":" + readString(record.AdapterData, "sessionID")
		if readString(record.AdapterData, "sessionID") == "" {
			key += ":" + record.ID
		}
		groups[key] = append(groups[key], record)
	}
	paths, failures := map[string][]string{}, map[string]error{}
	files := func(root string) ([]string, error) {
		if _, ok := paths[root]; !ok {
			paths[root], failures[root] = transcriptFiles(root)
		}
		return paths[root], failures[root]
	}
	for _, key := range slices.Sorted(maps.Keys(groups)) {
		record := groups[key][0]
		session := readString(record.AdapterData, "sessionID")
		observed := hostcapacity.SessionUsage{Provider: outage.Provider(record.Adapter), Session: session, WorkingDirectory: record.WorkingDirectory, Problems: []string{}}
		eligible := false
		for _, r := range groups[key] {
			observed.Provisional = observed.Provisional || !r.State.Terminal()
			end, parseErr := time.Parse(time.RFC3339Nano, r.FinishedAt)
			eligible = eligible || !r.State.Terminal() || parseErr != nil || !end.Before(at.Add(-time.Hour))
			if r.WorkingDirectory != observed.WorkingDirectory {
				observed.Problems = append(observed.Problems, "conflicting working directory attribution: "+r.WorkingDirectory)
			}
			if r.State.Terminal() && r.Measurement.UsageRead && (!record.Measurement.UsageRead || r.Measurement.TotalTokens() > record.Measurement.TotalTokens()) {
				record.Measurement = r.Measurement
			}
		}
		if !eligible {
			continue
		}
		reader, ok := m.Adapters[record.Adapter].(interface {
			measureTranscript(string, *Measurement, func(string) ([]string, error)) error
		})
		measurement, readErr := Measurement{observedAt: at, window: &Measurement{}}, fmt.Errorf("session identity or timestamped usage reader unavailable: runtime %s, launch %s", record.Adapter, record.ID)
		if ok && session != "" {
			readErr = reader.measureTranscript(session, &measurement, files)
		}
		if readErr != nil {
			observed.Problems = append(observed.Problems, readErr.Error())
			if record.Measurement.UsageRead {
				observed.Totals = tokenCounts(record.Measurement)
			}
		} else {
			observed.Totals, observed.TrailingHour = tokenCounts(measurement), tokenCounts(*measurement.window)
			if measurement.missingTimestamps > 0 {
				observed.Problems = append(observed.Problems, fmt.Sprintf("%d calls have missing or invalid timestamps; trailing hour is incomplete", measurement.missingTimestamps))
			}
		}
		if observed.Provisional || len(observed.Problems) > 0 || observed.TrailingHour == nil || observed.TrailingHour.Calls > 0 {
			u.Sessions = append(u.Sessions, observed)
		}
	}
	return u, err
}

// transcriptFiles is a per-observation inventory, shared by resumed launches.
func transcriptFiles(root string) ([]string, error) {
	paths := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
			paths = append(paths, path)
		}
		return err
	})
	return paths, err
}

func (m *Measurement) observeCall(call Measurement, timestamp string) bool {
	if m.observedAt.IsZero() {
		return true
	}
	at, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		m.missingTimestamps++
		return true
	}
	if !at.Before(m.observedAt) {
		return false
	}
	if !at.Before(m.observedAt.Add(-time.Hour)) {
		addCall(m.window, call)
	}
	return true
}
