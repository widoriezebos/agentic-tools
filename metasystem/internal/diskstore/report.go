package diskstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ReportSchema is the pass report's schema.
const ReportSchema = "metasystem.disk-report/1"

// Report is one pass's report (3.3): free bytes per volume, bytes per store
// class, actions, kept and pending items with their reasons and commands,
// cursors and backlog, the floor's trim and consumer lines, strays and
// foreign entries, the use census, and the settings lines.
type Report struct {
	Schema  string    `json:"schema"`
	Kind    string    `json:"kind"`
	Name    string    `json:"name"`
	At      time.Time `json:"at"`
	Mode    Mode      `json:"mode"`
	Running bool      `json:"running,omitempty"`
	Plan    string    `json:"plan,omitempty"`

	Volumes []Volume          `json:"volumes,omitempty"`
	Floor   *FloorReport      `json:"floor,omitempty"`
	Classes []ClassReport     `json:"classes,omitempty"`
	Actions []Line            `json:"actions,omitempty"`
	Planned []Item            `json:"planned,omitempty"`
	Kept    []Line            `json:"kept,omitempty"`
	Pending []Line            `json:"pending,omitempty"`
	Strays  []Item            `json:"strays,omitempty"`
	Foreign []Item            `json:"foreign,omitempty"`
	Backlog []string          `json:"backlog,omitempty"`
	Cursors map[string]string `json:"cursors,omitempty"`
	Census  *UseCensus        `json:"census,omitempty"`

	Notes       []string `json:"notes,omitempty"`
	HostUnknown []string `json:"hostUnknown,omitempty"`
	Health      Health   `json:"health"`
}

// Volume is one filesystem's free space against the floor.
type Volume struct {
	Path       string `json:"path"`
	FreeBytes  int64  `json:"freeBytes"`
	FloorBytes int64  `json:"floorBytes"`
	BelowFloor bool   `json:"belowFloor,omitempty"`
}

// FloorReport is what floor mode did and found.
type FloorReport struct {
	Active    bool       `json:"active"`
	MinAge    string     `json:"minAge"`
	Trim      string     `json:"trim"`
	Consumers []Consumer `json:"consumers,omitempty"`
	Remedy    string     `json:"remedy"`
}

// ClassReport is one class's totals.
type ClassReport struct {
	Name     string `json:"name"`
	Items    int    `json:"items"`
	Bytes    int64  `json:"bytes,omitempty"`
	Released int    `json:"released,omitempty"`
	Backlog  int    `json:"backlog,omitempty"`
}

// Line is one kept, pending or acted item with its reason and the public
// command that ends its owner or settles it (H1: never a bare pid).
type Line struct {
	Class   string `json:"class"`
	Path    string `json:"path,omitempty"`
	Reason  string `json:"reason"`
	Command string `json:"command,omitempty"`
	Item    *Item  `json:"item,omitempty"`
}

// HealthStatus is the disk role's reading.
type HealthStatus string

const (
	HealthOK        HealthStatus = "ok"
	HealthAttention HealthStatus = "attention"
)

// Health is the report's contribution to the steward's disk role.
type Health struct {
	Status HealthStatus `json:"status"`
	Reason string       `json:"reason"`
	Remedy string       `json:"remedy,omitempty"`
}

func (r *Report) add(item Item, verdict Verdict) {
	copied := item
	copied.Verdict = verdict
	line := Line{Class: item.Class, Path: item.Path, Reason: verdict.Reason, Command: verdict.Command, Item: &copied}
	if verdict.Decision == Keep {
		r.Kept = append(r.Kept, line)
		return
	}
	r.Pending = append(r.Pending, line)
}

func healthOf(report Report) Health {
	switch {
	case len(report.HostUnknown) != 0:
		return Health{Status: HealthAttention, Reason: report.HostUnknown[0], Remedy: "metasystem settings check"}
	case report.Floor != nil && report.Floor.Active:
		free := ""
		for _, volume := range report.Volumes {
			if volume.BelowFloor {
				free = fmt.Sprintf(": %s has %s free, below the floor of %s", volume.Path, formatBytes(volume.FreeBytes), formatBytes(volume.FloorBytes))
				break
			}
		}
		return Health{Status: HealthAttention, Reason: "free space is below the floor" + free, Remedy: "metasystem disk clean --preview"}
	}
	return Health{Status: HealthOK, Reason: "free space is above the floor"}
}

