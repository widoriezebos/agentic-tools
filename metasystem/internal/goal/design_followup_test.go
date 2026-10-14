package goal

import "testing"

func TestDesignFollowUpRequiresRecordedHumanOpening(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		file *GoalFile
		want bool
	}{
		{"absent source", nil, false},
		{"legacy human origin", &GoalFile{Origin: OriginHuman}, true},
		{"person opened main", &GoalFile{Origin: OriginMain, History: []HistoryLine{{Verb: "open", Actor: "human:fixture-person"}}}, true},
		{"seat opened main", &GoalFile{Origin: OriginMain, History: []HistoryLine{{Verb: "open", Actor: "seat:fixture-seat"}}}, false},
		{"person edited main", &GoalFile{Origin: OriginMain, History: []HistoryLine{{Verb: "edit", Actor: "human:fixture-person"}}}, false},
		{"missing person", &GoalFile{Origin: OriginMain, History: []HistoryLine{{Verb: "open", Actor: "human: "}}}, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			if got := scenario.file.CanOpenDesignFollowUp(); got != scenario.want {
				t.Fatalf("follow-up authorization = %v, want %v", got, scenario.want)
			}
		})
	}
}
