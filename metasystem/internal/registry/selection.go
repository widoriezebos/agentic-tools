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

// OwnerCheckoutPath joins an owner tag to exactly one recorded checkout.
// The caller decides what to do when the tag has no row yet, which is valid
// during the bounded interval before an owner writes its first launch record.
func OwnerCheckoutPath(path, ownerTag string) (string, bool, error) {
	frames, err := ReadFrames(path)
	if err != nil {
		return "", false, err
	}
	if _, err := Reduce(frames); err != nil {
		return "", false, err
	}
	var checkout string
	for _, frame := range frames {
		if frame.Record == nil {
			continue
		}
		record, err := ParseRecord(frame.Record)
		if err != nil {
			return "", false, fmt.Errorf("line %d: %w", frame.Line, err)
		}
		if !claimEvent(record.Event) || record.OwnerTag != ownerTag {
			continue
		}
		if checkout == "" {
			checkout = record.CheckoutPath
			continue
		}
		if checkout != record.CheckoutPath {
			return "", false, fmt.Errorf("owner tag %q has registry rows for checkout %q and checkout %q", ownerTag, checkout, record.CheckoutPath)
		}
	}
	return checkout, checkout != "", nil
}

func claimEvent(event string) bool {
	switch event {
	case EventArming, EventArmed, EventRelaunched, EventLaunched, EventExited, EventReaped, EventSwept:
		return true
	default:
		return false
	}
}
