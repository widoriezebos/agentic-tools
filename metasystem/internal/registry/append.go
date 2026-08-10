package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// LockedAppend is the REG-4 mutation: acquire the registry lock, frame
// one record (REG-1), release. Every writer in the system uses this,
// so the lock discipline lives in exactly one place. The record is a
// pre-validated map; the caller (the supervise ledger, the janitor)
// owns its schema, this owns durability and framing.
//
// Path is the registry file (~/.metasystem/armed-checkouts.jsonl by
// REG-1); self identifies the lock holder for death-only takeover.
func LockedAppend(path string, self lock.Identity, payload []byte, wait, poll time.Duration, probe lock.Probe) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("registry directory: %w", err)
	}
	held, err := lock.Acquire(path+".lock.d", self, lock.Options{Wait: wait, Poll: poll, Probe: probe})
	if err != nil {
		return fmt.Errorf("registry lock: %w", err)
	}
	defer held.Release()
	if err := AppendFrame(path, payload); err != nil {
		return fmt.Errorf("registry append: %w", err)
	}
	return nil
}
