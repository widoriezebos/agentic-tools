package decisions

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/rulings"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/notifications"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
)

// The clock every test composes against. It is injected because every date
// arithmetic on this page — a due date that has passed, an alert inside the
// seven-day window — is measured from it, and a page that read the wall clock
// would answer a different question every day.
var observed = time.Date(2026, 9, 25, 11, 0, 0, 0, time.UTC)

func ago(d time.Duration) string { return observed.Add(-d).UTC().Format(time.RFC3339) }

func row(id, intent string, lane backlog.Lane) backlog.Row {
	return backlog.Row{
		Ref:   backlog.Ref{Kind: "goal", ID: id, Revision: 1},
		Where: backlog.WhereLive, Lane: lane, State: goal.StateQueued,
		Intent: intent, Priority: 2, Sequence: 1, OpenedAt: ago(48 * time.Hour),
	}
}

func needOf(t *testing.T, page Page, kind string) Need {
	t.Helper()
	for _, need := range page.NeedsYou {
		if need.Kind == kind {
			return need
		}
	}
	t.Fatalf("the inbox carries no %s row: %+v", kind, page.NeedsYou)
	return Need{}
}

// everyKind is one workspace with one of every inbox kind in it, so that the
// silence lines, the acts and the order are all asserted over the same page
// rather than over ten pages that each prove one rule.
func everyKind() Inputs {
	awaiting := row("g1-s40", "Approve this one. The rest of the intent.", backlog.LaneToDo)
	awaiting.Priority, awaiting.Sequence = 2, 1

	second := row("g1-s41", "The second thing waiting for an approval", backlog.LaneToDo)
	second.Priority, second.Sequence = 2, 2

	renew := row("g1-s42", "The approval on this one has expired", backlog.LaneToDo)
	renew.State = goal.StateApproved
	renew.Priority, renew.Sequence = 1, 1
	renew.Approved = &backlog.Approval{
		By: "human:Wido", At: ago(30 * 24 * time.Hour), Authority: "relayed",
		ReviewBy: "2026-09-06", Expired: true, ExpiredWhy: "the review date 2026-09-06 has passed",
	}

	claimed := row("g1-s43", "Claimed work whose approval expired underneath it", backlog.LaneInProgress)
	claimed.State = goal.StateClaimed
	claimed.Priority, claimed.Sequence = 1, 2
	claimed.Approved = &backlog.Approval{
		By: "human:Wido", At: ago(29 * 24 * time.Hour), Authority: "relayed",
		ReviewBy: "2026-09-07", Expired: true, ExpiredWhy: "the review date 2026-09-07 has passed",
	}
	claimed.Claim = &backlog.Claim{Machine: "m1e", Lineage: "coordinator", At: ago(3 * time.Hour)}

	park := row("g1-s45", "A goal a person parked", backlog.LaneWaiting)
	park.State = goal.StateParked
	park.Waiting = &backlog.Waiting{
		Reason: "the census format is still being decided", Since: ago(30 * time.Hour),
		By: "human:Wido", From: goal.StateApproved,
	}

	dependency := row("g1-s46", "A goal an open dependency holds", backlog.LaneWaiting)
	dependency.State = goal.StateApproved
	dependency.Waiting = &backlog.Waiting{From: goal.StateApproved}
	dependency.Approved = &backlog.Approval{By: "human:Wido", At: ago(20 * time.Hour), Authority: "proven"}

	blockerPark := row("g1-s47", "A dependency park that returns by itself", backlog.LaneWaiting)
	blockerPark.State = goal.StateParked
	blockerPark.Waiting = &backlog.Waiting{
		Reason: "blocked by g1-s46", Since: ago(20 * time.Hour), By: "human:Wido", Blocker: "g1-s46",
	}

	// A seat's park is the one that stays in the inbox, because a human has
	// not seen it. It is told from a person's by the By prefix, which is the
	// engine's own test of a human park.
	seatPark := row("g1-s50", "A goal a seat parked", backlog.LaneWaiting)
	seatPark.State = goal.StateParked
	seatPark.Waiting = &backlog.Waiting{
		Reason: "the implementer paused it to finish g1-s48 first", Since: ago(30 * time.Hour),
		By: "m2a+implementer", From: goal.StateApproved,
	}

	fenced := row("g1-s48", "Work a breach fence stopped", backlog.LaneWaiting)
	fenced.State = goal.StateClaimed
	fenced.Claim = &backlog.Claim{Machine: "m2a", Lineage: "implementer", At: ago(9 * time.Hour)}
	fenced.Fence = &backlog.Fence{Reason: "the attempt limit was reached", ClosedAt: ago(4 * time.Hour)}

	ready := row("g1-s49", "Approved and ready to claim", backlog.LaneReady)
	ready.State = goal.StateApproved
	ready.Approved = &backlog.Approval{By: "human:Wido", At: ago(2 * time.Hour), Authority: "proven"}

	return Inputs{
		Rows:   []backlog.Row{awaiting, second, renew, claimed, park, dependency, blockerPark, seatPark, fenced, ready},
		Closed: []backlog.Row{},
		Project: project.Pane{
			Goals: []project.Goal{
				{ID: "g1-s9", State: "done"},
				{ID: "g1-s10", State: "done"},
				{ID: "g1-s13", State: "queued"},
			},
			Records: []project.Record{
				{Kind: "design", ID: "d-open", Status: "draft", Title: "A draft nobody accepted",
					Path: "plans/designs/draft.md", ChangedAt: ago(70 * time.Hour)},
				{Kind: "design", ID: "d-landed", Status: "accepted", Title: "Every goal of this one landed",
					Path: "plans/designs/landed.md", Goals: []string{"g1-s9", "g1-s10"}, ChangedAt: ago(26 * time.Hour)},
				{Kind: "design", ID: "d-partly", Status: "accepted", Title: "This one still has work out",
					Path: "plans/designs/partly.md", Goals: []string{"g1-s9", "g1-s13"}, ChangedAt: ago(25 * time.Hour)},
				{Kind: "decision", ID: "dec-1", Status: "accepted", Title: "The roster for design work",
					Path: "docs/decisions/roster.md", ChangedAt: ago(5 * 24 * time.Hour)},
				{Kind: "decision", ID: "dec-2", Status: "accepted", Title: "The newer decision",
					Path: "docs/decisions/newer.md", ChangedAt: ago(24 * time.Hour)},
			},
			Questions: []project.Question{
				{ID: "Q-1", Opened: ago(80 * time.Hour), Question: "Which census format?", Status: "open"},
				{ID: "Q-2", Opened: ago(200 * time.Hour), Question: "Who owns the sweep?", Status: "answered: R-124"},
				{ID: "Q-3", Opened: ago(300 * time.Hour), Question: "A withdrawn one", Status: "withdrawn"},
			},
		},
		Journal: []notifications.Notice{
			{ID: "n-1", At: ago(90 * time.Minute), Message: "the steward could not reach the operator", Source: "alert"},
			{ID: "n-2", At: ago(10 * 24 * time.Hour), Message: "an alert from outside the window", Source: "alert"},
			{ID: "n-3", At: ago(30 * time.Minute), Message: "a tick ran", Source: "tick"},
		},
		Asks: []channel.Question{{
			ID: "ask-1", Goal: "g1-s49", Kind: "budget-above-norm", Machine: "m1e",
			OpenedAt: observed.Add(-6 * time.Hour), State: "open",
			Facts:          []string{"the attempt limit was reached twice"},
			Wants:          "a larger box for the last review round",
			Options:        []channel.Option{{Label: "raise", Consequence: "the goal gets one more round"}},
			Recommendation: "raise it once and stop there",
			Budget: &goalbudget.Budget{
				ElapsedLimit: "8h", AttemptLimit: 10, ReservedJobMinutesLimit: 1200,
				ActiveJobLimit: 1, ReviewRoundLimit: 3,
			},
		}},
		Register: rulings.Register{
			Rows: []rulings.Row{
				{ID: "R-1", Date: "2026-08-26", Words: "The first ruling, which mentions g1-s49 by name and dec-1 as the record of it",
					Context: "given at the start", Owner: "Wido", Condition: ""},
				{ID: "R-2", Date: "2026-09-01", Words: "A temporary ruling whose review has come round",
					Context: "given with the migration", Owner: "Wido",
					Condition: "class=temporary due=2026-09-20", Class: "temporary", Due: "2026-09-20"},
				{ID: "R-3", Date: "2026-09-10", Words: "An assumption-dependent ruling nobody here judges",
					Context: "given during the post-mortem", Owner: "Wido",
					Condition: "class=assumption-dependent event=first-measured-report-exists",
					Class:     "assumption-dependent", Event: "first-measured-report-exists"},
				{ID: "R-4", Date: "2026-09-20", Words: "A temporary ruling still inside its window",
					Context: "given last week", Owner: "Wido",
					Condition: "class=temporary due=2099-01-01", Class: "temporary", Due: "2099-01-01"},
			},
			Reviews: []rulings.Review{
				{ID: "R-2", Owner: "Wido", Class: "temporary", Due: "2026-09-20"},
				{ID: "R-3", Owner: "Wido", Class: "assumption-dependent", Event: "first-measured-report-exists"},
				{ID: "R-4", Owner: "Wido", Class: "temporary", Due: "2099-01-01"},
			},
			Defects: []rulings.Defect{{Label: "R-9", Reason: "review condition needs due= or event="}},
		},
		Human: Standing{Proven: true},
	}
}

