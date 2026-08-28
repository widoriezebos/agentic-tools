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
	appRoot := installationRoot
	if !templateMode(installationRoot) {
		appRoot, err = repositoryTop(installationRoot)
		if err != nil {
			return "", err
		}
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
