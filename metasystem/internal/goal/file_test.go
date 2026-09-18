package goal

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func claimedGolden() *GoalFile {
	return &GoalFile{
		Id:       "backlog-git-sync",
		State:    StateClaimed,
		Intent:   "Multiple machines work the backlog in parallel with git as the sync",
		Origin:   "main",
		NextStep: "Implement against the obligation matrix, fixtures first",
		OpenedAt: "2026-08-20T00:31:00Z",
		Revision: 3,
		Blocked:  []string{"wall-o15-head-accounting"},
		Arc:      "",
		Budget: &Budget{
			ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 2,
		},
		Claimed: &ClaimRecord{Machine: "mac-studio", Lineage: "session-a", At: "2026-08-20T00:35:00Z", Revision: 2},
		History: []HistoryLine{
			{At: "2026-08-20T00:31:00Z", Opid: "01J5X0000000000000000000A0-mac-studio-1a2b3c4d", Verb: "open", Actor: "mac-studio+session-a", Targets: []string{"backlog-git-sync"}, Keep: -1},
			{At: "2026-08-20T00:35:00Z", Opid: "01J5X0000000000000000000B1-mac-studio-1a2b3c4d", Verb: "claim", Actor: "mac-studio+session-a", Targets: []string{"backlog-git-sync"}, Keep: -1},
			{At: "2026-08-20T01:00:00Z", Opid: "01J5X0000000000000000000C2-mac-studio-1a2b3c4d", Verb: "edit", Actor: "mac-studio+session-a", Targets: []string{"backlog-git-sync"}, Keep: -1},
		},
	}
}

func TestGoldenClaimedFileRoundTrips(t *testing.T) {
	t.Parallel()
	golden := claimedGolden()
	bytes1 := RenderFile(golden)
	parsed, problems := ParseFile(bytes1)
	if len(problems) != 0 {
		t.Fatalf("golden claimed file must parse clean, got %v", problems)
	}
	bytes2 := RenderFile(parsed)
	if string(bytes1) != string(bytes2) {
		t.Fatalf("render/parse/render is not a fixed point:\n%s\n---\n%s", bytes1, bytes2)
	}
	if parsed.Claimed == nil || parsed.Claimed.Machine != "mac-studio" {
		t.Fatalf("claim record lost: %+v", parsed.Claimed)
	}
	if parsed.Claimed.Revision != 2 || parsed.Budget == nil || parsed.Budget.ReservedJobMinutesLimit != 240 {
		t.Fatalf("budget tuple or claim-revision binding lost: claimed=%+v budget=%+v", parsed.Claimed, parsed.Budget)
	}
	if parsed.Claimed.AccountingRevision != 2 || !strings.Contains(string(bytes1), "revision=2 accountingRevision=2") {
		t.Fatalf("accounting revision did not round-trip beside the claim revision: %+v\n%s", parsed.Claimed, bytes1)
	}
	legacy := strings.Replace(string(bytes1), " accountingRevision=2", "", 1)
	legacyParsed, legacyProblems := ParseFile([]byte(withFreshIntegrity(legacy)))
	if len(legacyProblems) != 0 || legacyParsed.Claimed.AccountingRevision != legacyParsed.Claimed.Revision {
		t.Fatalf("legacy claim did not default accounting revision to claim revision: claim=%+v problems=%v", legacyParsed.Claimed, legacyProblems)
	}
	if parsed.Revision != 3 || len(parsed.History) != 3 {
		t.Fatalf("revision/history lost: rev=%d len=%d", parsed.Revision, len(parsed.History))
	}
}

func TestEmptyReadItemListPreservesRecordBytesAndDigest(t *testing.T) {
	t.Parallel()
	file := claimedGolden()
	before := RenderFile(file)
	file.ReadItems = []ReadItem{}
	after := RenderFile(file)
	if string(after) != string(before) || IntegrityDigest(before[:strings.LastIndex(string(before), "Integrity:")]) != IntegrityDigest(after[:strings.LastIndex(string(after), "Integrity:")]) {
		t.Fatalf("empty read-item list changed an existing record:\n%s\n---\n%s", before, after)
	}
}

func TestStopSurfaceMovePermissionRoundTripsAndRejectsOtherValues(t *testing.T) {
	t.Parallel()
	file := claimedGolden()
	file.StopSurfaceMoves = true
	rendered := RenderFile(file)
	parsed, problems := ParseFile(rendered)
	if len(problems) != 0 || !parsed.StopSurfaceMoves || string(RenderFile(parsed)) != string(rendered) {
		t.Fatalf("StopSurface permission did not round-trip: permission=%v problems=%v\n%s", parsed.StopSurfaceMoves, problems, rendered)
	}

	invalid := strings.Replace(string(rendered), "- StopSurface: moves", "- StopSurface: anything", 1)
	if _, problems := ParseFile([]byte(withFreshIntegrity(invalid))); !problemsContain(problems, `StopSurface "anything" is not moves`) {
		t.Fatalf("invalid StopSurface permission problems = %v", problems)
	}
}

func TestReadItemsRoundTripBesideNextStep(t *testing.T) {
	t.Parallel()
	file := claimedGolden()
	file.ReadItems = []ReadItem{
		{ID: "critic-1", Read: "critic", Text: "Name the edge case.", State: ReadItemOpen, AddedAt: "2026-09-17T10:00:00Z"},
		{ID: "critic-2", Read: "critic", Text: "Intentional behavior.", State: ReadItemAccepted, AddedAt: "2026-09-17T10:00:00Z", ChangedAt: "2026-09-17T11:00:00Z", ClosingReference: "not a defect"},
	}
	rendered := RenderFile(file)
	parsed, problems := ParseFile(rendered)
	if len(problems) != 0 || string(RenderFile(parsed)) != string(rendered) {
		t.Fatalf("read-item record did not round-trip: problems=%v\n%s", problems, rendered)
	}
	text := string(rendered)
	next := strings.Index(text, "- Next step:")
	block := strings.Index(text, "Open read items (fix unit critic): 1")
	opened := strings.Index(text, "- OpenedAt:")
	if next < 0 || block < next || opened < block || parsed.NextStep != file.NextStep {
		t.Fatalf("read fix unit is not immediately beside the unchanged Next step:\n%s", rendered)
	}
}

