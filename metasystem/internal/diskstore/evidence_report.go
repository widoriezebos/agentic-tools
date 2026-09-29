package diskstore

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// EvidenceSegment is one evidence segment's position after a machine pass
// (3.3 report, 3.12): its total against its cap, what the pass compacted,
// and, when it is still over after compaction, the bytes each exclusion
// holds, what a person may remove, and the command pair. Machinery never
// removes an item; this line is how a person learns there is something to
// decide.
type EvidenceSegment struct {
	Segment         string           `json:"segment"`
	Root            string           `json:"root"`
	Checkout        string           `json:"checkout,omitempty"`
	TotalBytes      int64            `json:"totalBytes"`
	CapBytes        int64            `json:"capBytes"`
	BlobChargeBytes int64            `json:"blobChargeBytes,omitempty"`
	Compacted       int              `json:"compacted,omitempty"`
	FreedBytes      int64            `json:"freedBytes,omitempty"`
	Over            bool             `json:"over,omitempty"`
	Held            map[string]int64 `json:"held,omitempty"`
	Removable       int              `json:"removable,omitempty"`
	RemovableBytes  int64            `json:"removableBytes,omitempty"`
	Oldest          time.Time        `json:"oldest,omitempty"`
	YoungBytes      int64            `json:"youngBytes,omitempty"`
	AwaitingBytes   int64            `json:"awaitingBytes,omitempty"`
	Unknown         string           `json:"unknown,omitempty"`
	Pending         []string         `json:"pending,omitempty"`
	Commands        []string         `json:"commands,omitempty"`
	LedgerTip       string           `json:"ledgerTip,omitempty"`
}

// Lines renders the segment: nothing when it is under its cap and nothing
// happened; otherwise one line, and the command pair when a person has
// something to decide.
func (e EvidenceSegment) Lines(verbose bool) []string {
	name := e.Segment
	if e.Checkout != "" {
		name += " of " + e.Checkout
	}
	var lines []string
	if e.Unknown != "" {
		lines = append(lines, fmt.Sprintf("  evidence %s: %s of %s; Unknown, nothing compacted: %s", name, formatBytes(e.TotalBytes), formatBytes(e.CapBytes), e.Unknown))
	}
	if e.Compacted > 0 {
		lines = append(lines, fmt.Sprintf("  evidence %s: compacted %d item(s), freed %s", name, e.Compacted, formatBytes(e.FreedBytes)))
	}
	pending := e.Pending
	if !verbose && len(pending) > examplePaths {
		pending = append(append([]string(nil), pending[:examplePaths]...), fmt.Sprintf("and %d more (--verbose lists them)", len(e.Pending)-examplePaths))
	}
	for _, line := range pending {
		lines = append(lines, "  evidence "+name+": pending: "+line)
	}
	if e.Over {
		var held []string
		var reasons []string
		for reason := range e.Held {
			reasons = append(reasons, reason)
		}
		sort.Strings(reasons)
		for _, reason := range reasons {
			held = append(held, fmt.Sprintf("%s %s", reason, formatBytes(e.Held[reason])))
		}
		line := fmt.Sprintf("  over the bound: %s: %s of %s after compaction", name, formatBytes(e.TotalBytes), formatBytes(e.CapBytes))
		if e.BlobChargeBytes > 0 {
			line += fmt.Sprintf(" (blob charges %s)", formatBytes(e.BlobChargeBytes))
		}
		if len(held) > 0 {
			line += "; held: " + strings.Join(held, ", ")
		}
		if e.YoungBytes > 0 {
			line += "; younger than the age floor " + formatBytes(e.YoungBytes)
		}
		line += fmt.Sprintf("; removable by a person: %d compacted item(s), %s", e.Removable, formatBytes(e.RemovableBytes))
		if !e.Oldest.IsZero() {
			line += ", oldest " + e.Oldest.UTC().Format("2006-01-02")
		}
		lines = append(lines, line)
		for _, command := range e.Commands {
			lines = append(lines, "    "+command)
		}
	}
	return lines
}