func TestTheInboxCarriesOneRowOfEveryKindTheDesignNames(t *testing.T) {
	t.Parallel()
	page := Compose(everyKind(), observed)
	counted := map[string]int{}
	for _, need := range page.NeedsYou {
		counted[need.Kind]++
	}
	want := map[string]int{
		KindApproval: 2, KindRenewal: 2, KindAsk: 1, KindQuestion: 1,
		KindParked: 1, KindStopped: 1, KindDraft: 1, KindLanded: 1,
		KindRulingReview: 1, KindAlert: 1,
	}
	for kind, count := range want {
		if counted[kind] != count {
			t.Errorf("%s rows: got %d, want %d", kind, counted[kind], count)
		}
	}
	if len(counted) != len(want) {
		t.Errorf("the inbox carries a kind the design does not name: %v", counted)
	}
	if page.Counts.NeedsYou != len(page.NeedsYou) {
		t.Errorf("the inbox count is not the inbox: %d rows, count %d", len(page.NeedsYou), page.Counts.NeedsYou)
	}
	if page.SchemaVersion != SchemaVersion || page.ReadAt != observed.Format(time.RFC3339) {
		t.Errorf("the page did not stamp itself: %d %q", page.SchemaVersion, page.ReadAt)
	}
}

// Every silence line, word for word. They are the page's claims about the
// engine, and a reworded one is a different claim.
func TestEverySilenceLineIsTheOneTheEngineMakesTrue(t *testing.T) {
	t.Parallel()
	page := Compose(everyKind(), observed)
	said := map[string][]string{}
	for _, need := range page.NeedsYou {
		said[need.Kind] = append(said[need.Kind], need.Silence)
	}
	for _, expected := range []struct {
		kind  string
		lines []string
	}{
		{KindApproval, []string{"it stays in To Do and no seat may claim it", "it stays in To Do and no seat may claim it"}},
		// The two renewals are two claims: the gate refuses a fresh claim on
		// an expired approval, and it is the gate for creating a claim rather
		// than for continuing under one.
		{KindRenewal, []string{"no fresh claim is admitted", "work already claimed continues; renew at the goal"}},
		{KindAsk, []string{"no recorded consequence"}},
		{KindQuestion, []string{"it stays open"}},
		{KindParked, []string{"it stays parked"}},
		{KindStopped, []string{"it stays stopped; its claim keeps the goal"}},
		{KindDraft, []string{"it stays a draft, shown as one on Project"}},
		{KindLanded, []string{"it stays marked accepted"}},
		{KindRulingReview, []string{"it stays in force as written"}},
		{KindAlert, []string{"no recorded consequence"}},
	} {
		if strings.Join(said[expected.kind], "|") != strings.Join(expected.lines, "|") {
			t.Errorf("%s says %q, want %q", expected.kind, said[expected.kind], expected.lines)
		}
	}
	for _, need := range page.NeedsYou {
		if need.Silence == "" {
			t.Errorf("%s row %s says nothing about silence", need.Kind, need.ID)
		}
	}
}

