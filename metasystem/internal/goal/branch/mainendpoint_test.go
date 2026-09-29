package branch

import (
	"errors"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// A goal branch lands only on main: an endpoint serving another branch is
// refused by name, a resolution error passes through, and main is kept.
func TestMainEndpointServesMainOnly(t *testing.T) {
	t.Parallel()
	if _, err := mainEndpoint(goal.Endpoint{Remote: "origin", Branch: "refs/heads/dev"}, nil); err == nil ||
		!strings.Contains(err.Error(), "GOAL_BRANCH_ENDPOINT_UNSUPPORTED: endpoint refs/heads/dev is not refs/heads/main") {
		t.Fatalf("a dev endpoint = %v; want the refusal", err)
	}
	resolveErr := errors.New("unreadable configuration")
	if _, err := mainEndpoint(goal.Endpoint{Branch: "refs/heads/main"}, resolveErr); !errors.Is(err, resolveErr) {
		t.Fatalf("a resolution error = %v; want it passed through", err)
	}
	endpoint, err := mainEndpoint(goal.Endpoint{Root: "/seat", Remote: "origin", Branch: "refs/heads/main"}, nil)
	if err != nil || endpoint.Root != "/seat" || endpoint.Remote != "origin" {
		t.Fatalf("main endpoint = %+v, %v", endpoint, err)
	}
}
