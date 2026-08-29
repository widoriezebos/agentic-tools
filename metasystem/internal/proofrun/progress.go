package proofrun

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ProgressHeader starts one run in the append-only suite progress journal.
// The paths are the watchdog's evidence inventory, not liveness claims.
type ProgressHeader struct {
	TmpPaths []string `json:"tmpPaths"`
	LogPaths []string `json:"logPaths"`
}

// SectionEvent is the stable section vocabulary projected by the validation
// selector. Output growth, not these boundary records, proves liveness.
type SectionEvent struct {
	Suite   string `json:"suite"`
	Section string `json:"section"`
	Event   string `json:"event"`
	At      string `json:"at"`
	Depth   int    `json:"depth"`
}

type ProgressRun struct {
	Header ProgressHeader
	Events []SectionEvent
}

func AppendProgressHeader(path string, header ProgressHeader) error {
	if len(header.LogPaths) == 0 {
		return errors.New("suite progress header requires at least one log path")
	}
	return appendJSONLine(path, header)
}

func AppendSectionEvent(path string, event SectionEvent) error {
	if event.Suite == "" || event.Section == "" || event.Depth < 0 {
		return errors.New("suite progress event requires suite, section, and non-negative depth")
	}
	if event.Event != "start" && event.Event != "end" {
		return fmt.Errorf("suite progress event must be start or end, got %q", event.Event)
	}
	if _, err := time.Parse(time.RFC3339Nano, event.At); err != nil {
		return fmt.Errorf("suite progress event has invalid time: %w", err)
	}
	return appendJSONLine(path, event)
}

func appendJSONLine(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create suite progress directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open suite progress journal: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("append suite progress journal: %w", err)
	}
	return nil
}

// ReadLatestProgressRun returns the suffix beginning at the most recent
// header. Historical runs remain append-only and do not satisfy a new run.
func ReadLatestProgressRun(path string) (ProgressRun, error) {
	file, err := os.Open(path)
	if err != nil {
		return ProgressRun{}, fmt.Errorf("open suite progress journal: %w", err)
	}
	defer file.Close()

	var run ProgressRun
	foundHeader := false
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		var shape map[string]json.RawMessage
		if err := json.Unmarshal(scanner.Bytes(), &shape); err != nil {
			return ProgressRun{}, fmt.Errorf("suite progress line %d is not JSON: %w", line, err)
		}
		if _, isHeader := shape["tmpPaths"]; isHeader {
			var header ProgressHeader
			if err := json.Unmarshal(scanner.Bytes(), &header); err != nil || len(header.LogPaths) == 0 {
				return ProgressRun{}, fmt.Errorf("suite progress line %d is an invalid header", line)
			}
			run = ProgressRun{Header: header}
			foundHeader = true
			continue
		}
		if !foundHeader {
			continue
		}
		var event SectionEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return ProgressRun{}, fmt.Errorf("suite progress line %d is an invalid event: %w", line, err)
		}
		if event.Event != "start" && event.Event != "end" {
			return ProgressRun{}, fmt.Errorf("suite progress line %d has unknown event %q", line, event.Event)
		}
		if event.Suite == "" || event.Section == "" || event.Depth < 0 {
			return ProgressRun{}, fmt.Errorf("suite progress line %d has an incomplete event", line)
		}
		if _, err := time.Parse(time.RFC3339Nano, event.At); err != nil {
			return ProgressRun{}, fmt.Errorf("suite progress line %d has invalid time: %w", line, err)
		}
		run.Events = append(run.Events, event)
	}
	if err := scanner.Err(); err != nil {
		return ProgressRun{}, fmt.Errorf("read suite progress journal: %w", err)
	}
	if !foundHeader {
		return ProgressRun{}, errors.New("suite progress journal has no run header")
	}
	return run, nil
}

func AssertSectionProgress(run ProgressRun, suite string, expected []string, twiceConsulted map[string]bool) error {
	expectedSet := make(map[string]bool, len(expected))
	for _, section := range expected {
		if section == "" || expectedSet[section] {
			return fmt.Errorf("selector contains an empty or duplicate section %q", section)
		}
		expectedSet[section] = true
	}
	for section := range twiceConsulted {
		if !expectedSet[section] {
			return fmt.Errorf("twice-consulted section %q is absent from the selector", section)
		}
	}

	type state struct {
		starts int
		ends   int
		open   bool
	}
	states := make(map[string]*state, len(expected))
	for _, section := range expected {
		states[section] = &state{}
	}
	for _, event := range run.Events {
		if event.Suite != suite {
			return fmt.Errorf("progress event for suite %q appeared in %q run", event.Suite, suite)
		}
		current, ok := states[event.Section]
		if !ok {
			return fmt.Errorf("progress event names section %q outside the selector", event.Section)
		}
		switch event.Event {
		case "start":
			if current.open {
				return fmt.Errorf("section %q started again before it ended", event.Section)
			}
			current.starts++
			current.open = true
		case "end":
			if !current.open {
				return fmt.Errorf("section %q ended without a preceding start", event.Section)
			}
			current.ends++
			current.open = false
		}
	}

	var problems []string
	for _, section := range expected {
		want := 1
		if twiceConsulted[section] {
			want = 2
		}
		got := states[section]
		if got.starts != want || got.ends != want || got.open {
			problems = append(problems, fmt.Sprintf("%s has %d starts and %d ends; expected %d of each", section, got.starts, got.ends, want))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("suite progress structure is incomplete: %s", strings.Join(problems, "; "))
	}
	return nil
}

func CurrentSection(run ProgressRun, suite string) (string, time.Time) {
	current := "startup"
	var started time.Time
	for _, event := range run.Events {
		if event.Suite != suite {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, event.At)
		if err != nil {
			continue
		}
		if event.Event == "start" {
			current, started = event.Section, at
		} else if event.Section == current {
			current, started = "between-sections", time.Time{}
		}
	}
	return current, started
}

// DeepestLiveHeartbeat projects the most deeply nested section that has
// started but not ended. Watchers use this read-only view; section structure
// remains owned by AssertSectionProgress at suite completion.
func DeepestLiveHeartbeat(run ProgressRun, now time.Time) (string, bool) {
	type sectionKey struct {
		suite   string
		section string
		depth   int
	}
	type liveSection struct {
		key     sectionKey
		started time.Time
		order   int
	}

	live := make(map[sectionKey]liveSection)
	for order, event := range run.Events {
		started, err := time.Parse(time.RFC3339Nano, event.At)
		if err != nil {
			continue
		}
		key := sectionKey{suite: event.Suite, section: event.Section, depth: event.Depth}
		if event.Event == "start" {
			live[key] = liveSection{key: key, started: started, order: order}
		} else {
			delete(live, key)
		}
	}

	var deepest liveSection
	found := false
	for _, section := range live {
		if !found || section.key.depth > deepest.key.depth ||
			(section.key.depth == deepest.key.depth && section.order > deepest.order) {
			deepest = section
			found = true
		}
	}
	if !found {
		return "", false
	}
	age := now.Sub(deepest.started)
	if age < 0 {
		age = 0
	}
	return fmt.Sprintf("%s:%s since %dmin", deepest.key.suite, deepest.key.section, int64(age/time.Minute)), true
}