// A design with no status of its own does not lose the end of its sentence.
func TestALandedDesignWithNoStatusSaysSoRatherThanTrailingOff(t *testing.T) {
	t.Parallel()
	in := everyKind()
	for index := range in.Project.Records {
		if in.Project.Records[index].ID == "d-landed" {
			in.Project.Records[index].Status = ""
		}
	}
	need := needOf(t, Compose(in, observed), KindLanded)
	if need.Silence != "it stays marked with no status of its own" {
		t.Fatalf("the silence line trailed off: %q", need.Silence)
	}
}

func TestOnlyTheActsTheInterfaceHasAreOffered(t *testing.T) {
	t.Parallel()
	page := Compose(everyKind(), observed)
	for _, need := range page.NeedsYou {
		switch need.Kind {
		case KindApproval:
			if need.Act != ActApprove || need.Row == nil {
				t.Errorf("an approval was not offered the sheet with its row: %+v", need)
			}
		case KindRenewal:
			// The sheet only for an unclaimed row: the engine refuses a
			// budget on claimed work, so a prefilled sheet there is a form
			// the engine throws away.
			claimed := need.ID == "g1-s43"
			if claimed && (need.Act != "" || need.Row != nil) {
				t.Errorf("a claimed renewal was offered the sheet: %+v", need)
			}
			if !claimed && (need.Act != ActApprove || need.Row == nil) {
				t.Errorf("an unclaimed renewal was not offered the sheet: %+v", need)
			}
		case KindParked:
			// A seat's park carries the act that returns it to the queue,
			// which a signed-in session may make (R-125-m1u). It carries no
			// row: nothing about it is prefilled from the board.
			if need.Act != ActUnpark || need.Row != nil {
				t.Errorf("a seat's park was not offered the act that returns it: %+v", need)
			}
		default:
			if need.Act != "" || need.Row != nil {
				t.Errorf("%s offered an act this interface does not have: %+v", need.Kind, need)
			}
		}
	}
}

