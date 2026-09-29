package lease

import (
	"strings"
	"testing"
)

// The landing gate's clock is the holder's (g1-s70 D3, S70-07): the resident
// steward finds nothing due and lands nothing, because the holder check every
// landing makes before it prepares refuses a steward caller, whoever holds
// the checkout.
func TestALandingInTheStewardsNameIsRefusedByTheHolderCheck(t *testing.T) {
	root := t.TempDir()
	bin := stageStewardInstall(t, root)
	announceLiveChild(t, root) // a session holds the checkout
	pid := spawnAndSettle(t, bin)
	stageTerminalFact(t, root, pid, false)
	if got, err := Classify(root, pid); err != nil || got.Class != ClassSteward {
		t.Fatalf("the stand-in does not classify as the steward: %+v %v", got, err)
	}
	_, err := RequireHolder(root, pid, nil)
	if err == nil || !strings.Contains(err.Error(), "OWNED-ELSEWHERE") {
		t.Fatalf("the steward passed the holder check a landing makes: %v", err)
	}
}
