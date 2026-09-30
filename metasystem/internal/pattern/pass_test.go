package pattern

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// Only the steward of the checkout the host's lane record names runs
// patterns (D6): a seat's steward on the same host reads nothing and keeps
// no state, and without a lane on the host no pattern runs at all.
func TestPassRunsOnlyInTheLaneSteward(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeBatch(batchID("p"), refused(bedStart, 5), b.seat)
	seatRepo := filepath.Join(b.seat, "metasystem")
	if err := os.MkdirAll(seatRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := b.pass.Run(seatRepo, bedStart.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(steward.PatternStatePath(seatRepo)); !os.IsNotExist(err) || len(b.sent) != 0 {
		t.Fatalf("a seat's steward ran the patterns: %v %q", err, b.sent)
	}
	unregistered := b.pass
	unregistered.Home = func() (string, error) { return t.TempDir(), nil }
	if err := unregistered.Run(b.repo, bedStart.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(steward.PatternStatePath(b.repo)); !os.IsNotExist(err) {
		t.Fatalf("a host without a lane ran the patterns: %v", err)
	}
	b.cycle(bedStart.Add(10 * time.Minute))
	if len(b.open()) != 1 {
		t.Fatalf("the lane's steward did not run the patterns: %+v", b.episodes())
	}
}
