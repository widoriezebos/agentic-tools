package lane

import (
	"errors"
	"fmt"
	"time"
)

// The lane's runner states as a person reads them: the landing agent runs;
// a person stopped the lane; no agent runs and none is needed until there is
// work, which is normal (design r10 §3: an idle lane runs no model); or the
// lane cannot run an agent, with why and the fix.
const (
	OwnerRunning = "running"
	OwnerStopped = "stopped"
	OwnerIdle    = "idle"
	OwnerHeld    = "held"
	OwnerUnready = "unready"
)

// View is the host's landing lane as every reader shows it: landing status
// prints it and the interface's /api/board carries it as "lane". Every key is
// always present; an absent value is null.
type View struct {
	Root         *string   `json:"root"`
	RegisteredBy *string   `json:"registered_by"`
	RegisteredAt *string   `json:"registered_at"`
	Owner        OwnerView `json:"owner"`
	// The plain lane's status reader supplies its recorded selection here.
	// A view of the host registration alone has no selection to report.
	Batch any       `json:"batch"`
	Next  *struct{} `json:"next"`
	// Wake is why the landing agent would run now (§3 Wake): the reasons the
	// keeper wakes it on, read the same way; null with no lane.
	Wake    *Wake  `json:"wake"`
	Summary string `json:"summary"`
	// Unreadable is why this computer's lane record could not be read,
	// which a reader tells from a lane that was never registered; absent
	// when the record was read or there is none.
	Unreadable string `json:"unreadable,omitempty"`
}

// OwnerView is what runs the lane: its landing agent (lane design r10 §3),
// started on demand by the keeper, or a person's stop. The page reads it as
// the lane's "owner".
type OwnerView struct {
	State     string  `json:"state"`
	Barren    int     `json:"barren,omitempty"`
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
	// Unread is what of the owner this view could not read, as LastExit
	// says it: whether the landing agent runs, or whether the lane can run,
	// when that check failed for a reason other than a refusal. The lane's
	// status carries it as a problem (plain.Status); empty when it was read.
	Unread string `json:"-"`
}

// OwnerProbe is whether the lane's landing agent runs, and since when.
type OwnerProbe struct {
	Alive bool
	PID   int64
	Since time.Time
}

// ViewSources are the reads a view is built from.
type ViewSources struct {
	Home string
	Now  time.Time
	// Owner reads the landing agent.
	Owner func(root string) (OwnerProbe, error)
	// Ready says whether the lane can run at root (a *Refusal naming the
	// fix when it cannot); asked only when no agent runs. nil asks nothing.
	Ready func(root string) error
	// Fingerprint reads the lane as the keeper's barren hold does.
	Fingerprint func(root string) (string, error)
	// Wake are the reads of the keeper's wake, shown as the view's wake.
	Wake WakeSources
}

// BuildView reads the lane once and says it for a person and a page.
func BuildView(sources ViewSources) View {
	view := View{Owner: OwnerView{State: OwnerUnready}}
	record, ok, err := Read(sources.Home)
	if err != nil {
		view.Summary = "this computer's landing lane record can't be read (" + err.Error() + "); metasystem landing set replaces it"
		view.Unreadable = err.Error()
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
	wake := ReadWake(record, sources.Wake)
	if _, err := ReadAgentState(sources.Home); err != nil {
		wake.Unread = append(wake.Unread, UnreadableAgentRecord(sources.Home))
	}
	view.Wake = &wake
	view.Summary = view.SummaryWithWaiting(len(wake.Reasons) > 0)
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
		owner.Unread = "whether the landing agent runs is unknown: " + err.Error()
		owner.LastExit = text(owner.Unread)
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
		state, err := ReadAgentState(sources.Home)
		if err != nil {
			owner.Unread = UnreadableAgentRecord(sources.Home)
			owner.LastExit = text(owner.Unread)
			return owner
		}
		if state.Barren >= barrenLimit && sources.Fingerprint != nil {
			fingerprint, err := sources.Fingerprint(root)
			if err != nil {
				owner.Unread = "whether the lane changed after its held runs is unknown: " + err.Error()
				owner.LastExit = text(owner.Unread)
				return owner
			}
			if fingerprint == state.BarrenFingerprint {
				owner.State, owner.Barren = OwnerHeld, state.Barren
				return owner
			}
		}
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
		owner.Unread = "whether the lane can run is unknown: " + err.Error()
		owner.LastExit = text(owner.Unread)
		return
	}
	owner.LastExit, owner.RetryHint, owner.Fix = text(refusal.Message), text(refusal.Fix), refusal.Argv
}

// SummaryWithWaiting says the lane's state with the waiting work read by the caller.
func (view View) SummaryWithWaiting(waiting bool) string {
	return "landing lane " + *view.Root + ": " + view.AgentSummary(waiting)
}

// AgentSummary is the agent's state as every lane reader says it.
func (view View) AgentSummary(waiting bool) string {
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
		if waiting {
			agent = fmt.Sprintf("idle; its agent starts within one tick (%d s)", int(AgentTick/time.Second))
		}
	case OwnerHeld:
		agent = fmt.Sprintf("held after %d runs that left the lane unchanged; a person's metasystem landing run starts it", view.Owner.Barren)
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
	return agent
}
