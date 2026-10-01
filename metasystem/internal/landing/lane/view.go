package lane

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

// The lane's runner states as a person reads them: the landing agent runs;
// a person stopped the lane; no agent runs and none is needed until there is
// work, which is normal (design r10 §3: an idle lane runs no model); or the
// lane cannot run an agent, with why and the fix.
const (
	OwnerRunning = "running"
	OwnerStopped = "stopped"
	OwnerIdle    = "idle"
	OwnerUnready = "unready"
)

// The current batch's states as a person reads them.
const (
	BatchCollecting = "collecting"
	BatchWaiting    = "waiting"
	BatchProving    = "proving"
	BatchPushing    = "pushing"
	// BatchHeld is a batch held whole because a seat that joined it is at
	// the helm; it starts when that seat returns the helm.
	BatchHeld = "held"
)

// View is the host's landing lane as every reader shows it: landing status
// prints it and the interface's /api/board carries it as "lane". Every key is
// always present; an absent value is null.
type View struct {
	Root         *string    `json:"root"`
	RegisteredBy *string    `json:"registered_by"`
	RegisteredAt *string    `json:"registered_at"`
	Owner        OwnerView  `json:"owner"`
	Batch        *BatchView `json:"batch"`
	Next         *NextView  `json:"next"`
	// Spend is what the lane charged to its own account: the proofs of
	// batches whose members are all changes (U11b).
	Spend *Spend `json:"spend"`
	// Wake is why the landing agent would run now (§3 Wake): the reasons the
	// keeper wakes it on, read the same way; null with no lane.
	Wake    *Wake  `json:"wake"`
	Summary string `json:"summary"`
}

// Spend is the lane's own proof spend: its accounting identity, the attempts
// charged to it and their reserved minutes. No goal's budget holds them.
type Spend struct {
	Account         string `json:"account"`
	Attempts        int    `json:"attempts"`
	ReservedMinutes uint64 `json:"reserved_minutes"`
}

// OwnerView is what runs the lane: its landing agent (lane design r10 §3),
// started on demand by the keeper, or a person's stop. The page reads it as
// the lane's "owner".
type OwnerView struct {
	State     string  `json:"state"`
	PID       *int64  `json:"pid"`
	Since     *string `json:"since"`
	LastExit  *string `json:"last_exit"`
	StoppedBy *string `json:"stopped_by"`
	// StoppedBecause is the reason the stop was given with; absent when
	// none was.
	StoppedBecause *string `json:"stopped_because,omitempty"`
	RetryHint      *string `json:"retry_hint"`
	// Fix is RetryHint as one command a person runs, when it is one; the
	// verbs print it as their next step, the page shows RetryHint.
	Fix []string `json:"-"`
}

// Member is one goal in a batch and the seat it came from.
type Member struct {
	Goal string `json:"goal"`
	Seat string `json:"seat"`
}

// Waiting is a goal the current batch waits for, and when it is expected.
type Waiting struct {
	Goal     string  `json:"goal"`
	Seat     string  `json:"seat"`
	Expected *string `json:"expected"`
}

// BatchView is the batch the lane works on now.
type BatchView struct {
	ID         string    `json:"id"`
	State      string    `json:"state"`
	Members    []Member  `json:"members"`
	WaitingFor []Waiting `json:"waiting_for"`
	// Returned are the changes that left the batch, with why: a change holds
	// no goal whose Next could carry it, so its asker reads it here (U11b).
	Returned []Returned `json:"returned"`
	Since    string     `json:"since"`
	Reason   string     `json:"reason"`
}

// Returned is one change that left a batch: its id, the asker's seat, the
// outcome and the reason.
type Returned struct {
	Goal    string `json:"goal"`
	Seat    string `json:"seat"`
	Outcome string `json:"outcome"`
	Reason  string `json:"reason"`
}

// NextView is the batch collecting behind the current one.
type NextView struct {
	ID      string   `json:"id"`
	Members []Member `json:"members"`
}

