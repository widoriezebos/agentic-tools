package goal

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
)

func TestApprovalTokensRenderTheInstalledTuple(t *testing.T) {
	budget := testBudget()
	args := budgetIntentArgs(budget)
	wantResume := "goal=fleet-channel resume elapsed=" + args["elapsedLimit"] + " attempts=" + args["attemptLimit"] + " minutes=" + args["reservedJobMinutesLimit"] + " active=" + args["activeJobLimit"]
	if got := ResumeApprovalToken("fleet-channel", budget); got != wantResume {
		t.Fatalf("resume token = %q, want %q", got, wantResume)
	}
	wantObligation := "goal=fleet-channel set-obligation state=OBSERVE owner=fleet-one"
	if got := SetObligationApprovalToken("fleet-channel", ObligationObserve, "fleet-one"); got != wantObligation {
		t.Fatalf("set-obligation token = %q, want %q", got, wantObligation)
	}
}

func TestContainsContiguousFieldsMatchesExactlyOnce(t *testing.T) {
	token := "goal=g resume elapsed=4h"
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"one run across whitespace", "approve\n goal=g\tresume  elapsed=4h now", true},
		{"no match", "goal=g resume elapsed=5h", false},
		{"two matches", token + " and " + token, false},
		{"interrupted fields", "goal=g resume unexpected elapsed=4h", false},
		{"empty token", token, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := containsContiguousFields(test.text, token)
			if test.name == "empty token" {
				got = containsContiguousFields(test.text, "")
			}
			if got != test.want {
				t.Fatalf("containsContiguousFields(%q) = %v, want %v", test.text, got, test.want)
			}
		})
	}
}

func TestAskedAppendsTheAskedMarkerOnce(t *testing.T) {
	for _, test := range []struct {
		name, initial, want string
	}{
		{"empty next step", "", "ASKED question-a (other): First fact."},
		{"filled next step", "Start here.", "Start here.; ASKED question-a (other): First fact."},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, root, _ := twoClones(t)
			seedLedger(t, root)
			if result, err := Open(verbReq(root, "01J5X0000000000000000000D0", "mac-a"), "asked", "Ask something.", "main", test.initial); err != nil || result.Outcome != OutcomeConfirmed {
				t.Fatalf("open: %+v %v", result, err)
			}
			req := verbReq(root, "01J5X0000000000000000000D1", "mac-a")
			for attempt := 0; attempt < 2; attempt++ {
				if result, err := Asked(req, "asked", "question-a", "other", "First fact."); err != nil || result.Outcome != OutcomeConfirmed {
					t.Fatalf("asked attempt %d: %+v %v", attempt+1, result, err)
				}
			}
			projection, err := Project(endpointFor(root), true, req.Now)
			if err != nil {
				t.Fatal(err)
			}
			file := projection.Tree.Live["asked"]
			askedLines := 0
			for _, history := range file.History {
				if history.Verb == "ask" {
					askedLines++
				}
			}
			if file.NextStep != test.want || askedLines != 1 {
				t.Fatalf("next=%q ask history=%d", file.NextStep, askedLines)
			}
			if result, err := Asked(verbReq(root, "01J5X0000000000000000000D2", "mac-a"), "missing", "question-b", "other", "Fact."); err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "not live") {
				t.Fatalf("missing goal refusal: %+v %v", result, err)
			}
		})
	}

	err := (&TreeReadError{Tip: "1234567890abcdef", Problems: []Problem{"first problem", "second problem"}}).Error()
	if want := "the ledger tree at 1234567890ab does not parse:\nfirst problem\nsecond problem"; err != want {
		t.Fatalf("TreeReadError = %q, want %q", err, want)
	}
}

func TestAnswerRecordsAuthenticatedChannelWordOnce(t *testing.T) {
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	if result, err := Open(verbReq(root, "01J5X0000000000000000000E0", "mac-a"), "answered", "Record the answer.", "main", "Wait."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	req := verbReq(root, "01J5X0000000000000000000E1", "mac-a")
	complete := AnswerProof{Provider: "slack", User: "UWIDO", Ref: "1/2", Step: 42}
	if _, err := Answer(req, "answered", "q", "yes", "", AnswerProof{}); err == nil {
		t.Fatal("incomplete proof was accepted")
	}
	if _, err := Answer(req, "answered", "q", " \n ", "", complete); err == nil {
		t.Fatal("blank answer text was accepted")
	}
	wants := "goal=answered resume elapsed=4h attempts=4 minutes=240 active=2"
	for attempt := 0; attempt < 2; attempt++ {
		if result, err := Answer(req, "answered", "q", "yes", wants, complete); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("answer attempt %d: %+v %v", attempt+1, result, err)
		}
	}
	projection, err := Project(endpointFor(root), true, req.Now)
	if err != nil {
		t.Fatal(err)
	}
	file := projection.Tree.Live["answered"]
	answers := []HistoryLine{}
	for _, history := range file.History {
		if history.Verb == "answer" {
			answers = append(answers, history)
		}
	}
	if len(answers) != 1 {
		t.Fatalf("answer history count = %d", len(answers))
	}
	history := answers[0]
	if history.Actor != "human:wido" || history.AuthorityOutcome != AuthorityOutcomeAuthenticatedChannelWord || history.ChannelProvider != complete.Provider || history.ChannelUser != complete.User || history.ChannelRef != complete.Ref || history.ChannelStep != complete.Step || history.Reason != "yes "+wants || !strings.Contains(file.NextStep, "ANSWERED q: yes") {
		t.Fatalf("recorded answer or marker is incomplete: history=%+v next=%q", history, file.NextStep)
	}
}

