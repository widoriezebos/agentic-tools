package main

import (
	"io"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// The Stop report a session status --id prints is a short summary that
// leads with what the person or the seat must do; a plan line untouched
// for more than seven days shows only with --verbose, and the whole
// report's sections stay in the file it names.
func TestSessionStatusLeadsWithWhatToDoAndHidesStalePlans(t *testing.T) {
	t.Parallel()
	c := layoutCase{name: "session-status", args: []string{"session", "status", "--id", "1", "--root", "ROOT"}, bed: stopReportLayoutBed}
	bed := c.bed(t)
	env := layoutEnv(t, bed.now, "", "")
	bed.owners.textEnv = func(io.Writer) textui.Env { return env }
	code, stdout, stderr := runLayoutCase(t, c, bed)
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if code != 0 || stderr != "" || len(lines) < 3 {
		t.Fatalf("session status = %d:\n%s%s", code, stdout, stderr)
	}
	if !strings.HasPrefix(lines[0], "! ") || !strings.Contains(lines[0], "no steward runner is recorded") || !strings.Contains(lines[1], "→ metasystem system start") {
		t.Fatalf("the report does not lead with the repair and its command:\n%s", stdout)
	}
	flat := flatPage(stdout)
	if !strings.Contains(flat, "metasystem goal claim verbs-match-intent") || !strings.Contains(flat, "plans/work.md") {
		t.Fatalf("the seat's action or the recent plan is missing:\n%s", stdout)
	}
	if strings.Contains(flat, "plans/old.md") || !strings.Contains(flat, "1 plan line older than 7 days") {
		t.Fatalf("a plan untouched for twenty days shows by default:\n%s", stdout)
	}
	if strings.Contains(stdout, "```") || len(lines) > 30 {
		t.Fatalf("the default is not a short summary (%d lines):\n%s", len(lines), stdout)
	}
	code, verbose, _ := runLayoutCase(t, c, bed, "--verbose")
	if code != 0 || !strings.Contains(flatPage(verbose), "plans/old.md") {
		t.Fatalf("--verbose does not show the stale plan = %d:\n%s", code, verbose)
	}
}
