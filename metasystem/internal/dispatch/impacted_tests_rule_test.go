package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	path := filepath.Join("..", "..", "scripts", "agents", "templates", "brief.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s cannot be read: %v", path, err)
	}
	if !strings.Contains(string(data), ImpactedTestsRule) {
		t.Errorf("%s does not carry the impacted-tests rule", path)
	}
}