func TestApprovalEpisodeRevisionGrammar(t *testing.T) {
	t.Parallel()
	file := approvedGoalFixture(vGoal("approval-episode", StateQueued), testBudget())
	rendered := RenderFile(file)
	if !strings.Contains(string(rendered), " episode=2") {
		t.Fatalf("Approved line does not render its episode revision:\n%s", rendered)
	}
	parsed, problems := ParseFile(rendered)
	if len(problems) != 0 || parsed.Approved == nil || parsed.Approved.EpisodeRevision != 2 || string(RenderFile(parsed)) != string(rendered) {
		t.Fatalf("episode revision did not parse and round-trip: approved=%+v problems=%v", parsed.Approved, problems)
	}

	invalid := *file
	invalid.Approved = new(ApprovalRecord)
	*invalid.Approved = *file.Approved
	invalid.Approved.EpisodeRevision = 1
	if err := invalid.ValidateApprovalRecord(); err == nil || !strings.Contains(err.Error(), "approve or set-budget") {
		t.Fatalf("episode revision naming a non-budget event was accepted: %v", err)
	}

	malformed := strings.Replace(string(rendered), " episode=2", " episode=0", 1)
	if _, malformedProblems := ParseFile([]byte(withFreshIntegrity(malformed))); len(malformedProblems) == 0 {
		t.Fatal("episode=0 parsed as a valid approval coordinate")
	}
	empty := strings.Replace(string(rendered), " episode=2", " episode=", 1)
	if _, emptyProblems := ParseFile([]byte(withFreshIntegrity(empty))); len(emptyProblems) == 0 {
		t.Fatal("empty episode= parsed as a legacy-absent approval coordinate")
	}
}

func TestBudgetEpisodeRevisionLegacyMinimum(t *testing.T) {
	t.Parallel()
	budget := testBudget()
	base := &GoalFile{Budget: &budget, Approved: &ApprovalRecord{Revision: 9}}
	tests := []struct {
		name string
		edit func(*GoalFile)
		want uint64
	}{
		{name: "approval only", want: 9},
		{name: "claimed accounting is older", edit: func(f *GoalFile) { f.Claimed = &ClaimRecord{AccountingRevision: 7} }, want: 7},
		{name: "kept accounting is oldest", edit: func(f *GoalFile) { f.Episode = &EpisodeRecord{AccountingRevision: 5} }, want: 5},
		{name: "explicit episode wins", edit: func(f *GoalFile) {
			f.Approved.EpisodeRevision = 4
			f.Claimed = &ClaimRecord{AccountingRevision: 2}
		}, want: 4},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := *base
			file.Approved = new(ApprovalRecord)
			*file.Approved = *base.Approved
			if test.edit != nil {
				test.edit(&file)
			}
			if got := BudgetEpisodeRevision(&file); got != test.want {
				t.Fatalf("BudgetEpisodeRevision = %d, want %d", got, test.want)
			}
		})
	}
	if BudgetEpisodeRevision(nil) != 0 || BudgetEpisodeRevision(&GoalFile{Approved: base.Approved}) != 0 || BudgetEpisodeRevision(&GoalFile{Budget: &budget}) != 0 {
		t.Fatal("an unapproved or unbudgeted goal acquired a consumption episode")
	}
}

func TestLegacyApprovalWithReadItemsRoundTripsByteIdentical(t *testing.T) {
	t.Parallel()
	file := approvedGoalFixture(vGoal("legacy-approval-read", StateQueued), testBudget())
	file.Approved.EpisodeRevision = 0
	file.ReadItems = []ReadItem{{ID: "critic-1", Read: "critic", Text: "Keep the legacy grammar beside this open item.", State: ReadItemOpen, AddedAt: "2026-09-17T10:00:00Z"}}
	legacy := RenderFile(file)
	if strings.Contains(string(legacy), " episode=") {
		t.Fatalf("legacy approval unexpectedly rendered the new field:\n%s", legacy)
	}
	parsed, problems := ParseFile(legacy)
	if len(problems) != 0 || parsed.Approved.EpisodeRevision != 0 || len(parsed.ReadItems) != 1 || string(RenderFile(parsed)) != string(legacy) {
		t.Fatalf("legacy approval with read items did not round-trip byte-identically: approved=%+v reads=%+v problems=%v", parsed.Approved, parsed.ReadItems, problems)
	}
}

func episodeGolden() *GoalFile {
	f := claimedGolden()
	f.Revision = 5
	f.History = append(f.History,
		HistoryLine{At: "2026-08-20T01:30:00Z", Opid: "01J5X0000000000000000000C3-mac-studio-1a2b3c4d", Verb: "set-obligation", Actor: "human:wido", Targets: []string{f.Id}, Keep: -1},
		HistoryLine{At: "2026-08-20T02:00:00Z", Opid: "01J5X0000000000000000000C4-mac-studio-1a2b3c4d", Verb: "set-budget", Actor: "human:wido", Targets: []string{f.Id}, Keep: -1},
	)
	f.Claimed.At = f.History[4].At
	f.Claimed.Revision = 5
	f.Claimed.AccountingRevision = 5
	f.Claimed.EpisodeAt = f.History[1].At
	f.Claimed.EpisodeRevision = 2
	return f
}

func TestClaimedEpisodeRoundTrip(t *testing.T) {
	t.Parallel()
	for _, obligationRevision := range []uint64{0, 4} {
		t.Run(fmt.Sprintf("obligation revision %d", obligationRevision), func(t *testing.T) {
			file := episodeGolden()
			file.Claimed.EpisodeObligationRevision = obligationRevision
			rendered := RenderFile(file)
			parsed, problems := ParseFile(rendered)
			if len(problems) != 0 || parsed.Claimed == nil || *parsed.Claimed != *file.Claimed || string(RenderFile(parsed)) != string(rendered) {
				t.Fatalf("episode fields did not round-trip: claim=%+v problems=%v\n%s", parsed.Claimed, problems, rendered)
			}
			if obligationRevision == 0 && strings.Contains(string(rendered), "episodeObligationRevision=") {
				t.Fatalf("zero inherited obligation revision was rendered: %s", rendered)
			}
		})
	}
}

func TestEpisodeObligationRevisionParse(t *testing.T) {
	t.Parallel()
	legacy := string(RenderFile(claimedGolden()))
	thirdOnly := strings.Replace(legacy, " accountingRevision=2", " accountingRevision=2 episodeObligationRevision=1", 1)
	if _, problems := ParseFile([]byte(withFreshIntegrity(thirdOnly))); !problemsContain(problems, "Claimed episodeObligationRevision requires the episode binding (episodeAt and episodeRevision)") {
		t.Fatalf("third episode key without its pair did not refuse exactly: %v", problems)
	}
	file := episodeGolden()
	file.Claimed.EpisodeObligationRevision = 4
	parsed, problems := ParseFile(RenderFile(file))
	if len(problems) != 0 || parsed.Claimed.EpisodeObligationRevision != 4 {
		t.Fatalf("third episode key beside its pair did not parse: claim=%+v problems=%v", parsed.Claimed, problems)
	}
}