// OwnerProbe is whether the lane's landing agent runs, and since when.
type OwnerProbe struct {
	Alive bool
	PID   int64
	Since time.Time
}

// ViewSources are the reads a view is built from. Records nil reads the
// lane's batch records from its checkout.
type ViewSources struct {
	Home string
	Now  time.Time
	// Owner reads the landing agent.
	Owner   func(root string) (OwnerProbe, error)
	Records func(root string) ([]batch.Record, error)
	// Spend reads what the lane at root charged to account; nil shows none.
	Spend func(root, account string) (Spend, error)
	// Ready says whether the lane can run at root (a *Refusal naming the
	// fix when it cannot); asked only when no agent runs. nil asks nothing.
	Ready func(root string) error
	// Helm reads whether a unit's seat is at the helm, which holds a batch
	// whole (batch.HelmHeldSeat); nil asks nothing.
	Helm func(seatRoot string) helm.State
}

// BuildView reads the lane once and says it for a person and a page.
func BuildView(sources ViewSources) View {
	view := View{Owner: OwnerView{State: OwnerUnready}}
	record, ok, err := Read(sources.Home)
	if err != nil {
		view.Summary = "this computer's landing lane record can't be read (" + err.Error() + "); metasystem landing set replaces it"
		return view
	}
	if !ok {
		view.Summary = "no landing lane is registered on this computer; metasystem landing set registers one"
		return view
	}
	view.Root, view.RegisteredBy, view.RegisteredAt = text(record.Root), text(record.RegisteredBy), text(record.At)
	if gone(record.Root) {
		refusal := goneRefusal(record)
		view.Summary = refusal.Message + "; " + refusal.Fix
		view.Owner.RetryHint = text(refusal.Fix)
		return view
	}
	view.Owner = ownerView(sources, record.Root)
	if sources.Spend != nil {
		if spend, err := sources.Spend(record.Root, AccountID(record.Root)); err == nil {
			view.Spend = &spend
		}
	}
	records, recordsErr := readRecords(sources, record.Root)
	wake := wakeOf(records, recordsErr)
	if _, err := ReadAgentState(sources.Home); err != nil {
		wake.Unread = append(wake.Unread, UnreadableAgentRecord(sources.Home))
	}
	view.Wake = &wake
	view.Batch, view.Next = currentBatches(records, sources.Helm)
	view.Summary = summary(record.Root, view, recordsErr)
	return view
}

func text(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func ownerView(sources ViewSources, root string) OwnerView {
	owner := OwnerView{State: OwnerUnready}
	if pause, paused := ReadPause(sources.Home); paused {
		owner.State, owner.StoppedBy, owner.Since = OwnerStopped, text(pause.By), text(pause.At)
		if pause.Reason != "" {
			owner.StoppedBecause = text(pause.Reason)
		}
		owner.RetryHint = text("metasystem landing start resumes it")
		return owner
	}
	probe, err := sources.Owner(root)
	switch {
	case err != nil:
		owner.LastExit = text("whether the landing agent runs is unknown: " + err.Error())
		return owner
	case probe.Alive:
		owner.State = OwnerRunning
		if probe.PID != 0 {
			pid := probe.PID
			owner.PID = &pid
		}
		if !probe.Since.IsZero() {
			owner.Since = text(probe.Since.UTC().Format(time.RFC3339))
		}
		return owner
	}
	if sources.Ready != nil {
		notReady(&owner, sources.Ready(root))
	}
	if owner.LastExit == nil && owner.RetryHint == nil {
		owner.State = OwnerIdle
	}
	return owner
}

// notReady says why the lane cannot run and what a person runs.
func notReady(owner *OwnerView, err error) {
	if err == nil {
		return
	}
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		owner.LastExit = text("whether the lane can run is unknown: " + err.Error())
		return
	}
	owner.LastExit, owner.RetryHint, owner.Fix = text(refusal.Message), text(refusal.Fix), refusal.Argv
}

