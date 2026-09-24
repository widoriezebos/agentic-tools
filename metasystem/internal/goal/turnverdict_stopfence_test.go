package goal

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

func TestTurnVerdictDoesNotBlockAndDescribesTheDurableStopPhase(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		phase     string
		survivors []stopfence.Survivor
		want      func(string) string
	}{
		{
			name: "completed", phase: stopfence.PhaseStopped,
			want: func(root string) string {
				return "the metasystem is stopped for " + root + " since 2026-09-07T00:00:00Z, by stop pid 71; run: metasystem arm --repo " + root
			},
		},
		{
			name: "unfinished", phase: stopfence.PhaseStopping,
			want: func(root string) string {
				return "stop unfinished for " + root + " since 2026-09-07T00:00:00Z by stop pid 71; run: metasystem stop --repo " + root
			},
		},
		{
			name: "incomplete", phase: stopfence.PhaseStopIncomplete,
			survivors: []stopfence.Survivor{{Component: "run", ID: "one", Reason: "survived"}},
			want: func(root string) string {
				return "stop incomplete for " + root + " since 2026-09-07T00:00:00Z by stop pid 71; 1 unresolved entries from the last stop; run: metasystem stop --repo " + root
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			store := legacyVerdictStore(t, false)
			root := store.Root
			if err := stopfence.Write(root, stopfence.Record{
				State: stopfence.StateClosed, Phase: test.phase, Generation: 1,
				ChangedAt: "2026-09-07T00:00:00Z", Checkout: root,
				By:         stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 71, PidStartedAt: 70}},
				NotStopped: test.survivors,
			}); err != nil {
				t.Fatal(err)
			}
			verdict, err := legacyTurnVerdict(store, ScanResult{}, "session", "", "main")
			if err != nil {
				t.Fatal(err)
			}
			if verdict.ShouldBlock || verdict.LedgerStatus != "stopped" || verdict.Display != test.want(root) {
				t.Fatalf("%s verdict = %#v", test.name, verdict)
			}
			if verdict.Facts == nil || verdict.Facts.Verdict.LedgerStatus != "stopped" ||
				verdict.Facts.FullDisplay != test.want(root) || verdict.Facts.Work.ReadSucceeded ||
				verdict.Facts.Work.Selection != "unknown" || len(verdict.Facts.Actions) != 0 {
				t.Fatalf("%s stopped presentation facts = %#v", test.name, verdict.Facts)
			}
		})
	}
}
