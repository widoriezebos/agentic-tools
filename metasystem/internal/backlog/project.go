package backlog

import (
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
)

// PhaseNotRecorded is what the projection says where the master's lane
// meaning asks for an execution phase the ledger does not carry. The claim
// record proves ownership and landing, and nothing finer, so the row shows
// the lane and names the gap rather than inventing a phase.
const PhaseNotRecorded = "not recorded"

// DraftStatement is what the Draft lane says instead of counting proposals.
// No engine code reads the drafts directory, and counting its files would be
// a read of the very source this statement says nothing reads.
const DraftStatement = "plans/goals-drafts/ has no reader in the engine; a drafts owner arrives with gate 5 (M6)"

// The two places a goal record lives.
const (
	WhereLive     = "live"
	WhereArchived = "archived"
)

// Ref identifies one record the way every view of it identifies it: what kind
// of thing it is, which one, and at which revision. No route and no component
// name belongs here.
type Ref struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Revision uint64 `json:"revision"`
}

// Approval is the human admission recorded on the goal, with this
// observation's verdict on whether it still admits new work.
type Approval struct {
	By         string `json:"by"`
	At         string `json:"at"`
	Authority  string `json:"authority"`
	ReviewBy   string `json:"reviewBy"`
	Expired    bool   `json:"expired"`
	ExpiredWhy string `json:"expiredWhy"`
}

// Claim is who holds the goal and since when, with the landing stamp when
// the work is built and waiting to land.
type Claim struct {
	Machine   string `json:"machine"`
	Lineage   string `json:"lineage"`
	At        string `json:"at"`
	LandingAt string `json:"landingAt"`
}

// Waiting is why the goal is held and, where the record proves it, the state
// it is waiting from.
type Waiting struct {
	Reason  string `json:"reason"`
	Since   string `json:"since"`
	By      string `json:"by"`
	From    string `json:"from"`
	Blocker string `json:"blocker"`
}

// Abandoned is the recorded reason work was dropped.
type Abandoned struct {
	By      string `json:"by"`
	At      string `json:"at"`
	Because string `json:"because"`
}

// Fence is the breach stop that closed launch admission for a claim.
type Fence struct {
	Reason   string `json:"reason"`
	ClosedAt string `json:"closedAt"`
}

// Row is one goal as a reader sees it: its identity, its lane, the record's
// own facts, and every gap the record leaves open.
type Row struct {
	Ref          `json:"ref"`
	Where        string    `json:"where"`
	Lane         Lane      `json:"lane"`
	Phase        string    `json:"phase"`
	State        string    `json:"state"`
	Intent       string    `json:"intent"`
	NextStep     string    `json:"nextStep"`
	Concluded    string    `json:"concluded"`
	Origin       string    `json:"origin"`
	Priority     uint8     `json:"priority"`
	Sequence     uint64    `json:"sequence"`
	Tier         uint8     `json:"tier"`
	Labels       []string  `json:"labels"`
	Arc          string    `json:"arc"`
	Pinned       string    `json:"pinned"`
	BlockedBy    []string  `json:"blockedBy"`
	OpenBlockers []string  `json:"openBlockers"`
	Approved     *Approval `json:"approved,omitempty"`
	// Budget is the complete limit tuple the goal's record carries, where it
	// carries one. It is here because the approval sheet prefills from it:
	// the machinery never invents a budget, so the one the record already
	// holds is the only one a human can be offered without being asked.
	Budget       *goalbudget.Budget `json:"budget,omitempty"`
	Claim        *Claim             `json:"claim,omitempty"`
	Waiting      *Waiting           `json:"waiting,omitempty"`
	Abandoned    *Abandoned         `json:"abandoned,omitempty"`
	Fence        *Fence             `json:"fence,omitempty"`
	Sliced       bool               `json:"sliced"`
	Decomposed   bool               `json:"decomposed"`
	OpenedAt     string             `json:"openedAt"`
	LastChangeAt string             `json:"lastChangeAt"`
	LastVerb     string             `json:"lastVerb"`
	Gaps         []string           `json:"gaps"`
}

// DraftGap says why the Draft lane carries no rows and no count.
type DraftGap struct {
	Statement string `json:"statement"`
}

// Board is one ledger tree placed: the live goals in backlog order, the
// concluded ones behind them, and a count for every lane that can hold rows.
type Board struct {
	Rows   []Row        `json:"rows"`
	Closed []Row        `json:"closed"`
	Counts map[Lane]int `json:"counts"`
	Draft  DraftGap     `json:"draft"`
}