func TestEpisodeBindingContradictionsRefuse(t *testing.T) {
	t.Parallel()
	base := string(RenderFile(claimedGolden()))
	for _, test := range []struct {
		name, suffix, want string
	}{
		{name: "lone episode time", suffix: " episodeAt=2026-08-20T00:35:00Z", want: "Claimed episode binding is incomplete (episodeAt and episodeRevision travel together)"},
		{name: "lone episode revision", suffix: " episodeRevision=2", want: "Claimed episode binding is incomplete (episodeAt and episodeRevision travel together)"},
		{name: "empty episode revision", suffix: " episodeAt=2026-08-20T00:35:00Z episodeRevision=", want: `Claimed episodeRevision="" is not a positive integer`},
		{name: "zero episode revision", suffix: " episodeAt=2026-08-20T00:35:00Z episodeRevision=0", want: `Claimed episodeRevision="0" is not a positive integer`},
		{name: "empty inherited obligation", suffix: " episodeAt=2026-08-20T00:35:00Z episodeRevision=2 episodeObligationRevision=", want: `Claimed episodeObligationRevision="" is not a positive integer`},
		{name: "zero inherited obligation", suffix: " episodeAt=2026-08-20T00:35:00Z episodeRevision=2 episodeObligationRevision=0", want: `Claimed episodeObligationRevision="0" is not a positive integer`},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := strings.Replace(base, " accountingRevision=2", " accountingRevision=2"+test.suffix, 1)
			if _, problems := ParseFile([]byte(withFreshIntegrity(raw))); !problemsContain(problems, test.want) {
				t.Fatalf("malformed episode keys did not refuse with %q: %v", test.want, problems)
			}
		})
	}

	for _, test := range []struct {
		name   string
		mutate func(*GoalFile)
		want   string
	}{
		{name: "episode later than claim revision", mutate: func(f *GoalFile) { f.Claimed.EpisodeRevision = 6 }, want: "claimed episodeRevision=6 is later than claim revision=5"},
		{name: "malformed episode time", mutate: func(f *GoalFile) { f.Claimed.EpisodeAt = "not-a-time" }, want: "the claim episode timestamp is malformed"},
		{name: "episode contradicts history", mutate: func(f *GoalFile) { f.Claimed.EpisodeAt = "2026-08-20T00:34:00Z" }, want: "claimed episodeAt=2026-08-20T00:34:00Z contradicts History revision=2 at=2026-08-20T00:35:00Z"},
		{name: "episode later than claim time", mutate: func(f *GoalFile) { f.History[1].At = "2026-08-20T03:00:00Z"; f.Claimed.EpisodeAt = f.History[1].At }, want: "claimed episodeAt=2026-08-20T03:00:00Z is later than claimed at=2026-08-20T02:00:00Z"},
		{name: "inherited obligation equals episode", mutate: func(f *GoalFile) { f.Claimed.EpisodeObligationRevision = 2 }, want: "claimed episodeObligationRevision=2 must be later than episodeRevision=2 and earlier than claim revision=5"},
		{name: "inherited obligation below episode", mutate: func(f *GoalFile) { f.Claimed.EpisodeObligationRevision = 1 }, want: "claimed episodeObligationRevision=1 must be later than episodeRevision=2 and earlier than claim revision=5"},
		{name: "inherited obligation equals claim", mutate: func(f *GoalFile) { f.Claimed.EpisodeObligationRevision = 5 }, want: "claimed episodeObligationRevision=5 must be later than episodeRevision=2 and earlier than claim revision=5"},
		{name: "inherited obligation above claim", mutate: func(f *GoalFile) { f.Claimed.EpisodeObligationRevision = 6 }, want: "claimed episodeObligationRevision=6 must be later than episodeRevision=2 and earlier than claim revision=5"},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := episodeGolden()
			test.mutate(file)
			if _, problems := ParseFile(RenderFile(file)); !problemsContain(problems, "BUDGET_UNKNOWN "+test.want) {
				t.Fatalf("contradictory episode binding did not refuse with %q: %v", test.want, problems)
			}
		})
	}
	valid := episodeGolden()
	valid.Claimed.EpisodeObligationRevision = 3
	if _, problems := ParseFile(RenderFile(valid)); len(problems) != 0 {
		t.Fatalf("a strict interior inherited obligation revision did not remain readable: %v", problems)
	}
	missing := episodeGolden()
	missing.Claimed.EpisodeAt = ""
	missing.Claimed.EpisodeRevision = 0
	missing.Claimed.EpisodeObligationRevision = 3
	if err := missing.ValidateClaimRevision(); err == nil || err.Error() != "claimed episodeObligationRevision=3 has no episode binding" {
		t.Fatalf("in-memory inherited identity without its episode did not refuse exactly: %v", err)
	}
}

func TestSTR2P2A05ZeroClaimRevisionObligationIsAProblemNotAPanic(t *testing.T) {
	t.Parallel()
	f := vGoal("zero-claim-obligation", StateClaimed)
	f.Budget = &Budget{ElapsedLimit: "1h", AttemptLimit: 1, ReservedJobMinutesLimit: 5, ActiveJobLimit: 1, ReviewRoundLimit: 0}
	f.Claimed = &ClaimRecord{Machine: "mac-a", Lineage: "m1", At: f.OpenedAt}
	o := testGovernedObligation(ObligationDraft)
	o.Revision = 1
	o.BudgetRevision = 1
	f.Obligation = &o
	_, problems := ParseFile(RenderFile(f))
	if joined := fmt.Sprint(problems); !strings.Contains(joined, "obligation budgetRevision=1 does not bind the claimed budget revision") {
		t.Fatalf("zero claim revision did not produce the existing obligation problem: %v", problems)
	}
}

func TestLegacyFourMemberBudgetUsesGoalTierReviewRounds(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		tier uint8
		want int64
	}{
		{name: "tierless uses tier three", tier: 0, want: 3},
		{name: "tier one", tier: 1, want: 0},
		{name: "tier two", tier: 2, want: 2},
		{name: "tier three", tier: 3, want: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := vGoal("legacy-budget", StateQueued)
			file.Tier = test.tier
			file.Budget = &Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 1, ReviewRoundLimit: 3}
			raw := strings.Replace(string(RenderFile(file)), " reviewRoundLimit=3", "", 1)
			parsed, problems := ParseFile([]byte(withFreshIntegrity(raw)))
			if len(problems) != 0 || parsed == nil || parsed.Budget == nil || parsed.Budget.ReviewRoundLimit != test.want || !parsed.legacyFourBudget {
				t.Fatalf("legacy tier %d budget parsed as %+v with problems %v; want review rounds %d", test.tier, parsed, problems, test.want)
			}
		})
	}
}

