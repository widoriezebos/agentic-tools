package main

// The landing object (batch-lane design U12): the landing lane on this
// computer, where every seat's work is proved and pushed. landing status
// reads it; landing set registers or moves it; landing start, stop and
// restart act on its owner. Each renders the one lane.View /api/board carries.

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// The landing verbs' refusal codes (register rows name their sites).
const (
	codeLandingLaneBusy    = "LANDING_LANE_BUSY"
	codeLandingLanePushing = "LANDING_LANE_PUSHING"
)

// laneVerbOwners are the landing verbs' seams; the zero value is production.
type laneVerbOwners struct {
	home     func() (string, error)
	probe    func(root string) (lane.OwnerProbe, error)
	start    func(root string) error
	records  func(root string) ([]batch.Record, error)
	validate func(root, seatRoot string, now time.Time) (string, error)
	by       func(installation string) string
	now      func() time.Time
	// end ends the running owner for a restart, by its recorded identity.
	end func(root string) (int64, error)
	// person proves the person at the enrolled terminal of an installation
	// and names them.
	person func(root string) (string, error)
}

func (inv *intentInvocation) landing() laneVerbOwners {
	owners := inv.owners.landing
	if owners.home == nil {
		owners.home = batchowner.LandingLaneHome
	}
	if owners.probe == nil {
		owners.probe = batchowner.LandingLaneOwnerProbe
	}
	if owners.start == nil {
		owners.start = batchowner.EnsureBatchOwner
	}
	if owners.validate == nil {
		owners.validate = batchowner.ValidateLandingCheckout
	}
	if owners.by == nil {
		owners.by = batchowner.LandingLaneRegistrant
	}
	if owners.now == nil {
		owners.now = func() time.Time { return time.Now().UTC() }
	}
	if owners.end == nil {
		owners.end = batchowner.EndLaneOwner
	}
	if owners.person == nil {
		owners.person = provenPerson(humanauthority.KernelReader{}, func() int64 { return int64(os.Getppid()) }, time.Now)
	}
	return owners
}

func landingIntentCommands() []intentCommand {
	byFlag := intentFlag{name: "by", value: "NAME", usage: "who acts (default: this seat's nickname)"}
	return []intentCommand{
		{
			object: "landing", action: "status", audience: "both", summary: "the landing lane on this computer: its checkout, its owner, the batch it proves and the next",
			usage: []string{"metasystem landing status [--verbose]"},
			details: []string{"One line: where the lane is, whether its owner runs, the batch it works on and the one collecting behind it.",
				"--verbose adds who registered the lane and when, the owner's pid, restarts and last exit, each batch's members and what the batch waits for.",
				"--json prints the same view the interface's Fleet page reads."},
			flags:    []intentFlag{intentVerboseFlag},
			maxArgs:  0,
			examples: []string{"metasystem landing status", "metasystem landing status --verbose"},
			run:      runIntentLandingStatus,
		},
		{
			object: "landing", action: "set", audience: "both", summary: "register or move this computer's landing lane",
			usage: []string{"metasystem landing set PATH [--by NAME]"},
			details: []string{"PATH is a dedicated landing checkout; every seat of this computer then lands through it, and a seat whose landing.batch-root names another is refused.",
				"The same PATH again changes nothing. Moving the lane while a batch proves or pushes in it is refused with the way forward: wait, or pause it with landing stop first."},
			flags:    []intentFlag{byFlag},
			maxArgs:  1,
			examples: []string{"metasystem landing set /Users/wido/LocalStorage/GitHub/agentic-tools-landing"},
			run:      runIntentLandingSet,
		},
		{
			object: "landing", action: "start", audience: "both", summary: "start the landing lane's owner now and clear its restart count",
			usage: []string{"metasystem landing start"},
			details: []string{"Ends a pause, forgets the keep-alive's restarts and give-up, and starts the owner when it is not running.",
				"An owner already running, unpaused and without restarts changes nothing."},
			maxArgs:  0,
			examples: []string{"metasystem landing start"},
			run:      runIntentLandingStart,
		},
		{
			object: "landing", action: "stop", audience: "both", summary: "pause the landing lane's owner for maintenance until landing start",
			usage: []string{"metasystem landing stop [--by NAME]"},
			details: []string{"The owner advances no batch and the keep-alive does not restart it until metasystem landing start; status shows who stopped it and when.",
				"A lane already stopped changes nothing. While a batch is pushing to main, stop is refused with the way forward."},
			flags:    []intentFlag{byFlag},
			maxArgs:  0,
			examples: []string{"metasystem landing stop", "metasystem landing stop --by Wido"},
			run:      runIntentLandingStop,
		},
		{
			object: "landing", action: "restart", audience: "both", summary: "stop and start the landing lane's owner",
			usage: []string{"metasystem landing restart [--by NAME]"},
			details: []string{"Pauses the lane, ends the running owner by its recorded identity and starts a fresh one: the pause and the restart count are cleared.",
				"The relaunched owner runs the engine this seat's supervision is pinned to; metasystem system restart moves the seat to a new engine.",
				"Refused like landing stop while a batch is pushing."},
			flags:    []intentFlag{byFlag},
			maxArgs:  0,
			examples: []string{"metasystem landing restart"},
			run:      runIntentLandingRestart,
		},
	}
}

