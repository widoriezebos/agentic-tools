package channel

import (
	"slices"
	"strings"
	"testing"
)

// The report's git call strips the runner's whole steering set.
func TestReportGitEnvironmentStripsTheRunnersWholeSteeringSet(t *testing.T) {
	t.Setenv("GIT_NAMESPACE", "x")
	t.Setenv("BASH_ENV", "/x")
	t.Setenv("GIT_IMPLICIT_WORK_TREE", "0")
	for _, entry := range reportGitEnv() {
		name, _, _ := strings.Cut(entry, "=")
		if slices.Contains([]string{"GIT_NAMESPACE", "BASH_ENV", "GIT_IMPLICIT_WORK_TREE"}, name) {
			t.Fatalf("report git environment kept %s", entry)
		}
	}
}