func readRecords(sources ViewSources, root string) ([]batch.Record, error) {
	if sources.Records != nil {
		return sources.Records(root)
	}
	paths, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "landing-batches", "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	store := batch.NewStore(root, nil)
	var records []batch.Record
	unreadable := 0
	for _, path := range paths {
		record, loadErr := store.Load(strings.TrimSuffix(filepath.Base(path), ".json"))
		if loadErr != nil {
			// Fail closed means visible: the rest is shown and the record the
			// lane cannot read is counted in the summary.
			unreadable++
			continue
		}
		records = append(records, record)
	}
	if unreadable != 0 {
		return records, &unreadableRecords{count: unreadable}
	}
	return records, nil
}

// unreadableRecords counts the batch records the lane could not read.
type unreadableRecords struct{ count int }

func (err *unreadableRecords) Error() string {
	return fmt.Sprintf("%d batch record%s unreadable", err.count, plural(err.count))
}

// currentBatches picks the batch the lane works on (pushing, then proving,
// then the oldest waiting or collecting one) and the next one collecting.
func currentBatches(records []batch.Record, helmOf func(string) helm.State) (*BatchView, *NextView) {
	var active, queued []batch.Record
	for _, record := range records {
		switch record.State {
		case batch.StateLanding, batch.StateProving, batch.StateDiagnosing:
			active = append(active, record)
		case batch.StateOpen, batch.StateSealed:
			queued = append(queued, record)
		}
	}
	sort.SliceStable(active, func(i, j int) bool {
		return active[i].State == batch.StateLanding && active[j].State != batch.StateLanding
	})
	sort.SliceStable(queued, func(i, j int) bool { return firstAt(queued[i]) < firstAt(queued[j]) })
	var current *BatchView
	switch {
	case len(active) > 0:
		current = batchView(active[0], helmOf)
	case len(queued) > 0:
		current, queued = batchView(queued[0], helmOf), queued[1:]
	}
	if len(queued) == 0 {
		return current, nil
	}
	return current, &NextView{ID: queued[0].BatchID, Members: members(queued[0])}
}

func firstAt(record batch.Record) string {
	if len(record.History) == 0 {
		return ""
	}
	return record.History[0].At
}

func members(record batch.Record) []Member {
	list := []Member{}
	for _, unit := range record.Units {
		if unit.State == batch.UnitJoined || unit.State == batch.UnitJoining {
			list = append(list, Member{Goal: unit.GoalID, Seat: unit.Claim.Machine})
		}
	}
	return list
}

// returned lists the changes that left the batch, with their outcome and
// reason (U11b); a goal's return is on the goal's own Next.
func returned(record batch.Record) []Returned {
	list := []Returned{}
	for _, unit := range record.Units {
		if !unit.IsChange() || unit.State == batch.UnitJoined || unit.State == batch.UnitJoining || unit.State == batch.UnitLanded {
			continue
		}
		outcome := unit.Outcome
		if outcome == "" {
			outcome = unit.State
		}
		list = append(list, Returned{Goal: unit.GoalID, Seat: unit.Claim.Machine, Outcome: outcome, Reason: unit.Failure})
	}
	return list
}

func batchView(record batch.Record, helmOf func(string) helm.State) *BatchView {
	view := &BatchView{ID: record.BatchID, Members: members(record), WaitingFor: []Waiting{}, Returned: returned(record), Since: stateSince(record)}
	switch record.State {
	case batch.StateLanding:
		view.State, view.Reason = BatchPushing, "the batch proved green and is being pushed to main"
	case batch.StateProving, batch.StateDiagnosing:
		view.State, view.Reason = BatchProving, "proving: "+record.StartReason
		if record.State == batch.StateDiagnosing {
			view.Reason = "a red is being diagnosed so the rest can land"
		}
	default:
		view.State, view.Reason = BatchCollecting, "collecting joins; it starts when the landing agent begins it"
		if record.Wait != nil {
			view.State, view.Reason = BatchWaiting, strings.TrimPrefix(batch.WaitLine(record, record.Wait.Since, time.Local), "batch "+record.BatchID+" ")
			if !record.Wait.Since.IsZero() {
				view.Since = record.Wait.Since.UTC().Format(time.RFC3339)
			}
			for _, waited := range record.Wait.For {
				expected := (*string)(nil)
				if !waited.ExpectedAt.IsZero() {
					expected = text(waited.ExpectedAt.UTC().Format(time.RFC3339))
				}
				view.WaitingFor = append(view.WaitingFor, Waiting{Goal: waited.Goal, Seat: waited.Seat, Expected: expected})
			}
		}
		if wait, waiting := batch.ProvingWait(record); waiting {
			view.State, view.Reason = BatchWaiting, wait.Detail
		}
	}
	if reason := batch.HoldReason(record); reason != "" {
		view.State, view.Reason = BatchWaiting, "holds: "+reason
	}
	if reason, held := helmHold(record, helmOf); held {
		view.State, view.Reason = BatchHeld, reason
	}
	return view
}

