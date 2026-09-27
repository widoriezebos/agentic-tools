package hostturn

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/acp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter/supervisor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atif"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/host"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/jsonedit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	usagepkg "github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

// The Devin host turn: the port of scripts/agents/hosts/devin.sh. The
// transport selector mirrors the delegate adapter's on the SAME
// configuration key (D81/D82), so the dispatch flip extends to host turns.

func init() {
	register("devin", hostRuntime{cli: "devin", run: runDevin})
}

// devinHostTurn is the turn's Devin paths.
type devinHostTurn struct {
	t                                              *Turn
	turnRecord, raw, returnPath, transcript        string
	cumulative, configFile, usagePath, log, schema string
	permissions, devinPrompt, returnFile           string
	model                                          string
}

func runDevin(t *Turn) int {
	if !t.requireCLI("devin") {
		return 3
	}
	// An ABSENT key still resolves legacy; an unreadable configuration or an
	// unrecognized value REFUSES — a broken config must never fail open into
	// the dangerous path.
	transport, err := t.d.ConfigValue("dispatch.transport.devin", "legacy")
	if err != nil {
		fmt.Fprintln(t.d.Stderr, "devin host: transport configuration unreadable")
		return 3
	}
	if transport != "legacy" && transport != "acp" {
		fmt.Fprintf(t.d.Stderr, "devin host: transport configuration invalid: %s\n", transport)
		return 3
	}
	h := &devinHostTurn{
		t:           t,
		turnRecord:  t.Path("turn.json"),
		raw:         t.Path("raw.out"),
		returnPath:  t.Path("return.json"),
		transcript:  t.Path("transcript.atif.json"),
		cumulative:  t.Path("session-usage.json"),
		configFile:  t.Path("devin-config.json"),
		usagePath:   t.Path("usage.json"),
		log:         t.Path("host.log"),
		schema:      t.schema(),
		permissions: t.permissions(),
		devinPrompt: t.Path("prompt.devin.md"),
		returnFile:  t.Path("devin-return.json"),
	}
	model, ok := t.turnModel()
	if !ok {
		fmt.Fprintf(t.d.Stderr, "cannot read the model of %s\n", h.turnRecord)
		return 1
	}
	h.model = model
	// The host edits AND executes in the repository it is advancing; on
	// this CLI a working non-interactive host requires the dangerous mode
	// (the bm-2d evidence), and the boundary is NOT enforced on this
	// runtime — the capability snapshot declares it (D83 confines devin
	// hosts to the VM for that reason).
	if err := host.DevinConfig(t.d.Root, h.configFile); err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	// The named return path (D64 phase 2): this model delivers by writing
	// files; the prompt names the exact path and the walk below reads it.
	if _, err := os.Lstat(h.returnFile); err == nil {
		fmt.Fprintf(t.d.Stderr, "stale named return file from a crashed earlier attempt: %s\n", h.returnFile)
		return 3
	}
	// This runtime has no schema flag, so the schema goes in the prompt;
	// the runner's prompt file is left untouched as evidence.
	if err := adapter.DevinPrompt(t.Prompt, h.schema, h.devinPrompt, h.returnFile); err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	if transport == "acp" {
		return h.runACP()
	}
	return h.runLegacy()
}