func TestAuthenticatedChannelApprovalRequiresTheTokenOnce(t *testing.T) {
	t.Run("lookup and strict token", func(t *testing.T) {
		_, root, _ := twoClones(t)
		seedLedger(t, root)
		if result, err := Open(verbReq(root, "01J5X0000000000000000000F0", "mac-a"), "authority", "Authorize an act.", "main", "Wait."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open: %+v %v", result, err)
		}
		now := verbReq(root, "01J5X0000000000000000000F1", "mac-a").Now
		if _, err := AuthenticatedChannelApproval(root, "missing", "op", "token", now); err == nil || !strings.Contains(err.Error(), "not live") {
			t.Fatalf("missing goal was not refused by name: %v", err)
		}
		if _, err := AuthenticatedChannelApproval(root, "authority", "not-an-answer", "token", now); err == nil || !strings.Contains(err.Error(), "not an authenticated channel word") {
			t.Fatalf("non-answer was not refused: %v", err)
		}
		proof := AnswerProof{Provider: "slack", User: "UWIDO", Ref: "1/2", Step: 42}
		answerReq := verbReq(root, "01J5X0000000000000000000F2", "mac-a")
		if result, err := Answer(answerReq, "authority", "q", "approved", "", proof); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("answer: %+v %v", result, err)
		}
		opid := answerReq.opid()
		token := "goal=authority resume elapsed=4h attempts=4 minutes=240 active=2"
		if _, err := AuthenticatedChannelApproval(root, "authority", opid, token, now); err == nil || !strings.Contains(err.Error(), "required token") {
			t.Fatalf("missing token was accepted: %v", err)
		}
		carryingReq := verbReq(root, "01J5X0000000000000000000F3", "mac-a")
		if result, err := Answer(carryingReq, "authority", "q2", token, "", proof); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("token answer: %+v %v", result, err)
		}
		got, err := AuthenticatedChannelApproval(root, "authority", carryingReq.opid(), token, now)
		want := governanceChannelAuthority(proof)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("approval=%+v want=%+v err=%v", got, want, err)
		}
	})

	t.Run("resume uses standing approval without consuming an answer", func(t *testing.T) {
		_, root, _ := twoClones(t)
		seedLedger(t, root)
		budget := testBudget()
		token := ResumeApprovalToken("resume-goal", budget)
		if result, err := Open(verbReq(root, "01J5X0000000000000000000G0", "mac-a"), "resume-goal", "Resume once.", "main", "Wait."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open: %+v %v", result, err)
		}
		approveGoalForTest(t, verbReq(root, "01J5X0000000000000000000G5", "mac-a"), "resume-goal", budget)
		answerReq := verbReq(root, "01J5X0000000000000000000G1", "mac-a")
		proof := AnswerProof{Provider: "slack", User: "UWIDO", Ref: "1/2", Step: 43}
		answerText := token + " goal=resume-goal minutes=240 reviewRounds=3 goalRevision=4"
		if result, err := Answer(answerReq, "resume-goal", "q", answerText, "", proof); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("answer: %+v %v", result, err)
		}
		claim := verbReq(root, "01J5X0000000000000000000G2", "mac-a")
		claim.ClaimEpoch = 9
		if result, err := Claim(claim, "resume-goal"); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("claim: %+v %v", result, err)
		}
		projection, _ := Project(endpointFor(root), true, claim.Now)
		file := projection.Tree.Live["resume-goal"]
		stop := CloseStopRequest{VerbRequest: VerbRequest{Endpoint: endpointFor(root), Actor: Actor{Machine: "mac-a", Lineage: "goal-stop-custodian"}, Ulid: "01J5X0000000000000000000G3", Now: claim.Now.Add(time.Minute), ClaimEpoch: 9}, GoalID: "resume-goal", StopID: "stop-resume-goal-r3-f1", Reason: StopReasonElapsedLimit, Capability: *file.StopCapability}
		if result, err := CloseStop(stop); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("stop: %+v %v", result, err)
		}
		projection, _ = Project(endpointFor(root), true, stop.Now)
		stopped := projection.Tree.Live["resume-goal"]
		stamp := stop.Now.Add(time.Minute).UTC().Format(time.RFC3339)
		if err := WriteStopBatch(root, StopBatch{StopID: stop.StopID, GoalID: "resume-goal", GoalRevision: stopped.Claimed.Revision, FenceEpoch: stopped.StopFence.Epoch, CapabilityGeneration: stopped.StopCapability.Generation, Machine: "mac-a", ClaimEpoch: 9, Reason: StopReasonElapsedLimit, State: StopBatchComplete, OpenedAt: stop.Now.UTC().Format(time.RFC3339), UpdatedAt: stamp, CompletedAt: stamp, Pass: 1}); err != nil {
			t.Fatal(err)
		}
		resumeReq := verbReq(root, "01J5X0000000000000000000G4", "mac-a")
		resumeReq.Actor.Human, resumeReq.Now = "wido", stop.Now.Add(time.Minute)
		humanProof := testHumanAuthority(t, root, resumeReq.Now)
		if result, err := Resume(ResumeRequest{VerbRequest: resumeReq, GoalID: "resume-goal", Budget: budget, Authority: humanProof}); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("resume: %+v %v", result, err)
		}
		if _, err := AuthenticatedChannelApproval(root, "resume-goal", answerReq.opid(), token, resumeReq.Now); err != nil {
			t.Fatalf("standing-approval resume unexpectedly consumed the independent answer: %v", err)
		}
	})

	t.Run("set-obligation consumes the answer", func(t *testing.T) {
		root := obligationAuthorityLocalRoot(t, "obligation-goal")
		if err := os.WriteFile(root+"/metasystem.conf", []byte("metasystem.governance.correlation-policy=\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		token := SetObligationApprovalToken("obligation-goal", ObligationDraft, "Wido")
		answerReq := obligationAuthorityVerbReq(root, "01J5X0000000000000000000H0", "mac-a")
		proof := AnswerProof{Provider: "slack", User: "UWIDO", Ref: "3/4", Step: 44}
		if result, err := Answer(answerReq, "obligation-goal", "q", token, "", proof); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("answer: %+v %v", result, err)
		}
		request := obligationAuthorityVerbReq(root, "01J5X0000000000000000000H1", "mac-a")
		request.Actor.Human, request.ApprovedRef = "Wido", answerReq.opid()
		authority := testTemporaryGoalProof(t, root, "Wido authorizes this obligation", "2026-09-06")
		if result, err := SetObligation(request, "obligation-goal", testGovernedObligation(ObligationDraft), &authority); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("set-obligation: %+v %v", result, err)
		}
		if _, err := AuthenticatedChannelApproval(root, "obligation-goal", answerReq.opid(), token, request.Now); err == nil || !strings.Contains(err.Error(), "already consumed by set-obligation") {
			t.Fatalf("consumed set-obligation approval was reusable: %v", err)
		}
	})
}

