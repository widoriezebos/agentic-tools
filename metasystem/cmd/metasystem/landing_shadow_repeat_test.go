package main

import (
	"bytes"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

func TestLandingShadowRefreshRespectsVerbOutcome(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		outcome string
		code    int
	}{{intentUnchanged, 0}, {intentConfirmed, 0}, {intentFailed, 2}, {intentRefused, 1}} {
		t.Run(tc.outcome, func(t *testing.T) {
			t.Parallel()
			b := newResolveVerbFixture(t)
			command, ok := findIntentAction("landing", "status")
			if !ok {
				t.Fatal("landing status is missing")
			}
			command.run = func(inv *intentInvocation) int {
				inv.laneCallerObserved, inv.laneCallerPerson = true, false
				return inv.render(intentResult{Outcome: tc.outcome, code: tc.code, Summary: "fixture result"})
			}
			before := idemTreeDigest(t, b.install)
			var stdout, stderr bytes.Buffer
			if code := runIntentIn(command, nil, &stdout, &stderr, b.root, b.owners); code != tc.code {
				t.Fatalf("verb exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
			}
			_, exists, err := state.Read(plain.Dir(b.install))
			if err != nil || exists != (tc.outcome == intentConfirmed) {
				t.Fatalf("outcome=%s shadow exists=%v error=%v", tc.outcome, exists, err)
			}
			if tc.outcome != intentConfirmed {
				idemSameTree(t, tc.outcome+" landing verb", before, idemTreeDigest(t, b.install))
			}
		})
	}
}
