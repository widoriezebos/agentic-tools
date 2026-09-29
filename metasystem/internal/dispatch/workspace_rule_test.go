package dispatch

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/protocol"
)

// Every build brief tells the builder where a bed or a second tree goes
// (disk-lifetimes Part B 3.6): a registered workspace of the goal, released
// when done, never a copy of the checkout under /tmp or a clone.
func TestBuildBriefsSendBedsToAWorkspace(t *testing.T) {
	t.Parallel()
	data, err := protocol.Template("brief.md")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(strings.Fields(string(data)), " ")
	for _, want := range []string{"metasystem work workspace G", "--copy-of REV", "--release", "never copy the checkout into /tmp or clone it"} {
		if !strings.Contains(text, want) {
			t.Errorf("the brief template lacks %q", want)
		}
	}
}
