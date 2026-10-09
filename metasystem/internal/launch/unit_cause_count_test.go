package launch

import "testing"

func TestUnitCountedRoundsRequireDemonstratedOwn(t *testing.T) {
	t.Parallel()
	record := UnitRunRecord{Rounds: []UnitRound{{Cause: "own"}, {Cause: "unclassified"}, {Cause: "environment"}, {Cause: "main"}, {Cause: "flake"}, {Cause: "other"}, {}}}
	counted, uncounted := countedRounds(record)
	if counted != 1 || uncounted != 6 {
		t.Fatalf("only demonstrated own consumes an attempt: counted=%d uncounted=%d", counted, uncounted)
	}
}
