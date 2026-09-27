package refusal

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// forwardWindowRadius is how far from its site a guide row's message may
// name its public command: the message and its format arguments.
const forwardWindowRadius = 8

var forwardForm = regexp.MustCompile(`^[a-z]+ [a-z][a-z-]*$`)

// h1GuideDefectCeiling is the number of guide rows the register admits with
// no public command yet (rule H1 gaps recorded as Defects). It only falls.
const h1GuideDefectCeiling = 1

// Rule H1 / witness R14, register half: every row has a standing; a row a
// person meets (Question) states it explicitly; a guide row names the public
// command its message carries, within the emission window of its site.
func TestH1EveryRowHasAStanding(t *testing.T) {
	t.Parallel()
	valid := map[Standing]bool{StandingInput: true, StandingAgent: true, StandingIdentity: true, StandingGuide: true}
	defects := map[string]bool{}
	for _, defect := range Defects {
		defects[defect.Code] = true
	}
	root := moduleRoot(t)
	gaps := 0
	for _, row := range Rows {
		standing := row.Standing()
		if !valid[standing] {
			t.Errorf("row %s has no rule-H1 standing (H1=%q shape=%s): a refusal a person can meet must say whether it asks for a corrected word or guides", row.Code, row.H1, row.Shape)
			continue
		}
		if row.Shape == Question && row.H1 == "" {
			t.Errorf("Question row %s must declare its H1 standing explicitly", row.Code)
		}
		if row.H1 == StandingIdentity && row.Shape == Agent || row.H1 == StandingAgent && row.Shape == Identity {
			t.Errorf("row %s declares standing %s against its shape %s", row.Code, row.H1, row.Shape)
		}
		if standing != StandingGuide {
			if row.Forward != "" || row.ForwardSite != "" {
				t.Errorf("row %s names a forward command but is not a guide row", row.Code)
			}
			continue
		}
		if row.Forward == "" {
			if !defects[row.Code] {
				t.Errorf("guide row %s names no public command and is not a recorded Defect", row.Code)
			}
			gaps++
			continue
		}
		if !forwardForm.MatchString(row.Forward) {
			t.Errorf("guide row %s forward %q is not one OBJECT ACTION pair", row.Code, row.Forward)
			continue
		}
		path, line := filepath.Join(root, filepath.FromSlash(row.Owner)), row.Site
		if row.ForwardSite != "" {
			path, line = root, row.ForwardSite
		}
		if !forwardNamedNear(t, path, line, "metasystem "+row.Forward) {
			t.Errorf("guide row %s: no message within %d lines of %s names metasystem %s", row.Code, forwardWindowRadius, line, row.Forward)
		}
	}
	if gaps > h1GuideDefectCeiling {
		t.Errorf("%d guide rows name no public command, above the ceiling %d", gaps, h1GuideDefectCeiling)
	}
	if gaps < h1GuideDefectCeiling {
		t.Errorf("%d guide rows name no public command, below the ceiling %d: lower h1GuideDefectCeiling", gaps, h1GuideDefectCeiling)
	}
}

func forwardNamedNear(t *testing.T, directory, site, needle string) bool {
	t.Helper()
	file, lineText, ok := strings.Cut(site, ":")
	line, err := strconv.Atoi(lineText)
	if !ok || err != nil || line < 1 {
		t.Errorf("site %q does not name a positive line", site)
		return false
	}
	data, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(file)))
	if err != nil {
		t.Errorf("read %s: %v", site, err)
		return false
	}
	lines := strings.Split(string(data), "\n")
	first, last := max(0, line-1-forwardWindowRadius), min(len(lines), line+forwardWindowRadius)
	return first < last && strings.Contains(strings.Join(lines[first:last], "\n"), needle)
}

// The witness fails on the shapes it exists to catch.
func TestH1WitnessRejectsADeadEnd(t *testing.T) {
	t.Parallel()
	if (Row{Shape: Question}).Standing() != "" {
		t.Fatal("a Question row without a declared standing must have none")
	}
	if (Row{Shape: Agent}).Standing() != StandingAgent || (Row{Shape: Identity}).Standing() != StandingIdentity {
		t.Fatal("Agent and Identity rows imply their standing")
	}
	if (Row{Shape: Agent, H1: StandingGuide}).Standing() != StandingGuide {
		t.Fatal("an explicit standing overrides the shape")
	}
	for _, form := range []string{"goal", "goal budget G", "land.sh --carried", "Goal budget"} {
		if forwardForm.MatchString(form) {
			t.Errorf("forward form %q must be refused", form)
		}
	}
}
