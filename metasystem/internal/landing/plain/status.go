package plain

import (
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// Status is the plain lane as landing status --json carries it and as
// /api/board carries it under "lane" (goal fleet-card-can-land-now): one
// reader, so the card reads what the terminal reads. It is the lane's view,
// whether the lane is paused and its agent alive, the queue, the running
// proof, the last proof and the last push. An absent value is null.
type Status struct {
	lane.View
	PendingActions []PolicyRequest `json:"pending-actions,omitempty"`
	BatchPolicy    *PolicyValue    `json:"batch-policy,omitempty"`
	AdmittedBatch  string          `json:"admitted-batch,omitempty"`
	Paused         bool            `json:"paused"`
	AgentAlive     bool            `json:"agent_alive"`
	// Queue is every hand-in of queue.jsonl, oldest first, with its state:
	// waiting, returned, superseded by a newer hand-in of its goal, or
	// landed when origin's main (as the lane checkout last fetched it)
	// contains its sha. A landed hand-in a push of the last day brought
	// carries that push's time.
	Queue               []Entry              `json:"queue"`
	RunningProof        *RunningProof        `json:"running_proof"`
	RunningRegeneration *RunningRegeneration `json:"running_regeneration,omitempty"`
	// LastProof is the newest line of results.jsonl.
	LastProof *Result `json:"last_proof"`
	LastGate  *Result `json:"last_gate,omitempty"`
	// LastPush is the newest push landing push made.
	LastPush *Pushed `json:"last_push"`
	Stop     *Stop   `json:"stop,omitempty"`
	// Problems are the lane's records this read could not read, one plain
	// sentence each: a queue, proof or push that could not be read, or a
	// line of one that does not decode, is said here rather than read as
	// none, and so is what the view could not read (its pause, its agent,
	// its wake). Empty when everything was read.
	Problems []string `json:"problems"`
}

// RunningProof is the lane's proof recorded running.
type RunningProof struct {
	Admission    *ExecutionAdmission `json:"admission,omitempty"`
	Person       *ActProvenance      `json:"person,omitempty"`
	Trunk        bool                `json:"trunk,omitempty"`
	BatchID      string              `json:"batch-id,omitempty"`
	BatchMembers []GoalSHA           `json:"batch-members,omitempty"`
	Gate         bool                `json:"gate,omitempty"`
	Tree         string              `json:"tree"`
	Commit       string              `json:"commit,omitempty"`
	Since        string              `json:"since"`
	// Attempt and Log name the run; State is running while its process
	// runs, died when it ended without a result (the next prove runs it
	// again).
	Attempt string `json:"attempt"`
	Log     string `json:"log,omitempty"`
	State   string `json:"state"`
	// Goals are the waiting hand-ins the proof's commit holds, oldest
	// first, by the containment that derives landed; none for a proof that
	// died or whose commit is not recorded.
	Goals []string `json:"goals,omitempty"`
}

// landedWindow is how far back a landing is timed. The page lists what
// landed today, and pushes.jsonl grows by a line with every landing, so a
// read asks only the pushes of the last day what they brought; an older
// landing carries no time.
const landedWindow = 24 * time.Hour

// laneGit is what a status read asks the lane checkout's Git: origin's main
// as the checkout last fetched it, whether that main contains a commit, and
// which commits one push brought to main.
type laneGit struct {
	main     func() (string, error)
	contains func(main, sha string) (bool, error)
	brought  func(old, commit string) ([]string, error)
}

// checkoutGit answers laneGit from the lane checkout at dir.
func checkoutGit(dir string, seams ProveSeams) laneGit {
	return laneGit{
		main: func() (string, error) {
			return seams.git(dir, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/main^{commit}")
		},
		contains: func(main, sha string) (bool, error) {
			if seams.Git == nil {
				return ContainedIn(dir, main)(sha)
			}
			for _, commit := range []string{main, sha} {
				if _, err := seams.git(dir, "cat-file", "-e", commit+"^{commit}"); err != nil {
					return false, nil
				}
			}
			_, err := seams.git(dir, "merge-base", "--is-ancestor", sha, main)
			var exit *exec.ExitError
			if errors.As(err, &exit) && exit.ExitCode() == 1 {
				return false, nil
			}
			return err == nil, err
		},
		brought: func(old, commit string) ([]string, error) {
			listed, err := seams.git(dir, "rev-list", old+".."+commit)
			if err != nil || listed == "" {
				return nil, err
			}
			return strings.Fields(listed), nil
		},
	}
}

// ReadStatus reads the plain lane around its view: home is the lane's home
// (its pause), record the registered lane, seams the proof's (its process
// check and its clock).
func ReadStatus(home string, record lane.Record, view lane.View, seams ProveSeams) Status {
	layout, _ := record.Layout()
	return readStatus(home, record, view, seams, checkoutGit(string(layout.Checkout), seams))
}

func readStatus(home string, record lane.Record, view lane.View, seams ProveSeams, git laneGit) Status {
	pause, paused := lane.ReadPause(home)
	status := Status{AdmittedBatch: PersonBatchContinuation(record.Install, record, home), View: view, Paused: paused, AgentAlive: view.Owner.State == lane.OwnerRunning, Queue: []Entry{}, Problems: []string{}}
	if view.Root == nil {
		// A registration that can't be read is a lane this read could not
		// read, not a lane that is not there.
		if view.Unreadable != "" {
			status.Problems = append(status.Problems, "the lane's registration can't be read: "+view.Unreadable)
		}
		return status
	}
	// Every read failure this status could know of is a problem, one plain
	// line each (fix round 4). First what the lane's own records and the
	// view could not read: the pause (which then reads as stopped), whether
	// the agent runs or the lane can run, and the wake's unread sources, an
	// unreadable keeper record among them; never the same line twice.
	said := func(line string) {
		line = strings.Join(strings.Fields(strings.ReplaceAll(line, "\n", "; ")), " ")
		if line == "" || slices.Contains(status.Problems, line) {
			return
		}
		status.Problems = append(status.Problems, line)
	}
	if pause.Unreadable() {
		said("the lane's pause record can't be read, so the lane reads as stopped")
	}
	said(view.Owner.Unread)
	if view.Wake != nil {
		for _, line := range view.Wake.Unread {
			said(line)
		}
	}
	layout, err := record.Layout()
	if err != nil {
		status.Problems = append(status.Problems, "the lane's record can't be placed: "+err.Error())
		return status
	}
	install := string(layout.Install)
	unread := func(what string, err error) {
		if err != nil {
			status.Problems = append(status.Problems, what+" can't be read: "+err.Error())
		}
	}
	selected, err := ReadBatch(install)
	if selected != nil {
		status.Batch = selected
	}
	unread("the batch selection", err)
	status.PendingActions, err = PolicyRequests(install)
	unread("pending policy actions", err)
	if seams.Policy != nil {
		policy, err := seams.batchPolicy()
		unread("the batch policy", err)
		if err == nil {
			status.BatchPolicy = &policy
		}
	}
	// damaged says the lines of a record file that do not decode: the lane's
	// own readers skip them, and the status says them (fix round 4).
	damaged := func(what string, skipped int, path string) {
		if skipped > 0 {
			lines := "lines"
			if skipped == 1 {
				lines = "line"
			}
			status.Problems = append(status.Problems, fmt.Sprintf("%s %d %s that can't be read (%s)", what, skipped, lines, path))
		}
	}
	status.Stop, err = NewestStop(install)
	unread("the stop record", err)
	if status.Stop != nil && status.Stop.Loop == "lane-return" && status.Stop.Subject == "lane" && status.Owner.State == lane.OwnerHeld {
		command := status.Stop.Command()
		status.Owner.RetryHint = &command
		status.Summary += "; run: " + command
	}
	if status.Stop != nil && strings.HasPrefix(status.Stop.Handoff, "hold ") {
		main, readErr := git.main()
		if readErr == nil {
			var incidents []goal.TrunkRedEntry
			incidents, readErr = seams.incidents(install, string(layout.Checkout), main)
			if readErr == nil && !slices.ContainsFunc(incidents, func(entry goal.TrunkRedEntry) bool {
				return entry.Identity == strings.TrimPrefix(status.Stop.Handoff, "hold ") && entry.Closed == nil
			}) {
				status.Stop = nil
			}
		}
		unread("the stop's main incident", readErr)
	}
	lines, skipped, err := countedLines[Line](queuePath(install))
	unread("the queue", err)
	damaged("the queue has", skipped, queuePath(install))
	if err == nil {
		status.Queue = entriesOf(lines)
	}
	if len(status.Queue) > 0 {
		main, err := git.main()
		if err == nil {
			contains := func(sha string) (bool, error) { return git.contains(main, sha) }
			var again error
			status.Queue, err = Landed(status.Queue, contains)
			incidents, readErr := seams.incidents(install, string(layout.Checkout), main)
			unread("main's incidents", readErr)
			if readErr == nil {
				status.Queue = HoldEntries(status.Queue, incidents)
			}
			status.Queue, again = landedBeforeAgain(status.Queue, seams.now().Add(-landedWindow), contains)
			err = errors.Join(err, again)
		}
		unread("whether main holds the queued work", err)
	}
	regenerating, err := ReadRunningRegeneration(install, seams)
	unread("the running regeneration", err)
	status.RunningRegeneration = regenerating
	running, err := readRunningProof(install, seams)
	unread("the running proof", err)
	status.RunningProof = running
	if running != nil && running.State == "running" && running.Commit != "" {
		running.Goals, err = proving(status.Queue, running.Commit, git.contains)
		unread("what the running proof holds", err)
	}
	results, skipped, err := countedLines[Result](resultsPath(install))
	unread("the proof results", err)
	damaged("the proof results have", skipped, resultsPath(install))
	if err == nil && len(results) > 0 {
		status.LastProof = &results[len(results)-1]
	}
	gates, skipped, err := countedLines[Result](gatesPath(install))
	unread("the gate results", err)
	damaged("the gate results have", skipped, gatesPath(install))
	if err == nil && len(gates) > 0 {
		status.LastGate = &gates[len(gates)-1]
	}
	pushes, skipped, err := countedLines[Pushed](pushesPath(install))
	unread("the push record", err)
	damaged("the push record has", skipped, pushesPath(install))
	if err == nil && len(pushes) > 0 {
		last := pushes[len(pushes)-1]
		status.LastPush = &last
		status.Queue, err = LandingTimes(status.Queue, pushes, seams.now().Add(-landedWindow), git.brought)
		unread("when the queued work landed", err)
	}
	return status
}

// proving are the goals of the waiting entries the proof's commit contains,
// in queue order; an entry whose containment can't be read is named in the
// error and left out.
func proving(queue []Entry, commit string, contains func(main, sha string) (bool, error)) ([]string, error) {
	checked, err := proofGoals(queue, commit, contains)
	var goals []string
	for _, one := range checked {
		goals = append(goals, one.Goal)
	}
	return goals, err
}

func proofGoals(queue []Entry, commit string, contains func(main, sha string) (bool, error)) ([]GoalSHA, error) {
	var goals []GoalSHA
	var problems []error
	for _, entry := range queue {
		if entry.State != StateWaiting {
			continue
		}
		inside, err := contains(commit, entry.SHA)
		switch {
		case err != nil:
			problems = append(problems, fmt.Errorf("%s: %w", entry.Goal, err))
		case inside:
			goals = append(goals, GoalSHA{Goal: entry.Goal, SHA: entry.SHA})
		}
	}
	return goals, errors.Join(problems...)
}

// LandingTimes gives each landed entry the time of the push that brought its
// commit to main: the first push, oldest first, whose old main did not
// contain it and whose commit does, which is what brought lists. Only pushes
// at or after since are asked; a landing no such push brought (an older one,
// or work that reached main some other way) carries no time. A push whose
// commits can't be listed, or whose line names no old main, commit or
// readable time (landing push writes all three), is named in the error, and
// the others still time their entries.
func LandingTimes(entries []Entry, pushes []Pushed, since time.Time, brought func(old, commit string) ([]string, error)) ([]Entry, error) {
	out := append([]Entry(nil), entries...)
	waiting := map[string][]int{}
	for index, entry := range out {
		if entry.State == StateLanded && entry.SHA != "" {
			waiting[entry.SHA] = append(waiting[entry.SHA], index)
		}
	}
	var problems []error
	for _, push := range pushes {
		at, err := time.Parse(time.RFC3339, push.At)
		if err == nil && at.Before(since) {
			continue
		}
		if err != nil || push.Old == "" || push.Commit == "" {
			// landing push writes all three, so a line without one is
			// damaged: it can't say what it brought, and is named.
			problems = append(problems, fmt.Errorf("a push line can't be read (old %q, commit %q, at %q)", Short(push.Old), Short(push.Commit), push.At))
			continue
		}
		if len(waiting) == 0 {
			// Every landing is timed; the rest of the last day's lines
			// are still read for damage.
			continue
		}
		commits, err := brought(push.Old, push.Commit)
		if err != nil {
			problems = append(problems, fmt.Errorf("push %s: %w", Short(push.Commit), err))
			continue
		}
		for _, commit := range commits {
			for _, index := range waiting[commit] {
				out[index].LandedAt = push.At
			}
			delete(waiting, commit)
		}
	}
	return out, errors.Join(problems...)
}

// landedBeforeAgain asks main about each hand-in superseded by a later
// hand-in of its goal made at or after since: it may have landed before its
// goal was handed in again, and today's list keeps that landing. A hand-in
// superseded before since is not asked; the queue only grows.
func landedBeforeAgain(entries []Entry, since time.Time, contains func(sha string) (bool, error)) ([]Entry, error) {
	out := append([]Entry(nil), entries...)
	var problems []error
	for index, entry := range out {
		if entry.State != StateSuperseded || !handedInAgainSince(out[index+1:], entry.Goal, since) {
			continue
		}
		inside, err := contains(entry.SHA)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", entry.Goal, err))
		} else if inside {
			out[index].State = StateLanded
		}
	}
	return out, errors.Join(problems...)
}

// handedInAgainSince is whether the first later hand-in of goal was made at
// or after since.
func handedInAgainSince(later []Entry, goal string, since time.Time) bool {
	for _, entry := range later {
		if entry.Goal == goal {
			at, err := time.Parse(time.RFC3339, entry.At)
			return err == nil && !at.Before(since)
		}
	}
	return false
}

// ReadRunningProof is the lane's running proof; nil when none is recorded
// running or it can't be read.
func ReadRunningProof(install string, seams ProveSeams) *RunningProof {
	running, _ := readRunningProof(install, seams)
	return running
}

// readRunningProof is the lane's running proof, nil when none is recorded,
// and why it can't be read.
func readRunningProof(install string, seams ProveSeams) (*RunningProof, error) {
	running, recorded, alive, err := ReadRunning(install, seams)
	if err != nil || !recorded {
		return nil, err
	}
	state := "running"
	if !alive {
		state = "died"
	}
	if running.Admission != nil && (running.Admission.State == "pending" || running.Admission.State == "failed") {
		state = running.Admission.State
		if state == "pending" && !alive {
			state = "failed"
		}
	}
	return &RunningProof{Admission: running.Admission, Person: running.Person, Trunk: running.Trunk, BatchID: running.BatchID, BatchMembers: running.BatchMembers, Gate: running.Gate, Attempt: running.Attempt, Tree: running.Tree, Commit: running.Commit, Since: running.Since, Log: running.Log, State: state}, nil
}
