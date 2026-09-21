package backlog

import (
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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
	Where        string     `json:"where"`
	Lane         Lane       `json:"lane"`
	Phase        string     `json:"phase"`
	State        string     `json:"state"`
	Intent       string     `json:"intent"`
	NextStep     string     `json:"nextStep"`
	Concluded    string     `json:"concluded"`
	Origin       string     `json:"origin"`
	Priority     uint8      `json:"priority"`
	Sequence     uint64     `json:"sequence"`
	Tier         uint8      `json:"tier"`
	Labels       []string   `json:"labels"`
	Arc          string     `json:"arc"`
	Pinned       string     `json:"pinned"`
	BlockedBy    []string   `json:"blockedBy"`
	OpenBlockers []string   `json:"openBlockers"`
	Approved     *Approval  `json:"approved,omitempty"`
	Claim        *Claim     `json:"claim,omitempty"`
	Waiting      *Waiting   `json:"waiting,omitempty"`
	Abandoned    *Abandoned `json:"abandoned,omitempty"`
	Fence        *Fence     `json:"fence,omitempty"`
	Sliced       bool       `json:"sliced"`
	Decomposed   bool       `json:"decomposed"`
	OpenedAt     string     `json:"openedAt"`
	LastChangeAt string     `json:"lastChangeAt"`
	LastVerb     string     `json:"lastVerb"`
	Gaps         []string   `json:"gaps"`
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
	if last, ok := lastVerbOn(f.History); ok {
		row.LastChangeAt, row.LastVerb = last.At, last.Verb
	}
	return row
}

// rankFanOut is the reason the engine writes on a history line that records a
// rank change rather than something done to the goal. All three writers use
// it: two as "priority-order from=… to=…" (order.go:234, abandon.go:417) and
// one as "priority-order subject=… from=…" (order.go:146).
const rankFanOut = "priority-order"

// lastVerbOn is the last line that records something done to THIS goal.
//
// A priority compaction writes its own verb into the history of every goal it
// re-ranks, so concluding one goal appends a `done` line to dozens of others.
// Taking the last line whatever it is made 47 of this ledger's 155 live goals
// report "last done" while queued or parked — a row telling a human their work
// had finished when it had not started. A rank change is a real event and the
// record keeps it; it is simply not this goal's latest verb, and the detail
// page shows both classes separately.
//
// A goal whose every line is a fan-out has no verb of its own to report, and
// the row says nothing rather than borrowing another goal's.
func lastVerbOn(history []goal.HistoryLine) (goal.HistoryLine, bool) {
	for i := len(history) - 1; i >= 0; i-- {
		if !strings.HasPrefix(history[i].Reason, rankFanOut) {
			return history[i], true
		}
	}
	return goal.HistoryLine{}, false
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