// Project places every goal of one tree exactly once. Live goals keep the
// engine's own backlog order so the browser and the terminal agree on what
// comes first; concluded goals are ordered by id, completed before dropped.
func Project(tree *goal.TreeGoals, horizon goal.ApprovalHorizon, admission Admission) Board {
	board := Board{Rows: []Row{}, Closed: []Row{}, Counts: map[Lane]int{}, Draft: DraftGap{Statement: DraftStatement}}
	for _, lane := range LaneOrder {
		if lane != LaneDraft {
			board.Counts[lane] = 0
		}
	}
	if tree == nil {
		return board
	}
	for _, id := range goal.OrderedOpenGoalIDs(tree.Live) {
		row := rowOf(tree.Live[id], WhereLive, tree, horizon, admission)
		board.Rows = append(board.Rows, row)
		board.Counts[row.Lane]++
	}
	for _, id := range goal.SortedGoalIds(tree.Done) {
		row := rowOf(tree.Done[id], WhereArchived, tree, horizon, admission)
		board.Closed = append(board.Closed, row)
		board.Counts[row.Lane]++
	}
	for _, id := range goal.SortedGoalIds(tree.Abandoned) {
		row := rowOf(tree.Abandoned[id], WhereArchived, tree, horizon, admission)
		board.Closed = append(board.Closed, row)
		board.Counts[row.Lane]++
	}
	return board
}

// LaneOf decides where one goal stands, what its execution phase is, and what
// the record does not say. Waiting takes precedence over the active and ready
// lanes, never over To Do: an approval the gate will not act on is an intake
// gap, and the goal keeps its To Do placement with the gap visible, which is
// also the order the engine's own frontier judges in.
func LaneOf(f *goal.GoalFile, tree *goal.TreeGoals, horizon goal.ApprovalHorizon, admission Admission) (Lane, string, []string) {
	if f == nil {
		return LaneUnknown, "", []string{"the record is missing"}
	}
	switch f.State {
	case goal.StateClaimed:
		switch {
		case f.StopFence != nil:
			// A breach fence outranks a landing: the work is stopped and
			// waiting on a human whatever else the claim carries.
			return LaneWaiting, PhaseNotRecorded, nil
		case f.Landing != nil:
			return LaneReview, "landing", nil
		default:
			return LaneInProgress, PhaseNotRecorded, []string{"phase not recorded"}
		}
	case goal.StateParked:
		return LaneWaiting, PhaseNotRecorded, nil
	case goal.StateApproved:
		switch {
		case admission.Awaiting[f.Id]:
			_, why := f.ApprovalExpired(horizon)
			return LaneToDo, "", []string{"approval expired: " + why}
		case admission.Blocked[f.Id]:
			return LaneWaiting, "", nil
		case admission.Ready[f.Id]:
			return LaneReady, "", nil
		}
		if cause, refused := admission.Refused[f.Id]; refused {
			return LaneToDo, "", []string{"claim refused: " + cause}
		}
		if !admission.Answered {
			return LaneUnknown, "", []string{"claim admission not answered: " + admission.Message}
		}
		return LaneUnknown, "", []string{"the engine's frontier did not place this goal"}
	case goal.StateQueued:
		return LaneToDo, "", []string{"not approved"}
	case goal.StateDone:
		return LaneDone, "", nil
	case goal.StateAbandoned:
		return LaneAbandoned, "", nil
	}
	return LaneUnknown, "", []string{"state " + f.State + " is not placed by this build"}
}

