package launch

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
)

// A launched agent outlives the request that started it: the cache context
// it would inherit names an issuer that cannot stay its live ancestor, so
// every Go command the agent ran would be refused. The child gets none; the
// rest of the parent's environment and the launch's own settings stay.
func TestLaunchedChildInheritsNoCacheContext(t *testing.T) {
	t.Parallel()
	parent := []string{"PATH=/fixture/bin", "GOCACHE=/fixture/cache",
		gocache.ContextEnv + `={"domain":"engine","gocache":"/fixture/cache","staticcheck":"/fixture/static","issuer":"pid=1;micro=1"}`}
	child := childEnvironment(parent, []string{"METASYSTEM_OWNER_LINEAGE=steward-seat"})
	if value, set := environmentValue(child, gocache.ContextEnv); set {
		t.Fatalf("the child inherited the cache context %q; environment=%q", value, child)
	}
	for key, want := range map[string]string{"PATH": "/fixture/bin", "GOCACHE": "/fixture/cache", "METASYSTEM_OWNER_LINEAGE": "steward-seat"} {
		if got, set := environmentValue(child, key); !set || got != want {
			t.Fatalf("child environment %s = %q set=%t, want %q; environment=%q", key, got, set, want, child)
		}
	}
}
