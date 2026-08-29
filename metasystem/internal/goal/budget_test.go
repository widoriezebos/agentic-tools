package goal

import (
	"strings"
	"testing"
	"time"
)

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

func TestElapsedBreachDurationAppliesGraceAtTheStopBoundary(t *testing.T) {
	budget := Budget{ElapsedLimit: "1m", AttemptLimit: 1, ReservedJobMinutesLimit: 1, ActiveJobLimit: 1}
	for _, test := range []struct {
		percent uint64
		want    time.Duration
	}{
		{percent: 0, want: time.Minute},
		{percent: 50, want: 90 * time.Second},
		{percent: 200, want: 3 * time.Minute},
	} {
		got, err := budget.ElapsedBreachDuration(test.percent)
		if err != nil || got != test.want {
			t.Fatalf("grace %d: duration=%s want=%s err=%v", test.percent, got, test.want, err)
		}
	}

	overflow := budget
	overflow.ElapsedLimit = "153722867m"
	if _, err := overflow.ElapsedBreachDuration(200); err == nil || !strings.Contains(err.Error(), "duration range") {
		t.Fatalf("an unrepresentable stop boundary was accepted: %v", err)
	}
}

func TestBudgetValidationNamesEveryInvalidLimit(t *testing.T) {
	tests := []struct {
		name     string
		budget   Budget
		fragment string
	}{
		{name: "elapsed", budget: Budget{AttemptLimit: 1, ReservedJobMinutesLimit: 1, ActiveJobLimit: 1}, fragment: "elapsedLimit"},
		{name: "attempts", budget: Budget{ElapsedLimit: "1h", ReservedJobMinutesLimit: 1, ActiveJobLimit: 1}, fragment: "attemptLimit"},
		{name: "reserved minutes", budget: Budget{ElapsedLimit: "1h", AttemptLimit: 1, ActiveJobLimit: 1}, fragment: "reservedJobMinutesLimit"},
		{name: "active jobs", budget: Budget{ElapsedLimit: "1h", AttemptLimit: 1, ReservedJobMinutesLimit: 1}, fragment: "activeJobLimit"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.budget.Validate(); err == nil || !strings.Contains(err.Error(), test.fragment) {
				t.Fatalf("invalid budget was not refused by field name: %v", err)
			}
		})
	}

	newBudgetTests := []struct {
		name                                  string
		elapsed                               string
		attempts, reservedMinutes, activeJobs int64
		fragment                              string
	}{
		{name: "elapsed", elapsed: "1.5h", attempts: 1, reservedMinutes: 1, activeJobs: 1, fragment: "elapsedLimit"},
		{name: "attempts", elapsed: "1h", attempts: 0, reservedMinutes: 1, activeJobs: 1, fragment: "attemptLimit"},
		{name: "reserved minutes", elapsed: "1h", attempts: 1, reservedMinutes: 0, activeJobs: 1, fragment: "reservedJobMinutesLimit"},
		{name: "active jobs", elapsed: "1h", attempts: 1, reservedMinutes: 1, activeJobs: 0, fragment: "activeJobLimit"},
	}
	for _, test := range newBudgetTests {
		t.Run("new "+test.name, func(t *testing.T) {
			_, err := NewBudget(test.elapsed, test.attempts, test.reservedMinutes, test.activeJobs)
			if err == nil || !strings.Contains(err.Error(), test.fragment) {
				t.Fatalf("invalid command budget was not refused by field name: %v", err)
			}
		})
	}
}

func TestStoredBudgetRequiresCompleteNumericLimits(t *testing.T) {
	valid := map[string]string{
		"elapsedLimit": "8h", "attemptLimit": "2",
		"reservedJobMinutesLimit": "60", "activeJobLimit": "1",
	}
	if budget, err := budgetFromIntentArgs(valid); err != nil || budget.ElapsedLimit != "1d" {
		t.Fatalf("valid stored budget did not parse canonically: %+v %v", budget, err)
	}

	tests := []struct {
		name     string
		change   func(map[string]string)
		fragment string
	}{
		{name: "incomplete", change: func(args map[string]string) { delete(args, "activeJobLimit") }, fragment: "all four fields"},
		{name: "attempts", change: func(args map[string]string) { args["attemptLimit"] = "many" }, fragment: "attemptLimit"},
		{name: "reserved minutes", change: func(args map[string]string) { args["reservedJobMinutesLimit"] = "many" }, fragment: "reservedJobMinutesLimit"},
		{name: "active jobs", change: func(args map[string]string) { args["activeJobLimit"] = "many" }, fragment: "activeJobLimit"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := map[string]string{}
			for key, value := range valid {
				args[key] = value
			}
			test.change(args)
			if _, err := budgetFromIntentArgs(args); err == nil || !strings.Contains(err.Error(), test.fragment) {
				t.Fatalf("stored budget was not refused by field name: %v", err)
			}
		})
	}
}