// A renewal has been waiting since its approval stopped admitting work, not
// since the approval was given: that is the instant the gate began refusing,
// and it is what "how long past due" measures.
func TestARenewalIsDatedFromWhenItsApprovalStoppedAdmittingWork(t *testing.T) {
	t.Parallel()
	page := Compose(everyKind(), observed)
	for _, need := range page.NeedsYou {
		if need.Kind != KindRenewal {
			continue
		}
		want := map[string]string{"g1-s42": "2026-09-06T00:00:00Z", "g1-s43": "2026-09-07T00:00:00Z"}
		if need.Since != want[need.ID] {
			t.Errorf("%s is dated %q, want %q", need.ID, need.Since, want[need.ID])
		}
		if !strings.HasPrefix(need.Asked, "Renew the approval of ") || !strings.HasSuffix(need.Asked, " has passed") {
			t.Errorf("the renewal did not say what expired and why: %q", need.Asked)
		}
	}

	// An approval that expired for either of the other two reasons the horizon
	// carries is dated from this read, and the review date it still names is not
	// the answer. Every relayed approval the ledger accepts names a review date,
	// and ApprovalExpired refuses on the fleet's first enrollment before it ever
	// reads that date, so both rows below keep a review date well in the future:
	// dating them from it would say the renewal began waiting after this read.
	// The approval's own instant is no answer either — that is when the approval
	// was given, the one instant it certainly is not.
	for _, why := range []string{
		"the fleet's first terminal was enrolled at 2026-09-10T00:00:00Z",
		"the temporary authority horizon 2026-09-20 has passed",
	} {
		in := everyKind()
		in.Rows[2].Approved.ReviewBy = "2026-12-31"
		in.Rows[2].Approved.ExpiredWhy = why
		found := false
		for _, need := range Compose(in, observed).NeedsYou {
			if need.Kind != KindRenewal || need.ID != "g1-s42" {
				continue
			}
			found = true
			if need.Since != observed.Format(time.RFC3339) {
				t.Errorf("%q is dated %q, want this read's own instant %q",
					why, need.Since, observed.Format(time.RFC3339))
			}
			if need.Since == "2026-12-31T00:00:00Z" {
				t.Errorf("%q was dated from a review date that has not passed", why)
			}
			if need.Since == ago(30*24*time.Hour) {
				t.Errorf("%q was dated from when the approval was given", why)
			}
		}
		if !found {
			t.Errorf("the renewal that expired by %q left the inbox", why)
		}
	}
}

func TestTheRowsThatAreDecidedAtATerminalNameTheirCommand(t *testing.T) {
	t.Parallel()
	page := Compose(everyKind(), observed)
	commands := map[string]string{}
	for _, need := range page.NeedsYou {
		if need.Command != "" {
			commands[need.Kind] = need.Command
		}
	}
	// The parked row used to name a terminal command. It names an act now:
	// a signed-in session may return a seat's park from the page itself
	// (R-125-m1u), and a command beside a button that does the same thing is
	// one more thing to read.
	want := map[string]string{
		KindStopped: "metasystem goal resume --id g1-s48",
	}
	for kind, command := range want {
		if commands[kind] != command {
			t.Errorf("%s names %q, want %q", kind, commands[kind], command)
		}
	}
	if len(commands) != len(want) {
		t.Errorf("a row named a command the engine was not asked for: %v", commands)
	}
}

// The inbox's parked row is a SEAT's park and nothing else. A person's park
// is a decision they already made and is in Not now; an approved goal an open
// dependency holds is in the same lane with the same empty blocker and is not
// a park at all; a dependency park names its blocker.
func TestParkedIsASeatsParkAndNotAPersonsOrADependencyWait(t *testing.T) {
	t.Parallel()
	page := Compose(everyKind(), observed)
	parked := []string{}
	for _, need := range page.NeedsYou {
		if need.Kind == KindParked {
			parked = append(parked, need.ID)
		}
	}
	if strings.Join(parked, ",") != "g1-s50" {
		t.Fatalf("the parked rows are not the seat's own parks: %v", parked)
	}
	need := needOf(t, page, KindParked)
	if need.Asked != "Unpark A goal a seat parked? parked by m2a+implementer "+ago(30*time.Hour)+": the implementer paused it to finish g1-s48 first" {
		t.Fatalf("the park's own words did not reach the row: %q", need.Asked)
	}
	if need.Act != ActUnpark {
		t.Fatalf("the seat park offers %q, want the act that returns it", need.Act)
	}
}

