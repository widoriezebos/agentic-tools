package supervisor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/acp"
)

// The shared protocol transport (ACP): any runtime whose prepare declares a
// ProtocolLaunch gets the server child on a per-attempt fifo pair, the
// in-process client (acp.RunFileTurn, the body of the deleted `acp turn`
// verb), custody of the server, the client's cancellation on TERM/INT, and
// the pair's removal on every exit — in the delegate round and the host
// turn alike.
//
// The client is in process: a goroutine of the supervisor, cancelled (never
// signalled) to terminate it. It is not registered in custody — it has no
// process of its own, and the supervisor is already the custody root — and
// the KIND-launched event names it with the supervisor's pid. The second
// pre-fork mark stays where the client launch was.

// signalHooks holds, per installation root, a test's delivery of a
// termination signal to that root's supervisor or host turn, so a test
// signals the process it drives without signalling the test process.
// Production roots have no hook and take the real signals.
var signalHooks sync.Map // root -> func(chan<- os.Signal)

// HookTermination routes the termination signals of the supervisor or host
// turn of root to fn instead of the process's own (a test seam); the
// returned function removes the hook.
func HookTermination(root string, fn func(chan<- os.Signal)) (remove func()) {
	signalHooks.Store(root, fn)
	return func() { signalHooks.Delete(root) }
}

// notifyTermination delivers TERM and INT to c and returns the stop.
func notifyTermination(root string, c chan<- os.Signal) (stop func()) {
	if hook, ok := signalHooks.Load(root); ok {
		hook.(func(chan<- os.Signal))(c)
		return func() {}
	}
	signal.Notify(c, syscall.SIGTERM, os.Interrupt)
	return func() { signal.Stop(c) }
}

// protocolSignalBound is how long a signalled process waits for the
// in-process client's courtesy cancel before it ends regardless.
const protocolSignalBound = 3 * time.Second

// signalExitCode is the shell's status for a death by signal.
func signalExitCode(sig os.Signal) int32 {
	if sig == os.Interrupt {
		return 130
	}
	return 143
}

// signalGuard turns TERM or INT into the client's cancellation, so the
// courtesy cancel and the typed cancelled outcome still happen. The caller
// returns the recorded code through its own flow; a flow that has not
// finished within the bound is ended with it after cleanup.
type signalGuard struct {
	code     atomic.Int32
	finished chan struct{}
	stop     func()
}

func guardSignals(root string, cancel, cleanup func()) *signalGuard {
	g := &signalGuard{finished: make(chan struct{})}
	signals := make(chan os.Signal, 1)
	g.stop = notifyTermination(root, signals)
	go func() {
		select {
		case sig := <-signals:
			code := signalExitCode(sig)
			g.code.Store(code)
			cancel()
			select {
			case <-g.finished:
				return
			case <-time.After(protocolSignalBound):
			}
			cleanup()
			os.Exit(int(code))
		case <-g.finished:
		}
	}()
	return g
}

func (g *signalGuard) signalled() int { return int(g.code.Load()) }

func (g *signalGuard) done() {
	g.stop()
	close(g.finished)
}

// protocolClient drives the shared protocol client of a protocol launch as
// an in-process child: cancelling it is its termination. A client blocked
// opening a fifo cannot see its cancellation, so stopping it also unblocks
// the pipes until it returns. Its outcome file is started empty, as the
// former verb's redirected stdout was.
func protocolClient(p *ProtocolLaunch, pipes *ACPPipes, workspace string, log io.Writer) *child {
	ctx, cancel := context.WithCancel(context.Background())
	c := &child{done: make(chan struct{}), pid: os.Getpid()}
	c.stop = func() {
		cancel()
		go pipes.Unblock(c.done)
	}
	_ = os.WriteFile(p.Outcome, nil, 0o644)
	go func() {
		defer cancel()
		outcome, err := acp.RunFileTurn(ctx, acp.FileTurnConfig{
			ServerOut: pipes.ServerOut, ServerIn: pipes.ServerIn, JournalPath: p.Journal, Workspace: workspace,
			EnvelopePath: p.Envelope, PromptFile: p.PromptFile, LoadSession: p.LoadSession, Mode: p.Mode,
			ExpectedProtocol: p.ExpectedProtocol, SessionFile: p.SessionFile,
		})
		if err != nil {
			fmt.Fprintln(log, err)
			c.status = 1
		} else if err := acp.WriteFileTurnOutcome(p.Outcome, outcome); err != nil {
			fmt.Fprintln(log, err)
			c.status = 1
		}
		close(c.done)
	}()
	return c
}