func TestLegacyBudgetAndNormApprovalShareInferredReviewRounds(t *testing.T) {
	t.Parallel()
	file := vGoal("both-legacy-rounds", StateQueued)
	file.Budget = &Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 1, ReviewRoundLimit: 3}
	file.NormApproval = &GoalNormApprovalClaim{ApprovedRef: "R-legacy", Minutes: 240, ReviewRounds: 3, GoalRevision: 1}
	raw := string(RenderFile(file))
	raw = strings.Replace(raw, " reviewRoundLimit=3", "", 1)
	raw = strings.Replace(raw, " reviewRounds=3", "", 1)
	parsed, problems := ParseFile([]byte(withFreshIntegrity(raw)))
	if len(problems) != 0 || parsed.Budget.ReviewRoundLimit != 3 || parsed.NormApproval.ReviewRounds != 3 ||
		!parsed.legacyFourBudget || !parsed.legacyThreeNormApproval {
		t.Fatalf("both-legacy review rounds did not share the tier-three box: parsed=%+v problems=%v", parsed, problems)
	}
	reparsed, problems := ParseFile(RenderFile(parsed))
	if len(problems) != 0 || reparsed.legacyFourBudget || reparsed.legacyThreeNormApproval {
		t.Fatalf("the next write did not render both inferred members explicitly: parsed=%+v problems=%v", reparsed, problems)
	}
}

func TestMixedLegacyReviewRoundMemberUsesExplicitValue(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name         string
		legacyBudget bool
	}{
		{name: "legacy budget uses explicit norm approval", legacyBudget: true},
		{name: "legacy norm approval uses explicit budget", legacyBudget: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := vGoal("mixed-legacy-rounds", StateQueued)
			file.Tier = 2
			file.Budget = &Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 1, ReviewRoundLimit: 7}
			file.NormApproval = &GoalNormApprovalClaim{ApprovedRef: "R-explicit", Minutes: 240, ReviewRounds: 7, GoalRevision: 1}
			raw := string(RenderFile(file))
			if test.legacyBudget {
				raw = strings.Replace(raw, " reviewRoundLimit=7", "", 1)
			} else {
				raw = strings.Replace(raw, " reviewRounds=7", "", 1)
			}
			parsed, problems := ParseFile([]byte(withFreshIntegrity(raw)))
			if len(problems) != 0 || parsed.Budget.ReviewRoundLimit != 7 || parsed.NormApproval.ReviewRounds != 7 {
				t.Fatalf("mixed legacy record did not preserve the explicit seven-round value: parsed=%+v problems=%v", parsed, problems)
			}
			if parsed.legacyFourBudget != test.legacyBudget || parsed.legacyThreeNormApproval == test.legacyBudget {
				t.Fatalf("mixed legacy markers do not identify only the missing member: budget=%v norm=%v", parsed.legacyFourBudget, parsed.legacyThreeNormApproval)
			}
		})
	}
}

func TestSTR3Gap04ObligationRoundTrip(t *testing.T) {
	t.Parallel()
	f := claimedGolden()
	f.ReviewObligations = []ReviewObligation{{
		Finding: "F-1", Chain: "critic-root", Artifact: `NEW metasystem/a path/quoted "name".go`,
		Test: `prove: result=ok and "quoted"`, State: "open",
	}}
	parsed, problems := ParseFile(RenderFile(f))
	if len(problems) != 0 {
		t.Fatalf("quoted obligation did not parse: %v", problems)
	}
	if len(parsed.ReviewObligations) != 1 || parsed.ReviewObligations[0] != f.ReviewObligations[0] {
		t.Fatalf("obligation changed: got %+v want %+v", parsed.ReviewObligations, f.ReviewObligations)
	}
	rendered := string(RenderFile(f))
	start := strings.Index(rendered, " test=")
	end := strings.Index(rendered[start:], " state=")
	if start < 0 || end < 0 {
		t.Fatalf("rendered obligation has no quoted test field: %s", rendered)
	}
	broken := rendered[:start] + ` test="unterminated` + rendered[start+end:]
	_, problems = ParseFile([]byte(broken))
	if !problemsContain(problems, "line ") || !problemsContain(problems, "must be one quoted string") {
		t.Fatalf("malformed quoted obligation did not name its line: %v", problems)
	}
}

func TestSTR3GapDischargeSelect(t *testing.T) {
	t.Parallel()
	obligations := []ReviewObligation{
		{Finding: "F-1", Chain: "chain-a", State: "open"},
		{Finding: "F-1", Chain: "chain-b", State: "open"},
	}
	index, err := reviewObligationMatch(obligations, "F-1", "chain-b")
	if err != nil || index != 1 {
		t.Fatalf("chain-qualified selection = %d, %v", index, err)
	}
	obligations[index].State = "discharged"
	if obligations[0].State != "open" || obligations[1].State != "discharged" {
		t.Fatalf("selection changed the wrong obligation: %+v", obligations)
	}
	if _, err := reviewObligationMatch(obligations, "F-1", "missing"); err == nil || !strings.Contains(err.Error(), "no such obligation") {
		t.Fatalf("missing selection = %v", err)
	}
	duplicate := append(obligations, ReviewObligation{Finding: "F-1", Chain: "chain-a"})
	if _, err := reviewObligationMatch(duplicate, "F-1", "chain-a"); err == nil || !strings.Contains(err.Error(), "ambiguous obligation") {
		t.Fatalf("ambiguous selection = %v", err)
	}
}

func TestLabelsParseRawAndUnlabeledFilesStayUnchanged(t *testing.T) {
	t.Parallel()
	f := claimedGolden()
	unlabeled := string(RenderFile(f))
	if strings.Contains(unlabeled, "- Labels:") {
		t.Fatal("an unlabeled goal does not gain a Labels line")
	}

	f.Labels = []string{"zeta", "alpha", "zeta"}
	parsed, problems := ParseFile(RenderFile(f))
	if len(problems) != 0 {
		t.Fatalf("raw lawful labels parse: %v", problems)
	}
	if got := strings.Join(parsed.Labels, ","); got != "zeta,alpha,zeta" {
		t.Fatalf("parsing preserves hand-written order and duplicates, got %q", got)
	}

	f.Labels = []string{"Bad_Label"}
	_, problems = ParseFile(RenderFile(f))
	if !problemsContain(problems, `must match ^[a-z][a-z0-9-]{0,31}$`) {
		t.Fatalf("the one label grammar refuses by name: %v", problems)
	}
}

