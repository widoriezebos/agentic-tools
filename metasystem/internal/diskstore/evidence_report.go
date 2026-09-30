package diskstore

import (
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
	"time"
)

// EvidenceSegment is one evidence segment's position after a machine pass
// (3.3 report, 3.12; Round B2-3: machinery never compacts or removes): its
// total against its cap and, past the bound, by how much with the command
// pair whose preview shows what a person can remove. This line is how a
// person learns there is something to decide.
type EvidenceSegment struct {
	Segment         string    `json:"segment"`
	Root            string    `json:"root"`
	Checkout        string    `json:"checkout,omitempty"`
	TotalBytes      int64     `json:"totalBytes"`
	CapBytes        int64     `json:"capBytes"`
	BlobChargeBytes int64     `json:"blobChargeBytes,omitempty"`
	Over            bool      `json:"over,omitempty"`
	Unknown         string    `json:"unknown,omitempty"`
	Pending         []string  `json:"pending,omitempty"`
	Commands        []string  `json:"commands,omitempty"`
	LedgerTip       string    `json:"ledgerTip,omitempty"`
	Oldest          time.Time `json:"oldest,omitempty"`
}

// Lines renders the segment: nothing when it is under its cap and nothing
// is pending; otherwise one line, and the command pair when a person has
// something to decide.
func (e EvidenceSegment) Lines(verbose bool) []string {
	name := e.Segment
	if e.Checkout != "" {
		name += " of " + e.Checkout
	}
	var lines []string
	if e.Unknown != "" {
		lines = append(lines, fmt.Sprintf("  evidence %s: %s of %s; Unknown: %s", name, textui.Bytes(e.TotalBytes), textui.Bytes(e.CapBytes), e.Unknown))
	}
	pending := e.Pending
	if !verbose && len(pending) > examplePaths {
		pending = append(append([]string(nil), pending[:examplePaths]...), fmt.Sprintf("and %d more (--verbose lists them)", len(e.Pending)-examplePaths))
	}
	for _, line := range pending {
		lines = append(lines, "  evidence "+name+": pending: "+line)
	}
	if e.Over {
		line := fmt.Sprintf("  over the bound: %s: %s of %s, over by %s", name, textui.Bytes(e.TotalBytes), textui.Bytes(e.CapBytes), textui.Bytes(e.TotalBytes-e.CapBytes))
		if e.BlobChargeBytes > 0 {
			line += fmt.Sprintf(" (blob charges %s)", textui.Bytes(e.BlobChargeBytes))
		}
		if !e.Oldest.IsZero() {
			line += ", oldest item " + e.Oldest.UTC().Format("2006-01-02")
		}
		line += "; the preview shows what a person can remove:"
		lines = append(lines, line)
		for _, command := range e.Commands {
			lines = append(lines, "    "+command)
		}
	}
	return lines
}