// ReadReport reads a pass report.
func ReadReport(path string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, err
	}
	var report Report
	if err := json.Unmarshal(data, &report); err != nil || report.Schema != ReportSchema {
		if err == nil {
			err = fmt.Errorf("schema %q", report.Schema)
		}
		return Report{}, fmt.Errorf("disk report %s is unreadable: %w", path, err)
	}
	return report, nil
}

// CheckoutReportPath is a checkout pass's report.
func CheckoutReportPath(control string) string {
	return filepath.Join(control, "artifacts", "agents", "steward", "disk.json")
}

// MachineReportPath is the machine pass's report.
func MachineReportPath(homeStateRoot string) string {
	return filepath.Join(homeStateRoot, "stores", "report.json")
}

// Lines renders the report for a person: short lines, each kept or pending
// item with its reason and the command to run.
func (r Report) Lines() []string {
	var lines []string
	title := "machine"
	if r.Kind == "checkout" {
		title = "checkout " + r.Name
	}
	if r.Running {
		return []string{title + ": a pass is running; its report follows when it ends"}
	}
	lines = append(lines, fmt.Sprintf("%s: %s pass at %s", title, r.Mode, r.At.Format(time.RFC3339)))
	for _, volume := range r.Volumes {
		state := "above the floor"
		if volume.BelowFloor {
			state = "BELOW the floor"
		}
		lines = append(lines, fmt.Sprintf("  free: %s on %s, %s of %s", formatBytes(volume.FreeBytes), volume.Path, state, formatBytes(volume.FloorBytes)))
	}
	if r.Floor != nil && r.Floor.Active {
		lines = append(lines, "  floor mode: ageing lowered to "+r.Floor.MinAge+"; "+r.Floor.Trim)
		for _, consumer := range r.Floor.Consumers {
			lines = append(lines, "  consumer: "+consumer.Line())
		}
	}
	for _, class := range r.Classes {
		line := fmt.Sprintf("  %s: %d item(s)", class.Name, class.Items)
		if class.Bytes > 0 {
			line += ", " + formatBytes(class.Bytes)
		}
		if class.Released > 0 {
			line += fmt.Sprintf(", %d released", class.Released)
		}
		lines = append(lines, line)
	}
	for _, action := range r.Actions {
		lines = append(lines, fmt.Sprintf("  released: %s (%s)", action.Path, action.Reason))
	}
	for _, item := range r.Planned {
		lines = append(lines, fmt.Sprintf("  would release: %s (%s)", item.Path, item.Verdict.Reason))
	}
	for _, line := range r.Kept {
		lines = append(lines, renderLine("kept", line))
	}
	for _, line := range r.Pending {
		lines = append(lines, renderLine("pending", line))
	}
	for _, stray := range r.Strays {
		lines = append(lines, fmt.Sprintf("  stray: %s, %s, idle %s: %s; run %s", stray.Path, formatBytes(stray.Bytes),
			(time.Duration(stray.IdleSecs)*time.Second).String(), stray.Verdict.Reason, stray.Verdict.Command))
	}
	for _, item := range r.Foreign {
		lines = append(lines, fmt.Sprintf("  not the engine's: %s (%s)", item.Path, item.Verdict.Reason))
	}
	for _, backlog := range r.Backlog {
		lines = append(lines, "  backlog: "+backlog)
	}
	if r.Census != nil && !r.Census.Complete() {
		reason := r.Census.NotTaken
		if r.Census.Taken {
			reason = "incomplete: " + strings.Join(r.Census.GapLines(), "; ")
		}
		lines = append(lines, "  use census "+reason)
	}
	if r.Census != nil && len(r.Census.NotOurs) > 0 {
		lines = append(lines, fmt.Sprintf("  use census: unreadable, not ours: %d", len(r.Census.NotOurs)))
	}
	for _, note := range append(append([]string(nil), r.HostUnknown...), r.Notes...) {
		lines = append(lines, "  "+note)
	}
	return lines
}

func renderLine(kind string, line Line) string {
	text := fmt.Sprintf("  %s: %s: %s", kind, line.Path, line.Reason)
	if line.Path == "" {
		text = fmt.Sprintf("  %s: %s: %s", kind, line.Class, line.Reason)
	}
	if line.Command != "" {
		text += "; run " + line.Command
	}
	return text
}

func formatBytes(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(bytes)/(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(bytes)/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(bytes)/(1<<10))
	}
	return fmt.Sprintf("%d B", bytes)
}
