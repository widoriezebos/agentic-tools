package plain

import "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"

// Status is the plain lane as landing status --json carries it and as
// /api/board carries it under "lane" (goal fleet-card-can-land-now): one
// reader, so the card reads what the terminal reads. It is the lane's view,
// whether the lane is paused and its agent alive, the queue, the running
// proof, the last proof and the last push. An absent value is null.
type Status struct {
	lane.View
	Paused     bool `json:"paused"`
	AgentAlive bool `json:"agent_alive"`
	// Queue is every hand-in of queue.jsonl, oldest first, with its state:
	// waiting, returned, superseded by a newer hand-in of its goal, or
	// landed when origin's main (as the lane checkout last fetched it)
	// contains its sha.
	Queue        []Entry       `json:"queue"`
	RunningProof *RunningProof `json:"running_proof"`
	// LastProof is the newest line of results.jsonl.
	LastProof *Result `json:"last_proof"`
	// LastPush is the newest push landing push made.
	LastPush *Pushed `json:"last_push"`
}

// RunningProof is the lane's proof recorded running.
type RunningProof struct {
	Tree   string `json:"tree"`
	Commit string `json:"commit,omitempty"`
	Since  string `json:"since"`
	// Attempt and Log name the run; State is running while its process
	// runs, died when it ended without a result (the next prove runs it
	// again).
	Attempt string `json:"attempt"`
	Log     string `json:"log,omitempty"`
	State   string `json:"state"`
}

// ReadStatus reads the plain lane around its view: home is the lane's home
// (its pause), record the registered lane, seams the proof's (its process
// check).
func ReadStatus(home string, record lane.Record, view lane.View, seams ProveSeams) Status {
	_, paused := lane.ReadPause(home)
	status := Status{View: view, Paused: paused, AgentAlive: view.Owner.State == lane.OwnerRunning, Queue: []Entry{}}
	layout, err := record.Layout()
	if view.Root == nil || err != nil {
		return status
	}
	install, checkout := string(layout.Install), string(layout.Checkout)
	if entries, err := Entries(install); err == nil {
		if main, err := Git(checkout, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/main^{commit}"); err == nil {
			entries, _ = Landed(entries, ContainedIn(checkout, main))
		}
		status.Queue = entries
	}
	status.RunningProof = ReadRunningProof(install, seams)
	if result, ok, err := LastResult(install); err == nil && ok {
		status.LastProof = &result
	}
	if push, ok, err := LastPush(install); err == nil && ok {
		status.LastPush = &push
	}
	return status
}

// ReadRunningProof is the lane's running proof; nil when none is recorded
// running or it can't be read.
func ReadRunningProof(install string, seams ProveSeams) *RunningProof {
	running, recorded, alive, err := ReadRunning(install, seams)
	if err != nil || !recorded {
		return nil
	}
	state := "running"
	if !alive {
		state = "died"
	}
	return &RunningProof{Attempt: running.Attempt, Tree: running.Tree, Commit: running.Commit, Since: running.Since, Log: running.Log, State: state}
}
