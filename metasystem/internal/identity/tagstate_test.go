package identity

import (
	"os"
	"strings"
	"testing"
)

func TestTagStateLadder(t *testing.T) {
	self := int64(os.Getpid())
	exact, state, err := (KernelProber{}).Probe(self)
	if err != nil || state != Alive || !exact.ArgvKnown {
		t.Skipf("cannot inspect own process: state=%v err=%v", state, err)
	}
	ownTag := strings.Join(exact.Argv, " ")
	if len(ownTag) > 12 {
		ownTag = ownTag[:12]
	}

	if got := TagState(KernelProber{}, self, ownTag); got != "live" {
		t.Fatalf("own pid with own argv fragment = %q, want live", got)
	}
	if got := TagState(KernelProber{}, self, "no-such-tag-xyzzy"); got != "stale" {
		t.Fatalf("own pid with foreign tag = %q, want stale", got)
	}
	if got := TagState(KernelProber{}, self, ""); got != "stale" {
		t.Fatalf("empty recorded tag proves no ownership = %q, want stale", got)
	}
	if got := TagState(KernelProber{}, 999999, "x"); got != "dead" {
		t.Fatalf("vanished pid = %q, want dead", got)
	}
	if got := TagState(KernelProber{}, 0, "x"); got != "dead" {
		t.Fatalf("invalid pid = %q, want dead", got)
	}
}
