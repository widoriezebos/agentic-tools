package main

import (
	"strings"
	"testing"
)

func TestGoalStatusRetainsBudgetAndProcessReport(t *testing.T) {
	t.Parallel()
	for _, built := range []bool{false, true} {
		name := "no work"
		if built {
			name = "built work"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			if built {
				if code, result, output := processEvidenceBuild(bed); code != 0 {
					t.Fatalf("build: %d %+v %s", code, result, output)
				}
			}
			for _, argv := range [][]string{{"goal", "status", bed.id}, {"work", "status", bed.id}} {
				code, result, output := bed.work(argv...)
				if code != 0 {
					t.Fatalf("%v: %d %+v %s", argv, code, result, output)
				}
				data := resultData(t, result)
				if data["budget"] == nil || data["processReport"] == nil {
					t.Fatalf("%v lost budget or process report: %+v", argv, data)
				}
				code, text, stderr := bed.run(bed.workOwners(), argv...)
				if code != 0 || !strings.Contains(text, "budget: 4h/4/240m/2/2 (spent this claim)") || !strings.Contains(text, "spent: elapsed 5m0s of 4h") || !strings.Contains(text, "Recorded work:") {
					t.Fatalf("%v lost visible budget or process report: %d %s %s", argv, code, text, stderr)
				}
				if built && !strings.Contains(text, "evidence:") {
					t.Fatalf("%v lost the unit status: %s", argv, text)
				}
			}
		})
	}
}
