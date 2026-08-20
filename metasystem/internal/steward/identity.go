package steward

// The steward's installation identity: a record minted by the
// human-run install, authenticated by ownership, mode, and content.
// It exists to make the scheduled tick a NAMED caller instead of an
// unrecognized process — accident-proofing at the repository's trust
// level (D118): a stray cron job does not match a pinned record; a
// same-user adversary is out of scope repo-wide.

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"
)

// InstallIdentity is the minted record's content.
type InstallIdentity struct {
	// RepoIdentity pins which repository this installation serves.
	RepoIdentity string `json:"repoIdentity"`
	// Generation increments at every reinstall; intents bind to it,
	// so records from a superseded installation stop authorizing.
	Generation int `json:"generation"`
	// InstallPath is where the installed steward bytes live.
	InstallPath string `json:"installPath"`
	MintedAt    string `json:"mintedAt"`
}

// MintIdentity writes the record with owner-only permissions,
// replacing any previous generation.
func MintIdentity(path string, id InstallIdentity) error {
	data, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// VerifyIdentity authenticates the record for a repository: it must
// exist, be owned by the calling user, carry owner-only permissions,
// and name the expected repository. Every failure is named — the
// classifier refuses on any of them rather than guessing.
func VerifyIdentity(path, wantRepoIdentity string) (InstallIdentity, error) {
	var id InstallIdentity
	info, err := os.Stat(path)
	if err != nil {
		return id, fmt.Errorf("steward identity absent: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return id, fmt.Errorf("steward identity %s is not owner-only (mode %o)", path, info.Mode().Perm())
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return id, fmt.Errorf("steward identity %s is owned by uid %d, not the calling user", path, st.Uid)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return id, fmt.Errorf("steward identity unreadable: %w", err)
	}
	if err := json.Unmarshal(data, &id); err != nil {
		return id, fmt.Errorf("steward identity malformed: %w", err)
	}
	if id.RepoIdentity != wantRepoIdentity {
		return id, fmt.Errorf("steward identity serves repository %q, not %q", id.RepoIdentity, wantRepoIdentity)
	}
	if id.Generation < 1 {
		return id, fmt.Errorf("steward identity carries no valid generation")
	}
	return id, nil
}
