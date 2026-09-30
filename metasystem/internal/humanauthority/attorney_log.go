package humanauthority

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AttorneyLogPath is the local, append-only record of what a general power
// of attorney answered and what a hard limit refused under it.
func AttorneyLogPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "authority", "power-of-attorney.log")
}

// AppendAttorneyLog appends one line and makes it durable before returning.
func AppendAttorneyLog(root, line string) error {
	path := AttorneyLogPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(line + "\n"); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// RecordAttorneyRefusal logs that an act a general power of attorney
// answered was refused by one of its hard limits, so the log never reads it
// as admitted. A proof no grant answered records nothing.
func RecordAttorneyRefusal(root string, proof Proof, act, reason string, now time.Time) error {
	if proof.Helm == nil || proof.Helm.Grant == "" {
		return nil
	}
	checkout, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(checkout); resolveErr == nil {
		checkout = resolved
	}
	return AppendAttorneyLog(checkout, fmt.Sprintf("%s refused grant=%s by=%s act=%q reason=%q", now.UTC().Format(time.RFC3339), proof.Helm.Grant, proof.Helm.By, act, reason))
}
