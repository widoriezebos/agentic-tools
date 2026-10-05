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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostsetup"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ledgerfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
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
	// MergeDriver is the testing contract's merge-driver registration:
	// registered, unchanged, or no-git (Setup only).
	MergeDriver string
}

// Unchanged reports whether the switch found every effect already in place.
func (r Report) Unchanged() bool {
	return len(r.Changed) == 0 && (r.Fence == FenceUnchanged || r.Fence == FenceNoGit) &&
		(r.MergeDriver == "" || r.MergeDriver == contractgit.DriverUnchanged || r.MergeDriver == contractgit.DriverNoGit)
}

// Options are what Setup registers beyond the hooks.
type Options struct {
	// Runtimes are the runtimes registered in full: instruction pointers,
	// skills, profiles and hooks. Empty registers the runtimes the
	// installation's metasystem.runtimes enables; ["none"] registers none.
	Runtimes []string
	// CopySkills copies skill trees instead of linking them.
	CopySkills bool
}

// Setup is the checkout's one activation (system setup): it validates the
// engine as Switch does, then registers the chosen runtimes in full, switches
// every registered runtime's hooks to the engine, enrolls the commit fence,
// and registers the testing contract's merge driver. Nothing is written when
// the engine is refused; a repeat with everything in place writes nothing.
func Setup(path string, options Options, deps Deps) (Report, error) {
	return run(path, &options, deps)
}

// ConfiguredRuntimes are the adoptable runtimes the installation's
// metasystem.runtimes enables: every adoptable one when it names none, and
// ["none"] when it is "none".
func ConfiguredRuntimes(installation string) []string {
	value, present, err := config.ConfLookup(filepath.Join(installation, "metasystem.conf"), "metasystem.runtimes")
	value = strings.TrimSpace(value)
	if err != nil || !present || value == "" {
		return nil
	}
	if value == "none" {
		return []string{"none"}
	}
	var selected []string
	for _, name := range strings.Split(value, ",") {
		if declaration, ok := runtimes.Lookup(strings.TrimSpace(name)); ok && declaration.Adoptable {
			selected = append(selected, strings.TrimSpace(name))
		}
	}
	if len(selected) == 0 {
		return []string{"none"}
	}
	return selected
}

// TestingContract is the testing contract's path relative to the
// repository root, as .gitattributes names it.
func TestingContract(layout stateroot.Layout) string {
	name := "testing.json"
	if value, present, err := config.ConfLookup(layout.InstallationRoot.Path("metasystem.conf"), "testing.contract"); err == nil && present && strings.TrimSpace(value) != "" {
		name = strings.TrimSpace(value)
	}
	relative, err := filepath.Rel(layout.RepositoryRoot, layout.InstallationRoot.Path(filepath.FromSlash(name)))
	if err != nil {
		return name
	}
	return filepath.ToSlash(relative)
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
	EnsureFence func(installation stateroot.Installation) error
}

// Production is the switch's production seams.
func Production() Deps {
	return Deps{Resolve: stateroot.ResolveLayout, Git: git, Accepts: accepts, EnsureFence: ledgerfence.Ensure}
}

func run(path string, options *Options, deps Deps) (Report, error) {
	layout, err := deps.Resolve(path)
	if err != nil {
		return Report{}, &RefusalError{Reason: fmt.Sprintf("%s is not inside one metasystem installation: %v", path, err),
			Remedy: "run this inside the checkout, or name its installation with --installation DIR"}
	}
	report := Report{Installation: layout.InstallationRoot.Path()}
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
	if options != nil {
		selected := options.Runtimes
		if len(selected) == 0 {
			selected = ConfiguredRuntimes(layout.InstallationRoot.Path())
		}
		result, err := hostsetup.SetupWithResolver(hostsetup.Options{RepositoryPath: path, Runtimes: selected, CopySkills: options.CopySkills}, deps.Resolve)
		if err != nil {
			return report, err
		}
		report.Changed = append(report.Changed, result.Changed...)
	}
	report.Runtimes = registeredRuntimes(layout.RepositoryRoot)
	if len(report.Runtimes) > 0 {
		result, err := hostsetup.SetupWithResolver(hostsetup.Options{RepositoryPath: path, Runtimes: report.Runtimes, HooksOnly: true}, deps.Resolve)
		if err != nil {
			return report, err
		}
		report.Changed = append(report.Changed, result.Changed...)
	}
	report.Fence, report.FenceHook, err = ensureFence(layout.InstallationRoot, deps)
	if err != nil || options == nil {
		return report, err
	}
	report.MergeDriver, err = contractgit.Register(layout.RepositoryRoot, TestingContract(layout), engine, deps.Git)
	return report, err
}

// RegisteredRuntimes are the adoptable runtimes whose hook settings file
// this checkout carries.
func RegisteredRuntimes(repository string) []string { return registeredRuntimes(repository) }

// registeredRuntimes are the adoptable runtimes whose hook settings file this
// checkout already carries; the switch registers no new runtime.
func registeredRuntimes(repository string) []string {
	return runtimes.RegisteredRuntimes(repository)
}

func ensureFence(installation stateroot.Installation, deps Deps) (string, string, error) {
	hookPath, err := ledgerfence.HookPath(installation.Path())
	if err != nil {
		if _, probe := deps.Git("-C", installation.Path(), "rev-parse", "--git-dir"); probe != nil {
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

// Git runs git with the switch's scrubbed environment.
func Git(args ...string) (string, error) { return git(args...) }

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

// accepts runs the engine's hook --accepts: its exit decides, and what it
// says goes to this process's stderr for a person, never read here.
func accepts(engine string) error {
	command := exec.Command(engine, "internal", "hook", "--accepts")
	command.Env = ledgerfence.EnvironWithoutGitSteering()
	command.Stdout, command.Stderr = os.Stderr, os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("%s does not accept hooks (%v); what it said is above", engine, err)
	}
	return nil
}
