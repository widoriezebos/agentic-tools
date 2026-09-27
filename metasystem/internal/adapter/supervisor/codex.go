package supervisor

import (
	"fmt"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
)

func init() {
	register(runtimeAdapter{
		name:           "codex",
		configIdentity: codexConfigIdentity,
		probe:          codexProbe,
		contract:       commonContract("codex"),
		outputStream: func(_ Deps, roundDir string) (string, error) {
			return roundDir + "/events.jsonl", nil
		},
		supervise: superviseCodex,
		selftest:  codexSelftest,
	})
}

func codexConfigIdentity(d Deps) (string, error) {
	version, err := d.cliVersion("codex")
	if err != nil {
		return "", err
	}
	configDir := d.Getenv("CODEX_HOME")
	if configDir == "" {
		home := d.Getenv("HOME")
		if home == "" {
			return "", fmt.Errorf("HOME is not set")
		}
		configDir = filepath.Join(home, ".codex")
	}
	project, err := d.projectRoot()
	if err != nil {
		return "", err
	}
	return d.configIdentity("codex", version, []string{
		filepath.Join(configDir, "config.toml"),
		filepath.Join(project, ".codex", "config.toml"),
		"/etc/codex/config.toml",
	})
}

// codexProbe: the in-process app server starts before `thread.started` is
// emitted, so a cold Codex launch needs a wider session-establishment
// window.
func codexProbe(d Deps, args []string) int {
	if len(args) != 0 {
		registry["codex"].usage(d)
		return 2
	}
	return probeCommon(d, "codex", codexConfigIdentity,
		func() bool { return d.cliAuthenticated("codex", "login", "status") },
		"codex authentication is unavailable; run codex login",
		`["stdin","jsonl","file"]`,
		`{"resume":true,"sessionEstablishedSignal":true,"sessionEstablishedTimeoutSec":30,"nativeStructuredOutput":true,"nativeEvents":true,"nativeUsage":true,"gracefulCancel":true,"hooks":true,"protocolServer":true,"nativeBudget":false}`,
		`{"unverified": []}`)
}

// CodexCommand builds the Codex argv. The envelope-to-sandbox/network
// mapping is the engine's: it derives from the job record (a delegate round)
// or a permission envelope file (a host turn), never from values handed in.
// `codex exec resume` has no --sandbox or -C flags: a resumed thread
// inherits its cwd and config and takes per-turn overrides through -c only.
func CodexCommand(verb, model, workspace, schema, output, instanceTag, reasoningEffort, permissionsPath, recordPath, session string) ([]string, error) {
	sandbox, network, err := adapter.CodexPermissionSettings(permissionsPath, recordPath)
	if err != nil {
		return nil, err
	}
	// Write roots outside the workspace — the worktree's git metadata
	// (issue #5) — must reach the sandbox explicitly.
	extraDirs, err := adapter.CodexExtraWriteRoots(permissionsPath, recordPath, workspace)
	if err != nil {
		return nil, err
	}
	if verb == "" || model == "" || schema == "" || output == "" || sandbox == "" || network == "" || instanceTag == "" {
		return nil, fmt.Errorf("codex command requires a verb, model, schema, output, envelope, and instance tag")
	}
	command, err := adapter.BuildCodexCommand(verb, model, workspace, schema, output, sandbox, network, session, instanceTag, reasoningEffort, extraDirs)
	if err != nil {
		return nil, err
	}
	if len(command) == 0 {
		return nil, fmt.Errorf("codex command assembly produced no argv")
	}
	return command, nil
}

