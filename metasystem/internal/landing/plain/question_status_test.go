package plain

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestStatusHeadlineNamesOpenLaneQuestion(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	q, err := channel.Ask(channel.AskRequest{RepoRoot: install, About: "lane", Kind: "other", Machine: "this-machine", Lineage: lane.AgentLineage, Facts: []string{"May I restore the checkout? More details follow."}, Now: bedNow})
	if err != nil {
		t.Fatal(err)
	}
	seams := ProveSeams{Machine: func(string) (string, error) { return "this-machine", nil }, Git: func(string, ...string) (string, error) { return "this-machine", nil }}
	root := install
	status := ReadStatus(t.TempDir(), lane.Record{Root: install, Install: install}, lane.View{Root: &root, Summary: "idle; its agent starts within one tick"}, seams)
	for _, want := range []string{"Waiting for a person's answer to question " + q.ID, " since ", "May I restore the checkout?", "metasystem question show channel:" + q.ID, "metasystem question withdraw channel:" + q.ID + " --reason TEXT"} {
		if !strings.Contains(status.Summary, want) {
			t.Fatalf("missing %q: %s", want, status.Summary)
		}
	}
	if strings.Contains(status.Summary, "More details") {
		t.Fatalf("headline includes more than first sentence: %s", status.Summary)
	}
	if _, err := channel.Withdraw(install, q.ID, "restored", nil, channel.DestinationConfig{}); err != nil {
		t.Fatal(err)
	}
	if status = ReadStatus(t.TempDir(), lane.Record{Root: install, Install: install}, lane.View{Root: &root, Summary: "idle"}, seams); status.Summary != "idle" {
		t.Fatalf("closed question holds status: %s", status.Summary)
	}
}
