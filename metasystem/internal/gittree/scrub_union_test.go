package gittree

import (
	"slices"
	"testing"
)

// The one steering list is the union of every copy the runner once kept:
// gittree's own, goal's, channel's and the hook launcher's shell unset line.
func TestScrubbedEnvironFromStripsTheWholeUnion(t *testing.T) {
	steering := []string{
		"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_CEILING_DIRECTORIES",
		"GIT_DISCOVERY_ACROSS_FILESYSTEM", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
		"GIT_NAMESPACE", "GIT_REPLACE_REF_BASE", "GIT_GRAFT_FILE", "GIT_SHALLOW_FILE",
		"GIT_IMPLICIT_WORK_TREE", "GIT_PREFIX", "BASH_ENV", "ENV",
		"GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT", "GIT_CONFIG_GLOBAL",
		"GIT_CONFIG_SYSTEM", "GIT_CONFIG_NOSYSTEM", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0",
	}
	environment := []string{"PATH=/usr/bin", "HOME=/home/someone", "GIT_AUTHOR_NAME=kept"}
	for _, name := range steering {
		environment = append(environment, name+"=steer")
	}
	got := ScrubbedEnvironFrom(environment, "GIT_INDEX_FILE=/own/index")
	want := []string{"PATH=/usr/bin", "HOME=/home/someone", "GIT_AUTHOR_NAME=kept", "GIT_INDEX_FILE=/own/index"}
	if !slices.Equal(got, want) {
		t.Fatalf("scrubbed environment = %q, want %q", got, want)
	}
}
