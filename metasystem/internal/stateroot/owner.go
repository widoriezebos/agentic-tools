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
	return OwnerForInstallation(installation, path)
}

// OwnerForInstallation applies the ownership rule for an explicitly located
// installation. Callers that inspect another checkout use this entry point so
// ownership is not accidentally derived from the running binary.
func OwnerForInstallation(installation, path string) (Ownership, string, error) {
	absoluteInstallation, err := filepath.Abs(installation)
	if err != nil {
		return OwnerOutside, "", fmt.Errorf("path owner: resolve installation: %w", err)
	}
	installation = filepath.Clean(absoluteInstallation)
	if resolved, resolveErr := filepath.EvalSymlinks(installation); resolveErr == nil {
		installation = resolved
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
	if resolved, resolveErr := filepath.EvalSymlinks(appRoot); resolveErr == nil {
		appRoot = resolved
	}
	absolute := path
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(appRoot, absolute)
	} else {
		absolute = canonicalEntryPath(absolute)
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
		// the shipped inventory of instruction-bearing files and trees
		// adoption creates. Completeness against adoption's full install
		// set is tracked by goal adoption-inventory-from-install-set.
		if shippedInventoryPath(slashed) {
			return OwnerMetasystem, mode, nil
		}
		return OwnerApp, mode, nil
	}
	return OwnerApp, mode, nil
}

// canonicalEntryPath resolves symlinks in the nearest existing ancestor while
// preserving the final entry itself, so ownership never follows a path entry's
// referent.
func canonicalEntryPath(entry string) string {
	ancestor := filepath.Dir(entry)
	suffix := filepath.Base(entry)
	for {
		if resolved, err := filepath.EvalSymlinks(ancestor); err == nil {
			return filepath.Join(resolved, suffix)
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return filepath.Clean(entry)
		}
		suffix = filepath.Join(filepath.Base(ancestor), suffix)
		ancestor = parent
	}
}

// shippedInventoryPath names the instruction-bearing files and the trees
// adoption creates, which are generic material in an unvendored layout.
// Goal adoption-inventory-from-install-set tracks completeness against
// adoption's full install set.
func shippedInventoryPath(slashed string) bool {
	for _, root := range []string{"cmd/", "internal/", "scripts/", "skills/", "optional-skills/", "docs/design/", "docs/examples/", "memory/", "plans/", "records/", ".github/"} {
		if strings.HasPrefix(slashed, root) {
			return true
		}
	}
	switch slashed {
	case "go.mod", "go.sum", "wow.md", "AGENTS.md", "CLAUDE.md",
		"metasystem.conf", "docs/orchestration.md", "docs/collaboration.md",
		"docs/project-rules.md", "docs/project-adaptation.md", "docs/metasystem-reconciliation.md",
		"docs/working-modes.md", "docs/working-with-agents.md":
		return true
	}
	return false
}