func TestGoldenArchivedFileCarriesExplicitDoneState(t *testing.T) {
	t.Parallel()
	done := &GoalFile{
		Id:       "custody-death-proof",
		State:    StateDone,
		Intent:   "Prove custody death is detected",
		Origin:   "main",
		Conclude: "Landed with the supervision chain; witness in the suite",
		OpenedAt: "2026-08-20T00:40:00Z",
		Revision: 2,
		History: []HistoryLine{
			{At: "2026-08-20T00:40:00Z", Opid: "01J5X0000000000000000000D3-mac-studio-1a2b3c4d", Verb: "open", Actor: "mac-studio+session-a", Keep: -1},
			{At: "2026-08-20T02:00:00Z", Opid: "01J5X0000000000000000000E4-mac-studio-1a2b3c4d", Verb: "done", Actor: "mac-studio+session-a", Keep: -1},
		},
	}
	parsed, problems := ParseFile(RenderFile(done))
	if len(problems) != 0 {
		t.Fatalf("golden archived file must parse clean, got %v", problems)
	}
	if parsed.State != StateDone || parsed.Conclude == "" {
		t.Fatalf("archived file must carry State: done and Concluded, got %+v", parsed)
	}
}

func TestAbandonRecordMustBindItsEvent(t *testing.T) {
	t.Parallel()
	fixture := func() *GoalFile {
		file := vGoal("bound-abandon", StateAbandoned)
		file.Revision = 2
		file.History = append(file.History, HistoryLine{
			At: "2026-08-20T11:00:00Z", Opid: "01J5X0000000000000000000C0-mac-a-1a2b3c4d",
			Verb: "abandon", Actor: "human:Wido", Targets: []string{"bound-abandon"}, Keep: -1,
			Reason: "the pursuit has stopped",
		})
		file.Abandoned = &AbandonRecord{
			By: "human:Wido", At: "2026-08-20T11:00:00Z", Revision: 2,
			Opid: "01J5X0000000000000000000C0-mac-a-1a2b3c4d", Because: "the pursuit has stopped",
		}
		return file
	}
	for _, test := range []struct {
		name   string
		mutate func(*GoalFile)
	}{
		{name: "human", mutate: func(file *GoalFile) { file.Abandoned.By = "human:Else" }},
		{name: "timestamp", mutate: func(file *GoalFile) { file.Abandoned.At = "2026-08-20T11:01:00Z" }},
		{name: "operation", mutate: func(file *GoalFile) { file.Abandoned.Opid = "01J5X0000000000000000000C1-mac-a-1a2b3c4d" }},
		{name: "revision points to the wrong verb", mutate: func(file *GoalFile) { file.Abandoned.Revision = 1 }},
		{name: "displaced", mutate: func(file *GoalFile) { file.History[1].Displaced = "mac-b+lin-2@2026-08-20T11:00:00Z" }},
		{name: "stop identifier", mutate: func(file *GoalFile) { file.History[1].StopID = "stop-fixture" }},
		{name: "carried successor", mutate: func(file *GoalFile) { file.History[1].Carried = "successor" }},
		{name: "reason", mutate: func(file *GoalFile) { file.Abandoned.Because = "a different reason" }},
		{name: "first target", mutate: func(file *GoalFile) { file.History[1].Targets = []string{"survivor", "bound-abandon"} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := fixture()
			test.mutate(file)
			_, problems := ParseFile(RenderFile(file))
			if len(problems) != 1 || !problemsContain(problems, "Abandoned record does not bind its abandon History event") {
				t.Fatalf("mismatch did not produce exactly the binding problem: %v", problems)
			}
		})
	}

	carriedFixture := func(successors ...string) *GoalFile {
		file := fixture()
		for index, successor := range successors {
			file.History = append(file.History, HistoryLine{
				At:   fmt.Sprintf("2026-08-20T11:%02d:00Z", index+1),
				Opid: fmt.Sprintf("01J5X0000000000000000000D%d-mac-a-1a2b3c4d", index),
				Verb: "carry", Actor: "human:Wido", Targets: []string{"bound-abandon"},
				Carried: successor, Keep: -1, Reason: "successor corrected",
			})
		}
		file.Revision = uint64(len(file.History))
		if len(successors) != 0 {
			file.Abandoned.Carried = successors[len(successors)-1]
		}
		return file
	}
	for _, test := range []struct {
		name string
		file *GoalFile
		want string
	}{
		{name: "one later carry binds", file: carriedFixture("successor-one")},
		{name: "newest of two later carries binds", file: carriedFixture("successor-one", "successor-two")},
		{name: "a carried field without a line refuses", file: func() *GoalFile {
			file := fixture()
			file.Abandoned.Carried = "unrecorded-successor"
			return file
		}(), want: "Abandoned record does not bind its abandon History event"},
		{name: "a carry before this abandonment is ignored", file: func() *GoalFile {
			file := fixture()
			currentAbandon := file.History[1]
			file.History = append(file.History[:1],
				HistoryLine{At: "2026-08-20T10:00:00Z", Opid: "01J5X0000000000000000000E0-mac-a-1a2b3c4d", Verb: "abandon", Actor: "human:Wido", Targets: []string{"bound-abandon"}, Keep: -1, Reason: "earlier abandonment"},
				HistoryLine{At: "2026-08-20T10:10:00Z", Opid: "01J5X0000000000000000000E1-mac-a-1a2b3c4d", Verb: "carry", Actor: "human:Wido", Targets: []string{"bound-abandon"}, Carried: "old-successor", Keep: -1, Reason: "earlier successor"},
				HistoryLine{At: "2026-08-20T10:20:00Z", Opid: "01J5X0000000000000000000E2-mac-a-1a2b3c4d", Verb: "reopen", Actor: "human:Wido", Targets: []string{"bound-abandon"}, Keep: -1},
				currentAbandon,
			)
			file.Revision = uint64(len(file.History))
			file.Abandoned.Revision = file.Revision
			file.Abandoned.Carried = "old-successor"
			return file
		}(), want: "Abandoned record does not bind its abandon History event"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, problems := ParseFile(RenderFile(test.file))
			if test.want == "" && len(problems) != 0 {
				t.Fatalf("valid binding refused: %v", problems)
			}
			if test.want != "" && !problemsContain(problems, test.want) {
				t.Fatalf("missing %q in %v", test.want, problems)
			}
		})
	}
}

