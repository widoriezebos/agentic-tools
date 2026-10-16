package main

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestLandingExecutionCompositionIncludesFourUnknownRoles(t *testing.T) {
	t.Parallel()
	seams := landingExecutionSeams(plain.ProveSeams{})
	got := seams.Executions(t.TempDir())
	if len(got) != 4 {
		t.Fatalf("roles: %+v", got)
	}
	for _, role := range []string{"owner", "watcher", "reaper", "runner"} {
		if got[role].Digest != "" || got[role].Cause == "" {
			t.Fatalf("missing UNKNOWN for %s: %+v", role, got)
		}
	}
	seams.Executions = func(string) map[string]enginepin.Execution {
		return map[string]enginepin.Execution{"fixture": {Digest: "fixture"}}
	}
	if got := landingExecutionSeams(seams).Executions(""); got["fixture"].Digest != "fixture" {
		t.Fatalf("overrode injected reader: %+v", got)
	}
}
