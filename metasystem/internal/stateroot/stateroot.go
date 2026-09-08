// Package stateroot owns the mode-aware locations of application state.
// Writers ask for a typed state kind instead of reconstructing paths from
// their own installation directory.
package stateroot

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// Kind names one application-state directory whose ownership does not change
// when the metasystem is installed beneath an application repository.
type Kind string

// Layout names an explicitly resolved installation and the repository that
// hosts its runtime entry points. Setup callers pass a path in the target
// repository; resolution never depends on the binary performing the setup.
type Layout struct {
	GitRoot          string // actual containing Git root, or the fresh installation before Git exists
	RepositoryRoot   string // root that owns host registration files
	InstallationRoot string // canonical installation that supplies enforcement and skills
	InstallationRel  string // installation path relative to GitRoot for generated launchers
	Template         bool   // template-only pointers belong at RepositoryRoot
}

const (
	Registers Kind = "registers"
	Receipts  Kind = "receipts"
	Records   Kind = "records"
	Goals     Kind = "goals"
	OpenWork  Kind = "openwork"
	Steward   Kind = "steward"
	Evidence  Kind = "evidence"
)

var executablePath = os.Executable

var gitSteeringVariables = map[string]struct{}{
	"GIT_DIR": {}, "GIT_WORK_TREE": {}, "GIT_COMMON_DIR": {},
	"GIT_INDEX_FILE": {}, "GIT_CEILING_DIRECTORIES": {}, "GIT_DISCOVERY_ACROSS_FILESYSTEM": {},
	"GIT_OBJECT_DIRECTORY": {}, "GIT_ALTERNATE_OBJECT_DIRECTORIES": {},
	"GIT_CONFIG": {}, "GIT_CONFIG_PARAMETERS": {}, "GIT_CONFIG_COUNT": {},
	"GIT_CONFIG_GLOBAL": {}, "GIT_CONFIG_SYSTEM": {}, "GIT_CONFIG_NOSYSTEM": {},
	"GIT_GRAFT_FILE": {}, "GIT_SHALLOW_FILE": {}, "GIT_REPLACE_REF_BASE": {},
	"GIT_IMPLICIT_WORK_TREE": {}, "GIT_NO_REPLACE_OBJECTS": {}, "GIT_PREFIX": {},
}

var repositoryTop = func(installationRoot string) (string, error) {
	command := exec.Command("git", "-C", installationRoot, "rev-parse", "--show-toplevel")
	command.Env = scrubGitSteering(os.Environ())
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("state root: installation is not inside a Git repository: %s", strings.TrimSpace(string(output)))
	}
	return filepath.Abs(strings.TrimSpace(string(output)))
}

func scrubGitSteering(environment []string) []string {
	clean := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, present := strings.Cut(entry, "=")
		if present {
			if _, steering := gitSteeringVariables[name]; steering {
				continue
			}
		}
		clean = append(clean, entry)
	}
	return clean
}

// StateRoot returns the absolute directory owned by kind. Template checkouts
// keep their self-hosted state beneath the installation; adopted installations
// resolve state against the containing application repository.
func StateRoot(kind Kind) (string, error) {
	relative, err := relativeRoot(kind)
	if err != nil {
		return "", err
	}
	installationRoot, err := installationRoot()
	if err != nil {
		return "", err
	}
	appRoot, err := RootForInstallation(installationRoot)
	if err != nil {
		return "", err
	}
	if kind == Evidence {
		value, _, err := config.Get(config.GetParams{
			Key: "evidence.root", ConfPath: filepath.Join(appRoot, "metasystem.conf"),
		})
		if err != nil {
			return "", fmt.Errorf("state root: evidence root: %w", err)
		}
		if !filepath.IsAbs(value) {
			return "", fmt.Errorf("state root: evidence.root must be absolute: %q", value)
		}
		return filepath.Clean(value), nil
	}
	return filepath.Join(appRoot, filepath.FromSlash(relative)), nil
}

// RootForInstallation returns the directory beneath which repository-local
// state lives. Template checkouts keep it in the metasystem installation;
// adopted installations use the containing application repository.
func RootForInstallation(installationRoot string) (string, error) {
	root, err := filepath.Abs(installationRoot)
	if err != nil {
		return "", fmt.Errorf("state root: locate installation: %w", err)
	}
	if templateMode(root) {
		return root, nil
	}
	return repositoryTop(root)
}