func (h *devinHostTurn) logWriter() (*os.File, error) {
	return os.OpenFile(h.log, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
}

func (h *devinHostTurn) logf(format string, args ...any) {
	if file, err := h.logWriter(); err == nil {
		fmt.Fprintf(file, format, args...)
		file.Close()
	}
}

// signalHooks holds, per installation root, a test's delivery of a
// termination signal to that root's host turn; production roots take the
// real signals.
var signalHooks sync.Map // root -> func(chan<- os.Signal)

func notifyTermination(root string, c chan<- os.Signal) (stop func()) {
	if hook, ok := signalHooks.Load(root); ok {
		hook.(func(chan<- os.Signal))(c)
		return func() {}
	}
	signal.Notify(c, syscall.SIGTERM, os.Interrupt)
	return func() { signal.Stop(c) }
}

const hostACPSignalBound = 3 * time.Second

// runACP is the ACP host turn: the same wire the delegate rides, without
// the dispatch record machinery — the host's evidence is its turn
// directory and the result envelope's transport pin. The session mode comes
// from the host permission envelope's tools grade, so the session carries
// the v1 grades rather than the dangerous mode. The client runs in process
// (acp.RunFileTurn); a TERM or INT cancels it so the courtesy session/cancel
// still reaches the wire, and the turn then ends with the shell's signal
// status (143 or 130) without a result, as the signalled script did.
func (h *devinHostTurn) runACP() int {
	t := h.t
	outcomeFile := t.Path("acp-outcome.json")
	journalFile := t.Path("acp-journal.log")
	sessionFile := t.Path("acp-session-id")
	envelope, err := acp.LoadEnvelopeFile(h.permissions)
	refused := err != nil
	if err != nil {
		h.logf("%v\n", err)
	} else if reason := acp.PreflightACP(envelope); reason != "" {
		h.logf("%s\n", reason)
		refused = true
	}
	if refused {
		fmt.Fprintln(t.d.Stderr, "devin host: ACP preflight refused the permission envelope")
		return 3
	}
	grade := ""
	if data, err := os.ReadFile(h.permissions); err == nil {
		fallback := ""
		grade, _ = jsonedit.Get(data, "tools", &fallback)
	}
	mode, err := supervisor.DevinACPMode(grade)
	if err != nil {
		h.logf("%v\n", err)
		fmt.Fprintf(t.d.Stderr, "devin host: no ACP session mode maps to tools grade '%s'\n", grade)
		return 3
	}
	declaration, found := runtimes.Lookup("devin")
	if !found || declaration.ExpectedACP == nil {
		fmt.Fprintln(t.d.Stderr, "devin host: the runtime registry declares no ACP protocol expectation")
		return 3
	}
	pipes, err := supervisor.ACPPipeNames(t.TurnDir)
	if err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	if err := pipes.Make(); err != nil {
		h.logf("%v\n", err)
		fmt.Fprintln(t.d.Stderr, "devin host: cannot create the ACP fifo pair")
		return 3
	}
	// Wire plumbing is not evidence: the pair is removed on every exit so a
	// later evidence-tree copy never meets a named pipe (KI-42).
	defer pipes.Remove()

	log, err := h.logWriter()
	if err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	defer log.Close()
	// argv0 devin-host-acp: the census signature distinguishes the HOST's
	// server child from both the raw CLI helper and the delegate-side
	// server.
	server := h.startACPServer(pipes, log)
	stopServer := func() {
		if server != nil {
			_ = server.Process.Signal(syscall.SIGTERM)
			_ = server.Wait()
			server = nil
		}
	}
	defer stopServer()

	turn := acp.FileTurnConfig{
		ServerOut: pipes.ServerOut, ServerIn: pipes.ServerIn,
		JournalPath: journalFile, Workspace: t.d.Root,
		EnvelopePath: h.permissions, PromptFile: h.devinPrompt,
		Mode: mode, ExpectedProtocol: declaration.ExpectedACP.ExpectedProtocolVersion,
		SessionFile: sessionFile, LoadSession: t.ResumeSession,
	}
	if err := os.WriteFile(outcomeFile, nil, 0o644); err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var signalled atomic.Int32
	finished := make(chan struct{})
	defer close(finished)
	signals := make(chan os.Signal, 1)
	stopSignals := notifyTermination(t.d.Root, signals)
	defer stopSignals()
	go func() {
		select {
		case sig := <-signals:
			code := int32(143)
			if sig == os.Interrupt {
				code = 130
			}
			signalled.Store(code)
			cancel()
			select {
			case <-finished:
				return
			case <-time.After(hostACPSignalBound):
			}
			pipes.Remove()
			os.Exit(int(code))
		case <-finished:
		}
	}()
	cliStatus := runHostACPClient(ctx, turn, outcomeFile, log)
	if code := signalled.Load(); code != 0 {
		return int(code)
	}
	// The client has closed its ends; releasing the keepers gives the
	// server the stdin EOF it met when the client verb exited. The server
	// is then stopped as the script stopped it: TERM and a reap.
	pipes.Release()
	stopServer()
	pipes.Remove()
	ensureFile(h.raw)

	accepted := ""
	if info, err := os.Stat(outcomeFile); cliStatus == 0 && err == nil && info.Size() > 0 {
		data, _ := os.ReadFile(outcomeFile)
		fallback := ""
		row, ok := jsonedit.Get(data, "row", &fallback)
		if !ok {
			return 1
		}
		if row == "delivered" {
			// The wire candidate is the reply; the raw capture doubles as
			// the accepted snapshot path finish and the extractor read.
			candidate, ok := jsonedit.Get(data, "candidate", &fallback)
			if !ok {
				return 1
			}
			if err := os.WriteFile(h.raw, []byte(candidate+"\n"), 0o644); err != nil {
				fmt.Fprintln(t.d.Stderr, err)
				return 1
			}
			accepted = h.raw
			if !h.devinReturn(h.raw) {
				return 1
			}
		}
	}
	if err := usagepkg.ACPUsage(h.usagePath, outcomeFile); err != nil {
		fmt.Fprintln(log, err)
	}
	session := ""
	if info, err := os.Stat(sessionFile); err == nil && info.Size() > 0 {
		session = supervisor.FirstLine(sessionFile)
	}
	return t.finish(session, h.usagePath, h.raw, h.returnPath, accepted, cliStatus, true, "acp")
}

// startACPServer launches `devin acp` in the checkout with argv0
// devin-host-acp. A server that cannot start is logged and the turn goes
// on: the client then meets a dead wire and its outcome says so.
func (h *devinHostTurn) startACPServer(pipes *supervisor.ACPPipes, log io.Writer) *exec.Cmd {
	t := h.t
	path, err := t.d.LookPath("devin")
	if err != nil {
		fmt.Fprintln(log, err)
		return nil
	}
	command, err := pipes.ServerCommand(path, "devin-host-acp", t.d.Root, t.d.Environ, log)
	if err != nil {
		fmt.Fprintln(log, err)
		return nil
	}
	err = command.Start()
	pipes.Started()
	if err != nil {
		fmt.Fprintln(log, err)
		return nil
	}
	return command
}

// runHostACPClient is the in-process prompt attempt; its status is the
// former `acp turn` verb's: 0 with an outcome, 1 when no attempt ran.
func runHostACPClient(ctx context.Context, turn acp.FileTurnConfig, outcomeFile string, log io.Writer) int {
	outcome, err := acp.RunFileTurn(ctx, turn)
	if err != nil {
		fmt.Fprintln(log, err)
		return 1
	}
	if err := acp.WriteFileTurnOutcome(outcomeFile, outcome); err != nil {
		fmt.Fprintln(log, err)
		return 1
	}
	return 0
}

func (h *devinHostTurn) ports() (delegate.Ports, bool) {
	ports, err := delegate.PortsFor("devin")
	if err != nil {
		fmt.Fprintln(h.t.d.Stderr, err)
		return delegate.Ports{}, false
	}
	return ports, true
}

// devinReturn extracts the return object from a reply (the former `host
// devin-return`).
func (h *devinHostTurn) devinReturn(raw string) bool {
	ports, ok := h.ports()
	if !ok || ports.HostReturn == nil {
		return false
	}
	if err := ports.HostReturn(raw, h.returnPath); err != nil {
		fmt.Fprintln(h.t.d.Stderr, err)
		return false
	}
	return true
}

// collect walks the host turn's delivery channels (the former `host
// devin-collect`): 0 delivered, 3 nothing qualified, 5 transcript over the
// ceiling, 1 mechanical.
func (h *devinHostTurn) collect() ([]byte, int) {
	ports, ok := h.ports()
	if !ok || ports.HostCollect == nil {
		return nil, 1
	}
	encoded, delivered, err := ports.HostCollect(delegate.HostCollectInputs{
		Root: h.t.d.Root, TurnRecordPath: h.turnRecord, TurnDir: h.t.TurnDir,
		Workspace: h.t.d.Root, StdoutPath: h.raw, NamedPath: h.returnFile,
		TranscriptPath: h.transcript,
	})
	if err != nil {
		fmt.Fprintln(h.t.d.Stderr, err)
		if errors.Is(err, atif.ErrOversize) {
			return nil, 5
		}
		return nil, 1
	}
	if delivered {
		return encoded, 0
	}
	return encoded, 3
}

// runLegacy is the legacy host turn: one `devin -p` turn in the checkout,
// stdout the raw capture, stderr the (truncated) host log.
func (h *devinHostTurn) runLegacy() int {
	t := h.t
	command := []string{"devin", "-p",
		"--prompt-file", h.devinPrompt,
		"--respect-workspace-trust", "false",
		"--model", h.model,
		"--permission-mode", "dangerous",
		"--config", h.configFile,
		"--export", h.transcript,
	}
	if t.ResumeSession != "" {
		command = append(command, "-r", t.ResumeSession)
	}
	cliStatus := h.runCLI(command)
	ensureFile(h.raw)

	// The delivery walk (D64 phase 2): stdout, the named file, then the
	// transcript's designated writes — the engine decides. With nothing
	// delivered the old raw path stands and the runner's validation reports
	// the absence.
	accepted := ""
	verdict, rc := h.collect()
	if rc == 0 {
		reply, ok := jsonedit.Get(verdict, "reply", nil)
		if !ok {
			return 1
		}
		accepted = reply
		if !h.devinReturn(accepted) {
			return 1
		}
	} else if !h.devinReturn(h.raw) {
		return 1
	}

	// final_metrics is CUMULATIVE for a session, and consumers ADD turn
	// records, so each turn publishes the delta against its predecessor's
	// stored totals. The predecessor is keyed by SESSION in a per-session
	// store in the turns' parent, so it survives across a mission's turns
	// and the delta is deterministic.
	parent, err := filepath.EvalSymlinks(filepath.Join(t.TurnDir, ".."))
	if err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	store := filepath.Join(parent, ".session-usage")
	if err := os.MkdirAll(store, 0o755); err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	previous := ""
	if t.ResumeSession != "" {
		candidate := filepath.Join(store, lease.Slug(t.ResumeSession)+".json")
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			previous = candidate
		}
	}
	ports, ok := h.ports()
	if !ok || ports.HostTurnUsage == nil {
		return 1
	}
	if err := ports.HostTurnUsage(h.usagePath, h.transcript, h.cumulative, previous, t.ResumeSession != ""); err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	session := ""
	if data, err := os.ReadFile(h.transcript); err == nil {
		fallback := ""
		session, _ = jsonedit.Get(data, "session_id", &fallback)
	}
	code := t.finish(session, h.usagePath, h.raw, h.returnPath, accepted, cliStatus, true, "")
	// Publish this turn's cumulative totals into the per-session store so
	// the next turn of THIS session subtracts the right predecessor; written
	// only on a completion.
	if code == 0 {
		if data, err := os.ReadFile(h.cumulative); err == nil && len(data) > 0 {
			_ = os.WriteFile(filepath.Join(store, lease.Slug(session)+".json"), data, 0o644)
		}
	}
	return code
}

// runCLI runs the Devin CLI in the checkout: stdout into the raw capture
// (truncated), stderr into the host log (truncated).
func (h *devinHostTurn) runCLI(argv []string) int {
	t := h.t
	stdout, err := os.OpenFile(h.raw, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	defer stdout.Close()
	log, err := os.OpenFile(h.log, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	defer log.Close()
	path, err := t.d.LookPath(argv[0])
	if err != nil {
		fmt.Fprintf(log, "%s: command not found\n", argv[0])
		return 127
	}
	command := exec.Command(path, argv[1:]...)
	command.Args[0] = argv[0]
	command.Dir = t.d.Root
	command.Env = t.d.Environ
	command.Stdout = stdout
	command.Stderr = log
	return shellStatus(command.Run())
}
