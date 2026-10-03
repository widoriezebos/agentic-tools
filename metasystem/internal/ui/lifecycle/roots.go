package lifecycle

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// Roots names the three directories a verb works from: the Git checkout a human
// runs in, the installation that serves it and keeps this slice's lifecycle
// state beneath its artifacts/, and the state root that holds the project's
// state. The lock's identity, and so "one server per workspace", is per
// installation.
type Roots struct {
	Checkout     string
	Installation stateroot.Installation
	StateRoot    stateroot.State
}

// RootsMismatchError refuses a checkout and an installation that do not belong
// together. Nothing has been created when it is returned.
type RootsMismatchError struct{ Checkout, Installation string }

func (e *RootsMismatchError) Error() string {
	return fmt.Sprintf("the installation at %s does not serve the checkout at %s", e.Installation, e.Checkout)
}

// ResolveRoots canonicalizes both paths, requires the installation's Git top to
// be the checkout, and derives the state root with stateroot.RootForInstallation:
// the installation itself in the self-hosted layout, the application's
// repository for an adopted installation. An empty repo means the installation's
// Git top, so every verb works from any directory. It is the one place the three
// roots are derived, so a launcher and its child cannot disagree.
func ResolveRoots(repo, installation string) (Roots, error) {
	return ResolveRootsWith(stateroot.RepositoryTop, stateroot.RootForInstallation, repo, installation)
}

// ResolveRootsWith is ResolveRoots with the Git top and state-root readers a
// caller already resolved its repository with.
func ResolveRootsWith(repositoryTop func(string) (string, error), rootForInstallation func(stateroot.Installation) (stateroot.State, error), repo, installation string) (Roots, error) {
	canonicalInstallation, err := canonicalRoot(installation)
	if err != nil {
		return Roots{}, fmt.Errorf("cannot resolve the installation at %s: %w", installation, err)
	}
	// The canonical path is admitted as the installation only when it holds
	// metasystem.conf, so a state directory cannot be served as the installation.
	installationRoot, err := stateroot.ParseInstallation(canonicalInstallation)
	if err != nil {
		return Roots{}, fmt.Errorf("cannot resolve the installation at %s: %w", installation, err)
	}
	top, err := repositoryTop(installationRoot.Path())
	if err != nil {
		return Roots{}, fmt.Errorf("cannot resolve the checkout of the installation at %s: %w", installationRoot, err)
	}
	checkout, err := canonicalRoot(top)
	if err != nil {
		return Roots{}, fmt.Errorf("cannot resolve the checkout at %s: %w", top, err)
	}
	if strings.TrimSpace(repo) != "" {
		given, err := canonicalRoot(repo)
		if err != nil {
			return Roots{}, fmt.Errorf("cannot resolve the checkout at %s: %w", repo, err)
		}
		// A path outside every Git checkout is refused with the same line as one
		// that names another checkout: neither is served by this installation.
		if givenTop, topErr := repositoryTop(given); topErr == nil {
			if given, err = canonicalRoot(givenTop); err != nil {
				return Roots{}, fmt.Errorf("cannot resolve the checkout at %s: %w", givenTop, err)
			}
		}
		if given != checkout {
			return Roots{}, &RootsMismatchError{Checkout: given, Installation: installationRoot.Path()}
		}
	}
	resolved, err := rootForInstallation(installationRoot)
	if err != nil {
		return Roots{}, err
	}
	canonicalState, err := canonicalRoot(resolved.Path())
	if err != nil {
		return Roots{}, fmt.Errorf("cannot resolve the state root at %s: %w", resolved, err)
	}
	stateRoot, err := stateroot.ParseState(canonicalState)
	if err != nil {
		return Roots{}, fmt.Errorf("cannot resolve the state root at %s: %w", canonicalState, err)
	}
	return Roots{Checkout: checkout, Installation: installationRoot, StateRoot: stateRoot}, nil
}

// canonicalRoot makes a path absolute and follows symbolic links, so that two
// spellings of one directory compare equal.
func canonicalRoot(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		return resolved, nil
	}
	return filepath.Clean(absolute), nil
}
