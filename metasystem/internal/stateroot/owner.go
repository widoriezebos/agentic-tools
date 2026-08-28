package stateroot

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Ownership is the oracle's four-way answer for a repository path.
type Ownership string

const (
	// OwnerMetasystem marks bytes the vendored tree owns: replaced
	// wholesale by an upgrade, never app content.
	OwnerMetasystem Ownership = "metasystem-generic"
	// OwnerApp marks bytes the application owns: untouchable by any
	// metasystem upgrade.
	OwnerApp Ownership = "app-owned"
	// OwnerRuntime marks machine state (artifacts, built binaries)
	// owned by neither documentation law nor upgrade law.
	OwnerRuntime Ownership = "runtime"
	// OwnerOutside refuses paths that do not live in the repository.
	OwnerOutside Ownership = "outside"
)

// Owner classifies a path against the ratified ownership rule: the
// vendored installation prefix is metasystem-generic, artifacts/ and
// bin/ under either owner are runtime, everything else in the
// repository is app-owned, and anything not contained in the
// repository is refused. Symlinks are judged by their entry path,
// never their referent. The repo mode rides along so callers needing
// the self-hosting distinction get both answers from one mouth.
func Owner(path string) (Ownership, string, error) {
	installation, err := installationRoot()
	if err != nil {
		return OwnerOutside, "", err
	}
	mode := "adopted"
	if templateMode(installation) {
		mode = "template"
	}
	appRoot := installation
	if mode == "adopted" {
		if appRoot, err = repositoryTop(installation); err != nil {
			return OwnerOutside, mode, err
		}
	} else {
		// The template's own repository contains the installation one
		// level down; ownership is judged against the repository. A
		// repository that cannot be identified refuses rather than
		// answering against the wrong boundary.
		top, topErr := repositoryTop(installation)
		if topErr != nil {
			return OwnerOutside, mode, fmt.Errorf("path owner: repository top unreadable: %w", topErr)
		}
		appRoot = top
	}
	absolute := path
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(appRoot, absolute)
	}
	absolute = filepath.Clean(absolute)
	relative, err := filepath.Rel(appRoot, absolute)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return OwnerOutside, mode, fmt.Errorf("path owner: %q is not contained in repository %q", path, appRoot)
	}
	slashed := filepath.ToSlash(relative)
	vendored, err := filepath.Rel(appRoot, installation)
	if err != nil {
		return OwnerOutside, mode, err
	}
	vendoredSlashed := filepath.ToSlash(vendored)
	inVendored := vendoredSlashed != "." &&
		(slashed == vendoredSlashed || strings.HasPrefix(slashed, vendoredSlashed+"/"))
	inner := slashed
	if inVendored && slashed != vendoredSlashed {
		inner = strings.TrimPrefix(slashed, vendoredSlashed+"/")
	}
	if inner == "artifacts" || strings.HasPrefix(inner, "artifacts/") ||
		inner == "bin" || strings.HasPrefix(inner, "bin/") {
		return OwnerRuntime, mode, nil
	}
	if inVendored {
		return OwnerMetasystem, mode, nil
	}
	if vendoredSlashed == "." {
		// The installation IS the repository root (the unvendored
		// adopted layout): no prefix can decide, so ownership follows
		// the shipped inventory — the same source of truth the
		// adoption tracer proves against.
		if shippedInventoryPath(slashed) {
			return OwnerMetasystem, mode, nil
		}
		return OwnerApp, mode, nil
	}
	return OwnerApp, mode, nil
}

// shippedInventoryPath names the trees and files adoption installs —
// generic material in an unvendored layout. Growing it is a reviewed
// change, in lockstep with adopt.sh's install set.
func shippedInventoryPath(slashed string) bool {
	for _, root := range []string{"cmd/", "internal/", "scripts/", "skills/", "optional-skills/", "docs/design/", "docs/examples/", ".github/"} {
		if strings.HasPrefix(slashed, root) {
			return true
		}
	}
	switch slashed {
	case "go.mod", "go.sum", "wow.md", "AGENTS.md", "CLAUDE.md",
		"metasystem.conf", "docs/orchestration.md", "docs/collaboration.md",
		"docs/project-adaptation.md", "docs/metasystem-reconciliation.md",
		"docs/working-modes.md", "docs/working-with-agents.md":
		return true
	}
	return false
}
