package steward

// The steward's identity record lives inside the repository —
// nothing about recognition depends on host paths (D121 third
// addendum). `steward arm` mints it; the classifier authenticates
// it by ownership, mode, and content; a reinstall bumps the
// generation so stale authorizations stop authorizing.

import "path/filepath"

// RepoIdentityPath is the record's one location.
func RepoIdentityPath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "identity.json")
}
