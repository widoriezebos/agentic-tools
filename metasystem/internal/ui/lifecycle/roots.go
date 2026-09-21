package lifecycle

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// Roots names the three directories a verb works from: the Git checkout a human
// runs in, the installation that serves it, and the state root this slice keeps
// its lifecycle state beneath. The lock's identity, and so "one server per
// workspace", is per state root.
type Roots struct{ Checkout, Installation, StateRoot string }

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
	installationRoot, err := canonicalRoot(installation)
	if err != nil {
		return Roots{}, fmt.Errorf("cannot resolve the installation at %s: %w", installation, err)
	}
	top, err := stateroot.RepositoryTop(installationRoot)
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
		if givenTop, topErr := stateroot.RepositoryTop(given); topErr == nil {
			if given, err = canonicalRoot(givenTop); err != nil {
				return Roots{}, fmt.Errorf("cannot resolve the checkout at %s: %w", givenTop, err)
			}
		}
		if given != checkout {
			return Roots{}, &RootsMismatchError{Checkout: given, Installation: installationRoot}
		}
	}
	stateRoot, err := stateroot.RootForInstallation(installationRoot)
	if err != nil {
		return Roots{}, err
	}
	stateRoot, err = canonicalRoot(stateRoot)
	if err != nil {
		return Roots{}, fmt.Errorf("cannot resolve the state root at %s: %w", stateRoot, err)
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
