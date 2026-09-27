package supervisor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
)

// The shared delegate round: one turn of any runtime, driven only through
// its operations (ops.go). Everything here is identical for every runtime:
// the launch capability and start gate, the envelope comparison and
// refusal, reference verification, pre-fork evidence and custody, the
// handshake, deadlines and the kill domain, the terminal record, return
// normalization, adjudication and the bounded repairs.

// turnOf is the runtime's view of a prepared delegate round.
func (s *Supervision) turnOf() *Turn {
	return &Turn{
		d: s.d, Role: RoleDelegate, Verb: s.verb, Runtime: s.runtime, Root: s.d.Root,
		Workspace: s.workspace, Dir: s.roundDir, Record: s.record, Job: s.job, Tag: s.tag,
		Round: s.round, RootJob: s.rootJob, Prompt: s.prompt, Schema: s.schema, Model: s.requestedModel,
		ResumeSession: s.requestedSession, Effective: s.effective, Requested: s.record, Events: s.events,
		Log: s.logWriter(), Env: s.childEnv,
	}
}

// land lands a runtime's named refusal: pending before the handshake,
// running after it.
func (s *Supervision) land(refusal *Refusal, usageFile string) {
	if s.handshakeDone {
		s.finishRunning("failed", refusal.Error, refusal.Phase, usageFile)
	} else {
		s.failPending(refusal.Error, refusal.Phase, usageFile)
	}
}

// start launches what prepare named: an in-process CLI stand-in, a protocol
// server with its shared client, or the CLI itself.
func (s *Supervision) start(launch Launch) (*child, *child, error) {
	if launch.Simulated != nil {
		return simulatedChild(launch.Simulated), nil, nil
	}
	if launch.SetupError != nil {
		s.logf("%v\n", launch.SetupError)
		fmt.Fprintf(s.d.Stderr, "%s child exited before custody identity was recorded\n", s.runtime)
		return nil, nil, launch.SetupError
	}
	env := withEnv(s.childEnv, launch.Env...)
	argv := launch.Argv
	if launch.Argv0 != "" {
		argv = append([]string{launch.Argv0}, argv[1:]...)
	}
	cli, err := s.launchProgram(launch.Argv[0], argv, env, launch.StdinPath, launch.StdoutPath)
	if err != nil {
		return nil, nil, err
	}
	return cli, nil, nil
}

// launchProgram starts program (resolved on the lookup path) with argv as
// its argument vector.
func (s *Supervision) launchProgram(program string, argv, env []string, stdinPath, stdoutPath string) (*child, error) {
	return s.launchAs(append([]string{program}, argv[1:]...), argv[0], env, stdinPath, stdoutPath)
}

// simulatedChild runs an in-process CLI stand-in as a child: stopping it
// closes its stop channel.
func simulatedChild(run func(stop <-chan struct{}) int) *child {
	stop := make(chan struct{})
	c := &child{done: make(chan struct{}), pid: os.Getpid()}
	closed := false
	c.stop = func() {
		if !closed {
			closed = true
			close(stop)
		}
	}
	go func() {
		c.status = run(stop)
		close(c.done)
	}()
	return c
}

// superviseRound is one delegate round of a runtime.
func superviseRound(s *Supervision, args []string, ops Operations) int {
	if admitter, ok := ops.(Admitter); ok {
		if err := admitter.Admit(s.d, s.verb); err != nil {
			fmt.Fprintln(s.d.Stderr, err)
			return 1
		}
	}
	if !s.prepareOrUsage(args) {
		return 2
	}
	usageFile := filepath.Join(s.roundDir, "usage.json")
	t := s.turnOf()
	launch, err := ops.Prepare(t)
	if err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	if launch.EarlyRefusal != nil {
		s.failPending(launch.EarlyRefusal.Error, launch.EarlyRefusal.Phase, "")
		return 1
	}
	if !s.failIfEffectiveWider() {
		return 1
	}
	if launch.Refusal != nil {
		s.failPending(launch.Refusal.Error, launch.Refusal.Phase, "")
		return 1
	}
	if launch.BeforeLaunch != nil {
		refusal, err := launch.BeforeLaunch()
		if err != nil {
			fmt.Fprintln(s.d.Stderr, err)
			return 1
		}
		if refusal != nil {
			s.failPending(refusal.Error, refusal.Phase, "")
			return 1
		}
	}
	drive := s.driveCLI
	if launch.Protocol != nil {
		drive = s.driveProtocol
	}
	status, code, cont := drive(t, ops, launch)
	if !cont {
		return code
	}
	t.HandshakeDone, t.SessionID = s.handshakeDone, s.sessionID
	final, err := ops.Finalize(t, FinalInput{Launch: launch, Status: status, Usage: usageFile})
	if err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	if final.Handshake != nil && final.Handshake.Session != "" && !s.handshakeDone {
		s.handshakeFrom(*final.Handshake)
	}
	if final.RecordModel != "" {
		s.recordResultEffectiveModel(final.RecordModel)
	}
	if final.Refusal != nil {
		s.land(final.Refusal, usageFile)
		return 1
	}
	if !s.settleResultIdentity(final.Session, final.Turn, final.HandshakeModel, final.ResultModel, usageFile) {
		return 1
	}
	if final.Status != nil {
		status = *final.Status
	}
	description, _ := ops.Describe(s.d)
	hooks := repairHooks{}
	if description.Capabilities.Repair {
		hooks = s.repairHooksOf(t, ops, launch)
	}
	if final.DeliveryRepair {
		return s.deliveryRepair(t, ops, launch, usageFile, hooks)
	}
	return terminal(s.completeFromCLI(status, usageFile, final.Candidate, final.Transcript, hooks))
}