// A person's park leaves the inbox for Not now, with the whole of what they
// said, newest first — and a blocker park a human directed is one too, named
// with the blocker it waits for.
func TestAPersonsParkIsDecidedAndNamesWhatItWaitsFor(t *testing.T) {
	t.Parallel()
	page := Compose(everyKind(), observed)
	got := []string{}
	for _, park := range page.Decided.NotNow {
		got = append(got, park.ID)
	}
	// g1-s47 was parked ten hours after g1-s45, so it is first.
	if strings.Join(got, ",") != "g1-s47,g1-s45" {
		t.Fatalf("Not now is not the person's own parks newest first: %v", got)
	}
	plain := page.Decided.NotNow[1]
	if plain.Title != "A goal a person parked" || plain.By != "human:Wido" ||
		plain.At != ago(30*time.Hour) || plain.Because != "the census format is still being decided" {
		t.Fatalf("the park's own words did not reach the row: %+v", plain)
	}
	if plain.Blocker != "" {
		t.Fatalf("a park with no blocker named one: %q", plain.Blocker)
	}
	if plain.Where.Kind != WhereGoal || plain.Where.ID != "g1-s45" {
		t.Fatalf("the row does not open its goal: %+v", plain.Where)
	}
	if page.Decided.NotNow[0].Blocker != "g1-s46" {
		t.Fatalf("a human-directed blocker park did not name its blocker: %+v", page.Decided.NotNow[0])
	}
	for _, need := range page.NeedsYou {
		if need.ID == "g1-s45" || need.ID == "g1-s47" {
			t.Fatalf("a park this human already made is still counted as waiting on them: %+v", need)
		}
	}
}

// The header's two counts are the one list split, so they always sum to it.
func TestTheTwoCountsAreTheInboxSplitAndSumToIt(t *testing.T) {
	t.Parallel()
	page := Compose(everyKind(), observed)
	approvals := 0
	for _, need := range page.NeedsYou {
		if need.Kind == KindApproval {
			approvals++
		}
	}
	if page.Counts.Waiting != approvals {
		t.Errorf("waiting = %d, want the %d approvals", page.Counts.Waiting, approvals)
	}
	if page.Counts.Asked != len(page.NeedsYou)-approvals {
		t.Errorf("asked = %d, want the %d rows that are not approvals",
			page.Counts.Asked, len(page.NeedsYou)-approvals)
	}
	if page.Counts.Asked+page.Counts.Waiting != page.Counts.NeedsYou {
		t.Errorf("the two blocks do not sum to the inbox: %d + %d != %d",
			page.Counts.Asked, page.Counts.Waiting, page.Counts.NeedsYou)
	}
	// Four, since a mention says where it opens. The number is asserted as a
	// literal rather than against the constant, because a reader parses the
	// number and not the constant.
	if page.SchemaVersion != 4 {
		t.Errorf("schema = %d, want 4", page.SchemaVersion)
	}
}

// The register is where the caller says it is, and every destination naming
// it says the same thing, so "open the register" opens.
func TestTheRegisterPathTravelsAndEveryDestinationUsesIt(t *testing.T) {
	t.Parallel()
	in := everyKind()
	in.RegisterPath = "metasystem/memory/rulings.md"
	page := Compose(in, observed)
	if page.Register != "metasystem/memory/rulings.md" {
		t.Fatalf("the payload names the register at %q", page.Register)
	}
	reviews := 0
	for _, need := range page.NeedsYou {
		if need.Kind != KindRulingReview {
			continue
		}
		reviews++
		if need.Where.Kind != WhereRegister || need.Where.ID != page.Register {
			t.Errorf("a review card opens %+v, want the register the payload names", need.Where)
		}
	}
	if reviews == 0 {
		t.Fatal("no review card was composed, so nothing was proven")
	}
	// A caller that names no path leaves the kit's own layout, which is what
	// every reader saw before this field existed.
	if plain := Compose(everyKind(), observed); plain.Register != "memory/rulings.md" {
		t.Errorf("an unnamed register answered %q", plain.Register)
	}
}

func TestAnAskShowsTheRecordAndTheSeatsRecommendation(t *testing.T) {
	t.Parallel()
	page := Compose(everyKind(), observed)
	need := needOf(t, page, KindAsk)
	if need.Asked != "a larger box for the last review round · the attempt limit was reached twice · raise: the goal gets one more round · proposed budget 8h/10/1200/1/3" {
		t.Fatalf("the ask did not show the record as recorded: %q", need.Asked)
	}
	if need.Recommend != "raise it once and stop there" {
		t.Fatalf("the seat's recommendation was dropped: %q", need.Recommend)
	}
	if need.By != "seat m1e" || need.Title != "g1-s49 · budget-above-norm" {
		t.Fatalf("the ask did not say who asks or what about: %+v", need)
	}
	if need.Where.Kind != WhereChannel {
		t.Fatalf("the ask pointed somewhere other than the channel it is answered on: %+v", need.Where)
	}
	if need.Deadline != "" {
		t.Fatalf("a channel question has no deadline field and the page invented one: %q", need.Deadline)
	}
}