func governanceChannelAuthority(proof AnswerProof) governance.RecordedChannelAuthority {
	return governance.RecordedChannelAuthority{Outcome: AuthorityOutcomeAuthenticatedChannelWord, Provider: proof.Provider, UserID: proof.User, MessageRef: proof.Ref, Step: proof.Step}
}

func TestHistoryLineApprovedRefRoundTripsOnConsumersOnly(t *testing.T) {
	for _, verb := range []string{"resume", "set-obligation"} {
		line := HistoryLine{At: "2026-09-03T12:00:00Z", Opid: Opid("01J5X0000000000000000000J0", "mac-a", verb), Verb: verb, Actor: "human:wido", Targets: []string{"g"}, Keep: -1, ApprovedRef: "answer-operation"}
		file := &GoalFile{Id: "g", State: StateQueued, Intent: "Round trip.", Origin: OriginMain, NextStep: "Wait.", OpenedAt: line.At, Revision: 1, History: []HistoryLine{line}}
		rendered := string(RenderFile(file))
		var historyText string
		for _, candidate := range strings.Split(rendered, "\n") {
			if strings.Contains(candidate, " approvedRef=") {
				historyText = candidate
				break
			}
		}
		parsed, err := ParseHistoryLine(historyText)
		if err != nil || !reflect.DeepEqual(parsed, line) {
			t.Fatalf("%s round trip: parsed=%+v want=%+v err=%v\n%s", verb, parsed, line, err, rendered)
		}
	}
	other := HistoryLine{At: "2026-09-03T12:00:00Z", Opid: Opid("01J5X0000000000000000000J1", "mac-a", "answer"), Verb: "answer", Actor: "human:wido", Targets: []string{"g"}, Keep: -1, ApprovedRef: "answer-operation"}
	if _, err := ParseHistoryLine(RenderHistoryLine(other)); err == nil || !strings.Contains(err.Error(), "approvedRef=") {
		t.Fatalf("approvedRef on another verb was not refused by key: %v", err)
	}
}