func rowOf(f *goal.GoalFile, where string, tree *goal.TreeGoals, horizon goal.ApprovalHorizon, admission Admission) Row {
	lane, phase, gaps := LaneOf(f, tree, horizon, admission)
	open, unknown := openBlockers(f, tree)
	row := Row{
		Ref:          Ref{Kind: "goal", ID: f.Id, Revision: f.Revision},
		Where:        where,
		Lane:         lane,
		Phase:        phase,
		State:        f.State,
		Intent:       f.Intent,
		NextStep:     f.NextStep,
		Concluded:    f.Conclude,
		Origin:       f.Origin,
		Priority:     f.Priority,
		Sequence:     f.Sequence,
		Tier:         f.Tier,
		Labels:       append([]string{}, f.Labels...),
		Arc:          f.Arc,
		Pinned:       f.Pinned,
		BlockedBy:    append([]string{}, f.Blocked...),
		OpenBlockers: open,
		Sliced:       f.Sliced != nil,
		Decomposed:   decomposed(tree, f.Id),
		OpenedAt:     f.OpenedAt,
		Gaps:         append([]string{}, gaps...),
	}
	row.Gaps = append(row.Gaps, unknown...)
	if f.Budget != nil {
		budget := *f.Budget
		row.Budget = &budget
	}
	if approval := f.Approved; approval != nil {
		expired, why := f.ApprovalExpired(horizon)
		row.Approved = &Approval{
			By: approval.By, At: approval.At, Authority: approval.Authority,
			ReviewBy: approval.ReviewBy, Expired: expired, ExpiredWhy: why,
		}
	}
	if claim := f.Claimed; claim != nil {
		row.Claim = &Claim{Machine: claim.Machine, Lineage: claim.Lineage, At: claim.At}
		if f.Landing != nil {
			row.Claim.LandingAt = f.Landing.At
		}
	}
	if fence := f.StopFence; fence != nil {
		row.Fence = &Fence{Reason: fence.Reason, ClosedAt: fence.ClosedAt}
	}
	if parked := f.Parked; parked != nil {
		row.Waiting = &Waiting{
			Reason: parked.Because, Since: parked.At, By: parked.By,
			From: parkedFrom(f), Blocker: parked.Blocker,
		}
	} else if lane == LaneWaiting && f.State == goal.StateApproved {
		// An open dependency holds approved work. The state it waits from is
		// the state the record carries; admission is not judged for a goal
		// the frontier never reached.
		row.Waiting = &Waiting{From: goal.StateApproved}
	}
	if abandoned := f.Abandoned; abandoned != nil {
		row.Abandoned = &Abandoned{By: abandoned.By, At: abandoned.At, Because: abandoned.Because}
	}
	if last, verbIsThisGoal := lastVerbOn(f.History); last.At != "" {
		row.LastChangeAt = last.At
		if verbIsThisGoal {
			row.LastVerb = last.Verb
		}
	}
	return row
}

// rankFanOut is the reason all three of the engine's priority writers put on a
// line whose subject is a rank: "priority-order subject=… from=… to=…" when a
// re-rank fans out (order.go:146), and "priority-order from=… to=…" when a
// compaction is merged into an event (order.go:234, abandon.go:417).
const rankFanOut = "priority-order"

// lastVerbOn reports the goal's last history line, and whether that line's verb
// can be trusted to describe this goal.
//
// It cannot always. A priority compaction writes the operation's own verb into
// the history of every goal it re-ranks, so concluding one goal appends a
// `done` line to dozens of others; taking the verb at face value made 47 of
// this ledger's 155 live goals report "last done" while queued or parked. But
// the obvious repair — skip back past every priority-order line — is also
// wrong, because mergePriorityEvent folds the rank reason INTO the subject's
// own event when the opid matches (order.go:226 to :239). A goal reopened at a
// new rank gets one line, verb `reopen`, reason `priority-order from=… to=…`.
// Skipping it would walk back to the older `done` and report a just-reopened
// goal as finished: the same lie, inverted.
//
// The two cases are indistinguishable from the record. A bystander's line and
// a merged subject's line carry the same verb shape, the same targets and the
// same reason grammar; nothing stored says which goal the operation was about.
// So the row does not guess. When the reason is a rank clause the line's date
// is still exact — something did happen to this goal then — and only the verb
// is withheld. Goal detail shows the whole History, where a human can see both
// the verb and the reason and decide for themselves.
func lastVerbOn(history []goal.HistoryLine) (line goal.HistoryLine, verbIsThisGoal bool) {
	if len(history) == 0 {
		return goal.HistoryLine{}, false
	}
	last := history[len(history)-1]
	return last, !strings.HasPrefix(last.Reason, rankFanOut)
}

// openBlockers lists the dependencies that are not concluded, and names the
// ones that are not in the ledger at all. A blocker that is live, dropped, or
// missing is open: only a completed goal clears the way.
func openBlockers(f *goal.GoalFile, tree *goal.TreeGoals) (open, gaps []string) {
	open, gaps = []string{}, []string{}
	for _, id := range f.Blocked {
		if tree != nil && tree.Done[id] != nil {
			continue
		}
		open = append(open, id)
		if tree != nil && !tree.Exists(id) {
			gaps = append(gaps, "blocker "+id+" is an unknown goal")
		}
	}
	return open, gaps
}

// parkedFrom reports the state a park interrupted where the record proves it.
// A displaced claimant proves another pair held the goal; an accounting
// episode released at the park's own stamp proves the parking pair held it.
// Queued and approved leave no such trace, so the record does not say.
func parkedFrom(f *goal.GoalFile) string {
	if f.Parked == nil {
		return ""
	}
	if f.Parked.Displaced != "" {
		return goal.StateClaimed
	}
	if f.Episode != nil && f.Episode.Released != "" && f.Episode.Released == f.Parked.At {
		return goal.StateClaimed
	}
	return PhaseNotRecorded
}

func decomposed(tree *goal.TreeGoals, id string) bool {
	if tree == nil || tree.Root == nil {
		return false
	}
	for _, entry := range tree.Root.Decomposed {
		if entry.Id == id {
			return true
		}
	}
	return false
}