// laneContext is what every landing verb reads first: the home and the
// registered lane, or the result that ends the verb.
func (inv *intentInvocation) laneContext(needLane bool) (owners laneVerbOwners, home string, record lane.Record, problem *intentResult) {
	owners = inv.landing()
	home, err := owners.home()
	if err != nil {
		return owners, "", lane.Record{}, &intentResult{Outcome: intentFailed, code: 1, Summary: "the landing lane cannot be read: this computer has no home for it: " + err.Error()}
	}
	record, ok, err := lane.Read(home)
	if err != nil && !needLane {
		// An unreadable record is status's to report and landing set's to
		// replace (Register replaces it).
		return owners, home, lane.Record{}, nil
	}
	if err != nil {
		return owners, home, record, &intentResult{Outcome: intentFailed, code: 1, Summary: err.Error(),
			next: inv.publicArgv("landing", "set", "PATH"), nextReason: "a person registers the lane again, which replaces the unreadable record"}
	}
	if needLane && !ok {
		return owners, home, record, &intentResult{Outcome: intentRefused, code: 1,
			Summary: "no landing lane is registered on this computer; nothing was done",
			next:    inv.publicArgv("landing", "set", "PATH"), nextReason: "register the landing checkout every seat lands through"}
	}
	return owners, home, record, nil
}

func (inv *intentInvocation) laneView(owners laneVerbOwners, home string) lane.View {
	sources := lane.ViewSources{Home: home, Now: owners.now(), Owner: owners.probe, Records: owners.records}
	if inv.input.switched("verbose") {
		// The lane's spend is a full read of its proof store: only --verbose
		// pays for it (N-5).
		sources.Spend = batchowner.LaneSpend
	}
	return lane.BuildView(sources)
}

func laneTargets(root string) []intentTarget {
	return []intentTarget{{Kind: "landing-lane", ID: root}}
}

func runIntentLandingStatus(inv *intentInvocation) int {
	owners, home, _, problem := inv.laneContext(false)
	if problem != nil {
		return inv.render(*problem)
	}
	view := inv.laneView(owners, home)
	result := intentResult{Outcome: intentConfirmed, Summary: view.Summary, Data: view}
	if view.Root != nil {
		result.Targets = laneTargets(*view.Root)
	}
	if inv.input.switched("verbose") {
		result.text = landingViewDetail(view)
	}
	if view.Owner.RetryHint != nil && view.Owner.State == lane.OwnerGivenUp {
		result.next, result.nextReason = inv.publicArgv("landing", "start"), "the keep-alive gave up; "+*view.Owner.RetryHint
	}
	return inv.render(result)
}

