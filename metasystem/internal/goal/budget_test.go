package goal

import "testing"

func TestBudgetTupleIsCompletePositiveAndCanonical(t *testing.T) {
	budget, err := NewBudget("8h", 3, 180, 2)
	if err != nil {
		t.Fatal(err)
	}
	if budget.ElapsedLimit != "1d" || budget.AttemptLimit != 3 ||
		budget.ReservedJobMinutesLimit != 180 || budget.ActiveJobLimit != 2 {
		t.Fatalf("canonical tuple = %+v", budget)
	}
	for _, record := range []string{
		"elapsedLimit=4h attemptLimit=2 reservedJobMinutesLimit=60",
		"elapsedLimit=4h attemptLimit=2 reservedJobMinutesLimit=60 activeJobLimit=1 extra=1",
		"elapsedLimit=4h attemptLimit=0 reservedJobMinutesLimit=60 activeJobLimit=1",
	} {
		if _, err := parseBudgetRecord(record); err == nil {
			t.Fatalf("incomplete, extra, or non-positive tuple parsed: %q", record)
		}
	}
}
