package goal

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

const testSessionRef = "sess-01J5X0SESSION"

func sessionProofForTest(t *testing.T, root string, now time.Time) *humanauthority.Proof {
	t.Helper()
	proof, err := humanauthority.SignedInSessionProof(root, "Wido", testSessionRef, "browser", now)
	if err != nil {
		t.Fatalf("mint a signed-in session proof: %v", err)
	}
	return &proof
}

func parsedSessionProofForTest(t *testing.T, root string, now time.Time) *humanauthority.Proof {
	t.Helper()
	encoded, err := json.Marshal(sessionProofForTest(t, root, now))
	if err != nil {
		t.Fatal(err)
	}
	var parsed humanauthority.Proof
	if err := json.Unmarshal(encoded, &parsed); err != nil {
		t.Fatal(err)
	}
	return &parsed
}

// assertSessionLine checks one History line names its signed-in session with
// the outcome and the three channel keys, and writes no key beside them.
func assertSessionLine(t *testing.T, event HistoryLine) {
	t.Helper()
	if event.AuthorityOutcome != AuthorityOutcomeSignedInSession {
		t.Fatalf("History outcome = %q, want SIGNED_IN_SESSION", event.AuthorityOutcome)
	}
	if event.ChannelProvider != "browser" || event.ChannelUser != "Wido" || event.ChannelRef != testSessionRef {
		t.Fatalf("History line does not name its issuer, human, and session: %+v", event)
	}
	if event.ChannelContext != "" || event.ChannelStep != 0 || event.AuthorityReviewBy != "" ||
		event.AuthorityRuling != "" || event.TemporaryHumanWord != "" || event.AuthorityGeneration != 0 {
		t.Fatalf("a session History line carried relay or channel-thread facts: %+v", event)
	}
	line := RenderHistoryLine(event)
	want := " authorityOutcome=SIGNED_IN_SESSION channelProvider=browser channelUser=Wido channelRef=" + testSessionRef
	if !strings.Contains(line, want) {
		t.Fatalf("History line %q does not carry %q", line, want)
	}
	for _, forbidden := range []string{"channelStep=", "channelContext=", "authorityReviewBy=", "authorityRuling=", "authorityGeneration=", "sessionUser=", "sessionRef="} {
		if strings.Contains(line, forbidden) {
			t.Fatalf("History line %q wrote %q; a session act uses existing keys only", line, forbidden)
		}
	}
}

func assertSessionApproval(t *testing.T, file *GoalFile) HistoryLine {
	t.Helper()
	if file == nil || file.Approved == nil {
		t.Fatalf("no approval record: %+v", file)
	}
	if ApprovalAuthoritySession != "session" || AuthorityOutcomeSignedInSession != "SIGNED_IN_SESSION" {
		t.Fatalf("the written values drifted: authority=%q outcome=%q", ApprovalAuthoritySession, AuthorityOutcomeSignedInSession)
	}
	if file.Approved.Authority != ApprovalAuthoritySession {
		t.Fatalf("approval authority = %q, want session", file.Approved.Authority)
	}
	if file.Approved.ReviewBy != "" {
		t.Fatalf("a signed-in session approval carried a review date: %+v", file.Approved)
	}
	event := file.History[file.Approved.Revision-1]
	assertSessionLine(t, event)
	if parsed, problems := ParseFile(RenderFile(file)); parsed == nil || len(problems) != 0 {
		t.Fatalf("ParseFile of the written session approval reported %v", problems)
	}
	return event
}

