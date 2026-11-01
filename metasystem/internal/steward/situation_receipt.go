package steward

// A read receipt says this seat's main session read its page: one line per
// read, appended and made durable before the read returns.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// RecordSituationRead appends the session, the time and the page's first
// thing to the steward's read receipts.
func RecordSituationRead(root, session string, at time.Time, first FirstThing) error {
	line, err := json.Marshal(map[string]any{"session": session, "at": at.UTC(), "first": map[string]string{"kind": first.Kind, "subject": first.Subject}})
	if err == nil {
		err = os.MkdirAll(runnerDir(root), 0o755)
	}
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(runnerDir(root), "situation-reads.jsonl"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	if _, err = file.Write(append(line, '\n')); err == nil {
		err = file.Sync()
	}
	return errors.Join(err, file.Close())
}
