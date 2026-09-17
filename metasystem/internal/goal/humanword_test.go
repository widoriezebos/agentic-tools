package goal

import "testing"

func TestNextStepNamesAPendingHumanWord(t *testing.T) {
	tests := []struct {
		name     string
		nextStep string
		want     bool
	}{
		{name: "ruling needed", nextStep: "RULING NEEDED: choose the retention period", want: true},
		{name: "waiting on the human", nextStep: "waiting on the human before publishing", want: true},
		{name: "named call", nextStep: "WAITING ON WIDO'S CALL about the rollout", want: true},
		{name: "named ruling", nextStep: "Waiting On Owner Ruling", want: true},
		{name: "named decision", nextStep: "WAITING ON OPERATOR DECISION", want: true},
		{name: "named answer", nextStep: "waiting on reviewer answer", want: true},
		{name: "named word", nextStep: "WAITING ON MAINTAINER WORD", want: true},
		{name: "question to the human", nextStep: "QUESTION TO THE HUMAN: which boundary applies?", want: true},
		{name: "park request", nextStep: "PARK REQUEST: external access is unavailable", want: true},
		{name: "park requested", nextStep: "Park Requested while the decision is pending", want: true},
		{name: "unicode before earlier marker", nextStep: "ı RULING NEEDED EARLIER: ordinary work", want: true},
		{name: "marker only in earlier entry", nextStep: "Implement the accepted option. EARLIER: RULING NEEDED", want: false},
		{name: "empty", nextStep: "", want: false},
		{name: "ordinary", nextStep: "Run the focused package tests.", want: false},
		{name: "more than one name", nextStep: "WAITING ON A HUMAN DECISION", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NextStepNamesAPendingHumanWord(test.nextStep); got != test.want {
				t.Fatalf("NextStepNamesAPendingHumanWord(%q) = %t, want %t", test.nextStep, got, test.want)
			}
		})
	}
}