func TestAnAskWithoutABudgetShowsNoBudget(t *testing.T) {
	t.Parallel()
	in := everyKind()
	in.Asks[0].Budget = nil
	in.Asks[0].Kind = "decision"
	need := needOf(t, Compose(in, observed), KindAsk)
	if strings.Contains(need.Asked, "proposed budget") {
		t.Fatalf("a question with no budget carried one: %q", need.Asked)
	}
	if need.Asked != "a larger box for the last review round · the attempt limit was reached twice · raise: the goal gets one more round" {
		t.Fatalf("the rest of the record did not survive: %q", need.Asked)
	}
}

// The due arithmetic is the clock's. R-2 is due and in the inbox; R-3 is an
// event and is never judged here; R-4 is not due yet.
func TestAnInboxReviewIsADueDateAndNeverAnEvent(t *testing.T) {
	t.Parallel()
	page := Compose(everyKind(), observed)
	reviews := []string{}
	for _, need := range page.NeedsYou {
		if need.Kind == KindRulingReview {
			reviews = append(reviews, need.ID)
		}
	}
	if strings.Join(reviews, ",") != "R-2" {
		t.Fatalf("the inbox judged an event or missed a due date: %v", reviews)
	}
	need := needOf(t, page, KindRulingReview)
	if need.Asked != "Review R-2, due 2026-09-20: adopt, revise or withdraw" {
		t.Fatalf("the review did not say what is being asked: %q", need.Asked)
	}
	if need.Where.Kind != WhereRegister || need.Where.ID != "memory/rulings.md" {
		t.Fatalf("the review did not open the register: %+v", need.Where)
	}

	// A day earlier the review is not due, and the inbox is one row shorter.
	earlier := Compose(everyKind(), time.Date(2026, 9, 19, 23, 0, 0, 0, time.UTC))
	for _, need := range earlier.NeedsYou {
		if need.Kind == KindRulingReview {
			t.Fatalf("a review came due before its own date: %+v", need)
		}
	}
	// And on the day itself it is.
	sameDay := Compose(everyKind(), time.Date(2026, 9, 20, 0, 1, 0, 0, time.UTC))
	found := false
	for _, need := range sameDay.NeedsYou {
		if need.Kind == KindRulingReview {
			found = true
		}
	}
	if !found {
		t.Fatal("a review was not due on its own due date")
	}
}

func TestAlertsAreTheLastSevenDaysOfWhatTheStewardAddressedToAHuman(t *testing.T) {
	t.Parallel()
	page := Compose(everyKind(), observed)
	alerts := []string{}
	for _, need := range page.NeedsYou {
		if need.Kind == KindAlert {
			alerts = append(alerts, need.ID)
		}
	}
	if strings.Join(alerts, ",") != "n-1" {
		t.Fatalf("the alert window or the source filter moved: %v", alerts)
	}
}

func TestTheOrderIsDeadlinesThenPastDueThenApprovalsThenTheOldest(t *testing.T) {
	t.Parallel()
	page := Compose(everyKind(), observed)
	var read []string
	for _, need := range page.NeedsYou {
		read = append(read, need.Kind+"/"+need.ID)
	}
	want := []string{
		// Past due, most overdue first: the two approvals whose review dates
		// passed nineteen and eighteen days ago, then the ruling review that
		// came due five days ago.
		"renewal/g1-s42",
		"renewal/g1-s43",
		"ruling-review/R-2",
		// Then the unapproved work, in the backlog's own rank order.
		"approval/g1-s40",
		"approval/g1-s41",
		// Then everything else, oldest first.
		"question/Q-1",
		"draft/d-open",
		"parked/g1-s50",
		"landed/d-landed",
		"ask/ask-1",
		"stopped/g1-s48",
		"alert/n-1",
	}
	if strings.Join(read, ",") != strings.Join(want, ",") {
		t.Fatalf("the inbox order moved:\n got %v\nwant %v", read, want)
	}
}

// The first tier is deadlines, and nothing this build composes carries one.
// The rule is still the rule, so it is asserted where it lives.
func TestADeadlineLeadsTheInboxWhateverKindCarriesIt(t *testing.T) {
	t.Parallel()
	needs := []Need{
		{Kind: KindApproval, ID: "a"},
		{Kind: KindRulingReview, ID: "r", Since: "2026-01-01T00:00:00Z"},
		{Kind: KindAsk, ID: "late", Deadline: "2026-12-01T00:00:00Z"},
		{Kind: KindAsk, ID: "soon", Deadline: "2026-10-01T00:00:00Z"},
		{Kind: KindAlert, ID: "old", Since: "2020-01-01T00:00:00Z"},
	}
	order(needs)
	var read []string
	for _, need := range needs {
		read = append(read, need.ID)
	}
	if strings.Join(read, ",") != "soon,late,r,a,old" {
		t.Fatalf("the four tiers are not in the design's order: %v", read)
	}
}