// helmHold says a batch held for a seat at the helm (batch.HelmHeldSeat),
// and the act that releases it.
func helmHold(record batch.Record, helmOf func(string) helm.State) (string, bool) {
	if helmOf == nil {
		return "", false
	}
	states := map[string]helm.State{}
	seat, held := batch.HelmHeldSeat(record, func(root string) bool {
		states[root] = helmOf(root)
		return states[root].Active
	})
	if !held {
		return "", false
	}
	machine := seat
	for _, unit := range record.Units {
		if unit.SeatRoot == seat && unit.Claim.Machine != "" {
			machine = unit.Claim.Machine
			break
		}
	}
	state := states[seat]
	by := state.By
	if by == "" {
		by = "unknown"
	}
	return fmt.Sprintf("seat %s (%s) is at the helm (%s); it starts when that seat runs: metasystem helm return", machine, seat, by), true
}

// stateSince is when the batch entered its state: the last history entry
// that changed it, else its first entry.
func stateSince(record batch.Record) string {
	for index := len(record.History) - 1; index >= 0; index-- {
		if entry := record.History[index]; entry.From != entry.To && entry.To == record.State {
			return entry.At
		}
	}
	return firstAt(record)
}

func summary(root string, view View, recordsErr error) string {
	agent := "landing agent " + view.Owner.State
	switch view.Owner.State {
	case OwnerRunning:
		if view.Owner.PID != nil {
			agent += fmt.Sprintf(" (pid %d)", *view.Owner.PID)
		}
	case OwnerStopped:
		agent = "stopped by " + *view.Owner.StoppedBy
		if view.Owner.StoppedBecause != nil {
			agent += " (" + *view.Owner.StoppedBecause + ")"
		}
		agent += "; metasystem landing start resumes it"
	case OwnerIdle:
		agent = "idle; its landing agent starts when there is work"
	default:
		// Why the lane cannot run and the one fix, in the one line (summary
		// by default).
		agent = "the landing agent can't run"
		if view.Owner.LastExit != nil {
			agent = *view.Owner.LastExit
		}
		if view.Owner.RetryHint != nil {
			agent += "; to fix: " + *view.Owner.RetryHint
		}
	}
	line := "landing lane " + root + ": " + agent
	var unreadable *unreadableRecords
	partial := errors.As(recordsErr, &unreadable)
	switch {
	case recordsErr != nil && !partial:
		line += "; its batches are unreadable: " + recordsErr.Error()
	case view.Batch == nil:
		line += "; no batch"
	default:
		line += fmt.Sprintf("; batch %s %s, %d member%s", view.Batch.ID, view.Batch.State, len(view.Batch.Members), plural(len(view.Batch.Members)))
		if view.Batch.State == BatchHeld {
			// The one act that releases it belongs in the one line.
			line += " (" + view.Batch.Reason + ")"
		}
	}
	if view.Next != nil {
		line += fmt.Sprintf("; next %s collecting, %d member%s", view.Next.ID, len(view.Next.Members), plural(len(view.Next.Members)))
	}
	if partial {
		line += "; " + unreadable.Error()
	}
	return line
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