// unblockOnDeath frees a client stranded in a fifo open when the server is
// gone before the client returns.
func unblockOnDeath(pipes *ACPPipes, serverDone, clientDone <-chan struct{}) {
	go func() {
		select {
		case <-serverDone:
			pipes.Unblock(clientDone)
		case <-clientDone:
		}
	}()
}

// protocolServerCommand is the launch's server command on the pair.
func protocolServerCommand(d Deps, pipes *ACPPipes, launch Launch, dir string, env []string, log io.Writer) (*exec.Cmd, error) {
	program, err := d.LookPath(launch.Argv[0])
	if err != nil {
		return nil, err
	}
	argv0 := launch.Argv[0]
	if launch.Argv0 != "" {
		argv0 = launch.Argv0
	}
	return pipes.ServerCommand(program, argv0, launch.Argv[1:], dir, withEnv(env, launch.Env...), log)
}

// handshakeFrom records a handshake the runtime reported, then its events
// lines and its record patch.
func (s *Supervision) handshakeFrom(events Events) bool {
	if !s.recordHandshake(events.Session, events.Turn, events.Model) {
		return false
	}
	for _, line := range events.Lines {
		s.appendEventLine(line)
	}
	if events.RecordPatch != "" {
		s.d.Dispatch.Run(s.logWriter(), s.logWriter(), "__record-cas", "--job", s.job,
			"--expect", "running", "--status", "running", "--patch", events.RecordPatch)
	}
	return true
}

// driveProtocol runs a protocol launch of a delegate round from the fifo
// pair to the client's exit. cont is false when the round already ended,
// with code its status.
func (s *Supervision) driveProtocol(t *Turn, ops Operations, launch Launch) (status, code int, cont bool) {
	p := *launch.Protocol
	pipes, err := ACPPipeNames(s.roundDir)
	if err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 0, 1, false
	}
	if err := pipes.Make(); err != nil {
		s.logf("%v\n", err)
		s.failPending(p.Kind+"_fifo_setup", "setup", "")
		return 0, 1, false
	}
	p.ServerOut, p.ServerIn = pipes.ServerOut, pipes.ServerIn
	var server, client *child
	// Both endpoints dead ⇒ the pair is inert and removed at every exit
	// (KI-42), the custodian's deadline exit included.
	defer func() {
		if client != nil {
			client.terminate()
		}
		if server != nil {
			server.terminate()
		}
		pipes.Remove()
		if p.Cleanup != nil {
			p.Cleanup()
		}
	}()
	if !s.verifyReferences() {
		return 0, 1, false
	}
	if err := s.markPrefork(); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		s.failPending("prefork_marker", "handshake", "")
		return 0, 1, false
	}
	command, err := protocolServerCommand(s.d, pipes, launch, s.workspace, s.childEnv, s.logWriter())
	if err == nil {
		server, err = startChild(command)
		pipes.Started()
	}
	if err != nil {
		s.logf("%v\n", err)
		fmt.Fprintf(s.d.Stderr, "%s child exited before custody identity was recorded\n", s.runtime)
		server = nil
		s.failPending("custody_registration", "handshake", "")
		return 0, 1, false
	}
	if err := s.registerCustody(server); err != nil {
		server.terminate()
		s.failPending("custody_registration", "handshake", "")
		return 0, 1, false
	}
	if err := s.markPrefork(); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		server.terminate()
		s.failPending("prefork_marker", "handshake", "")
		return 0, 1, false
	}
	client = protocolClient(&p, pipes, s.workspace, s.logWriter())
	unblockOnDeath(pipes, server.done, client.done)
	s.appendEventLine(fmt.Sprintf(`{"type":"%s-launched","server_pid":%d,"client_pid":%d,"client":"in-process","mode":"%s"}`,
		p.Kind, server.pid, client.pid, p.Mode))
	guard := guardSignals(s.d.Root, client.stop, func() {
		server.terminate()
		pipes.Remove()
	})
	defer guard.done()

	// The handshake loop: the session surfaces MID-TURN (critique F1), and
	// a server that dies before it bounds the missing-peer path (F2).
	for client.alive() {
		t.HandshakeDone, t.SessionID = s.handshakeDone, s.sessionID
		events, err := ops.Observe(t, Observation{Launch: launch, Running: true})
		if err != nil {
			s.logf("%v\n", err)
		}
		if events.Refusal != nil {
			s.failPending(events.Refusal.Error, events.Refusal.Phase, "")
			return 0, 1, false
		}
		if !s.handshakeDone && events.Session != "" {
			if !s.handshakeFrom(events) {
				return 0, 1, false
			}
		}
		if !s.handshakeDone && !server.alive() {
			client.terminate()
			s.failPending(p.Kind+"_server_died", "handshake", "")
			return 0, 1, false
		}
		if s.handshakeDone {
			break
		}
		touch(s.heartbeat)
		s.d.Clock.Sleep(50 * time.Millisecond)
	}
	status, err = s.waitForCLI(client)
	if err != nil {
		return 0, exitCodeOf(err, 1), false
	}
	if code := guard.signalled(); code != 0 {
		return 0, code, false
	}
	// The client has closed its ends; releasing the keepers gives the
	// server the stdin EOF it met when the client verb exited.
	pipes.Release()
	server.terminate()
	pipes.Remove()
	s.logf("%s client exit status=%d\n", p.Kind, status)
	return status, 0, true
}

