package lane

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// CheckoutRoot is a git toplevel: the lane's checkout, or a candidate's
// detached worktree.
type CheckoutRoot string

// InstallRoot is a MetaSystem installation, the folder that holds
// metasystem.conf: its checkout when the layout is flat, the checkout's
// metasystem folder when it is nested.
type InstallRoot string

// Layout is the lane's one layout (r6 U1, design r10 K-a): which root holds
// what, resolved once by landing set and recorded with the lane, never
// guessed again by a reader.
//
//   - Checkout holds landing-batches and the ensure lock;
//   - Install holds the landing agent's lease and its announcements, the
//     configuration, the ledger, proof runs, runs,
//     supervision, engine pins, enrollment and bin/metasystem;
//   - Execution(detached) is where a candidate's tests run;
//   - the host directory holds the lane record, pause, unset journal,
//     keeper and proving flock.
type Layout struct {
	Checkout CheckoutRoot
	Install  InstallRoot
	// rel is Install relative to Checkout: "" flat, nestedInstall nested.
	rel string
}

// nestedInstall is the installation folder of a nested checkout.
const nestedInstall = "metasystem"

const confName = "metasystem.conf"

// NewLayout resolves the layout of the landing checkout at path, the only
// place strings become lane roots. path must be the absolute top folder of a
// git checkout, and exactly one of path and path/metasystem must hold
// metasystem.conf; any other shape is refused, LANDING_LANE_REGISTER_INVALID.
func NewLayout(path string) (Layout, error) {
	path = strings.TrimSpace(path)
	invalid := func(message, fix string) (Layout, error) {
		return Layout{}, &Refusal{Code: CodeRegisterInvalid, Message: message + "; nothing was registered", Fix: fix}
	}
	if !filepath.IsAbs(path) {
		return invalid(fmt.Sprintf("the landing lane must be an absolute path, got %q", path),
			"name the landing checkout by its full path: metasystem landing set /path/to/landing-checkout")
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return invalid(fmt.Sprintf("%s is not an existing directory", path),
			"create or restore the landing checkout first, then run metasystem landing set again")
	}
	root := resolved(path)
	top, err := (gittree.Workspace{Dir: root}).TopLevel()
	if err != nil || resolved(top) != root {
		return invalid(fmt.Sprintf("%s is not the top folder of a git checkout", path),
			"name the landing checkout's top folder (git -C PATH rev-parse --show-toplevel prints it): metasystem landing set PATH")
	}
	flat, nested := isFile(filepath.Join(root, confName)), isFile(filepath.Join(root, nestedInstall, confName))
	switch {
	case flat && !nested:
		return Layout{Checkout: CheckoutRoot(root), Install: InstallRoot(root)}, nil
	case nested && !flat:
		return Layout{Checkout: CheckoutRoot(root), Install: InstallRoot(filepath.Join(root, nestedInstall)), rel: nestedInstall}, nil
	case flat && nested:
		return invalid(fmt.Sprintf("%s holds two MetaSystem installations (%s and %s), so which one lands is not known", path, confName, filepath.Join(nestedInstall, confName)),
			"keep one metasystem.conf in the landing checkout, then run metasystem landing set again")
	}
	return invalid(fmt.Sprintf("%s holds no MetaSystem installation (no %s there or in %s)", path, confName, nestedInstall),
		"clone the repository that holds the installation, then run metasystem landing set again")
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// Execution is the installation of a candidate's detached worktree: the
// same place relative to its checkout as the lane's.
func (l Layout) Execution(detached CheckoutRoot) InstallRoot {
	return InstallRoot(filepath.Join(string(detached), l.rel))
}

// Layout is the recorded layout: the strings landing set wrote, checked for
// shape and never probed on disk again.
func (r Record) Layout() (Layout, error) {
	root, install := filepath.Clean(r.Root), filepath.Clean(r.Install)
	if !filepath.IsAbs(root) || r.Install == "" {
		return Layout{}, fmt.Errorf("the landing lane record names checkout %q and installation %q", r.Root, r.Install)
	}
	switch install {
	case root:
		return Layout{Checkout: CheckoutRoot(root), Install: InstallRoot(root)}, nil
	case filepath.Join(root, nestedInstall):
		return Layout{Checkout: CheckoutRoot(root), Install: InstallRoot(install), rel: nestedInstall}, nil
	}
	return Layout{}, fmt.Errorf("the landing lane record's installation %s is neither its checkout %s nor its %s folder", r.Install, r.Root, nestedInstall)
}
