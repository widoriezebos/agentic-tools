package supervisor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
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
		selftest: codexSelftest,
		ops: codexOps{builtinOps{name: "codex", cli: "codex", usage: "native", host: true,
			configIdentity: codexConfigIdentity, probe: codexProbe, contract: commonContract("codex"), selftest: codexSelftest}},
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
// cacheDirs are the machine delegate cache directories a delegate round's
// sandbox is granted (disk-lifetimes A7); a host turn passes none.
// sandboxMode is the host's launch.codex.sandbox the turn runs under.
func CodexCommand(verb, model, workspace, schema, output, instanceTag, reasoningEffort, permissionsPath, recordPath, session, sandboxMode string, cacheDirs []string) ([]string, error) {
	sandbox, network, err := adapter.CodexPermissionSettings(permissionsPath, recordPath, sandboxMode)
	if err != nil {
		return nil, err
	}
	// Write roots outside the workspace — the worktree's git metadata
	// (issue #5) — must reach the sandbox explicitly.
	extraDirs, err := adapter.CodexExtraWriteRoots(permissionsPath, recordPath, workspace, cacheDirs)
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

// codexOps is the Codex runtime's operations.
type codexOps struct{ builtinOps }

type codexLaunch struct{ events, raw string }

func (codexOps) Prepare(t *Turn) (Launch, error) {
	d := t.Deps()
	if t.Role == RoleHost {
		model, ok := field(t.Record, "model")
		if !ok {
			return Launch{}, fmt.Errorf("codex host turn record has no model")
		}
		verb := "dispatch"
		if t.ResumeSession != "" {
			verb = "follow-up"
		}
		raw := filepath.Join(t.Dir, "raw.out")
		// A host turn has no admitted job record: it runs under the host's
		// setting as it reads now. An installation without a metasystem.conf
		// (a host bed, a bare checkout) runs under today's sandbox, as it did
		// before the setting existed.
		mode, err := d.configValue(config.CodexSandboxKey, config.CodexSandboxWorkspaceWrite)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return Launch{}, err
			}
			mode = config.CodexSandboxWorkspaceWrite
		}
		if mode, err = config.ParseCodexSandbox(mode); err != nil {
			return Launch{}, err
		}
		command, err := CodexCommand(verb, model, t.Workspace, t.Schema, raw, t.Tag, "", t.Requested, "", t.ResumeSession, mode, nil)
		if err != nil {
			return Launch{}, err
		}
		// `codex exec resume` takes no -C: the CLI runs in the checkout.
		return Launch{Argv: command, StdinPath: t.Prompt, StdoutPath: filepath.Join(t.Dir, "events.jsonl"),
			Private: codexLaunch{events: filepath.Join(t.Dir, "events.jsonl"), raw: raw}}, nil
	}
	caches := d.delegateCaches()
	recordBuildCachePath(d.git(), d.agents(), t.Workspace, t.Dir, caches)
	raw := filepath.Join(t.Dir, "raw.out")
	// The working directory is the write boundary: `codex exec resume` has
	// no -C, so entering the workspace makes the recorded boundary true on
	// both paths.
	if err := adapter.RewriteWriteScope(t.Effective, t.Workspace); err != nil {
		return Launch{}, err
	}
	if err := truncate(t.Events); err != nil {
		return Launch{}, err
	}
	if err := truncate(raw); err != nil {
		return Launch{}, err
	}
	effort, ok := field(t.Record, "reasoningEffort")
	if !ok || effort == "null" {
		effort = ""
	}
	// The envelope decides sandbox and network — in the engine, from the
	// record itself (KI-12). The host's sandbox was read when the job was
	// admitted and recorded as the request's widening, so the round runs
	// exactly as wide as its record says even when the setting has changed
	// since.
	command, err := CodexCommand(t.Verb, t.Model, t.Workspace, t.Schema, raw, t.Tag, effort, "", t.Record, t.ResumeSession, adapter.CodexAdmittedSandbox(t.Record), caches.Directories())
	if err != nil {
		return Launch{}, err
	}
	// The machine delegate cache, granted above as extra write roots: the
	// sandbox cannot write the user's (engine) Go cache.
	env := delegateRoundEnv(d, t.Workspace)
	return Launch{Argv: command, Env: env, StdinPath: t.Prompt, StdoutPath: t.Events,
		Private: codexLaunch{events: t.Events, raw: raw}}, nil
}

// Observe reads the thread and turn ids from the event stream.
func (codexOps) Observe(t *Turn, o Observation) (Events, error) {
	private := o.Launch.Private.(codexLaunch)
	session, found := adapter.CodexEventField(private.events, "session")
	if !found || session == "" {
		return Events{}, nil
	}
	turn, _ := adapter.CodexEventField(private.events, "turn")
	return Events{Session: session, Turn: turn, Model: t.Model}, nil
}

func (codexOps) Finalize(t *Turn, in FinalInput) (Final, error) {
	private := in.Launch.Private.(codexLaunch)
	ports, err := delegate.PortsFor("codex")
	if err != nil || ports.Usage == nil {
		return Final{}, fmt.Errorf("codex delegate ports are not registered")
	}
	session, _ := adapter.CodexEventField(private.events, "session")
	if t.Role == RoleHost {
		usagePath, returnPath := filepath.Join(t.Dir, "usage.json"), filepath.Join(t.Dir, "return.json")
		if _, err := os.Stat(private.raw); err != nil {
			_ = os.WriteFile(private.raw, nil, 0o644)
		}
		if err := ports.Usage(private.events, usagePath); err != nil {
			return Final{}, err
		}
		if info, err := os.Stat(private.raw); err == nil && info.Size() > 0 {
			copyOrEmptyFile(private.raw, returnPath)
		}
		return Final{HostSession: session, HostRaw: private.raw, HostReturn: returnPath, Usage: usagePath}, nil
	}
	if err := ports.Usage(private.events, in.Usage); err != nil {
		return Final{}, err
	}
	turn, _ := adapter.CodexEventField(private.events, "turn")
	return Final{Candidate: private.raw, Session: session, Turn: turn, HandshakeModel: t.Model}, nil
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
			TurnCeilingSec: turnCeiling, DenialEndsTurn: denialEndsTurn, ExtraEnv: extraEnv, Engine: d.Self,
			Status: d.lifecycleStatus, Reap: d.lifecycleReap,
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
