package dispatch

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/protocol"
)

func TestBuildBriefsCarryTheImpactedTestsRule(t *testing.T) {
	t.Parallel()

	for _, width := range []string{"area", "full"} {
		requirement, err := TestingRequirement("goal-a", width)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(requirement, ImpactedTestsRule) {
			t.Errorf("%s testing requirement does not carry the impacted-tests rule: %q", width, requirement)
		}
		if strings.Contains(requirement, "focused tests") {
			t.Errorf("%s testing requirement still asks for undefined focused tests: %q", width, requirement)
		}
	}

	// Decision B5 of briefs-carry-their-rules replaced the template's executable
	// test instruction with the owned Check slot, so a second template cannot
	// order a broad run: the rule lives in the testing requirement, not the template.
	if data, err := protocol.Template("brief.md"); err != nil {
		t.Fatalf("protocol:templates/brief.md cannot be read: %v", err)
	} else if strings.Contains(string(data), "run only the frozen command") == false {
		t.Errorf("protocol:templates/brief.md lost the owned Check instruction (Decision B5)")
	}
}