// An undated row is never the oldest: nothing recorded when it began.
func TestAnUndatedRowIsNeverTheOldest(t *testing.T) {
	t.Parallel()
	needs := []Need{
		{Kind: KindAlert, ID: "undated"},
		{Kind: KindAlert, ID: "dated", Since: "2026-01-01T00:00:00Z"},
	}
	order(needs)
	if needs[0].ID != "dated" {
		t.Fatalf("an undated row claimed to have begun first: %+v", needs)
	}
}

func TestSignInIsItsOwnFactAndNotAnInboxRow(t *testing.T) {
	t.Parallel()
	in := everyKind()
	in.Human = Standing{Proven: false}
	page := Compose(in, observed)
	if !page.SignIn {
		t.Fatal("a seat nobody is signed into did not say so")
	}
	for _, need := range page.NeedsYou {
		if need.Kind == "sign-in" {
			t.Fatalf("signing in became an inbox row: %+v", need)
		}
	}
	if Compose(everyKind(), observed).SignIn {
		t.Fatal("a proven seat was asked to sign in")
	}
}

func TestAnEmptyWorkspaceHasAnEmptyInboxAndAnEmptyRegister(t *testing.T) {
	t.Parallel()
	page := Compose(Inputs{Human: Standing{Proven: true}}, observed)
	if len(page.NeedsYou) != 0 || page.Counts.NeedsYou != 0 {
		t.Fatalf("an empty workspace had something waiting: %+v", page.NeedsYou)
	}
	if len(page.Decided.Rulings) != 0 || len(page.Decided.Defects) != 0 ||
		len(page.Decided.Decisions) != 0 || len(page.Decided.Answered) != 0 || len(page.Decided.Approved) != 0 {
		t.Fatalf("an empty workspace decided something: %+v", page.Decided)
	}
	if page.Counts.Rulings != 0 {
		t.Fatalf("an empty register counted %d rulings", page.Counts.Rulings)
	}
}

/* ------------------------------------------------------------ what you said -- */

func TestRulingsAreNewestFirstAndWholeWithTheirConditionAsWritten(t *testing.T) {
	t.Parallel()
	page := Compose(everyKind(), observed)
	var ids []string
	for _, ruling := range page.Decided.Rulings {
		ids = append(ids, ruling.ID)
	}
	if strings.Join(ids, ",") != "R-4,R-3,R-2,R-1" {
		t.Fatalf("the register is not newest first: %v", ids)
	}
	if page.Counts.Rulings != 4 {
		t.Fatalf("the register count is not the register: %d", page.Counts.Rulings)
	}
	first := page.Decided.Rulings[0]
	if first.Words != "A temporary ruling still inside its window" || first.Context != "given last week" ||
		first.Owner != "Wido" || first.Condition != "class=temporary due=2099-01-01" {
		t.Fatalf("a ruling did not come through whole: %+v", first)
	}
}

func TestDuePassedIsADateAndAnEventConditionIsShownAndNeverJudged(t *testing.T) {
	t.Parallel()
	page := Compose(everyKind(), observed)
	passed := map[string]bool{}
	events := map[string]string{}
	for _, ruling := range page.Decided.Rulings {
		passed[ruling.ID] = ruling.DuePassed
		events[ruling.ID] = ruling.Event
	}
	if !passed["R-2"] || passed["R-4"] || passed["R-1"] {
		t.Errorf("the due judgement moved: %v", passed)
	}
	if passed["R-3"] {
		t.Error("an event condition was judged due by this page")
	}
	if events["R-3"] != "first-measured-report-exists" {
		t.Errorf("the event condition was not shown: %q", events["R-3"])
	}
}

// A ruling's mentions are the goals and the records its words name, each with
// where it opens: a goal opens the goal, and a record opens the record at the
// path the reader opens it by rather than at the id the words wrote.
func TestMentionsAreTheGoalsAndRecordsTheWordsNameVerbatim(t *testing.T) {
	t.Parallel()
	page := Compose(everyKind(), observed)
	for _, ruling := range page.Decided.Rulings {
		if ruling.ID != "R-1" {
			if len(ruling.Mentions) != 0 {
				t.Errorf("%s mentioned something its words do not name: %v", ruling.ID, ruling.Mentions)
			}
			continue
		}
		want := []Mention{
			{ID: "g1-s49", Where: Where{Kind: WhereGoal, ID: "g1-s49"}},
			{ID: "dec-1", Where: Where{Kind: WhereRecord, ID: "docs/decisions/roster.md"}},
		}
		if len(ruling.Mentions) != len(want) {
			t.Fatalf("R-1 mentions %+v, want %+v", ruling.Mentions, want)
		}
		for index := range want {
			if ruling.Mentions[index] != want[index] {
				t.Errorf("mention %d is %+v, want %+v", index, ruling.Mentions[index], want[index])
			}
		}
	}
}

