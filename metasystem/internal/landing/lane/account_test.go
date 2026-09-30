package lane

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLaneAccountResolvesOnlyTheHostLane (U11b): the lane's accounting
// identity is derived from its registered checkout and is answered only for
// a control root inside that checkout; no record, another checkout or a gone
// lane is an error, never an identity (fail closed).
func TestLaneAccountResolvesOnlyTheHostLane(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	if _, err := ResolveAccount(home, root); err == nil || !strings.Contains(err.Error(), "LANE_ACCOUNT_UNRESOLVED") {
		t.Fatalf("no lane: %v", err)
	}
	if _, err := Resolve(home, root, "m1e", laneNow, true); err != nil {
		t.Fatal(err)
	}
	module := filepath.Join(root, "metasystem")
	if err := os.MkdirAll(module, 0o755); err != nil {
		t.Fatal(err)
	}
	id, err := ResolveAccount(home, module)
	if err != nil || !IsAccount(id) || id != AccountID(root) || len(id) != len("lane:")+12 {
		t.Fatalf("account=%q err=%v", id, err)
	}
	if _, err := ResolveAccount(home, t.TempDir()); err == nil || !strings.Contains(err.Error(), "is not inside the landing lane") {
		t.Fatalf("another checkout: %v", err)
	}
	if IsAccount("goal-a") || IsAccount("change:abc") {
		t.Fatal("a goal or change id reads as a lane account")
	}
}
