package run

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestWatchSpeaksOnEveryConclusion(t *testing.T) {
	s := testStore(t)
	prober := fakeProber{verdicts: map[int64]identity.Liveness{}, starts: map[int64]int64{}}
	s.Prober = prober
	nonce := launchOne(t, s, "spoken")
	prober.verdicts[101] = identity.Alive
	prober.starts[101] = 5000
	if err := s.Bind("spoken", nonce, 101, 101); err != nil {
		t.Fatal(err)
	}
	record, _ := s.Read("spoken")
	if err := s.WriteSidecar("spoken", record.Generation, record.LaunchNonce, 0); err != nil {
		t.Fatal(err)
	}
	prober.verdicts[101] = identity.Dead
	if _, err := s.Assess("spoken"); err != nil {
		t.Fatal(err)
	}
	// The waiter proves its own owner's liveness from the MainId's
	// pid-start encoding.
	// The waiter records its OWN process identity through the store's
	// prober; the fake must recognize the live test process.
	self := int64(os.Getpid())
	prober.verdicts[self] = identity.Alive
	prober.starts[self] = 1
	owner := mainCaller
	var out strings.Builder
	rc := s.Watch("spoken", owner, 20*time.Millisecond, &out)
	if rc != ExitGreen {
		t.Fatalf("watch rc = %d (%s)", rc, out.String())
	}
	if !strings.Contains(out.String(), "run spoken green rc=0") {
		t.Fatalf("the conclusion was not spoken: %q", out.String())
	}
	var silent strings.Builder
	if rc := s.Watch("no-such-run", owner, 20*time.Millisecond, &silent); rc != ExitNoRecord || !strings.Contains(silent.String(), "no-record") {
		t.Fatalf("a missing record must still speak: rc=%d %q", rc, silent.String())
	}
}
