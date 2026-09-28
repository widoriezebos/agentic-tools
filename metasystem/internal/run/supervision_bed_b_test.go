package run

import (
	"errors"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

// TestSupBCreationVerbsRefuseTheClosedFenceWithTheExactTwoLines ports the
// stop-fence run rows of supervision-fixtures part B: run launch, register
// and adopt under a completed stop return exactly the stopped sentence and
// the agent-free start remedy, and leave no run record or creation claim.
func TestSupBCreationVerbsRefuseTheClosedFenceWithTheExactTwoLines(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		call func(*Store) error
	}{
		{name: "launch", call: func(s *Store) error {
			_, err := s.Launch(mainCaller, LaunchParams{Id: "stop-fence-launch", Kind: "custom", Log: "run.log"})
			return err
		}},
		{name: "register", call: func(s *Store) error {
			return s.Register(mainCaller, LaunchParams{Id: "stop-fence-register", Kind: "custom", Log: "run.log"}, 41, "")
		}},
		{name: "adopt", call: func(s *Store) error { return s.Adopt(mainCaller, "stop-fence-adopt", 42) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			s := testStore(t)
			record := stopfence.Record{State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 5,
				ChangedAt: "2026-09-27T12:30:00Z", Checkout: s.Root,
				By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 72, PidStartedAt: 70}}}
			s.FenceRead = func(string) (stopfence.Record, error) { return record, nil }
			err := test.call(s)
			want := "the metasystem is stopped for " + s.Root + " since 2026-09-27T12:30:00Z, by stop pid 72\n" +
				"at an agent-free terminal, run: metasystem system start --repo " + s.Root
			var stopped *StoppedError
			if !errors.As(err, &stopped) || err.Error() != want {
				t.Fatalf("closed-fence %s = %v, want %q", test.name, err, want)
			}
			if records, unreadable := s.List(); len(records) != 0 || len(unreadable) != 0 {
				t.Fatalf("closed fence published records: %+v unreadable=%v", records, unreadable)
			}
			if claims, claimErr := stopfence.Claims(s.Root, 5); claimErr != nil || len(claims) != 0 {
				t.Fatalf("closed fence left creation claims %+v: %v", claims, claimErr)
			}
		})
	}
}