// driveCLI launches the CLI (or its in-process stand-in) and supervises it
// to its exit: custody, the handshake from observe, deadlines. cont is
// false when the round already ended, with code its status.
func (s *Supervision) driveCLI(t *Turn, ops Operations, launch Launch) (status, code int, cont bool) {
	if !s.verifyReferences() {
		return 0, 1, false
	}
	// A simulated CLI forks nothing and never enters custody, so it leaves
	// no pre-fork marker behind.
	if launch.Simulated == nil {
		if err := s.markPrefork(); err != nil {
			fmt.Fprintln(s.d.Stderr, err)
			s.failPending("prefork_marker", "handshake", "")
			return 0, 1, false
		}
	}
	cli, _, err := s.start(launch)
	if err != nil {
		s.failPending("custody_registration", "handshake", "")
		return 0, 1, false
	}
	if launch.Simulated == nil {
		if err := s.registerCustody(cli); err != nil {
			cli.terminate()
			s.failPending("custody_registration", "handshake", "")
			return 0, 1, false
		}
	}
	for cli.alive() {
		t.HandshakeDone, t.SessionID = s.handshakeDone, s.sessionID
		events, err := ops.Observe(t, Observation{Launch: launch, Running: true})
		if err != nil {
			s.logf("%v\n", err)
		}
		if events.Refusal != nil {
			s.failPending(events.Refusal.Error, events.Refusal.Phase, "")
			cli.terminate()
			return 0, 1, false
		}
		if events.Session != "" {
			if !s.handshakeFrom(events) {
				cli.terminate()
				return 0, 1, false
			}
			if launch.OnHandshake != nil {
				launch.OnHandshake()
			}
			break
		}
		touch(s.heartbeat)
		s.d.Clock.Sleep(pollTick)
	}
	status, err = s.waitForCLI(cli)
	if err != nil {
		return 0, exitCodeOf(err, 1), false
	}
	return status, 0, true
}

