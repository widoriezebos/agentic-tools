package dispatch

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/protocol"
)

// Every build brief carries the delegate cache rule (disk-lifetimes A7,
// 3.3): the round builds in the machine delegate cache, never points GOCACHE
// elsewhere, never runs go clean and never strips the delegate markers; no
// brief still promises a per-chain cache.
func TestBuildBriefsCarryTheDelegateCacheRule(t *testing.T) {
	t.Parallel()
	for _, want := range []string{"never point GOCACHE", "never run go clean", "never strip", "METASYSTEM_HOOK_DELEGATE_"} {
		if !strings.Contains(BuildCacheRule, want) {
			t.Errorf("BuildCacheRule lacks %q: %q", want, BuildCacheRule)
		}
	}
	requirement, err := TestingRequirement("goal-a", "area")
	if err != nil {
		t.Fatal(err)
	}
	path := "protocol:templates/brief.md"
	data, err := protocol.Template("brief.md")
	if err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"testing requirement": requirement, path: strings.Join(strings.Fields(string(data)), " ")} {
		if !strings.Contains(text, BuildCacheRule) {
			t.Errorf("%s does not carry the delegate cache rule", name)
		}
		if strings.Contains(text, "chain's cache") || strings.Contains(text, "for the whole chain") {
			t.Errorf("%s still promises a per-chain cache", name)
		}
	}
}
