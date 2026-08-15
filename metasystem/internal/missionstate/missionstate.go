// Package missionstate owns the active-mission rule as a leaf: does this
// checkout have a mission someone is actually driving? The rule is the one
// the mission status verb has always applied — the runner record plus
// kernel process identity, never either alone: a record claiming "running"
// counts only while the recorded runner process is provably alive
// (goal-system design, GOAL-16/GOAL-21/GOAL-22).
//
// Liveness answers are THREE-WAY and stay three-way here (the identity
// package's contract): Alive means ACTIVE, a proven-dead process means
// INACTIVE, and Unknown means INDETERMINATE — a caller deciding anything
// destructive or authoritative on Indeterminate must fail closed, because
// Unknown never authorizes anything.
//
// This package is a leaf on purpose: internal/goal consumes it for
// mutation refusal and internal/report's scanner consumes it for the Busy
// inventory, while missionrunner (its first consumer) applies the same
// rule from the status path. It imports identity and the standard library,
// nothing else.
package missionstate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Verdict classifies one recorded runner.
type Verdict int

const (
	// Inactive: the record is terminal, or its recorded process is
	// provably gone. No one is driving.
	Inactive Verdict = iota
	// Active: the record claims running and the recorded process is
	// alive at recorded identity. Someone is driving.
	Active
	// Indeterminate: the record claims running but the kernel could not
	// prove the process alive or dead, or the record's identity fields
	// are unusable. Unknown never authorizes anything.
	Indeterminate
)

func (v Verdict) String() string {
	switch v {
	case Active:
		return "active"
	case Indeterminate:
		return "indeterminate"
	default:
		return "inactive"
	}
}

// Runner is one classified runner record.
type Runner struct {
	MissionId string
	Path      string
	Verdict   Verdict
	// Detail is the bounded display line the scanner renders verbatim
	// (≤200 bytes at construction).
	Detail string
}

// Result is a survey of every runner record under one checkout root.
type Result struct {
	Runners []Runner
	// Unreadable lists inventory-source failures: unreadable or
	// unparsable records, and enumeration errors. Enumeration failure
	// must never collapse to idle (goal-system r6 finding 3).
	Unreadable []string
}

// ActiveMissions returns the runners someone is driving right now.
func (r Result) ActiveMissions() []Runner {
	var out []Runner
	for _, runner := range r.Runners {
		if runner.Verdict == Active {
			out = append(out, runner)
		}
	}
	return out
}

// Indeterminate reports whether any runner's liveness is unknowable.
func (r Result) Indeterminate() bool {
	for _, runner := range r.Runners {
		if runner.Verdict == Indeterminate {
			return true
		}
	}
	return false
}

// RecordFields is the slice of a runner record this rule reads.
type RecordFields struct {
	MissionId    string `json:"missionId"`
	Status       string `json:"status"`
	Pid          int64  `json:"pid"`
	PidStartedAt int64  `json:"pidStartedAt"`
}

// Classify applies the rule to one record's fields, three-way.
func Classify(record RecordFields, prober identity.Prober) Verdict {
	if record.Status != "running" {
		return Inactive
	}
	if record.Pid < 1 || record.PidStartedAt < 1 {
		// A running claim whose identity fields are unusable cannot be
		// verified either way.
		return Indeterminate
	}
	switch identity.AliveRef(prober, identity.Ref{Pid: record.Pid, StartedAtSec: record.PidStartedAt}) {
	case identity.Alive:
		return Active
	case identity.Unknown:
		return Indeterminate
	default:
		return Inactive
	}
}

// runnersDir is the checkout-scoped home of runner records.
func runnersDir(root string) string {
	return filepath.Join(root, "artifacts", "agents", "missions", "runners")
}

// Survey classifies every runner record under root. A missing directory is
// an empty result; every read, parse, or enumeration failure lands in
// Unreadable rather than being skipped.
func Survey(root string, prober identity.Prober) Result {
	var result Result
	dir := runnersDir(root)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return result
		}
		result.Unreadable = append(result.Unreadable, dir+": "+err.Error())
		return result
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			result.Unreadable = append(result.Unreadable, path+": "+err.Error())
			continue
		}
		var record RecordFields
		if err := json.Unmarshal(data, &record); err != nil {
			result.Unreadable = append(result.Unreadable, path+": "+err.Error())
			continue
		}
		verdict := Classify(record, prober)
		missionId := record.MissionId
		if missionId == "" {
			missionId = strings.TrimSuffix(name, ".json")
		}
		if verdict == Indeterminate {
			result.Unreadable = append(result.Unreadable, path+": runner liveness unknown")
		}
		result.Runners = append(result.Runners, Runner{
			MissionId: missionId,
			Path:      path,
			Verdict:   verdict,
			Detail:    clip("mission "+missionId+" [running]", 200),
		})
	}
	return result
}

// clip bounds a display string at limit bytes.
func clip(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit]
}