// landingViewDetail is --verbose: one line per fact of the view.
func landingViewDetail(view lane.View) []string {
	value := func(text *string) string {
		if text == nil {
			return "none"
		}
		return *text
	}
	local := func(stamp *string) string {
		if stamp == nil {
			return "none"
		}
		return lane.LocalText(*stamp)
	}
	lines := []string{"registered by " + value(view.RegisteredBy) + " at " + local(view.RegisteredAt)}
	owner := "owner: " + view.Owner.State
	if view.Owner.PID != nil {
		owner += fmt.Sprintf(", pid %d", *view.Owner.PID)
	}
	owner += fmt.Sprintf(", since %s, %d restarts", local(view.Owner.Since), view.Owner.Restarts)
	lines = append(lines, owner)
	if view.Owner.LastExit != nil {
		lines = append(lines, "last exit: "+*view.Owner.LastExit)
	}
	if view.Owner.StoppedBy != nil {
		lines = append(lines, "stopped by "+*view.Owner.StoppedBy)
	}
	if view.Owner.RetryHint != nil {
		lines = append(lines, "to retry: "+*view.Owner.RetryHint)
	}
	if view.Batch != nil {
		lines = append(lines, fmt.Sprintf("batch %s %s since %s: %s", view.Batch.ID, view.Batch.State, local(&view.Batch.Since), view.Batch.Reason))
		for _, member := range view.Batch.Members {
			lines = append(lines, "  member "+member.Goal+" from "+member.Seat)
		}
		for _, waited := range view.Batch.WaitingFor {
			lines = append(lines, "  waits for "+waited.Goal+" on "+waited.Seat+", expected "+local(waited.Expected))
		}
		for _, change := range view.Batch.Returned {
			lines = append(lines, "  "+change.Outcome+" "+change.Goal+" from "+change.Seat+": "+change.Reason)
		}
	}
	if view.Spend != nil {
		lines = append(lines, fmt.Sprintf("lane spend (%s, batches of changes; no goal's budget): %d attempts, %d reserved minutes",
			view.Spend.Account, view.Spend.Attempts, view.Spend.ReservedMinutes))
	}
	if view.Next != nil {
		lines = append(lines, "next batch "+view.Next.ID+" collecting:")
		for _, member := range view.Next.Members {
			lines = append(lines, "  member "+member.Goal+" from "+member.Seat)
		}
	}
	return lines
}

func (inv *intentInvocation) landingActor(owners laneVerbOwners) string {
	if by := strings.TrimSpace(inv.input.text("by")); by != "" {
		return by
	}
	if inv.resolveLayout() == nil {
		return owners.by(inv.layout.InstallationRoot)
	}
	return "a person at " + inv.cwd
}

// laneBusy names a batch of the lane that proves or pushes, for a move or a stop.
func laneBusy(view lane.View, pushingOnly bool) string {
	if view.Batch == nil || view.Owner.State == lane.OwnerStopped {
		return ""
	}
	if view.Batch.State == lane.BatchPushing || !pushingOnly && view.Batch.State == lane.BatchProving {
		return view.Batch.ID + " " + view.Batch.State
	}
	return ""
}

