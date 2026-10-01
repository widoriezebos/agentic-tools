package supervisor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/jsonedit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

func init() {
	ops := claudeOps{builtinOps{name: "claude", cli: "claude", usage: "native", host: true,
		configIdentity: claudeConfigIdentity, probe: claudeProbe, contract: commonContract("claude"),
		selftest: commonSelftest("claude", "native", "", nil)}}
	register(runtimeAdapter{
		name:           "claude",
		configIdentity: claudeConfigIdentity,
		probe:          claudeProbe,
		contract:       commonContract("claude"),
		outputStream: func(_ Deps, roundDir string) (string, error) {
			return roundDir + "/claude-stream.jsonl", nil
		},
		selftest: commonSelftest("claude", "native", "", nil),
		ops:      ops,
	})
}

// commonContract is the contract emission of a runtime with a static
// envelope-enforcement map.
func commonContract(runtime string) func(Deps) ([]byte, error) {
	return func(Deps) ([]byte, error) {
		enforcement, ok := runtimes.EnforcementMapJSON(runtime)
		if !ok {
			return nil, fmt.Errorf("%s declares no envelope-enforcement map", runtime)
		}
		return contractSnapshot(runtime, enforcement)
	}
}

// claudeConfigIdentity is the declared configuration identity: the user,
// project, project-local, and host-managed settings sources Claude merges
// for a session launched here.
func claudeConfigIdentity(d Deps) (string, error) {
	version, err := d.cliVersion("claude")
	if err != nil {
		return "", err
	}
	configDir := d.Getenv("CLAUDE_CONFIG_DIR")
	if configDir == "" {
		home := d.Getenv("HOME")
		if home == "" {
			return "", fmt.Errorf("HOME is not set")
		}
		configDir = filepath.Join(home, ".claude")
	}
	project, err := d.projectRoot()
	if err != nil {
		return "", err
	}
	return d.configIdentity("claude", version, []string{
		filepath.Join(configDir, "settings.json"),
		filepath.Join(project, ".claude", "settings.json"),
		filepath.Join(project, ".claude", "settings.local.json"),
		"/Library/Application Support/ClaudeCode/managed-settings.json",
		"/etc/claude-code/managed-settings.json",
	})
}

// cliAuthenticated runs the runtime CLI's authentication status check.
func (d Deps) cliAuthenticated(cli string, args ...string) bool {
	path, err := d.LookPath(cli)
	if err != nil {
		return false
	}
	command := exec.Command(path, args...)
	command.Env = d.Environ
	return command.Run() == nil
}

// probeCommon writes a probed runtime's capability snapshot from its
// configuration identity, after its authentication check.
func probeCommon(d Deps, runtime string, identity func(Deps) (string, error), authenticated func() bool, authFailure, transports, capabilities, permissions string) int {
	details, err := identity(d)
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	version, hash, keyHashes, err := identityFields(details)
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	if !authenticated() {
		fmt.Fprintln(d.Stderr, authFailure)
		return 1
	}
	enforcement, _ := runtimes.EnforcementMapJSON(runtime)
	if err := d.writeCapabilitySnapshot(runtime, version, hash, transports, capabilities, permissions, enforcement, keyHashes); err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	return 0
}

func claudeProbe(d Deps, args []string) int {
	if len(args) != 0 {
		registry["claude"].usage(d)
		return 2
	}
	return probeCommon(d, "claude", claudeConfigIdentity,
		func() bool { return d.cliAuthenticated("claude", "auth", "status") },
		"claude authentication is unavailable; run claude auth login",
		`["stdin","file","json","stream-json"]`,
		`{"resume":true,"sessionEstablishedSignal":true,"sessionEstablishedTimeoutSec":20,"nativeStructuredOutput":true,"nativeEvents":true,"nativeUsage":true,"gracefulCancel":true,"hooks":true,"protocolServer":true,"nativeBudget":true}`,
		`{"unverified": []}`)
}

