package goal

import (
	"slices"
	"testing"
)

// goal's git calls strip the runner's whole steering set, not a shorter copy.
func TestGoalGitEnvironmentStripsTheRunnersWholeSteeringSet(t *testing.T) {
	got := environWithoutGitSteeringFrom([]string{"PATH=/usr/bin", "GIT_NAMESPACE=x", "BASH_ENV=/x", "ENV=/x", "GIT_IMPLICIT_WORK_TREE=0", "GIT_DISCOVERY_ACROSS_FILESYSTEM=1"})
	if !slices.Equal(got, []string{"PATH=/usr/bin"}) {
		t.Fatalf("goal git environment kept steering variables: %q", got)
	}
}
