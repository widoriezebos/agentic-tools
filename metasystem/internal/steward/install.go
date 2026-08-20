package steward

// Where a repository's steward installation lives: outside every
// checkout, keyed by the repository's toplevel path, so a wedged or
// half-deleted checkout can still tick. The environment override
// exists for fixtures, which cannot stage the real home directory.

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
)

// InstallDir is the operator-owned directory holding the installed
// steward bytes and the installation identity for one repository.
func InstallDir(repoTop string) (string, error) {
	if base := os.Getenv("METASYSTEM_STEWARD_HOME"); base != "" {
		return filepath.Join(base, repoHash(repoTop)), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	var base string
	if runtime.GOOS == "darwin" {
		base = filepath.Join(home, "Library", "Application Support", "metasystem", "steward")
	} else {
		base = filepath.Join(home, ".local", "share", "metasystem", "steward")
	}
	return filepath.Join(base, repoHash(repoTop)), nil
}

// IdentityPath is the installation identity record's location.
func IdentityPath(repoTop string) (string, error) {
	dir, err := InstallDir(repoTop)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "identity.json"), nil
}

func repoHash(repoTop string) string {
	sum := sha256.Sum256([]byte(repoTop))
	return hex.EncodeToString(sum[:8])
}
