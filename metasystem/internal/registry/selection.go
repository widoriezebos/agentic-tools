package registry

import (
	"fmt"
	"os"
	"path/filepath"
)

const supervisionRegistryHomeEnv = "METASYSTEM_SUPERVISION_REGISTRY_HOME"

// DefaultPath returns the machine-wide registry selected for this process.
// Fixtures may redirect the complete registry home; production uses the
// current user's home directory.
func DefaultPath() (string, error) {
	if override := os.Getenv(supervisionRegistryHomeEnv); override != "" {
		if !filepath.IsAbs(override) {
			return override, fmt.Errorf("%s must name an absolute run-scoped home", supervisionRegistryHomeEnv)
		}
		return filepath.Join(override, ".metasystem", "armed-checkouts.jsonl"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".metasystem", "armed-checkouts.jsonl"), nil
	}
	return filepath.Join(home, ".metasystem", "armed-checkouts.jsonl"), nil
}

// OwnerCheckoutPath returns the checkout selected for an owner by reduction.
// The first structurally valid owner row selects the path; a later row for a
// different checkout is sequence-illegal and cannot replace it.
func OwnerCheckoutPath(path, ownerTag string) (string, bool, error) {
	frames, err := ReadFrames(path)
	if err != nil {
		return "", false, err
	}
	reduction, err := Reduce(frames)
	if err != nil {
		return "", false, err
	}
	checkout, found := reduction.OwnerCheckoutPath(ownerTag)
	return checkout, found, nil
}
