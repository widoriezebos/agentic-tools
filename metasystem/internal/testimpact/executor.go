package testimpact

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const cancellationGrace = 2 * time.Second

type Executor struct {
	Clock func(time.Duration) <-chan time.Time
}
type CommandOutcome struct {
	ExitCode  int
	Seconds   float64
	Cancelled bool
	StartErr  error
}
type FallbackGroup struct {
	Group          testpolicy.Group
	Argv           []string
	Outcome        CommandOutcome
	Status, Reason string
}
type FallbackOutcome struct {
	Status string
	Groups []FallbackGroup
}

func (executor Executor) Run(ctx context.Context, cwd string, environment, argv []string, stdin io.Reader, stdout, stderr io.Writer) CommandOutcome {
	if len(argv) == 0 {
		return CommandOutcome{StartErr: fmt.Errorf("command argv is empty")}
	}
	path, err := proofrun.ResolveTestingExecutable(ctx, cwd, environment, argv)
	if err != nil {
		return CommandOutcome{StartErr: err}
	}
	command := exec.Command(path, argv[1:]...)
	command.Dir, command.Env = cwd, environment
	command.Stdin, command.Stdout, command.Stderr = stdin, stdout, stderr
	return executor.runCommand(ctx, command)
}
func (executor Executor) runCommand(ctx context.Context, command *exec.Cmd) CommandOutcome {
	startedAt := time.Now()
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return CommandOutcome{StartErr: err}
	}
	waited := make(chan error, 1)
	go func() { waited <- command.Wait() }()
	finish := func(waitErr error, cancelled bool) CommandOutcome {
		outcome := CommandOutcome{Seconds: time.Since(startedAt).Seconds(), Cancelled: cancelled}
		if waitErr == nil {
			return outcome
		}
		var exit *exec.ExitError
		if errors.As(waitErr, &exit) {
			outcome.ExitCode = exit.ExitCode()
			return outcome
		}
		outcome.StartErr = waitErr
		return outcome
	}
	select {
	case waitErr := <-waited:
		return finish(waitErr, false)
	case <-ctx.Done():
		group := command.Process.Pid
		_ = syscall.Kill(-group, syscall.SIGTERM)
		clock := executor.Clock
		if clock == nil {
			clock = time.After
		}
		grace := clock(cancellationGrace)
		select {
		case waitErr := <-waited:
			if err := syscall.Kill(-group, 0); errors.Is(err, syscall.ESRCH) {
				return finish(waitErr, true)
			}
			<-grace
			_ = syscall.Kill(-group, syscall.SIGKILL)
			return finish(waitErr, true)
		case <-grace:
			_ = syscall.Kill(-group, syscall.SIGKILL)
			return finish(<-waited, true)
		}
	}
}
func RunFallback(ctx context.Context, root, mode string, contract testpolicy.Contract, executor Executor, progress io.Writer) FallbackOutcome {
	result := FallbackOutcome{Status: StatusPassed}
	if mode == ModeList {
		result.Status = StatusListed
	}
	for _, group := range contract.Groups {
		item := FallbackGroup{Group: group, Status: StatusNotRun}
		cwd := filepath.Join(root, filepath.FromSlash(group.CWD))
		environment := proofrun.TestingEnvironment(os.Environ(), group.Env)
		if group.Adapter == "go" {
			environment = proofrun.TestingEnvironment(environment, map[string]string{
				"GOPROXY": "off", "GOSUMDB": "off", "GOTOOLCHAIN": "local",
			})
		}
		argv, err := proofrun.GroupArguments(ctx, group, root, cwd, environment, func(command *exec.Cmd) error {
			outcome := executor.runCommand(ctx, command)
			if outcome.Cancelled {
				return context.Canceled
			}
			if outcome.StartErr != nil {
				return outcome.StartErr
			}
			if outcome.ExitCode != 0 {
				return fmt.Errorf("exit %d", outcome.ExitCode)
			}
			return nil
		})
		item.Argv = append([]string(nil), argv...)
		if err != nil {
			if len(item.Argv) == 0 {
				item.Argv = []string{group.Adapter}
			}
			item.Status, item.Reason = StatusIncomplete, err.Error()
			if mode == ModeList {
				item.Status = StatusNotRun
			} else {
				result.Status = StatusIncomplete
			}
			result.Groups = append(result.Groups, item)
			continue
		}
		if mode == ModeList {
			result.Groups = append(result.Groups, item)
			continue
		}
		fmt.Fprintf(progress, "FULL TEST FALLBACK: RUN %s\n", group.ID)
		outcome := executor.Run(ctx, cwd, environment, argv, nil, progress, progress)
		item.Outcome = outcome
		switch {
		case outcome.Cancelled:
			item.Status, item.Reason = StatusIncomplete, "cancelled"
		case outcome.StartErr != nil:
			item.Status, item.Reason = StatusIncomplete, outcome.StartErr.Error()
		case outcome.ExitCode != 0:
			item.Status, item.Reason = StatusFailed, fmt.Sprintf("exit %d", outcome.ExitCode)
		default:
			item.Status = StatusPassed
		}
		result.Groups = append(result.Groups, item)
		if item.Status == StatusFailed || item.Status == StatusIncomplete && result.Status != StatusFailed {
			result.Status = item.Status
		}
	}
	return result
}