func TestHistoryKeysStopIdAndCarriedOnlyOnAbandonLines(t *testing.T) {
	t.Parallel()
	base := vGoal("history-keys", StateQueued)
	base.Revision = 2
	base.History = append(base.History, HistoryLine{
		At: "2026-08-20T11:00:00Z", Opid: "01J5X0000000000000000000C0-mac-a-1a2b3c4d",
		Verb: "claim", Actor: "mac-a+lin-1", Targets: []string{"history-keys"}, Keep: -1,
		Reason: "fixture",
	})
	raw := strings.Replace(string(RenderFile(base)), " reason=fixture", " stopId=stop-fixture reason=fixture", 1)
	_, problems := ParseFile([]byte(withFreshIntegrity(raw)))
	if !problemsContain(problems, "claim line carries stopId=, which only an abandon or carry line may") {
		t.Fatalf("stopId on claim did not get the verb-specific problem: %v", problems)
	}

	base.History[1].Verb = "done"
	raw = strings.Replace(string(RenderFile(base)), " reason=fixture", " carried=successor reason=fixture", 1)
	_, problems = ParseFile([]byte(withFreshIntegrity(raw)))
	if !problemsContain(problems, "done line carries carried=, which only an abandon or carry line may") {
		t.Fatalf("carried on done did not get the verb-specific problem: %v", problems)
	}

	base.History[1].Verb = "carry"
	raw = strings.Replace(string(RenderFile(base)), " reason=fixture", " carried=successor reason=fixture", 1)
	_, problems = ParseFile([]byte(withFreshIntegrity(raw)))
	if !problemsContain(problems, "carry line on a goal that was not abandoned at that line") {
		t.Fatalf("carry on a never-abandoned goal was accepted: %v", problems)
	}

	reopened := string(abandonedFixtureBytes("history-keys"))
	reopened = strings.Replace(reopened, " reason=the pursuit has stopped", " carried=successor reason=the pursuit has stopped", 1)
	reopened = strings.Replace(reopened, "\nIntegrity:", "\n- 2026-08-20T12:00:00Z 01J5X0000000000000000000C1-mac-a-1a2b3c4d reopen actor=human:Wido targets=history-keys\nIntegrity:", 1)
	reopened = strings.Replace(reopened, "- Revision: 2", "- Revision: 3", 1)
	reopened = strings.Replace(reopened, "- State: abandoned", "- State: queued", 1)
	reopened = strings.Replace(reopened, "- Abandoned: by=human:Wido at=2026-08-20T11:00:00Z revision=2 opid=01J5X0000000000000000000C0-mac-a-1a2b3c4d because=the pursuit has stopped\n", "", 1)
	_, problems = ParseFile([]byte(withFreshIntegrity(reopened)))
	if len(problems) != 0 {
		t.Fatalf("a reopened record must keep its lawful carried abandon line: %v", problems)
	}

	historyProblems := func(file *GoalFile) []string {
		var result []string
		validateAbandonHistory(file, func(format string, args ...any) {
			result = append(result, fmt.Sprintf(format, args...))
		})
		return result
	}
	ownAbandon := HistoryLine{Verb: "abandon", Targets: []string{"history-keys"}, StopID: "stop-fixture", Carried: "successor"}
	for _, test := range []struct {
		name    string
		history []HistoryLine
		want    string
	}{
		{name: "abandon may carry both keys", history: []HistoryLine{ownAbandon}},
		{name: "carry after own abandon is valid", history: []HistoryLine{ownAbandon, {Verb: "carry", Carried: "successor-two"}}},
		{name: "carry after reopen refuses", history: []HistoryLine{ownAbandon, {Verb: "reopen"}, {Verb: "carry", Carried: "successor-two"}}, want: "carry line on a goal that was not abandoned at that line"},
		{name: "survivor compaction is not its own abandon", history: []HistoryLine{{Verb: "abandon", Targets: []string{"departed", "history-keys"}}, {Verb: "carry", Carried: "successor"}}, want: "carry line on a goal that was not abandoned at that line"},
		{name: "current state does not invalidate an earlier carry", history: []HistoryLine{ownAbandon, {Verb: "carry", Carried: "successor-two"}, {Verb: "reopen"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := &GoalFile{Id: "history-keys", History: test.history}
			got := historyProblems(file)
			if test.want == "" && len(got) != 0 {
				t.Fatalf("lawful history refused: %v", got)
			}
			if test.want != "" && !slices.Contains(got, test.want) {
				t.Fatalf("missing %q in %v", test.want, got)
			}
		})
	}
}

func TestTamperedBytesFailIntegrityByName(t *testing.T) {
	t.Parallel()
	bytes := RenderFile(claimedGolden())
	tampered := strings.Replace(string(bytes), "session-a", "session-b", 1)
	_, problems := ParseFile([]byte(tampered))
	found := false
	for _, p := range problems {
		if strings.Contains(string(p), "Integrity mismatch") {
			found = true
		}
	}
	if !found {
		t.Fatalf("a hand edit without a recomputed digest must fail Integrity by name, got %v", problems)
	}
}

func TestMissingIntegrityLineRefuses(t *testing.T) {
	t.Parallel()
	bytes := RenderFile(claimedGolden())
	body, _, _ := splitIntegrity(bytes)
	_, problems := ParseFile(body)
	found := false
	for _, p := range problems {
		if strings.Contains(string(p), "missing Integrity") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want missing-Integrity problem, got %v", problems)
	}
}

func TestStateRecordAgreementIsValidated(t *testing.T) {
	t.Parallel()
	f := claimedGolden()
	f.State = StateQueued // record says claimed, state says queued
	_, problems := ParseFile(RenderFile(f))
	found := false
	for _, p := range problems {
		if strings.Contains(string(p), "Claimed record on a queued goal") {
			found = true
		}
	}
	if !found {
		t.Fatalf("state/record divergence must refuse, got %v", problems)
	}
}

func TestClaimRevisionMustExistAndKeepItsHistoryTimestamp(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mutate func(*GoalFile)
		want   string
	}{
		{
			name: "revision does not exist",
			mutate: func(f *GoalFile) {
				f.Claimed.Revision = f.Revision + 1
			},
			want: "BUDGET_UNKNOWN claimed revision=4 does not exist in goal Revision=3",
		},
		{
			name: "timestamp contradicts revision",
			mutate: func(f *GoalFile) {
				f.Claimed.At = "2026-08-20T00:36:00Z"
			},
			want: "BUDGET_UNKNOWN claimed at=2026-08-20T00:36:00Z contradicts History revision=2 at=2026-08-20T00:35:00Z",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := claimedGolden()
			test.mutate(f)
			_, problems := ParseFile(RenderFile(f))
			if !problemsContain(problems, test.want) {
				t.Fatalf("contradictory claim binding did not refuse by type and fact: %v", problems)
			}
		})
	}
}

