package supervisor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

func init() {
	register(runtimeAdapter{
		name:           "claude",
		configIdentity: claudeConfigIdentity,
		probe:          claudeProbe,
		contract:       commonContract("claude"),
		outputStream: func(_ Deps, roundDir string) (string, error) {
			return roundDir + "/claude-stream.jsonl", nil
		},
		supervise: superviseClaude,
		selftest:  commonSelftest("claude", "native", "", nil),
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
		if err.Error() == "invalid_native_turn_limit" {
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

func superviseClaude(s *Supervision, args []string) int {
	if !s.prepareOrUsage(args) {
		return 2
	}
	recordBuildCachePath(s.d.agents(), s.workspace, s.roundDir)
	settingsFile := filepath.Join(s.roundDir, "claude-settings.json")
	signalFile := filepath.Join(s.roundDir, "claude-session-signal.json")
	resultFile := filepath.Join(s.roundDir, "claude-result.json")
	usageFile := filepath.Join(s.roundDir, "usage.json")
	tmp := s.d.Getenv("TMPDIR")
	if tmp == "" {
		tmp = "/tmp"
	}
	scratch := filepath.Join(tmp, "metasystem-claude", s.job+"-"+s.round)

	if err := s.recordWorkspaceWriteScope(); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	if !s.failIfEffectiveWider() {
		return 1
	}
	if err := truncate(s.events); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	// The emitted SessionStart hook runs the engine's session-signal
	// entry, which signals session establishment back to this supervisor.
	if err := adapter.BuildClaudeSettings(s.record, settingsFile, s.d.Engine, scratch); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	// The dispatch turn streams: the probe's nativeEvents declaration
	// matches the argv. The host turn keeps blocking json.
	session := ""
	if s.verb != runtimes.SupervisorDispatch {
		session = s.requestedSession
	}
	command, failure := ClaudeCommand(s.d, s.record, s.requestedModel, s.schema, settingsFile, session, "stream-json", s.logWriter())
	if failure != "" {
		s.failPending(failure, "handshake", "")
		return 1
	}
	if !s.verifyReferences() {
		return 1
	}
	if err := s.markPrefork(); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		s.failPending("prefork_marker", "handshake", "")
		return 1
	}
	// Claude's sandbox cannot write the user's caches or the system
	// temporary directory itself; the settings allow only this private
	// scratch directory. The chain's cache overrides the per-round one
	// when the job runs in a worktree, so follow-up rounds start warm.
	env := withEnv(s.childEnv,
		"TMPDIR="+scratch,
		"GOCACHE="+filepath.Join(scratch, "go-cache"),
		"GOTMPDIR="+filepath.Join(scratch, "go-tmp"))
	env = withEnv(env, jobBuildCacheEnv(s.d.agents(), s.workspace)...)
	env = withEnv(env,
		"METASYSTEM_CLAUDE_SESSION_SIGNAL="+signalFile,
		"METASYSTEM_CLAUDE_EVENTS="+s.events)
	env = withEnv(env, jobGitQuarantineEnv(s.workspace)...)
	launchErr := os.MkdirAll(filepath.Join(scratch, "go-tmp"), 0o755)
	if launchErr == nil {
		launchErr = os.MkdirAll(envValue(env, "GOCACHE"), 0o755)
	}
	cli, err := s.launch(command, env, s.prompt, filepath.Join(s.roundDir, "claude-stream.jsonl"), launchErr)
	if err != nil {
		s.failPending("custody_registration", "handshake", "")
		return 1
	}
	if err := s.registerCustody(cli); err != nil {
		cli.terminate()
		s.failPending("custody_registration", "handshake", "")
		return 1
	}
	for cli.alive() {
		if info, statErr := os.Stat(signalFile); statErr == nil && info.Size() > 0 {
			signalled, _ := field(signalFile, "session_id")
			model, ok := field(signalFile, "model")
			if !ok || model == "" || model == "null" {
				model = s.requestedModel
			}
			if !s.recordHandshake(signalled, "", model) {
				cli.terminate()
				return 1
			}
			break
		}
		touch(s.heartbeat)
		s.d.Clock.Sleep(pollTick)
	}
	status, err := s.waitForCLI(cli)
	if err != nil {
		return exitCodeOf(err, 1)
	}
	// Derive the blocking-shaped result document from the stream; a stream
	// with no result-typed line takes the missing-result path (an empty
	// result file downstream, exactly the old empty-stdout shape).
	if err := adapter.ClaudeDeriveResult(filepath.Join(s.roundDir, "claude-stream.jsonl"), resultFile); err != nil {
		s.logf("%v\n", err)
		_ = truncate(resultFile)
	}
	if data, err := os.ReadFile(resultFile); err == nil {
		_ = os.WriteFile(s.raw, data, 0o644)
	} else {
		_ = truncate(s.raw)
	}
	if err := adapter.ClaudeAppendResult(resultFile, s.events); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	ports, err := delegate.PortsFor("claude")
	if err != nil || ports.Usage == nil || ports.ResultField == nil {
		fmt.Fprintln(s.d.Stderr, "claude delegate ports are not registered")
		return 1
	}
	if err := ports.Usage(resultFile, usageFile); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	resultSession := claudeResultValue(ports, resultFile, "session_id")
	resultModel := claudeResultValue(ports, resultFile, "model")
	if !s.settleResultIdentity(resultSession, "", resultModel, resultModel, usageFile) {
		return 1
	}
	return terminal(s.completeFromCLI(status, usageFile, resultFile, "", repairHooks{}))
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
