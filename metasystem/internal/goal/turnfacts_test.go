package goal

import "testing"

func TestFreezeTurnVerdictFactsDoesNotMutateCallerScan(t *testing.T) {
	scan := ScanResult{
		Jobs: []JobFact{{Id: "job-1", MainId: "main-1", Ownership: "unknown"}},
		Runs: []RunFact{{Id: "run-1", MainId: "other", Ownership: "unknown"}},
	}
	facts := freezeTurnVerdictFacts("/installation", "session", "main-1", scan,
		ClaimableBudgetedWork{}, true, nil, Verdict{}, "display",
		&sessionState{LastTouched: "2026-09-13T12:00:00Z"}, TurnVerdictOptions{}, false)
	if scan.Jobs[0].Ownership != "unknown" || scan.Runs[0].Ownership != "unknown" {
		t.Fatalf("freeze changed its caller's scan: jobs=%+v runs=%+v", scan.Jobs, scan.Runs)
	}
	if facts.Scan.Jobs[0].Ownership != "owned" || facts.Scan.Runs[0].Ownership != "other" {
		t.Fatalf("frozen ownership was not classified: jobs=%+v runs=%+v", facts.Scan.Jobs, facts.Scan.Runs)
	}
}