func runIntentLandingSet(inv *intentInvocation) int {
	if len(inv.input.args) != 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "landing set needs the landing checkout: metasystem landing set PATH; nothing was done"})
	}
	owners, home, record, problem := inv.laneContext(false)
	if problem != nil {
		return inv.render(*problem)
	}
	path := inv.callerPath(inv.input.args[0])
	seat := ""
	if inv.resolveLayout() == nil {
		seat = inv.layout.InstallationRoot
	}
	root, err := owners.validate(path, seat, owners.now())
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: laneTargets(path),
			Summary:  fmt.Sprintf("%s: %s is not a landing checkout: %v; nothing was registered", lane.CodeRegisterInvalid, path, err),
			Decision: "name a dedicated landing checkout (a clone of this repository that no seat works in)"})
	}
	actor := ""
	if record.Root != "" && record.Root != root {
		if busy := laneBusy(inv.laneView(owners, home), false); busy != "" {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: laneTargets(record.Root),
				Summary:  fmt.Sprintf("%s: batch %s in the current lane %s; moving the lane now would leave that batch without its owner, so nothing was changed", codeLandingLaneBusy, busy, record.Root),
				Decision: "wait until it lands (metasystem landing status shows it), or pause the lane first with metasystem landing stop, then run metasystem landing set " + root + " again"})
		}
		// Moving the host's lane changes where every seat lands: a
		// person's act at an enrolled terminal. The first registration and
		// a repeat stay open to anyone.
		retry := "metasystem landing set " + root
		proveAt := seat
		if proveAt == "" {
			proveAt = inv.cwd
		}
		person, err := owners.person(proveAt)
		if err != nil {
			return inv.render(intentResult{Outcome: intentRefused, code: 3, Targets: laneTargets(record.Root),
				Summary:  "moving this computer's landing lane from " + record.Root + " to " + root + " is a person's act, and this shell was not proven to be one: " + humanauthority.PlainReason(err) + "; nothing was changed",
				Decision: humanauthority.PersonActRemedy(retry)})
		}
		actor = person
	}
	if by := strings.TrimSpace(inv.input.text("by")); by != "" || actor == "" {
		actor = inv.landingActor(owners)
	}
	previous, changed, err := lane.Register(home, root, actor, owners.now())
	var refusal *lane.Refusal
	if errors.As(err, &refusal) {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: laneTargets(path), Summary: refusal.Error(), Decision: refusal.Fix})
	}
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: laneTargets(root), Summary: "the landing lane could not be registered: " + err.Error()})
	}
	view := inv.laneView(owners, home)
	if !changed {
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: laneTargets(root), Data: view, Summary: "this computer's landing lane is already " + root})
	}
	summary := "this computer's landing lane is now " + root
	if previous.Root != "" {
		summary += " (it was " + previous.Root + ")"
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: laneTargets(root), Data: view, Summary: summary,
		next: inv.publicArgv("landing", "start"), nextReason: "start its owner now; otherwise the next landing or the steward starts it"})
}

func runIntentLandingStart(inv *intentInvocation) int {
	owners, home, record, problem := inv.laneContext(true)
	if problem != nil {
		return inv.render(*problem)
	}
	return inv.render(inv.startLane(owners, home, record, false))
}

// startLane ends a pause, forgets the keep-alive's restarts and starts an
// owner that is not running; unchanged when all three already hold.
func (inv *intentInvocation) startLane(owners laneVerbOwners, home string, record lane.Record, restarted bool) intentResult {
	targets := laneTargets(record.Root)
	resumed, err := lane.ClearPause(home)
	kept := lane.ReadKeeper(home) != (lane.KeeperState{})
	if err == nil && kept {
		err = lane.ResetKeeper(home)
	}
	if err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the landing lane's state could not be written: " + err.Error()}
	}
	probe, err := owners.probe(record.Root)
	if err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "whether the landing lane's owner runs is unknown: " + err.Error() + "; nothing was started",
			next: inv.publicArgv("landing", "status", "--verbose"), nextReason: "read the owner's state"}
	}
	started := false
	if !probe.Alive {
		if err := owners.start(record.Root); err != nil {
			return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the landing lane's owner at " + record.Root + " did not start: " + err.Error(),
				next: inv.publicArgv("landing", "status", "--verbose"), nextReason: "read its last exit"}
		}
		started = true
	}
	view := inv.laneView(owners, home)
	switch {
	case started && view.Owner.PID != nil:
		return intentResult{Outcome: intentConfirmed, Targets: targets, Data: view, Summary: fmt.Sprintf("started the landing lane's owner at %s (pid %d)", record.Root, *view.Owner.PID)}
	case started:
		return intentResult{Outcome: intentConfirmed, Targets: targets, Data: view, Summary: "started the landing lane's owner at " + record.Root + "; it is still coming up (metasystem landing status shows it)"}
	case resumed || kept || restarted:
		return intentResult{Outcome: intentConfirmed, Targets: targets, Data: view, Summary: fmt.Sprintf("the landing lane's owner at %s runs again (pid %d); its restart count is cleared", record.Root, probe.PID)}
	}
	return intentResult{Outcome: intentUnchanged, Targets: targets, Data: view, Summary: fmt.Sprintf("the landing lane's owner at %s is already running (pid %d)", record.Root, probe.PID)}
}