// ResolveLayout locates a checked-in metasystem installation from a repository
// path or any directory below it. It distinguishes the template's nested
// <repo>/metasystem installation, an adopted repository root, and an explicitly
// selected adopted installation beneath its application's Git root.
func ResolveLayout(repositoryPath string) (Layout, error) {
	if strings.TrimSpace(repositoryPath) == "" {
		return Layout{}, fmt.Errorf("state root: repository path is required")
	}
	absolute, err := filepath.Abs(repositoryPath)
	if err != nil {
		return Layout{}, fmt.Errorf("state root: locate repository path: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return Layout{}, fmt.Errorf("state root: inspect repository path: %w", err)
	}
	if !info.IsDir() {
		absolute = filepath.Dir(absolute)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		absolute = resolved
	}
	repository, err := repositoryTop(absolute)
	if err != nil {
		// Adoption has always supported a fresh target before `git init`.
		// In that one layout the installation and application repository root
		// coincide, so an ancestor carrying the complete installed shape is an
		// explicit root without consulting the executing binary.
		if adopted := adoptedAncestor(absolute); adopted != "" {
			return Layout{GitRoot: adopted, RepositoryRoot: adopted, InstallationRoot: adopted, InstallationRel: "."}, nil
		}
		return Layout{}, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(repository); resolveErr == nil {
		repository = resolved
	}
	if installation := installationAncestor(absolute, repository); installation != "" {
		if installation == filepath.Join(repository, "metasystem") && templateMode(installation) {
			return Layout{GitRoot: repository, RepositoryRoot: repository, InstallationRoot: installation, InstallationRel: filepath.ToSlash(filepath.Base(installation)), Template: true}, nil
		}
		relative, relErr := filepath.Rel(repository, installation)
		if relErr != nil {
			return Layout{}, fmt.Errorf("state root: locate installation beneath repository: %w", relErr)
		}
		return Layout{GitRoot: repository, RepositoryRoot: installation, InstallationRoot: installation, InstallationRel: filepath.ToSlash(relative)}, nil
	}
	nested := filepath.Join(repository, "metasystem")
	if templateMode(nested) && installationShape(nested) {
		return Layout{GitRoot: repository, RepositoryRoot: repository, InstallationRoot: nested, InstallationRel: "metasystem", Template: true}, nil
	}
	return Layout{}, fmt.Errorf("state root: path %q selects neither a nested template nor an adopted installation", absolute)
}

func installationAncestor(path, boundary string) string {
	relative, err := filepath.Rel(boundary, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return ""
	}
	for candidate := filepath.Clean(path); ; candidate = filepath.Dir(candidate) {
		if installationShape(candidate) {
			return candidate
		}
		if candidate == boundary {
			return ""
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return ""
		}
	}
}

func adoptedAncestor(path string) string {
	for candidate := filepath.Clean(path); ; candidate = filepath.Dir(candidate) {
		if installationShape(candidate) {
			if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
				candidate = resolved
			}
			return candidate
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return ""
		}
	}
}

func installationShape(root string) bool {
	conf, confErr := os.Stat(filepath.Join(root, "metasystem.conf"))
	scripts, scriptsErr := os.Stat(filepath.Join(root, "scripts", "agents"))
	return confErr == nil && !conf.IsDir() && scriptsErr == nil && scripts.IsDir()
}

// RelativeRoot returns the repository-relative directory owned by kind.
// Readers of historical Git trees use this form because an absolute runtime
// location has no meaning inside a commit.
func RelativeRoot(kind Kind) (string, error) {
	return relativeRoot(kind)
}

func relativeRoot(kind Kind) (string, error) {
	switch kind {
	case Registers, Receipts:
		return "memory", nil
	case Records:
		return "records", nil
	case Goals:
		return "plans/goals", nil
	case OpenWork:
		return "plans", nil
	case Steward:
		return "artifacts/agents/steward", nil
	case Evidence:
		return "", nil
	default:
		return "", fmt.Errorf("state root: unknown kind %q", kind)
	}
}

func installationRoot() (string, error) {
	executable, err := executablePath()
	if err != nil {
		return "", fmt.Errorf("state root: locate executable: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}
	root, err := filepath.Abs(filepath.Dir(filepath.Dir(executable)))
	if err != nil {
		return "", fmt.Errorf("state root: locate installation: %w", err)
	}
	if _, confErr := os.Stat(filepath.Join(root, "metasystem.conf")); confErr != nil {
		if info, scriptsErr := os.Stat(filepath.Join(root, "scripts", "agents")); scriptsErr != nil || !info.IsDir() {
			return "", fmt.Errorf("state root: executable %q is not installed at <installation>/bin/metasystem", executable)
		}
	}
	return root, nil
}

func templateMode(installationRoot string) bool {
	if filepath.Base(installationRoot) != "metasystem" {
		return false
	}
	info, err := os.Stat(filepath.Join(filepath.Dir(installationRoot), "development", "metasystem-design.md"))
	return err == nil && !info.IsDir()
}