// ClaudeCommand builds the Claude argv: the argv, the envelope's mode/tool
// mapping, and the budget policy are the engine's. A failure names the
// protocol error the round fails pending with.
func ClaudeCommand(d Deps, record, model, schemaPath, settings, session, outputMode string, log io.Writer) ([]string, string) {
	budget, turns, err := adapter.ClaudeBudget(func(name string) (string, bool) {
		for _, entry := range d.Environ {
			if found, value, ok := strings.Cut(entry, "="); ok && found == name {
				return value, true
			}
		}
		return "", false
	})
	if err != nil {
		fmt.Fprintln(log, err)
		if errors.Is(err, adapter.ErrInvalidNativeTurnLimit) {
			return nil, "invalid_native_turn_limit"
		}
		return nil, "invalid_native_budget"
	}
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		fmt.Fprintln(log, err)
		return nil, "runtime_error"
	}
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, schemaBytes); err != nil {
		fmt.Fprintf(log, "schema is not valid JSON: %v\n", err)
		return nil, "runtime_error"
	}
	command, err := adapter.BuildClaudeCommand(record, model, compacted.String(), settings, session, budget, turns, outputMode)
	if err != nil {
		fmt.Fprintln(log, err)
		return nil, "runtime_error"
	}
	if len(command) == 0 {
		return nil, "runtime_error"
	}
	return command, ""
}

// claudeOps is the Claude runtime's operations.
type claudeOps struct{ builtinOps }

// claudeLaunch is Claude's per-turn state.
type claudeLaunch struct {
	signalFile, streamFile, resultFile string
}