func runIntentLandingStop(inv *intentInvocation) int {
	owners, home, record, problem := inv.laneContext(true)
	if problem != nil {
		return inv.render(*problem)
	}
	result, _ := inv.stopLane(owners, home, record)
	return inv.render(result)
}

// stopLane pauses the lane at a person's word; a batch pushing to main is
// never paused mid-push.
func (inv *intentInvocation) stopLane(owners laneVerbOwners, home string, record lane.Record) (intentResult, bool) {
	targets := laneTargets(record.Root)
	if pause, paused := lane.ReadPause(home); paused {
		return intentResult{Outcome: intentUnchanged, Targets: targets, Summary: "the landing lane is already stopped by " + pause.By + " at " + lane.LocalText(pause.At) + "; metasystem landing start resumes it"}, true
	}
	if busy := laneBusy(inv.laneView(owners, home), true); busy != "" {
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets,
			Summary:  fmt.Sprintf("%s: batch %s to main now; stopping mid-push would leave main and the batch's record out of step, so nothing was stopped", codeLandingLanePushing, busy),
			Decision: "wait for it to land (metasystem landing status shows it), then run metasystem landing stop again"}, false
	}
	by := inv.landingActor(owners)
	if _, err := lane.SetPause(home, by, owners.now()); err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the landing lane could not be stopped: " + err.Error()}, false
	}
	return intentResult{Outcome: intentConfirmed, Targets: targets, Data: inv.laneView(owners, home),
		Summary: "stopped the landing lane at " + record.Root + " for " + by + ": its owner advances no batch and is not restarted until metasystem landing start"}, true
}

// runIntentLandingRestart gives the lane a fresh owner process (a person
// restarts for a new engine): pause, end the running owner by its recorded
// identity, then start: the pause and the restart count are cleared and a
// new owner runs.
func runIntentLandingRestart(inv *intentInvocation) int {
	owners, home, record, problem := inv.laneContext(true)
	if problem != nil {
		return inv.render(*problem)
	}
	if stopped, ok := inv.stopLane(owners, home, record); !ok {
		return inv.render(stopped)
	}
	ended, err := owners.end(record.Root)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: laneTargets(record.Root),
			Summary: err.Error() + "; the lane stays paused",
			next:    inv.publicArgv("landing", "start"), nextReason: "resume the lane with the owner it has"})
	}
	result := inv.startLane(owners, home, record, true)
	if result.Outcome == intentConfirmed || result.Outcome == intentUnchanged {
		result.Outcome = intentConfirmed
		if ended != 0 {
			result.Summary = fmt.Sprintf("ended the landing lane's owner pid %d; ", ended) + result.Summary
		}
	}
	return inv.render(result)
}

// statusLaneLine is the lane's line in status's board block: landing
// status's headline, when this computer has a lane.
func (inv *intentInvocation) statusLaneLine() string {
	owners := inv.landing()
	home, err := owners.home()
	if err != nil {
		return ""
	}
	if view := inv.laneView(owners, home); view.Root != nil {
		return view.Summary
	}
	return ""
}