func TestHistoryGrammarRoundTripsEveryField(t *testing.T) {
	t.Parallel()
	line := HistoryLine{
		At:        "2026-08-20T01:00:00Z",
		Opid:      "01J5X0000000000000000000F5-intel-nuc-9f8e7d6c",
		Verb:      "park",
		Actor:     "human:wido",
		Targets:   []string{"a-goal", "b-goal"},
		Displaced: "mac-studio+session-a@2026-08-20T00:35:00Z",
		Ack:       false,
		Keep:      -1,
		Reason:    "operator assignment: second machine takes the arc, free text with = and spaces",
	}
	rendered := RenderHistoryLine(line)
	parsed, err := ParseHistoryLine(rendered)
	if err != nil {
		t.Fatalf("parse of rendered line: %v", err)
	}
	if RenderHistoryLine(parsed) != rendered {
		t.Fatalf("history line is not a fixed point:\n%s\n%s", rendered, RenderHistoryLine(parsed))
	}
	if parsed.Reason != line.Reason {
		t.Fatalf("reason= must consume the remainder losslessly, got %q", parsed.Reason)
	}
}

func TestHistoryLineResumedField(t *testing.T) {
	t.Parallel()
	line := HistoryLine{
		At: "2026-09-18T10:00:00Z", Opid: "01J5X0000000000000000000R8-mac-a-1a2b3c4d",
		Verb: "set-budget", Actor: "human:Wido", Targets: []string{"fenced-goal"}, Resumed: "stop-fenced-r4-f1", Keep: -1,
	}
	rendered := RenderHistoryLine(line)
	if !strings.Contains(rendered, " resumed=stop-fenced-r4-f1") {
		t.Fatalf("set-budget history omitted its resumed stop: %s", rendered)
	}
	parsed, err := ParseHistoryLine(rendered)
	if err != nil || parsed.Resumed != line.Resumed || RenderHistoryLine(parsed) != rendered {
		t.Fatalf("resumed stop did not parse and round-trip: parsed=%+v err=%v", parsed, err)
	}

	legacy := line
	legacy.Resumed = ""
	legacyRendered := RenderHistoryLine(legacy)
	legacyParsed, err := ParseHistoryLine(legacyRendered)
	if err != nil || legacyParsed.Resumed != "" || RenderHistoryLine(legacyParsed) != legacyRendered {
		t.Fatalf("legacy set-budget history changed: parsed=%+v err=%v", legacyParsed, err)
	}
	for _, invalid := range []string{
		strings.Replace(rendered, " set-budget ", " resume ", 1),
		strings.Replace(rendered, "stop-fenced-r4-f1", "../unsafe", 1),
		strings.Replace(rendered, "resumed=stop-fenced-r4-f1", "resumed=", 1),
	} {
		if _, err := ParseHistoryLine(invalid); err == nil || !strings.Contains(err.Error(), "only valid on set-budget") {
			t.Fatalf("invalid resumed history was accepted: %q err=%v", invalid, err)
		}
	}
	duplicate := strings.Replace(rendered, "resumed=stop-fenced-r4-f1", "resumed=stop-fenced-r4-f1 resumed=stop-fenced-r4-f1", 1)
	if _, err := ParseHistoryLine(duplicate); err == nil || !strings.Contains(err.Error(), "duplicate resumed=") {
		t.Fatalf("duplicate resumed= was accepted: %v", err)
	}
	withStopID := line
	withStopID.StopID = "stop-fenced-r4-f1"
	var stopProblem string
	validateAbandonHistory(&GoalFile{Id: "fenced-goal", History: []HistoryLine{withStopID}}, func(format string, args ...any) {
		stopProblem = fmt.Sprintf(format, args...)
	})
	if !strings.Contains(stopProblem, "set-budget line carries stopId=") {
		t.Fatalf("set-budget stopId= was accepted: %q", stopProblem)
	}
}

func TestResumeHistoryRoundTripsVerbatimTemporaryAuthority(t *testing.T) {
	t.Parallel()
	line := HistoryLine{
		At: "2026-09-01T10:00:00Z", Opid: "01J5X0000000000000000000F6-intel-nuc-9f8e7d6c",
		Verb: "resume", Actor: "human:Wido", Targets: []string{"a-goal"}, Keep: -1,
		AuthorityOutcome: AuthorityOutcomeTemporaryHumanWord, AuthorityReviewBy: "2026-09-06",
		AuthorityRuling:    TemporaryGoalAuthorityRuling,
		TemporaryHumanWord: "  Wido authorizes\nthis reason=exact resume\t  ",
		Reason:             "a separate reason remains free text with temporaryHumanWord=inside it",
	}
	rendered := RenderHistoryLine(line)
	parsed, err := ParseHistoryLine(rendered)
	if err != nil {
		t.Fatalf("parse temporary resume history: %v", err)
	}
	if parsed.TemporaryHumanWord != line.TemporaryHumanWord || parsed.AuthorityRuling != TemporaryGoalAuthorityRuling || parsed.Reason != line.Reason {
		t.Fatalf("temporary authority did not round trip verbatim: %+v", parsed)
	}
	if RenderHistoryLine(parsed) != rendered {
		t.Fatalf("temporary resume history is not a fixed point:\n%s\n%s", rendered, RenderHistoryLine(parsed))
	}
}

func TestLandedTemporaryAuthorityRoundTripsAfterRulingRenewal(t *testing.T) {
	t.Parallel()
	line := HistoryLine{
		At: "2026-09-07T10:00:00Z", Opid: "01J5X0000000000000000000F7-intel-nuc-9f8e7d6c",
		Verb: "resume", Actor: "human:Wido", Targets: []string{"a-goal"}, Keep: -1,
		AuthorityOutcome: AuthorityOutcomeTemporaryHumanWord, AuthorityReviewBy: "2026-09-07",
		AuthorityRuling:    "R-33-m1",
		TemporaryHumanWord: "Wido renews this resume",
	}
	rendered := RenderHistoryLine(line)
	parsed, err := ParseHistoryLine(rendered)
	if err != nil {
		t.Fatalf("a landed authority fact was re-judged against today's ruling or horizon: %v", err)
	}
	if RenderHistoryLine(parsed) != rendered {
		t.Fatalf("renewed authority history is not a fixed point:\n%s\n%s", rendered, RenderHistoryLine(parsed))
	}
}

func TestHistoryReasonCannotSupplyAMissingRecordedWord(t *testing.T) {
	t.Parallel()
	line := `- 2026-09-01T10:00:00Z 01J5X0000000000000000000F8-intel-nuc-9f8e7d6c resume actor=human:Wido targets=a-goal authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 reason=mentions temporaryHumanWord="not a marker"`
	if _, err := ParseHistoryLine(line); err == nil || !strings.Contains(err.Error(), "TemporaryHumanWord is missing") {
		t.Fatalf("reason text completed an otherwise malformed authority marker: %v", err)
	}
}