func (claudeOps) Prepare(t *Turn) (Launch, error) {
	d := t.Deps()
	if t.Role == RoleHost {
		model, ok := field(t.Record, "model")
		if !ok {
			return Launch{}, fmt.Errorf("claude host turn record has no model")
		}
		// Host mode is the record-less claude-command call: acceptEdits
		// with the full tools, blocking json.
		log, err := os.OpenFile(filepath.Join(t.Dir, "host.log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
		if err != nil {
			return Launch{}, err
		}
		command, failure := ClaudeCommand(d, "", model, t.Schema, "", t.ResumeSession, "json", log)
		log.Close()
		if failure != "" {
			return Launch{Refusal: &Refusal{Error: "claude host native budget configuration is invalid"}}, nil
		}
		return Launch{Argv: command, StdinPath: t.Prompt, StdoutPath: filepath.Join(t.Dir, "claude-result.json"),
			TruncateLog: true, Private: claudeLaunch{resultFile: filepath.Join(t.Dir, "claude-result.json")}}, nil
	}
	caches := d.delegateCaches()
	recordBuildCachePath(d.git(), d.agents(), t.Workspace, t.Dir, caches)
	private := claudeLaunch{
		signalFile: filepath.Join(t.Dir, "claude-session-signal.json"),
		streamFile: filepath.Join(t.Dir, "claude-stream.jsonl"),
		resultFile: filepath.Join(t.Dir, "claude-result.json"),
	}
	settingsFile := filepath.Join(t.Dir, "claude-settings.json")
	tmp := d.Getenv("TMPDIR")
	if tmp == "" {
		tmp = "/tmp"
	}
	scratch := filepath.Join(tmp, "metasystem-claude", t.Job+"-"+t.Round)
	// The working directory is the write boundary: this runtime's choice of
	// effective envelope.
	if err := adapter.RewriteWriteScope(t.Effective, t.Workspace); err != nil {
		return Launch{}, err
	}
	if err := truncate(t.Events); err != nil {
		return Launch{}, err
	}
	// The emitted SessionStart hook runs the engine's session-signal
	// entry, which signals session establishment back to this supervisor.
	sandboxCaches := adapter.SandboxCaches{Grant: caches.Directories(), Deny: d.engineCaches().Directories()}
	if err := adapter.BuildClaudeSettings(t.Record, settingsFile, d.Engine, scratch, sandboxCaches); err != nil {
		return Launch{}, err
	}
	// The dispatch turn streams: the probe's nativeEvents declaration
	// matches the argv.
	session := ""
	if t.Verb != runtimes.SupervisorDispatch {
		session = t.ResumeSession
	}
	command, failure := ClaudeCommand(d, t.Record, t.Model, t.Schema, settingsFile, session, "stream-json", t.Log)
	if failure != "" {
		return Launch{Refusal: &Refusal{Error: failure, Phase: "handshake"}}, nil
	}
	// Claude's sandbox cannot write the system temporary directory itself;
	// the settings allow this private scratch directory for TMPDIR and
	// GOTMPDIR, and the machine delegate cache for GOCACHE and
	// STATICCHECK_CACHE (disk-lifetimes A7): one cache for every round and
	// chain, so follow-ups start warm, and a nested engine inherits the
	// absolute pair and never resolves the engine cache the sandbox denies.
	// A job worktree's GOTMPDIR moves into its git dir.
	env := []string{
		"TMPDIR=" + scratch,
		"GOTMPDIR=" + filepath.Join(scratch, "go-tmp"),
	}
	env = withEnv(env, jobBuildCacheEnv(d.git(), d.agents(), t.Workspace, caches)...)
	env = withEnv(env,
		"METASYSTEM_CLAUDE_SESSION_SIGNAL="+private.signalFile,
		"METASYSTEM_CLAUDE_EVENTS="+t.Events)
	env = withEnv(env, jobGitQuarantineEnv(d.git(), t.Workspace)...)
	setup := os.MkdirAll(filepath.Join(scratch, "go-tmp"), 0o755)
	if setup == nil && (envValue(env, "GOCACHE") == "" || envValue(env, "STATICCHECK_CACHE") == "") {
		setup = errors.New("the delegate build cache did not resolve under the user cache directory")
	}
	return Launch{Argv: command, Env: env, StdinPath: t.Prompt, StdoutPath: private.streamFile,
		SetupError: setup, Private: private}, nil
}

// Observe reads the SessionStart hook's session signal.
func (claudeOps) Observe(t *Turn, o Observation) (Events, error) {
	private := o.Launch.Private.(claudeLaunch)
	if info, err := os.Stat(private.signalFile); err != nil || info.Size() == 0 {
		return Events{}, nil
	}
	session, _ := field(private.signalFile, "session_id")
	model, ok := field(private.signalFile, "model")
	if !ok || model == "" || model == "null" {
		model = t.Model
	}
	return Events{Session: session, Model: model}, nil
}

func (claudeOps) Finalize(t *Turn, in FinalInput) (Final, error) {
	private := in.Launch.Private.(claudeLaunch)
	ports, err := delegate.PortsFor("claude")
	if err != nil || ports.Usage == nil || ports.ResultField == nil {
		return Final{}, fmt.Errorf("claude ports are not registered")
	}
	if t.Role == RoleHost {
		if ports.HostResult == nil {
			return Final{}, fmt.Errorf("claude host ports are not registered")
		}
		raw, returnPath, usagePath := filepath.Join(t.Dir, "raw.out"), filepath.Join(t.Dir, "return.json"), filepath.Join(t.Dir, "usage.json")
		copyOrEmptyFile(private.resultFile, raw)
		if err := ports.HostResult(private.resultFile, returnPath, usagePath); err != nil {
			return Final{}, err
		}
		session := ""
		if data, err := os.ReadFile(private.resultFile); err == nil {
			fallback := ""
			session, _ = jsonedit.Get(data, "session_id", &fallback)
		}
		return Final{HostSession: session, HostRaw: raw, HostReturn: returnPath, Usage: usagePath}, nil
	}
	// Derive the blocking-shaped result document from the stream; a stream
	// with no result-typed line takes the missing-result path.
	if err := adapter.ClaudeDeriveResult(private.streamFile, private.resultFile); err != nil {
		fmt.Fprintln(t.Log, err)
		_ = truncate(private.resultFile)
	}
	copyOrEmptyFile(private.resultFile, filepath.Join(t.Dir, "raw.out"))
	if err := adapter.ClaudeAppendResult(private.resultFile, t.Events); err != nil {
		return Final{}, err
	}
	if err := ports.Usage(private.resultFile, in.Usage); err != nil {
		return Final{}, err
	}
	resultSession := claudeResultValue(ports, private.resultFile, "session_id")
	resultModel := claudeResultValue(ports, private.resultFile, "model")
	return Final{Candidate: private.resultFile, Session: resultSession, HandshakeModel: resultModel, ResultModel: resultModel}, nil
}

// copyOrEmptyFile copies src over dst, leaving dst empty when src is
// unreadable.
func copyOrEmptyFile(src, dst string) {
	data, err := os.ReadFile(src)
	if err != nil {
		data = nil
	}
	_ = os.WriteFile(dst, data, 0o644)
}

func claudeResultValue(ports delegate.Ports, resultFile, name string) string {
	value, print, err := ports.ResultField(resultFile, name)
	if err != nil || !print {
		return ""
	}
	return value
}

func envValue(env []string, name string) string {
	value := ""
	for _, entry := range env {
		if found, current, ok := strings.Cut(entry, "="); ok && found == name {
			value = current
		}
	}
	return value
}
