package lease

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
)

// Reclaim deletes a directory that is provably an abandoned metasystem
// checkout: it carries a lease record naming a holder, and that exact
// identity (pid at its recorded start) is dead. The proof and the deletion
// are one operation so no caller can reorder or skip the guards, and the
// path itself must be shaped like a checkout — absolute, resolved, and deep
// enough that a derivation bug (an empty variable, a root, a home
// directory) is refused before anything is touched.
func Reclaim(target string) error {
	if target == "" || !filepath.IsAbs(target) {
		return fmt.Errorf("reclaim refused: target must be an absolute path")
	}
	cleaned := filepath.Clean(target)
	if cleaned != strings.TrimRight(target, "/") && cleaned != target {
		return fmt.Errorf("reclaim refused: target is not a resolved path: %s", target)
	}
	// A checkout lives at least three components deep (/Users/name/checkout);
	// anything shallower is a derivation bug, not a provisioning target.
	if len(strings.Split(strings.Trim(cleaned, "/"), "/")) < 3 {
		return fmt.Errorf("reclaim refused: target is too close to the filesystem root: %s", cleaned)
	}
	if home, err := os.UserHomeDir(); err == nil && cleaned == filepath.Clean(home) {
		return fmt.Errorf("reclaim refused: target is the home directory")
	}

	leasePath := filepath.Join(cleaned, "artifacts", "agents", "mains", "worktree-lease.json")
	data, err := os.ReadFile(leasePath)
	if err != nil {
		return fmt.Errorf("reclaim refused: target has no readable lease record; foreign content is never replaced silently: %s", cleaned)
	}
	var record struct {
		Pid          *int64 `json:"pid"`
		PidStartedAt *int64 `json:"pidStartedAt"`
	}
	if json.Unmarshal(data, &record) != nil || record.Pid == nil || record.PidStartedAt == nil ||
		*record.Pid < 1 || *record.PidStartedAt < 0 {
		return fmt.Errorf("reclaim refused: residue lease is malformed; uninspectable is alive")
	}
	if census.Alive(*record.Pid, *record.PidStartedAt) {
		return fmt.Errorf("reclaim refused: target lease holder pid %d is LIVE; not replacing a held target", *record.Pid)
	}
	if err := os.RemoveAll(cleaned); err != nil {
		return fmt.Errorf("reclaim: %w", err)
	}
	return nil
}