func superviseCodex(s *Supervision, args []string) int {
	if !s.prepareOrUsage(args) {
		return 2
	}
	recordBuildCachePath(s.d.git(), s.d.agents(), s.workspace, s.roundDir)
	usageFile := filepath.Join(s.roundDir, "usage.json")
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
	if err := truncate(s.raw); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	effort, ok := field(s.record, "reasoningEffort")
	if !ok || effort == "null" {
		effort = ""
	}
	// The envelope decides sandbox and network — in the engine, from the
	// record itself (KI-12: a hard-coded value made the recorded field
	// decorative).
	command, err := CodexCommand(s.verb, s.requestedModel, s.workspace, s.schema, s.raw, s.tag, effort, "", s.record, s.requestedSession)
	if err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 2
	}
	if !s.verifyReferences() {
		return 1
	}
	if err := s.markPrefork(); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		s.failPending("prefork_marker", "handshake", "")
		return 1
	}
	// The write boundary is the CLI's cwd: `codex exec resume` has no -C,
	// so entering the workspace makes the recorded boundary true on both
	// paths. The chain's build cache: the sandbox cannot write the user's
	// Go cache.
	env := withEnv(s.childEnv, jobGitQuarantineEnv(s.d.git(), s.workspace)...)
	env = withEnv(env, jobBuildCacheEnv(s.d.git(), s.d.agents(), s.workspace)...)
	cli, err := s.launch(command, env, s.prompt, s.events, nil)
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
		if session, found := adapter.CodexEventField(s.events, "session"); found && session != "" {
			turn, _ := adapter.CodexEventField(s.events, "turn")
			if !s.recordHandshake(session, turn, s.requestedModel) {
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
	ports, err := delegate.PortsFor("codex")
	if err != nil || ports.Usage == nil {
		fmt.Fprintln(s.d.Stderr, "codex delegate ports are not registered")
		return 1
	}
	if err := ports.Usage(s.events, usageFile); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	session, _ := adapter.CodexEventField(s.events, "session")
	turn, _ := adapter.CodexEventField(s.events, "turn")
	if !s.settleResultIdentity(session, turn, s.requestedModel, "", usageFile) {
		return 1
	}
	return terminal(s.completeFromCLI(status, usageFile, s.raw, "", repairHooks{}))
}

func codexSelftest(d Deps) int {
	model, err := d.configValue("role.default.model.codex", "")
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	return commonSelftest("codex", "native", "", []string{"METASYSTEM_ROLE_DESIGN_CRITIC_MODEL_CODEX=" + model})(d)
}

// commonSelftest runs the full-contract self-test for a runtime; the
// decisions live in adapter.SelftestRun. turnCeiling 0 is the shared 240s
// default; the knobs are properties of the RUNTIME, not the contract.
func commonSelftest(runtime, usageExpectation, probe string, extraEnv []string) func(Deps) int {
	return selftestWith(runtime, usageExpectation, probe, extraEnv, 240, false)
}

func selftestWith(runtime, usageExpectation, probeName string, extraEnv []string, turnCeiling int, denialEndsTurn bool) func(Deps) int {
	return func(d Deps) int {
		a := registry[runtime]
		p := adapter.SelftestParams{
			Root: d.Root, Runtime: runtime, Usage: usageExpectation,
			TurnCeilingSec: turnCeiling, DenialEndsTurn: denialEndsTurn, ExtraEnv: extraEnv,
			RunIdentity: func() error {
				_, err := a.configIdentity(d)
				return err
			},
			RunProbe: func() error {
				quiet := d
				quiet.Stdout = discard{}
				if code := a.probe(quiet, nil); code != 0 {
					return fmt.Errorf("%s probe failed", runtime)
				}
				return nil
			},
		}
		if probeName != "" {
			probe, err := adapter.SelftestProbeFor(runtime, probeName)
			if err != nil {
				fmt.Fprintln(d.Stderr, err)
				return 1
			}
			p.Probe = &probe
		}
		model, err := d.configValue("role.default.model."+runtime, "")
		if err != nil {
			fmt.Fprintln(d.Stderr, err)
			return 1
		}
		if err := adapter.SelftestRun(p, model, d.Stdout); err != nil {
			fmt.Fprintln(d.Stderr, err)
			return 1
		}
		return 0
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
