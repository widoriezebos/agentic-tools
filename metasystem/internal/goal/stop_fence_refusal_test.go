package goal

import (
	"strings"
	"testing"
)

// A breach-stopped goal refuses done, release and the other acts that clear
// its claim; the refusal's second line is the exact command that clears the
// fence, after which the act is repeated.
func TestAFencedClaimRefusalNamesTheResume(t *testing.T) {
	t.Parallel()
	file := &GoalFile{Id: "stopped-goal", StopFence: &StopFence{StopID: "stop-stopped-goal-r3-f1"}}
	err := clearClaimBinding(file)
	if err == nil {
		t.Fatal("a fenced claim was cleared")
	}
	lines := strings.Split(err.Error(), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "only goal resume may clear its launch fence") ||
		!strings.HasPrefix(lines[1], "run: metasystem goal resume stopped-goal") || !strings.Contains(lines[1], "repeat") {
		t.Fatalf("refusal = %q", err.Error())
	}
}