type ProviderOutcome struct {
	Result Result
	Run    CommandOutcome
	Argv   []string
}

func RunProvider(ctx context.Context, implementation testpolicy.Impacted, root string, request Request, executor Executor, progress io.Writer) (ProviderOutcome, error) {
	argv := append([]string(nil), implementation.Argv...)
	provider := ProviderOutcome{Argv: argv}
	if argv[0] == "metasystem" {
		executable, err := os.Executable()
		if err != nil {
			return provider, err
		}
		argv[0] = executable
		provider.Argv = argv
	}
	input, err := Encode(request)
	if err != nil {
		return provider, err
	}
	var stdout bytes.Buffer
	cwd := filepath.Join(root, filepath.FromSlash(implementation.CWD))
	outcome := executor.Run(ctx, cwd, os.Environ(), argv, bytes.NewReader(input), &stdout, progress)
	provider.Run = outcome
	switch {
	case outcome.Cancelled:
		return provider, fmt.Errorf("provider cancelled")
	case outcome.StartErr != nil:
		return provider, fmt.Errorf("start provider: %w", outcome.StartErr)
	case outcome.ExitCode != 0:
		return provider, fmt.Errorf("provider exit %d", outcome.ExitCode)
	}
	result, err := DecodeResult(stdout.Bytes(), request)
	if err != nil {
		return provider, err
	}
	provider.Result = result
	return provider, nil
}
func fallbackSelection(item FallbackGroup, mode string) Selection {
	scope, tests := ScopeCheck, []string{}
	if item.Group.Adapter == "go" {
		all, names, _ := testpolicy.GoTests(item.Group)
		if all {
			scope, tests = ScopePackage, append([]string(nil), item.Group.Packages...)
		} else {
			scope, tests = ScopeNamed, names
		}
	}
	reasons := []Reason{{Test: "*", Because: "full-test fallback"}}
	if scope == ScopeNamed {
		reasons = make([]Reason, len(tests))
		for i, test := range tests {
			reasons[i] = Reason{Test: test, Because: "full-test fallback"}
		}
	}
	if item.Reason != "" {
		for i := range reasons {
			reasons[i].Because += ": " + item.Reason
		}
	}
	seconds := item.Outcome.Seconds
	if mode == ModeList {
		seconds = 0
	}
	return Selection{Label: item.Group.ID, CWD: item.Group.CWD, Argv: append([]string(nil), item.Argv...), Scope: scope,
		Tests: tests, Reasons: reasons, Result: item.Status, Seconds: seconds}
}
func fallbackResult(request Request, outcome FallbackOutcome) Result {
	selections := make([]Selection, len(outcome.Groups))
	for i, group := range outcome.Groups {
		selections[i] = fallbackSelection(group, request.Mode)
	}
	return Result{SchemaVersion: SchemaVersion, Mode: request.Mode, Binding: request.Binding, Status: outcome.Status,
		Selections: selections, Uncertainty: []Uncertainty{}}
}