func TestPruneKeepFieldIsLawful(t *testing.T) {
	t.Parallel()
	parsed, err := ParseHistoryLine("- 2026-08-20T03:00:00Z 01J5X0000000000000000000A6-mac-studio-1a2b3c4d prune actor=mac-studio+session-a targets=old-one,old-two keep=50")
	if err != nil {
		t.Fatalf("prune keep= line must parse: %v", err)
	}
	if parsed.Keep != 50 {
		t.Fatalf("keep lost: %d", parsed.Keep)
	}
}

func TestUnknownHistoryKeyRefuses(t *testing.T) {
	t.Parallel()
	_, err := ParseHistoryLine("- 2026-08-20T03:00:00Z 01J5X0000000000000000000A0-mac-a-1a2b3c4d verb actor=a+b sneaky=1")
	if err == nil || !strings.Contains(err.Error(), "unknown History key") {
		t.Fatalf("unknown key must refuse by name, got %v", err)
	}
}

func TestHistoryPrefixIsADiagnosticHelper(t *testing.T) {
	t.Parallel()
	full := claimedGolden().History
	if !HistoryIsPrefix(full[:2], full) {
		t.Fatal("a strict prefix must be detected")
	}
	if HistoryIsPrefix(full, full) {
		t.Fatal("equal histories are not a strict prefix")
	}
	divergent := append([]HistoryLine{}, full[:1]...)
	divergent = append(divergent, HistoryLine{At: "x", Opid: "y", Verb: "z", Actor: "a+b", Keep: -1})
	if HistoryIsPrefix(divergent, full) {
		t.Fatal("a divergent history is not a prefix")
	}
}

func TestOpidAttributesExecution(t *testing.T) {
	t.Parallel()
	a := Opid("01J5X0000000000000000000A0", "mac-studio", "session-a")
	b := Opid("01J5X0000000000000000000A0", "mac-studio", "session-b")
	if a == b {
		t.Fatal("different lineages must hash differently")
	}
	if !strings.HasPrefix(a, "01J5X0000000000000000000A0-mac-studio-") {
		t.Fatalf("opid shape: %s", a)
	}
}

func TestParkedRecordRoundTripsWithDisplacementAndFreeText(t *testing.T) {
	t.Parallel()
	f := claimedGolden()
	f.State = StateParked
	f.Claimed = nil
	f.Parked = &ParkRecord{
		By:        "operator",
		At:        "2026-08-20T04:00:00Z",
		Because:   "yields to the sync build; free text with = signs and, commas",
		Displaced: "mac-studio+session-a@2026-08-20T00:35:00Z",
	}
	parsed, problems := ParseFile(RenderFile(f))
	if len(problems) != 0 {
		t.Fatalf("parked golden must parse clean, got %v", problems)
	}
	if parsed.Parked == nil || parsed.Parked.Because != f.Parked.Because || parsed.Parked.Displaced != f.Parked.Displaced {
		t.Fatalf("park record lost: %+v", parsed.Parked)
	}
	if string(RenderFile(parsed)) != string(RenderFile(f)) {
		t.Fatal("parked file is not a render fixed point")
	}
}

func TestParkedRecordCarriesItsBlockerAndRequiresTheEdge(t *testing.T) {
	t.Parallel()
	f := claimedGolden()
	f.State = StateParked
	f.Claimed = nil
	f.Blocked = []string{"the-blocker"}
	f.Parked = &ParkRecord{
		By: "mac-a+lin-1", At: "2026-09-12T08:00:00Z",
		Because: "blocked by the-blocker; returns when it is done", Blocker: "the-blocker",
	}
	rendered := RenderFile(f)
	if !strings.Contains(string(rendered), " blocker=the-blocker because=blocked by the-blocker; returns when it is done\n") {
		t.Fatalf("the blocker token precedes the free-text tail: %s", rendered)
	}
	parsed, problems := ParseFile(rendered)
	if len(problems) != 0 {
		t.Fatalf("blocker park must parse clean, got %v", problems)
	}
	if parsed.Parked == nil || parsed.Parked.Blocker != "the-blocker" || parsed.Parked.Because != f.Parked.Because {
		t.Fatalf("blocker park lost: %+v", parsed.Parked)
	}
	if string(RenderFile(parsed)) != string(rendered) {
		t.Fatal("blocker park is not a render fixed point")
	}
	// A blocker the edge does not carry is a diagnostic: only the blocker's
	// own open writes the token, and it always writes the edge beside it.
	f.Blocked = nil
	if _, problems := ParseFile(RenderFile(f)); len(problems) == 0 || !strings.Contains(string(problems[0]), "blocker=the-blocker is not in BlockedBy") {
		t.Fatalf("a blocker without its edge is a diagnostic: %v", problems)
	}
}

func TestHistoryLineCarriesAPowerOfAttorney(t *testing.T) {
	t.Parallel()
	entry := Opid("01J5X00000000000000000PA00", "mac-a", "lin-1")
	line := HistoryLine{At: "2026-09-12T10:00:00Z", Opid: Opid("01J5X00000000000000000PA01", "mac-a", "lin-1"), Verb: "approve",
		Actor: "mac-a+lin-1", Targets: []string{"small"}, Keep: -1, AuthorityOutcome: AuthorityOutcomePowerOfAttorney, AuthorityRuling: entry}
	rendered := RenderHistoryLine(line)
	if !strings.HasSuffix(rendered, " authorityOutcome=POWER_OF_ATTORNEY authorityRuling="+entry) {
		t.Fatalf("the line names the entry and nothing else: %s", rendered)
	}
	parsed, err := ParseHistoryLine(rendered)
	if err != nil || parsed.AuthorityOutcome != AuthorityOutcomePowerOfAttorney || parsed.AuthorityRuling != entry || parsed.AuthorityReviewBy != "" || parsed.TemporaryHumanWord != "" {
		t.Fatalf("round trip: %+v %v", parsed, err)
	}
	line.Actor = "human:Wido"
	if _, err := ParseHistoryLine(RenderHistoryLine(line)); err == nil || !strings.Contains(err.Error(), "seat actor") {
		t.Fatalf("an attorney act is never a human actor: %v", err)
	}
	line.Actor = "mac-a+lin-1"
	line.AuthorityRuling = "not-an-opid"
	if _, err := ParseHistoryLine(RenderHistoryLine(line)); err == nil {
		t.Fatal("the ruling must be an entry id")
	}
}
