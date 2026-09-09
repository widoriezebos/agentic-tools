package dispatch

import (
	"strings"
	"testing"
)

func TestRuntimeAndFollowupCannotLowerTestPolicy(t *testing.T) {
	area, err := TestingRequirement("goal-a", "area")
	if err != nil {
		t.Fatal(err)
	}
	full, err := TestingRequirement("goal-a", "full")
	if err != nil {
		t.Fatal(err)
	}
	for name, requirement := range map[string]string{"initial": area, "follow-up": full} {
		if !strings.Contains(requirement, "goal-a") || !strings.Contains(requirement, "metasystem test") ||
			!strings.Contains(requirement, "collect every independent result") ||
			strings.Contains(requirement, "dispatch-fixtures.sh") || strings.Contains(requirement, "goal-cli-fixtures.sh") {
			t.Fatalf("%s requirement does not preserve the shared policy: %q", name, requirement)
		}
	}
	if _, err := TestingRequirement("goal-a", "shallow"); err == nil {
		t.Fatal("unknown gate width lowered the shared testing requirement")
	}
}
