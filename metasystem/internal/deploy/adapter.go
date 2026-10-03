package deploy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Request is the one JSON request an adapter reads on standard input.
type Request struct {
	Schema    int       `json:"schema"`
	Operation string    `json:"operation"`
	Project   string    `json:"project"`
	Commit    string    `json:"commit"`
	Source    string    `json:"source"`
	Artifact  string    `json:"artifact"`
	Previous  *Deployed `json:"previous"`
}

// Deployed is a deploy as the request names it.
type Deployed struct {
	Commit   string `json:"commit"`
	Version  string `json:"version"`
	Artifact string `json:"artifact"`
	Digest   string `json:"digest"`
}

// Response is the one JSON response an adapter writes on standard output.
type Response struct {
	Outcome  string `json:"outcome"`
	Version  string `json:"version,omitempty"`
	Artifact string `json:"artifact,omitempty"`
	Digest   string `json:"digest,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// The states an adapter call ends in, by the contract's exit meanings.
const (
	stateDone        = "done"        // exit 0: the response is the truth
	stateFailed      = "failed"      // exit 1 with a failed response: nothing changed
	stateUnsupported = "unsupported" // exit 64
	stateUnknown     = "unknown"     // anything else: the state is unknown
	statePaused      = "paused"      // a run's call never started: deploys are paused
)

type answer struct {
	Response
	operation string
	state     string
	exit      int
}

func (a answer) done(outcome string) bool { return a.state == stateDone && a.Outcome == outcome }

// describe says in one phrase how the call ended.
func (a answer) describe() string {
	switch a.state {
	case stateFailed:
		return "the adapter's " + a.operation + " failed: " + oneLine(a.Reason)
	case stateUnsupported:
		return "the adapter does not support " + a.operation
	case statePaused:
		return "deploys were paused before the adapter's " + a.operation + " started"
	case stateDone:
		return fmt.Sprintf("the adapter's %s answered %q", a.operation, a.Outcome)
	}
	if a.exit < 0 {
		return "the adapter's " + a.operation + " was ended by a signal; what is active is unknown"
	}
	return fmt.Sprintf("the adapter's %s exited %d without a readable answer; what is active is unknown", a.operation, a.exit)
}

// call runs `<argv> OPERATION`, by the contract of the clean tree of
// commit, the commit the operation is about, and from its cwd inside that
// tree, in a process group of its own so a pause can end everything it
// started. It waits for the adapter to exit, however long that takes: a
// slow build is still a build. Standard error goes to log. A call that
// holds the lock (active not nil) is in run.json from before its process
// starts until it ends. A run's call starts only while deploys are not
// paused, and ends its own adapter when a pause came while it started: a
// pause that found no adapter in run.json to end is seen by the run.
func (r *Runner) call(operation, commit, artifact string, previous *Line, log string, active *Active) answer {
	result := answer{operation: operation, state: stateUnknown, exit: -1}
	tree, err := r.treeOf(commit)
	if err != nil {
		// Nothing ran, so nothing changed.
		result.state, result.Reason = stateFailed, err.Error()
		return result
	}
	argv := append(append([]string(nil), tree.contract.Adapter.Argv...), operation)
	body, err := json.Marshal(r.request(operation, commit, tree.source, artifact, previous))
	if err != nil {
		result.state, result.Reason = stateFailed, err.Error()
		return result
	}
	logFile, err := os.OpenFile(log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		result.state, result.Reason = stateFailed, "its log can't be opened: "+err.Error()
		return result
	}
	defer logFile.Close()
	fmt.Fprintf(logFile, "== %s %s\n", operation, commit)
	command := exec.Command(argv[0], argv[1:]...)
	command.Dir = filepath.Join(tree.source, filepath.FromSlash(tree.contract.Adapter.CWD))
	command.Stdin = bytes.NewReader(body)
	var stdout bytes.Buffer
	command.Stdout, command.Stderr = &stdout, logFile
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if active != nil {
		*active = Active{Runner: os.Getpid(), Kind: active.Kind, Operation: operation, Commit: active.Commit, StartedAt: r.Now(), Log: log}
		active.RunnerBorn, _ = identity.ProcessBirth(int64(active.Runner))
		if err := writeJSON(activePath(r.Dir), active); err != nil {
			result.state, result.Reason = stateFailed, "run.json can't be written, so deploy pause could not stop the "+operation+": "+err.Error()
			return result
		}
		// run.json stays only when this process dies with the call in
		// flight: the adapter may outlive it, and the next run waits for it.
		defer func() { _ = removeFile(activePath(r.Dir)) }()
		if r.Starting != nil {
			r.Starting(*active)
		}
		if paused, _ := r.paused(); paused && !r.acting {
			result.state = statePaused
			return result
		}
	}
	if err := command.Start(); err != nil {
		// Nothing ran, so nothing changed.
		result.state, result.Reason = stateFailed, "the adapter can't be started: "+err.Error()
		return result
	}
	if active != nil {
		active.PID = command.Process.Pid
		active.Born, _ = identity.ProcessBirth(int64(active.PID))
		if err := writeJSON(activePath(r.Dir), active); err != nil {
			fmt.Fprintf(logFile, "run.json can't name the adapter, so deploy pause can't end this %s: %v\n", operation, err)
		}
		if r.Started != nil {
			r.Started(*active)
		}
		if paused, _ := r.paused(); paused && !r.acting {
			_ = syscall.Kill(-active.PID, syscall.SIGKILL)
		}
	}
	_ = command.Wait()
	result.exit = command.ProcessState.ExitCode()
	output := bytes.TrimSpace(stdout.Bytes())
	readable := len(output) > 0 && json.Unmarshal(output, &result.Response) == nil
	switch {
	case result.exit == 0 && readable:
		result.state = stateDone
	case result.exit == 1 && readable && result.Outcome == "failed":
		result.state = stateFailed
	case result.exit == 64:
		result.state = stateUnsupported
	default:
		result.Response = Response{}
	}
	return result
}

func oneLine(text string) string {
	text = strings.TrimSpace(text)
	if index := strings.IndexByte(text, '\n'); index >= 0 {
		text = text[:index]
	}
	if text == "" {
		return "no reason was given"
	}
	return text
}

// Short is a commit's short form in sentences.
func Short(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}
