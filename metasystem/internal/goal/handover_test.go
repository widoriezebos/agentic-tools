package goal

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
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

func handoverBed(t *testing.T, id string, rich bool) (string, VerbRequest) {
	t.Helper()
	_, root := oneClone(t)
	seedLedger(t, root)
	req := verbReq(root, "01J5X00000000000000000HB01", "mac-studio")
	req.Actor.Lineage, req.ClaimEpoch = "session-a", 7
	if result, err := openClaimForTest(t, req, id, "Move custody without rebinding it.", OriginMain, "Hand it over.", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim handover fixture: %+v %v", result, err)
	}
	if rich {
		tree, _ := loadTree(root, acceptedTip(t, root))
		f := tree.Live[id]
		f.Claimed.IdleSeconds = 41
		obligation := testGovernedObligation(ObligationDraft)
		obligation.Revision, obligation.BudgetRevision = f.Revision, f.Claimed.Revision
		f.Obligation = &obligation
		f.ReviewObligations = []ReviewObligation{{Finding: "F-1", Chain: "critic", Artifact: "a.go", Test: "proof", State: "discharged"}}
		f.AcceptedRisks = []AcceptedRiskRecord{{Finding: "F-2", Chain: "critic", By: "Wido", Opid: "risk-op"}}
		f.StopCapability.FenceEpoch = 1
		f.StopFence = &StopFence{StopID: "stop-" + id + "-r3-f1", Revision: f.Claimed.Revision, Epoch: 1, CapabilityGeneration: f.StopCapability.Generation, ClosedAt: req.stamp(), Reason: StopReasonElapsedLimit}
		f.Landing = &LandingRecord{At: req.stamp(), Opid: f.History[len(f.History)-1].Opid}
		publishGoalFixtures(t, root, f)
	}
	return root, req
}
func TestGoalHandoverAppliesCompleteFieldTable(t *testing.T) {
	root, req := handoverBed(t, "field-complete", true)
	before, _ := loadTree(root, acceptedTip(t, root))
	req.Ulid, req.Now = "01J5X00000000000000000HB02", req.Now.Add(time.Minute)
	result, err := Handover(req, "field-complete", "landing", "landing-lineage", 11, "batch-a", func() (identity.Liveness, error) { return identity.Alive, nil })
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("holder handover: %+v %v", result, err)
	}
	after, _ := loadTree(root, result.Tip)
	want, got := before.Live["field-complete"], after.Live["field-complete"]
	rows := []struct {
		name string
		ok   bool
	}{
		{"claim and accounting fields preserve", got.Claimed.At == want.Claimed.At && got.Claimed.Revision == want.Claimed.Revision && got.Claimed.AccountingRevision == want.Claimed.AccountingRevision && got.Claimed.EpisodeAt == want.Claimed.EpisodeAt && got.Claimed.EpisodeRevision == want.Claimed.EpisodeRevision && got.Claimed.EpisodeObligationRevision == want.Claimed.EpisodeObligationRevision && got.Claimed.IdleSeconds == want.Claimed.IdleSeconds},
		{"custody and handover regenerate", got.Claimed.Machine == "landing" && got.Claimed.Lineage == "landing-lineage" && got.Claimed.HandedOver == (HandedOver{FromMachine: "mac-studio", FromLineage: "session-a", FromEpoch: 7, Batch: "batch-a"})},
		{"proof and landing fields preserve", reflect.DeepEqual(got.Obligation, want.Obligation) && reflect.DeepEqual(got.ReviewObligations, want.ReviewObligations) && reflect.DeepEqual(got.AcceptedRisks, want.AcceptedRisks) && reflect.DeepEqual(got.StopFence, want.StopFence) && reflect.DeepEqual(got.Landing, want.Landing)},
		{"stop capability regenerates whole", *got.StopCapability == (StopCapability{Generation: want.StopCapability.Generation, Revision: want.Claimed.Revision, Machine: "landing", ClaimEpoch: 11, FenceEpoch: want.StopCapability.FenceEpoch})},
		{"incompatible state fields stay clear", got.Episode == nil && got.Parked == nil && got.Abandoned == nil},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			if !row.ok {
				t.Fatalf("field-table row changed: before=%+v after=%+v", want, got)
			}
		})
	}

	nonholder := req
	nonholder.Ulid = "01J5X00000000000000000HB03"
	if res, err := Handover(nonholder, "field-complete", "other", "other-lineage", 4, "batch-a", func() (identity.Liveness, error) { return identity.Alive, nil }); err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "a foreign release is a human act") {
		t.Fatalf("non-holder authority: %+v %v", res, err)
	}
	holder := req
	holder.Actor, holder.ClaimEpoch = Actor{Machine: "landing", Lineage: "landing-lineage"}, 11
	for index, state := range []identity.Liveness{identity.Dead, identity.Unknown} {
		holder.Ulid = []string{"01J5X00000000000000000HB04", "01J5X00000000000000000HB05"}[index]
		if res, err := Handover(holder, "field-complete", "other", "other-lineage", 4, "batch-a", func() (identity.Liveness, error) { return state, nil }); err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, state.String()+" liveness") {
			t.Fatalf("%s target: %+v %v", state, res, err)
		}
	}
	holder.Ulid, holder.ClaimEpoch = "01J5X00000000000000000HB06", 12
	if res, err := Handover(holder, "field-complete", "landing", "landing-lineage", 12, "batch-a", func() (identity.Liveness, error) { return identity.Alive, nil }); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("same-pair epoch rebind: %+v %v", res, err)
	}
	rebound, _ := loadTree(root, acceptedTip(t, root))
	if rebound.Live["field-complete"].Claimed.HandedOver != got.Claimed.HandedOver || rebound.Live["field-complete"].StopCapability.ClaimEpoch != 12 {
		t.Fatalf("epoch rebind changed the return record or missed the epoch: %+v", rebound.Live["field-complete"])
	}
	reject := func(name string, request VerbRequest, id, machine, lineage string, epoch int64, live func() (identity.Liveness, error), want string) {
		t.Helper()
		res, err := Handover(request, id, machine, lineage, epoch, "batch-a", live)
		detail := res.Detail
		if err != nil {
			detail = err.Error()
		}
		if !strings.Contains(detail, want) {
			t.Fatalf("%s did not refuse with %q: result=%+v err=%v", name, want, res, err)
		}
	}
	holder.Ulid = "01J5X00000000000000000HB09"
	reject("same epoch", holder, "field-complete", "landing", "landing-lineage", 12, func() (identity.Liveness, error) { return identity.Alive, nil }, "must be higher")
	holder.Ulid, holder.ClaimEpoch = "01J5X00000000000000000HB10", 13
	reject("stale source epoch", holder, "field-complete", "other", "other-lineage", 4, func() (identity.Liveness, error) { return identity.Alive, nil }, "does not match")
	holder.Actor.Human = "Wido"
	reject("human caller", holder, "field-complete", "other", "other-lineage", 4, func() (identity.Liveness, error) { return identity.Alive, nil }, "agent-only")
	holder.Actor.Human = ""
	reject("incomplete input", holder, "field-complete", "", "", 0, nil, "requires a target")
	holder.Actor.Human, holder.ClaimEpoch, holder.Ulid = "", 12, "01J5X00000000000000000HB11"
	reject("missing goal", holder, "absent", "landing", "landing-lineage", 13, func() (identity.Liveness, error) { return identity.Alive, nil }, "is not live")
	holder.Ulid = "01J5X00000000000000000HB12"
	if res, err := Open(holder, "unclaimed", "Wait for a claim.", OriginMain, "Claim it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open unclaimed guard fixture: %+v %v", res, err)
	}
	holder.Ulid = "01J5X00000000000000000HB13"
	reject("unclaimed goal", holder, "unclaimed", "landing", "landing-lineage", 13, func() (identity.Liveness, error) { return identity.Alive, nil }, "no complete claimed authority")
}

