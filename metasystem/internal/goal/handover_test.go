package goal

import (
	"strings"
	"testing"
)

func handedOverGoal(id, machine, lineage string) *GoalFile {
	file := vGoal(id, StateClaimed)
	file.Claimed.Machine = machine
	file.Claimed.Lineage = lineage
	file.Claimed.HandedOver = HandedOver{FromMachine: "seat-a", FromLineage: "lineage-a", FromEpoch: 7, Batch: "batch-a"}
	return file
}

func handedOverTree(files ...*GoalFile) *TreeGoals {
	live := make(map[string]*GoalFile, len(files))
	for _, file := range files {
		live[file.Id] = file
	}
	return &TreeGoals{Root: vRoot(), Live: live, Done: map[string]*GoalFile{}}
}

func TestHandedOverClaimRoundTripsWithoutChangingExistingClaims(t *testing.T) {
	existing := RenderFile(claimedGolden())
	parsed, problems := ParseFile(existing)
	if len(problems) != 0 || parsed.Claimed.HandedOver.present() || strings.Contains(string(existing), "- HandedOver:") || string(RenderFile(parsed)) != string(existing) {
		t.Fatalf("existing claim bytes changed: handedOver=%+v problems=%v", parsed.Claimed.HandedOver, problems)
	}

	handed := handedOverGoal("handed-round-trip", "landing", "landing-lineage")
	rendered := RenderFile(handed)
	parsed, problems = ParseFile(rendered)
	if len(problems) != 0 || parsed.Claimed.HandedOver != handed.Claimed.HandedOver || string(RenderFile(parsed)) != string(rendered) {
		t.Fatalf("handed-over claim did not round-trip: claim=%+v problems=%v\n%s", parsed.Claimed, problems, rendered)
	}
}

func TestHandedOverRecordRequiresEveryCoordinate(t *testing.T) {
	rendered := string(RenderFile(handedOverGoal("handed-grammar", "landing", "landing-lineage")))
	for _, test := range []struct{ old, replacement, want string }{
		{"fromMachine=seat-a ", "", "missing fromMachine="},
		{"fromLineage=lineage-a ", "", "missing fromLineage="},
		{"fromEpoch=7", "fromEpoch=0", `fromEpoch="0" is not a positive integer`},
		{"batch=batch-a", "batch=", "missing batch="},
	} {
		mutated := strings.Replace(rendered, test.old, test.replacement, 1)
		if _, problems := ParseFile([]byte(withFreshIntegrity(mutated))); !problemsContain(problems, test.want) {
			t.Fatalf("mutation %q did not refuse with %q: %v", test.old, test.want, problems)
		}
	}
}

func TestHandedOverClaimsUseOneDerivedLandingPair(t *testing.T) {
	expectProblem(t, ValidateTree(handedOverTree(vGoal("seat-one", StateClaimed), vGoal("seat-two", StateClaimed))), "quota is one claim per machine")

	first := handedOverGoal("handed-one", "landing", "landing-lineage")
	second := handedOverGoal("handed-two", "landing", "landing-lineage")
	if problems := ValidateTree(handedOverTree(first, second)); len(problems) != 0 {
		t.Fatalf("one derived landing pair may hold both claims: %v", problems)
	}

	second.Claimed.Lineage = "other-lineage"
	expectProblem(t, ValidateTree(handedOverTree(first, second)), "all handed-over claims must share one holder pair")
}

func TestHandedOverClaimsDoNotConsumeLandingSlots(t *testing.T) {
	first := handedOverGoal("landing-one", "landing", "landing-lineage")
	second := handedOverGoal("landing-two", "landing", "landing-lineage")
	for _, file := range []*GoalFile{first, second} {
		file.Landing = &LandingRecord{At: "2026-08-20T10:06:00Z", Opid: "01J5X0000000000000000000B1-mac-a-1a2b3c4d"}
	}
	if problems := ValidateTree(handedOverTree(first, second)); len(problems) != 0 {
		t.Fatalf("handed-over claims do not consume the landing pair's slots: %v", problems)
	}
}

func TestHandedOverClaimGuards(t *testing.T) {
	tests := []struct {
		name, want string
		mutate     func(*GoalFile)
	}{
		{"self handover", "cannot hand a claim to its current holder pair", func(file *GoalFile) {
			file.Claimed.HandedOver.FromMachine, file.Claimed.HandedOver.FromLineage = file.Claimed.Machine, file.Claimed.Lineage
		}},
		{"unclaimed state", "HandedOver requires state claimed", func(file *GoalFile) { file.State = StateQueued }},
		{"empty batch", "HandedOver requires a non-empty batch", func(file *GoalFile) { file.Claimed.HandedOver.Batch = "" }},
		{"empty source machine", "HandedOver requires a complete source pair and epoch", func(file *GoalFile) { file.Claimed.HandedOver.FromMachine = "" }},
		{"empty source lineage", "HandedOver requires a complete source pair and epoch", func(file *GoalFile) { file.Claimed.HandedOver.FromLineage = "" }},
		{"zero source epoch", "HandedOver requires a complete source pair and epoch", func(file *GoalFile) { file.Claimed.HandedOver.FromEpoch = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := handedOverGoal("guard", "landing", "landing-lineage")
			test.mutate(file)
			problems := ValidateTree(handedOverTree(file))
			if !strings.Contains(stringProblems(problems), test.want) {
				t.Fatalf("missing %q problem: %v", test.want, problems)
			}
		})
	}
}

func stringProblems(problems []Problem) string {
	parts := make([]string, len(problems))
	for i, problem := range problems {
		parts[i] = string(problem)
	}
	return strings.Join(parts, "\n")
}
