package launch

import (
	"fmt"
	"strings"
)

func RoundTask(tag string, round int, previous []byte, constraints string) ([]byte, error) {
	text := string(previous)
	switch round {
	case 2:
		markers := []string{"for revision 1 of", "review round 1 of 3", tag + "-critique-r1.md", "-design-r1.md", tag + "-design-brief-r1.md"}
		if err := requireTaskMarkers(text, markers); err != nil {
			return nil, err
		}
		text = replaceEachLine(text, "for revision 1 of", "for revision 2 of")
		text = replaceEachLine(text, "review round 1 of 3", "review round 2 of 3")
		text = replaceEachLine(text, tag+"-critique-r1.md", tag+"-critique-r2.md")
		text = replaceEachLine(text, "-design-r1.md", "-design-r2.md")
		text = replaceEachLine(text, tag+"-design-brief-r1.md", tag+"-design-brief-r2.md")
		old := "written against a checkout 65 code files behind this worktree's origin/main: a cited line number that is merely shifted is NOT a finding; a cited symbol, branch, file or behaviour that does not exist at origin/main IS."
		text = replaceEachLine(text, old, "revision 2 was written in this worktree against its own tree: a cited symbol, branch, file, line or behaviour that does not exist here IS a finding.")
		text += "\nRound 1's critique is artifacts/reports/" + tag + "-critique-r1.md in this worktree. The page's \"Fold of critique r1\" section claims FIXED, REFUTED or DEFERRED for each round-1 finding. Check every claim FIRST: a FIXED that does not actually fix the failure is material; a REFUTED whose file:line evidence is wrong is material; a DEFERRED that leaves a DONE condition unmet is material. Do not re-raise a round-1 finding that was correctly fixed or correctly refuted. Then attack the content the fold added, with the same attack list.\n"
	case 3:
		markers := []string{"for revision 2 of the design", "review round 2 of 3.", tag + "-critique-r2.md in this worktree's", "-design-r2.md (untracked", "Its brief is artifacts/reports/" + tag + "-design-brief-r2.md and", "The design was revision 2 was written", "Round 1's critique is"}
		if err := requireTaskMarkers(text, markers); err != nil {
			return nil, err
		}
		text = replaceEachLine(text, "for revision 2 of the design", "for revision 3 of the design")
		text = replaceEachLine(text, "review round 2 of 3.", "review round 3 of 3, the LAST round.")
		text = replaceEachLine(text, tag+"-critique-r2.md in this worktree's", tag+"-critique-r3.md in this worktree's")
		text = replaceEachLine(text, "-design-r2.md (untracked", "-design-r3.md (untracked")
		text = replaceEachLine(text, "Its brief is artifacts/reports/"+tag+"-design-brief-r2.md and", "Its brief is artifacts/reports/"+tag+"-design-brief-r3.md (the seat's fold prompt with its binding seat constraints C1-"+constraints+"; check conformance to them, do not re-argue them) and")
		text = replaceEachLine(text, "The design was revision 2 was written", "The design revision 3 was written")
		if index := strings.Index(text, "Round 1's critique is"); index >= 0 {
			text = text[:index]
		}
		text += "Round 2's critique is artifacts/reports/" + tag + "-critique-r2.md in this worktree, and round 1's is " + tag + "-critique-r1.md beside it. The page's \"Fold of critique r2\" section claims FIXED, REFUTED, DEFERRED or ESCALATED for each round-2 finding. Check every claim FIRST, running each critic's named mutation against the r3 text:\n- A FIXED that does not actually fix the failure is material.\n- A REFUTED whose file:line evidence is wrong is material.\n- A DEFERRED or ESCALATED that leaves a DONE condition unmet without an Escalation section is material.\nDo not re-raise a finding that was correctly fixed or refuted in an earlier round. Then attack the content the fold added, using the same attack list. This is the last round: separate what blocks a build (material) from what a builder can settle inside a unit (not material), and be exact about which is which.\n"
	default:
		return nil, fmt.Errorf("round must be 2 or 3")
	}
	return []byte(text), nil
}

func requireTaskMarkers(text string, markers []string) error {
	for _, marker := range markers {
		if !strings.Contains(text, marker) {
			return fmt.Errorf("previous task is missing required text: %s", marker)
		}
	}
	return nil
}

func replaceEachLine(text, old, replacement string) string {
	lines := strings.SplitAfter(text, "\n")
	for index := range lines {
		lines[index] = strings.Replace(lines[index], old, replacement, 1)
	}
	return strings.Join(lines, "")
}