// RunHostProtocol runs a protocol launch of a host turn: the server child in
// the checkout, the client in the foreground. exit, when set, ends the turn
// before any finish: 3 for a pair that could not be made, 1 when no nonce
// could be drawn, the signal status when the turn was signalled. The server
// is stopped as the host script stopped it: TERM and a reap.
func RunHostProtocol(d Deps, t *Turn, launch Launch) (status int, exit *int) {
	p := *launch.Protocol
	code := func(c int) *int { return &c }
	pipes, err := ACPPipeNames(t.Dir)
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 0, code(1)
	}
	if err := pipes.Make(); err != nil {
		fmt.Fprintln(t.Log, err)
		fmt.Fprintf(d.Stderr, "%s host: cannot create the %s fifo pair\n", t.Runtime, strings.ToUpper(p.Kind))
		return 0, code(3)
	}
	p.ServerOut, p.ServerIn = pipes.ServerOut, pipes.ServerIn
	// Wire plumbing is not evidence: the pair is removed on every exit.
	defer pipes.Remove()
	var server *exec.Cmd
	serverDone := make(chan struct{})
	command, err := protocolServerCommand(d, pipes, launch, t.Workspace, t.Env, t.Log)
	if err == nil {
		err = command.Start()
		pipes.Started()
	}
	if err != nil {
		// A server that cannot start is logged and the turn goes on: the
		// client meets a dead wire and its outcome says so.
		fmt.Fprintln(t.Log, err)
		close(serverDone)
	} else {
		server = command
		go func() {
			_ = server.Wait()
			close(serverDone)
		}()
	}
	stopServer := func() {
		if server != nil {
			_ = server.Process.Signal(syscall.SIGTERM)
			<-serverDone
			server = nil
		}
	}
	defer stopServer()
	client := protocolClient(&p, pipes, t.Workspace, t.Log)
	unblockOnDeath(pipes, serverDone, client.done)
	guard := guardSignals(d.Root, client.stop, pipes.Remove)
	defer guard.done()
	status = client.wait()
	if signalled := guard.signalled(); signalled != 0 {
		return 0, code(signalled)
	}
	pipes.Release()
	stopServer()
	pipes.Remove()
	return status, nil
}
