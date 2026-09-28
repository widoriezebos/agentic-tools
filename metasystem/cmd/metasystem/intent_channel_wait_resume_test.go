package main

import (
	"encoding/json"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// TestWorkWaitResumesAChannelWaitThroughTheChannelOwner: a durable wait on
// a channel answer is resumed by the one public resume form, work wait
// wait:ID, which hands it to the channel wait owner (it polls the provider)
// in this process; the waiter records name that same form.
func TestWorkWaitResumesAChannelWaitThroughTheChannelOwner(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	root := b.root()
	waitID := strings.Repeat("d", 32)
	row := metarun.Waiter{
		SchemaVersion: 2, WaitID: waitID, Nonce: strings.Repeat("e", 32), Kind: "goal", TargetID: "goal-a", OwnerDigest: "owner-a", State: "pending",
		Selector: metarun.WaitSelector{Kind: "goal", TargetID: "goal-a", GoalID: "goal-a", Event: "human-act", Verb: "answer", Question: "question-a", After: strings.Repeat("a", 40), Poll: "channel"},
	}
	if err := os.MkdirAll(metarun.WaitersDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metarun.WaiterPath(root, row.Kind, row.TargetID, row.OwnerDigest), body, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := metarun.WaitResumeCommand(row); got != "metasystem work wait wait:"+waitID {
		t.Fatalf("a channel wait's resume command = %q", got)
	}
	owners := b.owners()
	owners.delivery = &intentDeliveryOwners{executable: func() (string, error) { return "/fake/metasystem", nil }}
	calls := defaultIntentOwnerCalls()
	var resumed [][]string
	calls.channelWait = func(caller processIdentity, _ string, stdout, _ io.Writer, args []string) int {
		resumed = append(resumed, args)
		_, _ = io.WriteString(stdout, "accepted answer text\n")
		return 0
	}
	owners.delivery.calls = calls
	code, result := b.runJSON(owners, "work", "wait", "wait:"+waitID)
	if code != 0 || result.Outcome != intentConfirmed || len(resumed) != 1 || !slices.Contains(resumed[0], "--resume") || !slices.Contains(resumed[0], waitID) {
		t.Fatalf("work wait of a channel wait = %d %+v, channel owner calls %v", code, result, resumed)
	}
}