func handedOverTerminalClearsRecord(t *testing.T, done bool) {
	t.Helper()
	root, req := handoverBed(t, "terminal-handover", false)
	req.Ulid = "01J5X00000000000000000HB07"
	if res, err := Handover(req, "terminal-handover", "landing", "landing-lineage", 11, "batch-a", func() (identity.Liveness, error) { return identity.Alive, nil }); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("handover: %+v %v", res, err)
	}
	req.Actor, req.ClaimEpoch, req.Ulid = Actor{Machine: "landing", Lineage: "landing-lineage"}, 11, "01J5X00000000000000000HB08"
	var result PublishResult
	var err error
	if done {
		result, err = Done(req, "terminal-handover", "The handed-over work landed.")
	} else {
		result, err = Release(req, "terminal-handover")
	}
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("terminal mutation: %+v %v", result, err)
	}
	tree, _ := loadTree(root, result.Tip)
	file := tree.Live["terminal-handover"]
	if done {
		file = tree.Done["terminal-handover"]
	}
	if file == nil || file.Claimed != nil || strings.Contains(string(RenderFile(file)), "- HandedOver:") {
		t.Fatalf("terminal state retained handover: %+v", file)
	}
}
func TestHandedOverClaimReleaseClearsRecord(t *testing.T) { handedOverTerminalClearsRecord(t, false) }
func TestHandedOverClaimConclusionClearsRecord(t *testing.T) {
	handedOverTerminalClearsRecord(t, true)
}
