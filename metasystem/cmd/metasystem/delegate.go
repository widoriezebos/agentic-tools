package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

const delegateClaimCapabilityEnv = "METASYSTEM_DELEGATE_CLAIM_CAPABILITY"

type delegateOutcome struct {
	Outcome  string `json:"outcome"`
	Headline string `json:"headline"`
	JobID    string `json:"jobId,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// runDelegate is the operator boundary and the delegate lifecycle's process
// entry: operator forms answer typed JSON; the runtime adapters' "__"
// callbacks and the guard member wrapper keep their process shape.
func runDelegate(args []string, stdout, stderr io.Writer) int {
	// An operator form begins with one of its options; a lifecycle
	// callback begins with its __ word and parses its own options.
	if len(args) > 0 && strings.HasPrefix(args[0], "--") && !delegateFormOptions[strings.SplitN(args[0], "=", 2)[0]] {
		return refuseUnknownOption(stdout, stderr, "delegate", args[0], "it takes --revive, --cancel, --follow-up, --adapter-selftest, a dispatch's --role/--brief/--goal/--destructive-reach, or a lifecycle callback")
	}
	return runDelegateIn(args, "", stdout, stderr)
}

// delegateFormOptions are the options an operator form of the delegate entry
// may begin with.
var delegateFormOptions = map[string]bool{
	"--revive": true, "--cancel": true, "--follow-up": true, "--adapter-selftest": true, "--role": true, "--brief": true,
	"--goal": true, "--destructive-reach": true, "--op": true, "--reviews": true, "--runtime": true, "--model": true,
	"--outputs": true, "--design": true, "--approved-ref": true, "--source": true, "--wait": true,
}

// runDelegateIn is runDelegate with its typed outcome written to stdout. dir
// is the working directory relative file arguments resolve against (empty is
// the current directory). It is the process entry: the installation comes
// from METASYSTEM_DELEGATE_ROOT or this engine, and the lifecycle reads this
// process's stdin.
func runDelegateIn(args []string, dir string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == delegation.RunMemberCallback {
		return runDelegateGuardMember(args[1:], stdout, stderr)
	}
	return runDelegateWith(delegateRequest{rootOverride: os.Getenv("METASYSTEM_DELEGATE_ROOT"), args: args, dir: dir, stdin: os.Stdin}, stdout, stderr)
}

// runDelegateAt is the delegate boundary for an explicit installation root.
func runDelegateAt(installation string, args []string, stdout, stderr io.Writer) int {
	return runDelegateWith(delegateRequest{rootOverride: installation, args: args, stdin: os.Stdin}, stdout, stderr)
}

// delegateRequest is one call of the delegate boundary made in the caller's
// process (design 6.2): everything the former `internal delegate` child took
// from its argv, inherited environment and stdin is a field here.
type delegateRequest struct {
	// rootOverride names the installation, as METASYSTEM_DELEGATE_ROOT did;
	// empty derives it from engine.
	rootOverride string
	// engine is the installation's engine executable the installation is
	// derived from; empty is this process's own.
	engine string
	args   []string
	// dir is the working directory relative file arguments resolve against.
	dir string
	// environment is configuration the request carries beside this
	// process's own (KEY=VALUE, config.EnvName keys): a critic read's
	// selected-installation roster. The lifecycle resolves it in this
	// process above the process environment and hands it to the adapter
	// processes it starts (delegation.Request.ConfigEnv, VOA-14).
	environment []string
	// stdin is the lifecycle's input (the escalation approval prompt); nil
	// is none.
	stdin io.Reader
}

// delegateRoot is the installation a delegate request dispatches from.
func (request delegateRequest) delegateRoot() (string, error) {
	if request.rootOverride != "" || request.engine == "" {
		return upMetasystemRoot(request.rootOverride)
	}
	return upMetasystemRootOf(request.engine)
}

// runDelegateWith is the delegate boundary on the caller's streams.
func runDelegateWith(request delegateRequest, stdout, stderr io.Writer) int {
	args, dir := request.args, request.dir
	root, err := request.delegateRoot()
	if err != nil {
		writeJSONLine(stdout, stderr, delegateOutcome{Outcome: "REFUSED-INTERNAL", Headline: "refused", Detail: err.Error()})
		return 1
	}
	if len(args) > 0 && (strings.HasPrefix(args[0], "__") || delegateRawCommand(args[0])) {
		return runDelegateRaw(root, args, stdout, stderr)
	}
	if len(args) > 0 && args[0] == "--adapter-selftest" && !delegateSelftestInternalAuthorized(root) {
		writeJSONLine(stdout, stderr, delegateOutcome{
			Outcome: "REFUSED-REQUEST", Headline: "refused",
			Detail: "delegate --adapter-selftest is reserved for the runtime adapters' own self-test",
		})
		return 2
	}
	internalArgs, mode, err := normalizeDelegateArgs(args)
	if err != nil {
		writeJSONLine(stdout, stderr, delegateOutcome{Outcome: "REFUSED-REQUEST", Headline: "refused", Detail: err.Error()})
		return 2
	}
	if dir != "" {
		internalArgs = absoluteDelegatePaths(internalArgs, dir)
	}
	if detail := brain.Fence(root, mode, goal.ExistingLedgerIdentity(root)); detail != "" {
		writeJSONLine(stdout, stderr, delegateOutcome{Outcome: "BRAIN_REFUSED", Headline: "refused", Detail: detail})
		return 2
	}
	claimCapability := ""
	if mode == "dispatch" || mode == "follow-up" {
		dispatchMode := dispatchcore.DispatchModeFresh
		if mode == "follow-up" {
			dispatchMode = dispatchcore.DispatchModeFollowUp
		}
		claimCapability, err = dispatchcore.MintDelegateClaimCapability(root, dispatchMode)
		if err != nil {
			writeJSONLine(stdout, stderr, delegateOutcome{Outcome: "REFUSED-INTERNAL", Headline: "refused", Detail: err.Error()})
			return 1
		}
		defer dispatchcore.RemoveDelegateClaimCapability(root, claimCapability)
	}
	lifecycle, err := newDelegationLifecycle(root, request.environment...)
	if err != nil {
		writeJSONLine(stdout, stderr, delegateOutcome{Outcome: "REFUSED-INTERNAL", Headline: "refused", Detail: err.Error()})
		return 1
	}
	var diagnostics bytes.Buffer
	lifecycleRequest := delegateLifecycleRequest(delegateOperatorEnv(os.LookupEnv, claimCapability), request.stdin, io.MultiWriter(stderr, &diagnostics))
	lifecycleRequest.ConfigEnv = request.environment
	result := lifecycle.Run(context.Background(), lifecycleRequest, internalArgs)
	return writeDelegateResult(result, mode, args, diagnostics.String(), stdout, stderr)
}

// delegateOperatorEnv is the operator boundary's invocation state: the
// inherited environment, with the boundary's own standing and claim
// capability replacing anything a caller inherited.
func delegateOperatorEnv(lookup func(string) (string, bool), claimCapability string) delegation.Env {
	env := delegation.EnvFromEnviron(lookup)
	env.RecordOutcome, env.DelegateInternal, env.ClaimCapability = true, true, claimCapability
	return env
}

// writeDelegateResult maps a lifecycle result to the boundary's one typed
// JSON line: the recorded outcome, else a JSON standard output, else the
// started job (or cancelled target), else the refusal with the diagnostics.
func writeDelegateResult(result delegation.Result, mode string, args []string, diagnostics string, stdout, stderr io.Writer) int {
	exitCode := result.ExitCode
	if encoded := bytes.TrimSpace(result.Outcome); json.Valid(encoded) && len(encoded) > 0 {
		writeDelegateOutcome(stdout, stderr, encoded)
		return exitCode
	}
	if encoded := bytes.TrimSpace(result.Stdout); json.Valid(encoded) && len(encoded) > 0 {
		fmt.Fprintln(stdout, string(encoded))
		return exitCode
	}
	if exitCode == 0 {
		job := strings.TrimSpace(string(result.Stdout))
		if line, _, found := strings.Cut(job, "\n"); found {
			job = line
		}
		outcome := "WON"
		headline := "started"
		if mode == "cancel" {
			outcome, headline, job = "CANCELLED", "cancelled", delegateTarget(args)
		}
		writeJSONLine(stdout, stderr, delegateOutcome{Outcome: outcome, Headline: headline, JobID: job})
		return 0
	}
	writeJSONLine(stdout, stderr, delegateOutcome{Outcome: "REFUSED-INTERNAL", Headline: "refused",
		Detail: delegateInternalRefusalDetail(diagnostics, nil, exitCode)})
	return exitCode
}

// delegateRawCommand names the lifecycle's own command words: the internal
// grammar the retired dispatch.sh answered. Outside the delegate boundary
// (METASYSTEM_DELEGATE_INTERNAL unset) the lifecycle refuses the
// authority-bearing ones exactly as the script did.
func delegateRawCommand(word string) bool {
	switch word {
	case "dispatch", "follow-up", "watch", "status", "cancel", "close", "reap":
		return true
	}
	return false
}

// runDelegateRaw runs one lifecycle command or callback with the raw
// process contract: its standard output, its diagnostics and its exit code.
func runDelegateRaw(root string, args []string, stdout, stderr io.Writer) int {
	lifecycle, err := newDelegationLifecycle(root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	result := lifecycle.Run(context.Background(), delegateLifecycleRequest(delegation.EnvFromEnviron(os.LookupEnv), os.Stdin, stderr), args)
	_, _ = stdout.Write(result.Stdout)
	if path := os.Getenv("METASYSTEM_DELEGATE_OUTCOME_FILE"); path != "" && len(result.Outcome) > 0 {
		_ = os.WriteFile(path, result.Outcome, 0o600)
	}
	return result.ExitCode
}

// runDelegateGuardMember is the guard member wrapper the lifecycle launches
// its adapter under: __run-member --root R -- COMMAND...
func runDelegateGuardMember(args []string, stdout, stderr io.Writer) int {
	if len(args) < 4 || args[0] != "--root" || args[1] == "" || args[2] != "--" {
		fmt.Fprintln(stderr, "checkout execution guard wrapper: root and command are required")
		return 2
	}
	return gaterun.RunGuardMember(args[1], args[3:], stdout, stderr)
}

// delegateLifecycleRequest is this process's invocation: the current process
// is the supplied identity (design 6.2), and its whole command line is the
// owner lock tag a contender re-reads. stdin is the approval prompt's input,
// a terminal only when it is this process's own terminal standard input.
func delegateLifecycleRequest(env delegation.Env, stdin io.Reader, stderr io.Writer) delegation.Request {
	stdinTTY := false
	if file, ok := stdin.(*os.File); ok && file != nil {
		stdinTTY = isTerminal(file.Fd())
	}
	return delegation.Request{
		Invocation: delegation.Invocation{CallerPid: int64(os.Getpid())},
		Env:        env, LockTag: delegation.LockTagOf(os.Args),
		Stdin: stdin, StdinTTY: stdinTTY, StderrTTY: isTerminal(os.Stderr.Fd()),
		Stderr: stderr,
	}
}

// delegateInProcess runs one delegate lifecycle command in this process for
// a resident caller (the mission runner's reap and close): the current
// process is the supplied identity, its environment the invocation state.
func delegateInProcess(root string) func(args ...string) (string, string, int) {
	return func(args ...string) (string, string, int) {
		lifecycle, err := newDelegationLifecycle(root)
		if err != nil {
			return "", err.Error(), 1
		}
		var diagnostics bytes.Buffer
		result := lifecycle.Run(context.Background(), delegateLifecycleRequest(delegation.EnvFromEnviron(os.LookupEnv), os.Stdin, &diagnostics), args)
		return string(result.Stdout), diagnostics.String(), result.ExitCode
	}
}

// delegateBreachStop is the steward tick's breach-stop custodian: the
// lifecycle's stop-custodian entry run in this (steward) process, its
// combined report returned.
func delegateBreachStop(root string) func(string, uint64) (string, error) {
	run := delegateInProcess(root)
	return func(goalID string, revision uint64) (string, error) {
		stdout, stderr, code := run("__breach-stop-goal", "--goal", goalID, "--revision", strconv.FormatUint(revision, 10))
		if code != 0 {
			return stdout + stderr, fmt.Errorf("exit status %d", code)
		}
		return stdout + stderr, nil
	}
}

// delegateCancel is the stop transition's job cancel: the delegate
// boundary's cancel form run in this process against the installation.
func delegateCancel(installation string) func(string) (string, error) {
	return func(job string) (string, error) {
		var stdout, stderr bytes.Buffer
		code := runDelegateAt(installation, []string{"--cancel", job}, &stdout, &stderr)
		output := stdout.String() + stderr.String()
		if code != 0 {
			return output, fmt.Errorf("exit status %d", code)
		}
		return output, nil
	}
}

// newDelegationLifecycle composes the lifecycle over the real owners; the
// adapter processes it starts receive configEnv, a request's carried
// configuration.
func newDelegationLifecycle(root string, configEnv ...string) (*delegation.Lifecycle, error) {
	engine := delegationEngine(root)
	ports, err := delegation.NewOwnerPorts(delegation.OwnerConfig{Root: root, Engine: engine, Host: engineHost{}, ConfigEnv: configEnv})
	if err != nil {
		return nil, err
	}
	return delegation.New(delegation.Config{Root: root, Engine: engine}, ports)
}

// delegationEngine is the installation's engine the lifecycle launches and
// records: METASYSTEM_BIN when set, else the installation's own.
func delegationEngine(root string) string {
	if engine := os.Getenv("METASYSTEM_BIN"); engine != "" {
		return engine
	}
	return filepath.Join(root, "bin", "metasystem")
}

// absoluteDelegatePaths resolves the file arguments of a normalized request
// against dir, where the retired script ran.
func absoluteDelegatePaths(args []string, dir string) []string {
	out := append([]string(nil), args...)
	for index := 0; index+1 < len(out); index++ {
		switch out[index] {
		case "--brief", "--message", "--outputs", "--workspace":
			if !filepath.IsAbs(out[index+1]) {
				out[index+1] = filepath.Join(dir, out[index+1])
			}
		}
	}
	return out
}

func delegateInternalRefusalDetail(stderr string, runErr error, exitCode int) string {
	if detail := strings.TrimSpace(stderr); detail != "" {
		return detail
	}
	if runErr != nil {
		return runErr.Error()
	}
	return fmt.Sprintf("delegate internal exited with status %d without detail", exitCode)
}

// delegateSelftestInternalAuthorized keeps the fixed self-test grammar behind
// its actual orchestrator. The environment marker is necessary but not
// sufficient: the live parent must also be the same binary's
// delegate-supervisor self-test for this installation.
func delegateSelftestInternalAuthorized(root string) bool {
	if os.Getenv("METASYSTEM_DELEGATE_SELFTEST_INTERNAL") != "1" {
		return false
	}
	self, err := os.Executable()
	if err != nil {
		return false
	}
	self = resolvedDelegatePath(self)
	parent := int64(os.Getppid())
	exact, state, err := (identity.KernelProber{}).Probe(parent)
	if err != nil || state != identity.Alive || !exact.ArgvKnown {
		return false
	}
	executable, ok := identity.ExecutablePath(parent)
	if !ok {
		return false
	}
	executable = resolvedDelegatePath(executable)
	if executable != self {
		return false
	}
	// The self-test orchestrator is the same binary's delegate-supervisor
	// entry running RUNTIME selftest for this installation root.
	args := exact.Argv[1:]
	if len(args) > 0 && args[0] == "internal" {
		args = args[1:]
	}
	return len(args) >= 5 && args[0] == runtimes.SupervisorEntry && args[2] == "selftest" &&
		args[3] == "--root" && resolvedDelegatePath(args[4]) == resolvedDelegatePath(root)
}

func resolvedDelegatePath(path string) string {
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path
}

func writeDelegateOutcome(stdout, stderr io.Writer, encoded []byte) {
	var object map[string]any
	if json.Unmarshal(encoded, &object) != nil {
		fmt.Fprintln(stdout, string(encoded))
		return
	}
	outcome, _ := object["outcome"].(string)
	if _, present := object["headline"]; !present {
		switch {
		case outcome == "WON":
			object["headline"] = "started"
		case outcome == "IN-PROGRESS" || outcome == "BOUND" || outcome == "RECONCILING" || strings.HasPrefix(outcome, "REPLAYED-"):
			object["headline"] = "already running"
		default:
			object["headline"] = "refused"
		}
	}
	if _, present := object["jobId"]; !present {
		if evidence, ok := object["evidence"].(map[string]any); ok {
			if recordPath, ok := evidence["recordPath"].(string); ok && recordPath != "" {
				object["jobId"] = strings.TrimSuffix(filepath.Base(recordPath), ".json")
			}
		}
	}
	writeJSONLine(stdout, stderr, object)
}

func commandExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode()
	}
	return 1
}

func normalizeDelegateArgs(args []string) ([]string, string, error) {
	if len(args) == 0 {
		return nil, "", fmt.Errorf("usage: metasystem internal delegate --role <role> --brief <file> --goal <id|none-explicit> [--op <id>] [--reviews <job-id>] [--outputs <file> --design <file>]")
	}
	if args[0] == "--cancel" {
		if len(args) != 2 {
			return nil, "", fmt.Errorf("delegate --cancel requires exactly one job id")
		}
		return []string{"cancel", "--job", args[1]}, "cancel", nil
	}
	if args[0] == "--revive" {
		if len(args) != 2 || args[1] == "" {
			return nil, "", fmt.Errorf("delegate --revive requires exactly one steward intent")
		}
		return []string{"dispatch", "--steward-intent", args[1]}, "dispatch", nil
	}
	if args[0] == "--adapter-selftest" {
		if (len(args) != 8 && len(args) != 9) || args[1] == "" || args[2] != "--brief" || args[3] == "" || args[4] != "--workspace" || args[5] == "" || args[6] != "--op" || args[7] == "" || (len(args) == 9 && args[8] != "--wait") {
			return nil, "", fmt.Errorf("delegate --adapter-selftest requires <runtime> --brief <file> --workspace <dir> --op <id>")
		}
		out := []string{"dispatch", "--role", "implementer", "--brief", args[3], "--runtime", args[1], "--workspace", args[5], "--permissions", "none", "--job-id", args[7], "--destructive-reach", "MECHANICAL"}
		if len(args) == 9 {
			out = append(out, "--wait")
		}
		return out, "dispatch", nil
	}
	if args[0] == "--follow-up" {
		if len(args) < 4 || args[1] == "" {
			return nil, "", fmt.Errorf("delegate --follow-up requires a job id and --brief <file>")
		}
		out := []string{"follow-up", "--job", args[1]}
		briefSeen := false
		opSeen := false
		for index := 2; index < len(args); index++ {
			switch args[index] {
			case "--brief":
				if index+1 >= len(args) || briefSeen {
					return nil, "", fmt.Errorf("delegate --follow-up requires exactly one --brief <file>")
				}
				briefSeen = true
				out = append(out, "--message", args[index+1])
				index++
			case "--approved-ref":
				if index+1 >= len(args) {
					return nil, "", fmt.Errorf("delegate --approved-ref requires a value")
				}
				out = append(out, args[index], args[index+1])
				index++
			case "--op":
				if index+1 >= len(args) || opSeen {
					return nil, "", fmt.Errorf("delegate --follow-up accepts at most one --op value")
				}
				opSeen = true
				out = append(out, "--operation-id", args[index+1])
				index++
			case "--wait":
				out = append(out, args[index])
			default:
				return nil, "", fmt.Errorf("delegate --follow-up does not accept %s", args[index])
			}
		}
		if !briefSeen {
			return nil, "", fmt.Errorf("delegate --follow-up requires --brief <file>")
		}
		return out, "follow-up", nil
	}

	goalSeen := false
	roleSeen := false
	role := ""
	briefSeen := false
	opSeen := false
	reviewsSeen := false
	reviews := ""
	outputsSeen, designSeen := false, false
	runtimeSeen, modelSeen := false, false
	destructiveReachSeen := false
	out := []string{"dispatch"}
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--goal":
			if index+1 >= len(args) || goalSeen {
				return nil, "", fmt.Errorf("delegate requires exactly one --goal <id|none-explicit>")
			}
			goalSeen = true
			if args[index+1] != "none-explicit" {
				out = append(out, "--goal", args[index+1])
			}
			index++
		case "--role", "--brief":
			if index+1 >= len(args) {
				return nil, "", fmt.Errorf("delegate %s requires a value", args[index])
			}
			if args[index] == "--role" {
				if roleSeen {
					return nil, "", fmt.Errorf("delegate requires exactly one --role")
				}
				roleSeen = true
				role = args[index+1]
			} else {
				if briefSeen {
					return nil, "", fmt.Errorf("delegate requires exactly one --brief")
				}
				briefSeen = true
			}
			out = append(out, args[index], args[index+1])
			index++
		case "--op":
			if index+1 >= len(args) || opSeen {
				return nil, "", fmt.Errorf("delegate accepts at most one --op value")
			}
			opSeen = true
			out = append(out, "--job-id", args[index+1])
			index++
		case "--reviews":
			if index+1 >= len(args) || reviewsSeen {
				return nil, "", fmt.Errorf("delegate accepts at most one --reviews value")
			}
			reviewsSeen = true
			reviews = args[index+1]
			out = append(out, "--reviews", args[index+1])
			index++
		case "--runtime", "--model":
			if index+1 >= len(args) || args[index+1] == "" {
				return nil, "", fmt.Errorf("delegate %s requires a value", args[index])
			}
			if args[index] == "--runtime" {
				if runtimeSeen {
					return nil, "", fmt.Errorf("delegate accepts one --runtime")
				}
				runtimeSeen = true
			} else {
				if modelSeen {
					return nil, "", fmt.Errorf("delegate accepts one --model")
				}
				modelSeen = true
			}
			out = append(out, args[index], args[index+1])
			index++
		case "--outputs", "--design":
			if index+1 >= len(args) {
				return nil, "", fmt.Errorf("delegate %s requires a value", args[index])
			}
			if args[index] == "--outputs" {
				if outputsSeen {
					return nil, "", fmt.Errorf("delegate accepts one --outputs")
				}
				outputsSeen = true
			} else {
				if designSeen {
					return nil, "", fmt.Errorf("delegate accepts one --design")
				}
				designSeen = true
			}
			out = append(out, args[index], args[index+1])
			index++
		case "--destructive-reach":
			if index+1 >= len(args) || destructiveReachSeen {
				return nil, "", fmt.Errorf("delegate requires exactly one --destructive-reach")
			}
			destructiveReachSeen = true
			value := args[index+1]
			if value != "MECHANICAL" && value != "DESIGN-BEARING" && value != "DESTRUCTIVE-REACH" {
				return nil, "", fmt.Errorf("delegate --destructive-reach must be MECHANICAL, DESIGN-BEARING, or DESTRUCTIVE-REACH")
			}
			out = append(out, "--destructive-reach", value)
			index++
		case "--approved-ref", "--source":
			if index+1 >= len(args) {
				return nil, "", fmt.Errorf("delegate %s requires a value", args[index])
			}
			out = append(out, args[index], args[index+1])
			index++
		case "--wait":
			out = append(out, args[index])
		default:
			return nil, "", fmt.Errorf("delegate does not accept %s", args[index])
		}
	}
	if !goalSeen || !roleSeen || !briefSeen || !destructiveReachSeen {
		return nil, "", fmt.Errorf("delegate requires --role, --brief, --goal <id|none-explicit>, and --destructive-reach <class>")
	}
	if reviewsSeen && role != "code-critic" && role != "warden" && role != "verifier" {
		return nil, "", fmt.Errorf("--reviews is only valid for the code-critic, warden, and verifier roles")
	}
	// The review relation is the critic's subject; refuse its absence at the
	// front door, before any checkout guard or dispatch record exists.
	if !reviewsSeen && (role == "code-critic" || role == "warden") {
		return nil, "", fmt.Errorf("%s dispatch requires --reviews <implementer-job-id>", role)
	}
	if (runtimeSeen || modelSeen) && (role != "code-critic" || !strings.HasPrefix(reviews, "commit:")) {
		return nil, "", fmt.Errorf("delegate --runtime and --model require a code-critic commit review")
	}
	if role == "design-critic" && (!outputsSeen || !designSeen) {
		return nil, "", fmt.Errorf("design-critic dispatch requires --outputs <file> and --design <file>")
	}
	if role != "design-critic" && (outputsSeen || designSeen) {
		return nil, "", fmt.Errorf("--outputs and --design are only valid for the design-critic role")
	}
	return out, "dispatch", nil
}

func delegateTarget(args []string) string {
	if len(args) >= 2 {
		return args[1]
	}
	return ""
}