func TestApproveUnderASignedInSessionWritesSessionAuthority(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)
	if result, err := Open(verbReq(root, "01J5X00000000000000000SE00", "mac-a"), "session-work", "Work a signed-in human admits.", OriginMain, "Run it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	request := verbReq(root, "01J5X00000000000000000SE10", "mac-a")
	request.Actor.Human = "Wido"
	budget := testBudget()
	if _, err := Approve(request, []string{"session-work"}, &budget, parsedSessionProofForTest(t, root, request.Now)); err == nil || !strings.Contains(err.Error(), "freshly observed") {
		t.Fatalf("a parsed session proof document approved execution: %v", err)
	}
	result, err := Approve(request, []string{"session-work"}, &budget, sessionProofForTest(t, root, request.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("approve under a signed-in session: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Live["session-work"]
	if file.State != StateApproved {
		t.Fatalf("state = %s, want %s", file.State, StateApproved)
	}
	assertSessionApproval(t, file)
	if problems := ValidateTree(tree); len(problems) != 0 {
		t.Fatalf("session-approved tree is invalid: %v", problems)
	}
}

func TestUnapproveUnderASignedInSessionWithdrawsApproval(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)
	if result, err := Open(verbReq(root, "01J5X00000000000000000SV00", "mac-a"), "session-undo", "Work a signed-in human withdraws.", OriginMain, "Run it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	approveRequest := verbReq(root, "01J5X00000000000000000SV10", "mac-a")
	approveRequest.Actor.Human = "Wido"
	budget := testBudget()
	if result, err := Approve(approveRequest, []string{"session-undo"}, &budget, sessionProofForTest(t, root, approveRequest.Now)); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("approve: %+v %v", result, err)
	}
	request := verbReq(root, "01J5X00000000000000000SV20", "mac-a")
	request.Actor.Human = "Wido"
	if _, err := Unapprove(request, "session-undo", "the human changed their mind", nil); err == nil || !strings.Contains(err.Error(), "freshly observed") {
		t.Fatalf("unapprove without proof: %v", err)
	}
	result, err := Unapprove(request, "session-undo", "the human changed their mind", sessionProofForTest(t, root, request.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("unapprove under a signed-in session: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Live["session-undo"]
	if file.Approved != nil || file.Budget != nil || file.State != StateQueued {
		t.Fatalf("approval survived the session unapprove: %+v", file)
	}
	// The withdrawal names the hand that made it. Its class is recoverable
	// from nowhere else: the approval record that carried it is gone.
	last := file.History[len(file.History)-1]
	if last.Verb != "unapprove" {
		t.Fatalf("the last verb is %q", last.Verb)
	}
	assertSessionLine(t, last)
	if parsed, problems := ParseFile(RenderFile(file)); parsed == nil || len(problems) != 0 {
		t.Fatalf("ParseFile after a session unapprove reported %v", problems)
	}
}

func TestSetPriorityUnderASignedInSession(t *testing.T) {
	t.Parallel()
	root := rankedGoalBed(t, map[string][2]uint64{"a": {1, 1}, "b": {1, 2}, "c": {1, 3}})
	request := priorityVerbReq(root, "01J5X000000000000000000SP1", "mac-a")
	if _, err := SetPriority(request, "c", 1, sequencePointer(2), parsedSessionProofForTest(t, root, request.Now)); err == nil ||
		!strings.Contains(err.Error(), "signed-in browser session") {
		t.Fatalf("a parsed session proof document reordered the backlog: %v", err)
	}
	result, err := SetPriority(request, "c", 1, sequencePointer(2), sessionProofForTest(t, root, request.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("set-priority under a signed-in session: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	assertPriorityOrder(t, tree, []string{"a", "c", "b"})

	// A re-rank renumbers the band, so it is never about one record: every
	// line this one act wrote names the same hand, and every file it wrote
	// reads back clean.
	moved := 0
	for _, id := range []string{"a", "b", "c"} {
		file := tree.Live[id]
		last := file.History[len(file.History)-1]
		if last.Verb != "set-priority" {
			continue
		}
		moved++
		assertSessionLine(t, last)
		if parsed, problems := ParseFile(RenderFile(file)); parsed == nil || len(problems) != 0 {
			t.Fatalf("ParseFile of re-ranked goal %s reported %v", id, problems)
		}
	}
	if moved < 2 {
		t.Fatalf("an insert renumbered %d goals, want at least two", moved)
	}
}

// Granting and revoking a power of attorney are human acts too, and the root
// record's History is parsed and rendered by the same two functions a goal
// file's is — so a session grant names its session with the same three keys
// and no new one.
func TestGrantAndRevokeUnderASignedInSessionNameTheSession(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)
	granting := verbReq(root, "01J5X00000000000000000SG10", "mac-a")
	granting.Actor.Human = "Wido"
	expires := granting.Now.UTC().AddDate(0, 0, 2).Format("2006-01-02")

	granted, err := Grant(granting, sessionProofForTest(t, root, granting.Now), []uint8{1}, []string{"approve"}, expires)
	if err != nil || granted.Outcome != OutcomeConfirmed {
		t.Fatalf("grant under a signed-in session: %+v %v", granted, err)
	}
	tree, err := loadTree(root, granted.Tip)
	if err != nil {
		t.Fatal(err)
	}
	entry := tree.Root.PowerOfAttorney[len(tree.Root.PowerOfAttorney)-1]
	assertSessionLine(t, tree.Root.History[len(tree.Root.History)-1])
	if parsed, problems := ParseRoot(RenderRoot(tree.Root)); parsed == nil || len(problems) != 0 {
		t.Fatalf("ParseRoot after a session grant reported %v", problems)
	}

	revoking := verbReq(root, "01J5X00000000000000000SG20", "mac-a")
	revoking.Actor.Human = "Wido"
	revoked, err := Revoke(revoking, sessionProofForTest(t, root, revoking.Now), entry.ID)
	if err != nil || revoked.Outcome != OutcomeConfirmed {
		t.Fatalf("revoke under a signed-in session: %+v %v", revoked, err)
	}
	after, err := loadTree(root, revoked.Tip)
	if err != nil {
		t.Fatal(err)
	}
	assertSessionLine(t, after.Root.History[len(after.Root.History)-1])
	if parsed, problems := ParseRoot(RenderRoot(after.Root)); parsed == nil || len(problems) != 0 {
		t.Fatalf("ParseRoot after a session revoke reported %v", problems)
	}
}

func TestTierOverrideOnOpenUnderASignedInSession(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)
	request := verbReq(root, "01J5X00000000000000000SQ10", "mac-a")
	request.Actor.Human = "Wido"
	risk := RiskRecord{Severity: 2, Novelty: 2, Exposure: 1, Accumulation: 1, Basis: "session override fixture"}
	if _, err := OpenRisked(request, "session-open", "Work opened below its derived tier.", OriginHuman, "Run it.", "", risk, 1, "the human judged it smaller", nil, nil); err == nil ||
		!strings.Contains(err.Error(), "freshly observed") {
		t.Fatalf("a tier override without proof opened: %v", err)
	}
	result, err := OpenRisked(request, "session-open", "Work opened below its derived tier.", OriginHuman, "Run it.", "", risk, 1, "the human judged it smaller", nil, sessionProofForTest(t, root, request.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("tier override under a signed-in session: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Live["session-open"]
	if file == nil || file.Tier != 1 {
		t.Fatalf("opened tier = %+v, want 1", file)
	}
	if parsed, problems := ParseFile(RenderFile(file)); parsed == nil || len(problems) != 0 {
		t.Fatalf("ParseFile of the session-overridden open reported %v", problems)
	}
}

func TestReaderRefusesSessionAuthorityWithoutItsSignedInSessionFacts(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)
	if result, err := Open(verbReq(root, "01J5X00000000000000000SR00", "mac-a"), "session-read", "Work whose record is then stripped.", OriginMain, "Run it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	request := verbReq(root, "01J5X00000000000000000SR10", "mac-a")
	request.Actor.Human = "Wido"
	budget := testBudget()
	result, err := Approve(request, []string{"session-read"}, &budget, sessionProofForTest(t, root, request.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("approve: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	approved := tree.Live["session-read"]
	assertSessionApproval(t, approved)
	for _, test := range []struct {
		name  string
		strip func(*HistoryLine)
		want  string
	}{
		{name: "no outcome", strip: func(h *HistoryLine) {
			h.AuthorityOutcome = ""
			h.ChannelProvider, h.ChannelUser, h.ChannelRef = "", "", ""
		}, want: "session authority"},
		{name: "no issuer", strip: func(h *HistoryLine) { h.ChannelProvider = "" }, want: "SIGNED_IN_SESSION requires channelProvider"},
		{name: "no human", strip: func(h *HistoryLine) { h.ChannelUser = "" }, want: "SIGNED_IN_SESSION requires channelProvider"},
		{name: "no session reference", strip: func(h *HistoryLine) { h.ChannelRef = "" }, want: "SIGNED_IN_SESSION requires channelProvider"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			stripped := *approved
			stripped.History = append([]HistoryLine(nil), approved.History...)
			test.strip(&stripped.History[approved.Approved.Revision-1])
			parsed, problems := ParseFile(RenderFile(&stripped))
			if len(problems) == 0 {
				t.Fatalf("a session approval without its History facts read clean: %+v", parsed)
			}
			if !strings.Contains(strings.Join(problemStrings(problems), "\n"), test.want) {
				t.Fatalf("problems %v do not name %q", problems, test.want)
			}
		})
	}
}

func problemStrings(problems []Problem) []string {
	out := make([]string, 0, len(problems))
	for _, problem := range problems {
		out = append(out, string(problem))
	}
	return out
}