// A mention is the whole id and nothing inside a longer name.
func TestAMentionIsAWholeIdAndNeverAPrefixOfALongerOne(t *testing.T) {
	t.Parallel()
	named := map[string]Where{}
	for _, id := range []string{"g1-s4", "g1-s44", "g1-s44b"} {
		named[id] = Where{Kind: WhereGoal, ID: id}
	}
	// A record id is a ULID in this checkout, which is one token like any
	// other: it is found the same way and nothing about its length matters.
	named["01M3F0YA88RXTV1A1FSEHVVJ9Z"] = Where{Kind: WhereRecord, ID: "plans/designs/a.md"}
	for _, read := range []struct {
		words string
		found string
	}{
		{"nothing at all", ""},
		{"about g1-s4 only", "g1-s4"},
		{"about g1-s44 only", "g1-s44"},
		{"about g1-s44b only", "g1-s44b"},
		{"g1-s44, then g1-s4.", "g1-s44,g1-s4"},
		{"(g1-s4)", "g1-s4"},
		{"xg1-s4x", ""},
		{"g1-s4 and g1-s4 again", "g1-s4"},
		{"recorded at 01M3F0YA88RXTV1A1FSEHVVJ9Z.", "01M3F0YA88RXTV1A1FSEHVVJ9Z"},
		{"01M3F0YA88RXTV1A1FSEHVVJ9Zx is not it", ""},
	} {
		found := []string{}
		for _, mention := range mentions(read.words, named) {
			found = append(found, mention.ID)
		}
		if got := strings.Join(found, ","); got != read.found {
			t.Errorf("%q mentions %q, want %q", read.words, got, read.found)
		}
	}
}

func TestDefectsAreListedInTheStewardsOwnWords(t *testing.T) {
	t.Parallel()
	page := Compose(everyKind(), observed)
	if strings.Join(page.Decided.Defects, "|") != "R-9: review condition needs due= or event=" {
		t.Fatalf("the register's defects were hidden or reworded: %v", page.Decided.Defects)
	}
}

func TestTheFourDecidedTabsAreTheirOwnLists(t *testing.T) {
	t.Parallel()
	page := Compose(everyKind(), observed)

	var decisions []string
	for _, item := range page.Decided.Decisions {
		decisions = append(decisions, item.ID)
	}
	if strings.Join(decisions, ",") != "dec-2,dec-1" {
		t.Errorf("the decision records are not newest first: %v", decisions)
	}
	if page.Decided.Decisions[0].Where.Kind != WhereRecord || page.Decided.Decisions[0].Where.ID != "docs/decisions/newer.md" {
		t.Errorf("a decision record does not open its own document: %+v", page.Decided.Decisions[0])
	}

	if len(page.Decided.Answered) != 1 || page.Decided.Answered[0].ID != "Q-2" {
		t.Errorf("the answered tab is not the register's answered rows: %+v", page.Decided.Answered)
	}
	if page.Decided.Answered[0].Note != "R-124" {
		t.Errorf("an answered question lost its answer reference: %q", page.Decided.Answered[0].Note)
	}

	var approved []string
	for _, one := range page.Decided.Approved {
		approved = append(approved, one.ID)
	}
	// Newest approval first, and every row that carries one, whatever lane
	// it stands in.
	if strings.Join(approved, ",") != "g1-s49,g1-s46,g1-s43,g1-s42" {
		t.Errorf("the approvals are not newest first: %v", approved)
	}
	if page.Decided.Approved[0].Row.Ref.ID != "g1-s49" || page.Decided.Approved[0].Row.Lane != backlog.LaneReady {
		t.Errorf("an approval did not carry its whole row: %+v", page.Decided.Approved[0].Row)
	}
	if !page.Decided.Approved[3].Expired {
		t.Errorf("an expired approval did not say so on the Approved tab: %+v", page.Decided.Approved[3])
	}
}

func TestAConcludedGoalsApprovalIsStillOnTheRecord(t *testing.T) {
	t.Parallel()
	in := everyKind()
	closed := row("g1-s8", "The toolchain and the committed bundle", backlog.LaneDone)
	closed.Where = backlog.WhereArchived
	closed.State = "done"
	closed.Approved = &backlog.Approval{By: "human:Wido", At: ago(40 * 24 * time.Hour), Authority: "proven"}
	in.Closed = []backlog.Row{closed}
	page := Compose(in, observed)
	last := page.Decided.Approved[len(page.Decided.Approved)-1]
	if last.ID != "g1-s8" || last.Row.Where != backlog.WhereArchived {
		t.Fatalf("a concluded goal's approval was dropped: %+v", last)
	}
}