func (s *Supervision) appendEventLine(line string) {
	file, err := os.OpenFile(s.events, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	fmt.Fprintln(file, line)
}

// repairHooksOf binds the shape repair to the runtime's repair operation.
func (s *Supervision) repairHooksOf(t *Turn, ops Operations, launch Launch) repairHooks {
	var usage string
	return repairHooks{
		turn: func(promptFile, outputFile string) bool {
			t.HandshakeDone, t.SessionID = s.handshakeDone, s.sessionID
			result := ops.Repair(t, RepairInput{Launch: launch, Stage: RepairTurn, PromptFile: promptFile, OutputFile: outputFile})
			return result.Status == 0 && nonEmptyFile(outputFile)
		},
		usageAfter: func(usageFile string) {
			usage = usageFile
			ops.Repair(t, RepairInput{Launch: launch, Stage: RepairUsage, Usage: usageFile})
		},
		settleAfter: func() bool {
			result := ops.Repair(t, RepairInput{Launch: launch, Stage: RepairSettle, Usage: usage})
			if result.Model != "" {
				s.recordResultEffectiveModel(result.Model)
			}
			return result.Settled
		},
	}
}

// deliveryRepair is the delivery repair (D64): adjudication recommends, the
// durable claim is won BEFORE the paid call, the repair CLI's exit is
// reported separately, and delivery is judged by the runtime's post-repair
// collection.
func (s *Supervision) deliveryRepair(t *Turn, ops Operations, launch Launch, usageFile string, hooks repairHooks) int {
	result := ops.Repair(t, RepairInput{Launch: launch, Stage: RepairNamedPath})
	verdict := s.adjudicate(adapter.AdjudicateParams{
		Stage: "empty-delivery", HandshakeDone: true, NamedRepairPath: result.NamedPath,
		RepairAvailable: s.returnRepairs == 0 && s.sessionID != "",
	})
	if verdict != "delivery-repair" {
		words := strings.Fields(verdict)
		switch {
		case len(words) >= 3 && words[0] == "fail-pending":
			s.failPending(words[1], words[2], usageFile)
		case len(words) >= 4 && words[0] != "fail-pending":
			s.finishRunning(words[1], words[2], words[3], usageFile)
		}
		return 1
	}
	claim := s.d.Dispatch.Run(s.logWriter(), s.logWriter(), "__repair-claim", "--job", s.job)
	if claim == 3 {
		// The repair is spent (this round or the malformed flow): the
		// pinned empty outcome stands.
		s.finishRunning("failed", "empty_reply", "delivery", usageFile)
		return 1
	} else if claim != 0 {
		s.finishRunning("failed", "collect_mechanical", "delivery", usageFile)
		return 1
	}
	s.returnRepairs = 1
	s.logf("%s delivery repair attempt 1: no return was delivered, asking session %s to write %s\n",
		s.d.nowISO(), s.sessionID, result.NamedPath)
	t.HandshakeDone, t.SessionID = s.handshakeDone, s.sessionID
	repaired := ops.Repair(t, RepairInput{Launch: launch, Stage: RepairDelivery,
		PromptFile: filepath.Join(s.roundDir, "repair-1.prompt.md"), OutputFile: filepath.Join(s.roundDir, "repair-1.out"),
		NamedPath: result.NamedPath, Usage: usageFile})
	violation := filepath.Join(s.roundDir, "protocol-violation.txt")
	if repaired.Status != 0 {
		_ = os.WriteFile(violation, []byte(fmt.Sprintf("delivery repair provider call failed rc=%d\n", repaired.Status)), 0o644)
		s.finishProtocolError(violation)
		return 1
	}
	if repaired.Candidate == "" {
		_ = os.WriteFile(violation, []byte(repaired.Violation+"\n"), 0o644)
		s.finishProtocolError(violation)
		return 1
	}
	// Delivered: the repaired session settles before the pipeline runs.
	if hooks.settleAfter == nil || !hooks.settleAfter() {
		s.finishRunning("failed", "session_identity_disagreement", "delivery", usageFile)
		return 1
	}
	return terminal(s.completeFromCLI(0, usageFile, repaired.Candidate, "", hooks))
}

// RepairStage names one step of a repair.
type RepairStage string

const (
	// RepairTurn runs the shape repair turn in the same session.
	RepairTurn RepairStage = "turn"
	// RepairUsage recomputes the round's usage after a repair.
	RepairUsage RepairStage = "usage"
	// RepairSettle re-certifies session and model from the repair.
	RepairSettle RepairStage = "settle"
	// RepairNamedPath names the delivery repair's return file.
	RepairNamedPath RepairStage = "named-path"
	// RepairDelivery runs the delivery repair and collects its reply.
	RepairDelivery RepairStage = "delivery"
)

// runHostCLI starts a host turn's CLI and waits for it, returning its
// status: stdin from the prompt, stdout into the launch's path, stderr into
// the host log (truncated first when the launch asks).
func runHostCLI(d Deps, t *Turn, launch Launch, hostLog string) int {
	program, err := d.LookPath(launch.Argv[0])
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 127
	}
	command := exec.Command(program, launch.Argv[1:]...)
	command.Args[0] = launch.Argv[0]
	if launch.Argv0 != "" {
		command.Args[0] = launch.Argv0
	}
	command.Dir = t.Workspace
	command.Env = withEnv(t.Env, launch.Env...)
	if launch.StdinPath != "" {
		stdin, err := os.Open(launch.StdinPath)
		if err != nil {
			fmt.Fprintln(d.Stderr, err)
			return 1
		}
		defer stdin.Close()
		command.Stdin = stdin
	}
	stdout, err := os.OpenFile(launch.StdoutPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	defer stdout.Close()
	command.Stdout = stdout
	flags := os.O_WRONLY | os.O_CREATE | os.O_APPEND
	if launch.TruncateLog {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	log, err := os.OpenFile(hostLog, flags, 0o644)
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	defer log.Close()
	command.Stderr = log
	return exitStatus(command.Run())
}

// HostTurnFacts are what the mission host turn knows before its runtime
// prepares.
type HostTurnFacts struct {
	Runtime, TurnDir, Mission, TurnID string
	Prompt, Schema, ResumeSession     string
	Tag, Requested                    string
	// Result is the turn's result envelope path.
	Result string
}

// NewHostTurn is a runtime's view of one mission host turn (role host): the
// CLI runs in the installation's checkout.
func NewHostTurn(d Deps, f HostTurnFacts) *Turn {
	return &Turn{d: d, Role: RoleHost, Verb: "start-turn", Runtime: f.Runtime, Root: d.Root,
		Workspace: d.Root, Dir: f.TurnDir, Record: filepath.Join(f.TurnDir, "turn.json"),
		Mission: f.Mission, TurnID: f.TurnID, Tag: f.Tag, Result: f.Result, Prompt: f.Prompt, Schema: f.Schema,
		ResumeSession: f.ResumeSession, Requested: f.Requested, Env: d.Environ,
		Log: appendLog(filepath.Join(f.TurnDir, "host.log"))}
}

// RunHostCLI runs a host turn's CLI to its exit and returns its status.
func RunHostCLI(d Deps, t *Turn, launch Launch, hostLog string) int {
	return runHostCLI(d, t, launch, hostLog)
}

// appendLog is a writer appending to a file per write.
type appendLog string

func (a appendLog) Write(p []byte) (int, error) {
	file, err := os.OpenFile(string(a), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	return file.Write(p)
}
