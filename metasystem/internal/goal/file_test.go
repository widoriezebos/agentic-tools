package goal

import (
	"fmt"
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

func TestSTR2P2A05ZeroClaimRevisionObligationIsAProblemNotAPanic(t *testing.T) {
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

func TestTamperedBytesFailIntegrityByName(t *testing.T) {
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

func TestResumeHistoryRoundTripsVerbatimTemporaryAuthority(t *testing.T) {
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
	line := `- 2026-09-01T10:00:00Z 01J5X0000000000000000000F8-intel-nuc-9f8e7d6c resume actor=human:Wido targets=a-goal authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 reason=mentions temporaryHumanWord="not a marker"`
	if _, err := ParseHistoryLine(line); err == nil || !strings.Contains(err.Error(), "TemporaryHumanWord is missing") {
		t.Fatalf("reason text completed an otherwise malformed authority marker: %v", err)
	}
}

func TestPruneKeepFieldIsLawful(t *testing.T) {
	parsed, err := ParseHistoryLine("- 2026-08-20T03:00:00Z 01J5X0000000000000000000A6-mac-studio-1a2b3c4d prune actor=mac-studio+session-a targets=old-one,old-two keep=50")
	if err != nil {
		t.Fatalf("prune keep= line must parse: %v", err)
	}
	if parsed.Keep != 50 {
		t.Fatalf("keep lost: %d", parsed.Keep)
	}
}

func TestUnknownHistoryKeyRefuses(t *testing.T) {
	_, err := ParseHistoryLine("- 2026-08-20T03:00:00Z 01J5X0000000000000000000A0-mac-a-1a2b3c4d verb actor=a+b sneaky=1")
	if err == nil || !strings.Contains(err.Error(), "unknown History key") {
		t.Fatalf("unknown key must refuse by name, got %v", err)
	}
}

func TestHistoryPrefixIsADiagnosticHelper(t *testing.T) {
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
