// Package hookswitch connects one checkout's runtime lifecycle hooks and its
// git pre-commit fence to the checkout's engine (plans/designs/
// verbs-object-action.md 3.3, unit U9's activation). It composes the hook
// settings renderer (internal/hooks, internal/hostsetup) and the fence
// enrollment (internal/ledgerfence); it decides nothing either owner decides.
//
// The switch validates first and writes second: the engine the direct
// settings command will run must answer `internal hook --accepts`, or nothing
// is written and the refusal names the build that fixes it. Then the settings
// of every runtime this checkout already registers are rendered to the direct
// command, and the pre-commit hook is enrolled, upgrading a composer that
// still runs the deleted pre-commit-guard.sh. A second switch finds every
// effect in place and writes nothing (R-129).
package hookswitch

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostsetup"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ledgerfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// BuildCommand is the bootstrap that builds or rebuilds an engine.
const BuildCommand = "go run ./cmd/devgate build"

// Fence states of one switch.
const (
	FenceUnchanged  = "unchanged"
	FenceEnrolled   = "enrolled"
	FenceReenrolled = "re-enrolled"
	FenceNoGit      = "no-repository"
)

// Report is what one switch found and changed.
type Report struct {
	Installation string
	Engine       string
	// Runtimes are the runtimes whose hook settings this checkout registers.
	Runtimes []string
	// Changed are the settings files rewritten, relative to the repository.
	Changed []string
	// Fence is the pre-commit hook's state: unchanged, enrolled, or
	// re-enrolled (a composer from before the engine guard was upgraded).
	Fence     string
	FenceHook string
}

// Unchanged reports whether the switch found every effect already in place.
func (r Report) Unchanged() bool {
	return len(r.Changed) == 0 && (r.Fence == FenceUnchanged || r.Fence == FenceNoGit)
}

// RefusalError is a refusal before any write, with the command that fixes it.
type RefusalError struct {
	Reason, Remedy string
}

func (e *RefusalError) Error() string { return e.Reason + "; " + e.Remedy }

// Deps are the switch's process seams; production fills them from the
// process (Production).
type Deps struct {
	Resolve func(string) (stateroot.Layout, error)
	Git     func(args ...string) (string, error)
	// Accepts runs engine `internal hook --accepts`.
	Accepts func(engine string) error
	// EnsureFence enrolls or upgrades the pre-commit fence.
	EnsureFence func(installation string) error
}

// Production is the switch's production seams.
func Production() Deps {
	return Deps{Resolve: stateroot.ResolveLayout, Git: git, Accepts: accepts, EnsureFence: ledgerfence.Ensure}
}

// Switch connects the checkout at path to its engine.
func Switch(path string, deps Deps) (Report, error) {
	layout, err := deps.Resolve(path)
	if err != nil {
		return Report{}, &RefusalError{Reason: fmt.Sprintf("%s is not inside one metasystem installation: %v", path, err),
			Remedy: "run this inside the checkout, or name its installation with --installation DIR"}
	}
	report := Report{Installation: layout.InstallationRoot}
	engine, err := hooks.DirectEngine(layout.InstallationRoot, deps.Git)
	if err != nil {
		return report, &RefusalError{Reason: "the engine this checkout's hooks would run cannot be found: " + err.Error(),
			Remedy: "check the checkout with: metasystem system check"}
	}
	report.Engine = engine
	if info, statErr := os.Stat(engine); statErr != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		return report, &RefusalError{Reason: fmt.Sprintf("no engine is installed at %s, so the hooks would only answer degraded; nothing was changed", engine),
			Remedy: fmt.Sprintf("build it with %s in %s, then run metasystem system setup again", BuildCommand, filepath.Dir(filepath.Dir(engine)))}
	}
	if err := deps.Accepts(engine); err != nil {
		return report, &RefusalError{Reason: fmt.Sprintf("the engine at %s does not serve the hook entry (%v); nothing was changed", engine, err),
			Remedy: fmt.Sprintf("rebuild it with %s in %s, then run metasystem system setup again", BuildCommand, filepath.Dir(filepath.Dir(engine)))}
	}
	report.Runtimes = registeredRuntimes(layout.RepositoryRoot)
	if len(report.Runtimes) > 0 {
		result, err := hostsetup.SetupWithResolver(hostsetup.Options{RepositoryPath: path, Runtimes: report.Runtimes, HooksOnly: true}, deps.Resolve)
		if err != nil {
			return report, err
		}
		report.Changed = result.Changed
	}
	report.Fence, report.FenceHook, err = ensureFence(layout.InstallationRoot, deps)
	return report, err
}

// registeredRuntimes are the adoptable runtimes whose hook settings file this
// checkout already carries; the switch registers no new runtime.
func registeredRuntimes(repository string) []string {
	var selected []string
	for _, runtime := range runtimes.Adoptable() {
		for _, row := range runtimes.RegistrationRows(runtime) {
			if row.Operation != runtimes.OpCopyFile && row.Operation != runtimes.OpJSONStripKey {
				continue
			}
			if info, err := os.Lstat(filepath.Join(repository, filepath.FromSlash(row.Destination))); err == nil && info.Mode().IsRegular() {
				selected = append(selected, runtime)
				break
			}
		}
	}
	return selected
}

func ensureFence(installation string, deps Deps) (string, string, error) {
	hookPath, err := ledgerfence.HookPath(installation)
	if err != nil {
		if _, probe := deps.Git("-C", installation, "rev-parse", "--git-dir"); probe != nil {
			return FenceNoGit, "", nil
		}
		return "", "", err
	}
	before, beforeErr := os.ReadFile(hookPath)
	if beforeErr != nil && !os.IsNotExist(beforeErr) {
		return "", hookPath, beforeErr
	}
	if err := deps.EnsureFence(installation); err != nil {
		return "", hookPath, err
	}
	after, err := os.ReadFile(hookPath)
	if err != nil && !os.IsNotExist(err) {
		return "", hookPath, err
	}
	switch {
	case bytes.Equal(before, after) && beforeErr == nil:
		return FenceUnchanged, hookPath, nil
	case beforeErr == nil && ledgerfence.RetiredComposer(string(before)):
		return FenceReenrolled, hookPath, nil
	default:
		return FenceEnrolled, hookPath, nil
	}
}

func git(args ...string) (string, error) {
	command := exec.Command("git", args...)
	command.Env = ledgerfence.EnvironWithoutGitSteering()
	out, err := command.Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
	}
	return string(out), err
}

func accepts(engine string) error {
	command := exec.Command(engine, "internal", "hook", "--accepts")
	command.Env = ledgerfence.EnvironWithoutGitSteering()
	output, err := command.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			return err
		}
		return fmt.Errorf("%v: %s", err, detail)
	}
	return nil
}
